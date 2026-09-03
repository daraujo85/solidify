// Package semgrep — Semgrep CE adapter.
//
// SAI-039: detecta config/rules e normaliza achados.
// Semgrep CLI retorna JSON com results[] (check_id, path, start, end,
// extra.severity). Suporta custom rules em .semgrep/.
package semgrep

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Severity classificação.
type Severity string

const (
	SevError    Severity = "error"
	SevWarning  Severity = "warning"
	SevInfo     Severity = "info"
	SevCritical Severity = "critical"
	SevHigh     Severity = "high"
	SevMedium   Severity = "medium"
	SevLow      Severity = "low"
)

// Finding normalizado.
type Finding struct {
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	FilePath string   `json:"file_path"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	EndLine  int      `json:"end_line"`
	Message  string   `json:"message"`
	CWE      []string `json:"cwe,omitempty"`
	OWASP    []string `json:"owasp,omitempty"`
	Category string   `json:"category"`
	Source   string   `json:"source"` // "semgrep"
}

// Report resultado.
type Report struct {
	Tool     string    `json:"tool"`
	Findings []Finding `json:"findings"`
	Counts   Counts    `json:"counts"`
	Errors   []string  `json:"errors,omitempty"`
	Rules    []string  `json:"rules_used,omitempty"`
}

// Counts agregado.
type Counts struct {
	Total      int              `json:"total"`
	BySeverity map[Severity]int `json:"by_severity"`
	Files      int              `json:"files_with_findings"`
}

// HasErrorSeverity devolve true se tem ≥1 error ou critical/high.
func (r *Report) HasErrorSeverity() bool {
	c := r.Counts.BySeverity
	return c[SevError] > 0 || c[SevCritical] > 0 || c[SevHigh] > 0
}

// AddFinding adiciona + atualiza counts.
func (r *Report) AddFinding(f Finding) {
	r.Findings = append(r.Findings, f)
	if r.Counts.BySeverity == nil {
		r.Counts.BySeverity = make(map[Severity]int)
	}
	r.Counts.BySeverity[f.Severity]++
	r.Counts.Total++
}

// SortFindings por severity, file, line.
func (r *Report) SortFindings() {
	sort.SliceStable(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return severityRank(r.Findings[i].Severity) < severityRank(r.Findings[j].Severity)
		}
		if r.Findings[i].FilePath != r.Findings[j].FilePath {
			return r.Findings[i].FilePath < r.Findings[j].FilePath
		}
		return r.Findings[i].Line < r.Findings[j].Line
	})
}

func severityRank(s Severity) int {
	switch s {
	case SevError, SevCritical:
		return 0
	case SevWarning, SevHigh:
		return 1
	case SevMedium:
		return 2
	case SevInfo, SevLow:
		return 3
	}
	return 9
}

// NormalizeSeverity Semgrep → interno.
// Semgrep usa INFO/WARNING/ERROR por padrão; regras custom podem
// definir qualquer string em metadata.severity.
func NormalizeSeverity(s string) Severity {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "ERROR":
		return SevError
	case "WARNING":
		return SevWarning
	case "INFO":
		return SevInfo
	case "CRITICAL":
		return SevCritical
	case "HIGH":
		return SevHigh
	case "MEDIUM":
		return SevMedium
	case "LOW":
		return SevLow
	}
	return SevInfo
}

// DetectConfig verifica presença de .semgrep/ ou .semgrep.yml.
// Devolve paths de rules encontradas.
func DetectConfig(root string) ([]string, error) {
	if root == "" {
		return nil, fmt.Errorf("semgrep: root vazio")
	}
	fi, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("semgrep: stat: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("semgrep: %s não é dir", root)
	}

	found := []string{}
	// .semgrep.yml/.yaml
	for _, name := range []string{".semgrep.yml", ".semgrep.yaml", "semgrep.yml"} {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	// .semgrep/ dir
	dir := filepath.Join(root, ".semgrep")
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
				found = append(found, filepath.Join(dir, e.Name()))
			}
		}
	}
	return found, nil
}

// semgrepResult schema subset.
type semgrepResult struct {
	CheckID string `json:"check_id"`
	Path    string `json:"path"`
	Start   struct {
		Line   int `json:"line"`
		Column int `json:"col"`
	} `json:"start"`
	End struct {
		Line   int `json:"line"`
		Column int `json:"col"`
	} `json:"end"`
	Extra struct {
		Message  string                 `json:"message"`
		Severity string                 `json:"severity"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"extra"`
}

// semgrepOutput schema raiz.
type semgrepOutput struct {
	Results []semgrepResult `json:"results"`
	Errors  []struct {
		Message string `json:"message"`
		Level   string `json:"level"`
	} `json:"errors"`
}

// ParseSemgrepJSON parseia output do Semgrep CLI.
func ParseSemgrepJSON(data []byte) (*Report, error) {
	var raw semgrepOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("semgrep: parse: %w", err)
	}
	rep := &Report{Tool: "semgrep"}
	for _, e := range raw.Errors {
		if e.Message != "" {
			rep.Errors = append(rep.Errors, e.Message)
		}
	}
	seenRule := map[string]bool{}
	for _, r := range raw.Results {
		sev := NormalizeSeverity(r.Extra.Severity)
		cwe := extractStringList(r.Extra.Metadata, "cwe")
		owasp := extractStringList(r.Extra.Metadata, "owasp")
		category, _ := r.Extra.Metadata["category"].(string)
		f := Finding{
			RuleID:   r.CheckID,
			Severity: sev,
			FilePath: r.Path,
			Line:     r.Start.Line,
			Column:   r.Start.Column,
			EndLine:  r.End.Line,
			Message:  r.Extra.Message,
			CWE:      cwe,
			OWASP:    owasp,
			Category: category,
			Source:   "semgrep",
		}
		rep.AddFinding(f)
		if !seenRule[r.CheckID] {
			rep.Rules = append(rep.Rules, r.CheckID)
			seenRule[r.CheckID] = true
		}
	}
	files := map[string]bool{}
	for _, f := range rep.Findings {
		files[f.FilePath] = true
	}
	rep.Counts.Files = len(files)
	return rep, nil
}

// extractStringList extrai []string de metadata (pode ser []any ou string).
func extractStringList(md map[string]interface{}, key string) []string {
	v, ok := md[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		out := []string{}
		for _, x := range val {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return []string{val}
	case []string:
		return val
	}
	return nil
}
