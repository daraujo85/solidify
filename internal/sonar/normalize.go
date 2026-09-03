// Package sonar — result normalizer.
//
// SAI-035: normaliza issues/severities/measures/QG do Web API
// pra formato interno consistente. Mapeia severities Sonar
// (BLOCKER/CRITICAL/MAJOR/MINOR/INFO) pra escala interna (high/medium/low/info).
// Marca issues como project-wide (histórico) ou diff-relevant (mudanças no range).
package sonar

import (
	"sort"
	"strings"
)

// Severity normalizada.
type Severity string

const (
	SevCritical Severity = "critical"
	SevHigh     Severity = "high"
	SevMedium   Severity = "medium"
	SevLow      Severity = "low"
	SevInfo     Severity = "info"
)

// Issue normalizado.
type Issue struct {
	Key         string   `json:"key"`
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity"`
	OriginalSev string   `json:"original_severity"`
	Type        string   `json:"type"` // BUG, VULNERABILITY, CODE_SMELL, SECURITY_HOTSPOT
	Message     string   `json:"message"`
	FilePath    string   `json:"file_path"`
	Line        int      `json:"line"`
	Status      string   `json:"status"` // OPEN, CONFIRMED, REOPENED, RESOLVED, CLOSED
	Scope       Scope    `json:"scope"`
}

// Scope classifica issue.
type Scope string

const (
	ScopeProjectWide Scope = "project-wide" // histórico, não causado pelo diff
	ScopeDiff        Scope = "diff"         // mudou no range
	ScopeUnknown     Scope = "unknown"
)

// Measure normalizada.
type MeasureN struct {
	Key   string  `json:"key"`   // coverage, bugs, vulnerabilities, code_smells
	Value float64 `json:"value"` // 0-100 (coverage) ou count (bugs)
	Raw   string  `json:"raw"`
}

// QualityGate normalizado.
type QualityGate struct {
	Status     QGStatus       `json:"status"`
	Conditions []QGConditionN `json:"conditions"`
}

// QGStatus — "OK" | "WARN" | "ERROR".
type QGStatus string

const (
	QGOK    QGStatus = "OK"
	QGWarn  QGStatus = "WARN"
	QGError QGStatus = "ERROR"
)

// QGConditionN — condição do QG normalizada.
type QGConditionN struct {
	Metric     string  `json:"metric"`
	Comparator string  `json:"comparator"`
	Threshold  float64 `json:"threshold"`
	Actual     float64 `json:"actual"`
	Status     string  `json:"status"`
}

// Report é o resultado normalizado.
type Report struct {
	HostURL     string       `json:"host_url"`
	ProjectKey  string       `json:"project_key"`
	Measures    []MeasureN   `json:"measures"`
	Issues      []Issue      `json:"issues"`
	QualityGate *QualityGate `json:"quality_gate,omitempty"`
	Counts      Counts       `json:"counts"`
}

// Counts agrega issues por severity e scope.
type Counts struct {
	Total      int              `json:"total"`
	BySeverity map[Severity]int `json:"by_severity"`
	ByScope    map[Scope]int    `json:"by_scope"`
	ByType     map[string]int   `json:"by_type"`
}

// NormalizeSeverity converte severity Sonar → interna.
func NormalizeSeverity(sonar string) Severity {
	switch strings.ToUpper(strings.TrimSpace(sonar)) {
	case "BLOCKER":
		return SevCritical
	case "CRITICAL":
		return SevHigh
	case "MAJOR":
		return SevMedium
	case "MINOR":
		return SevLow
	case "INFO":
		return SevInfo
	default:
		return SevInfo
	}
}

// NormalizeIssues converte IssueRef list → Issue normalizado + diff filter.
// diffPaths (path absoluto) marca issues como ScopeDiff se arquivo bate;
// projectKey prefixo em "component" é removido pra ficar só o file path.
func NormalizeIssues(refs []IssueRef, projectKey string, diffPaths []string) []Issue {
	out := make([]Issue, 0, len(refs))
	diffSet := make(map[string]bool, len(diffPaths))
	for _, p := range diffPaths {
		diffSet[normalizePath(p)] = true
	}
	for _, r := range refs {
		filePath := stripProjectPrefix(r.FilePath, projectKey)
		iss := Issue{
			Key:         r.Key,
			Rule:        r.Rule,
			Severity:    NormalizeSeverity(r.Severity),
			OriginalSev: r.Severity,
			Type:        r.Type,
			Message:     r.Message,
			FilePath:    filePath,
			Line:        r.Line,
			Status:      r.Status,
			Scope:       ScopeUnknown,
		}
		if filePath == "" {
			iss.Scope = ScopeProjectWide
		} else if isProjectWideStatus(r.Status) {
			iss.Scope = ScopeProjectWide
		} else if diffSet[normalizePath(filePath)] {
			iss.Scope = ScopeDiff
		} else {
			iss.Scope = ScopeUnknown
		}
		out = append(out, iss)
	}
	// Sort: critical > high > medium > low > info; estável por key.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return severityRank(out[i].Severity) < severityRank(out[j].Severity)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// severityRank menor = pior (ordenado primeiro).
func severityRank(s Severity) int {
	switch s {
	case SevCritical:
		return 0
	case SevHigh:
		return 1
	case SevMedium:
		return 2
	case SevLow:
		return 3
	case SevInfo:
		return 4
	}
	return 9
}

// isProjectWideStatus issues resolvidos/fechados não são diff-relevant.
func isProjectWideStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "RESOLVED", "CLOSED":
		return true
	}
	return false
}

// stripProjectPrefix remove "projectKey:" do início.
func stripProjectPrefix(component, projectKey string) string {
	if component == "" {
		return ""
	}
	prefix := projectKey + ":"
	if strings.HasPrefix(component, prefix) {
		return component[len(prefix):]
	}
	// fallback: take after last ':'.
	idx := strings.LastIndex(component, ":")
	if idx >= 0 && idx < len(component)-1 {
		return component[idx+1:]
	}
	return component
}

// normalizePath canonicaliza pra match.
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	return p
}

// NormalizeMeasures converte []Measure (string Value) → []MeasureN (float64).
func NormalizeMeasures(in []Measure) []MeasureN {
	out := make([]MeasureN, 0, len(in))
	for _, m := range in {
		val, _ := parseFloat(m.Value)
		out = append(out, MeasureN{
			Key:   m.Metric,
			Value: val,
			Raw:   m.Value,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// FindMeasure devolve measure por key.
func FindMeasure(in []MeasureN, key string) (MeasureN, bool) {
	for _, m := range in {
		if m.Key == key {
			return m, true
		}
	}
	return MeasureN{}, false
}

// NormalizeQualityGate converte QG do Web API.
func NormalizeQualityGate(qg QualityGateStatus) QualityGate {
	out := QualityGate{
		Status:     QGStatus(strings.ToUpper(qg.ProjectStatus.Status)),
		Conditions: make([]QGConditionN, 0, len(qg.ProjectStatus.Conditions)),
	}
	for _, c := range qg.ProjectStatus.Conditions {
		thr, _ := parseFloat(c.Error)
		act, _ := parseFloat(c.Actual)
		out.Conditions = append(out.Conditions, QGConditionN{
			Metric:     c.Metric,
			Comparator: c.Comparator,
			Threshold:  thr,
			Actual:     act,
			Status:     c.Status,
		})
	}
	return out
}

// BuildReport compõe Report a partir de medidas + issues + QG.
func BuildReport(host, project string, measures []MeasureN, issues []Issue, qg *QualityGate) *Report {
	r := &Report{
		HostURL:     host,
		ProjectKey:  project,
		Measures:    measures,
		Issues:      issues,
		QualityGate: qg,
	}
	r.Counts = countIssues(issues)
	return r
}

func countIssues(issues []Issue) Counts {
	c := Counts{
		BySeverity: make(map[Severity]int),
		ByScope:    make(map[Scope]int),
		ByType:     make(map[string]int),
	}
	for _, i := range issues {
		c.Total++
		c.BySeverity[i.Severity]++
		c.ByScope[i.Scope]++
		c.ByType[i.Type]++
	}
	return c
}

// DiffIssues devolve só issues ScopeDiff.
func (r *Report) DiffIssues() []Issue {
	out := []Issue{}
	for _, i := range r.Issues {
		if i.Scope == ScopeDiff {
			out = append(out, i)
		}
	}
	return out
}

// BlockingIssues devolve issues que bloqueiam (critical+high + abertas).
func (r *Report) BlockingIssues() []Issue {
	out := []Issue{}
	for _, i := range r.Issues {
		if i.Severity == SevCritical || i.Severity == SevHigh {
			if i.Status == "" || strings.EqualFold(i.Status, "OPEN") || strings.EqualFold(i.Status, "REOPENED") {
				out = append(out, i)
			}
		}
	}
	return out
}

// parseFloat — substituto simples de strconv.ParseFloat pra evitar import.
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	var sign float64 = 1
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = s[1:]
	}
	var whole, frac float64
	var fracDiv float64 = 1
	inFrac := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			d := float64(r - '0')
			if inFrac {
				frac = frac*10 + d
				fracDiv *= 10
			} else {
				whole = whole*10 + d
			}
		case r == '.':
			inFrac = true
		default:
			return 0, errBadFloat
		}
	}
	return sign * (whole + frac/fracDiv), nil
}

var errBadFloat = &parseErr{"bad float"}

type parseErr struct{ msg string }

func (e *parseErr) Error() string { return e.msg }
