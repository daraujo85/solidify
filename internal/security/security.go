// Package security — agregação de score/gates.
//
// SAI-040: consolida findings de múltiplas fontes (sonar/secrets/
// osv/semgrep) em score único e decisão de gate.
package security

import (
	"sort"
)

// Source identifica origem do finding.
type Source string

const (
	SourceSonar   Source = "sonar"
	SourceSecrets Source = "secrets"
	SourceOSV     Source = "osv"
	SourceSemgrep Source = "semgrep"
)

// Severity classifica finding.
type Severity string

const (
	SevCritical Severity = "critical"
	SevHigh     Severity = "high"
	SevMedium   Severity = "medium"
	SevLow      Severity = "low"
	SevInfo     Severity = "info"
	SevError    Severity = "error"
	SevWarning  Severity = "warning"
)

// Finding unificado (qualquer fonte).
type Finding struct {
	Source   Source            `json:"source"`
	Rule     string            `json:"rule"`
	Severity Severity          `json:"severity"`
	FilePath string            `json:"file_path"`
	Line     int               `json:"line"`
	Message  string            `json:"message"`
	Extras   map[string]string `json:"extras,omitempty"`
}

// Threshold configura gate.
type Threshold struct {
	MaxCritical         int  `json:"max_critical"`
	MaxHigh             int  `json:"max_high"`
	MaxMedium           int  `json:"max_medium"`
	MaxLow              int  `json:"max_low"`
	BlockOnSecrets      bool `json:"block_on_secrets"`
	BlockOnCriticalVuln bool `json:"block_on_critical_vuln"`
}

// DefaultThreshold devolve threshold conservador.
func DefaultThreshold() Threshold {
	return Threshold{
		MaxCritical:         0,
		MaxHigh:             0,
		MaxMedium:           5,
		MaxLow:              50,
		BlockOnSecrets:      true,
		BlockOnCriticalVuln: true,
	}
}

// Gate decision.
type Gate struct {
	Allow   bool      `json:"allow"`
	Reason  string    `json:"reason,omitempty"`
	Score   Score     `json:"score"`
	Blocked []Finding `json:"blocked,omitempty"`
}

// Score agregado.
type Score struct {
	Value      int              `json:"value"` // 0-100 (100 = perfect)
	Total      int              `json:"total"`
	BySeverity map[Severity]int `json:"by_severity"`
	BySource   map[Source]int   `json:"by_source"`
}

// CalcScore calcula score 0-100 baseado em findings.
// Cada critical = -25, high = -10, medium = -3, low = -1, info = 0.
// Floor 0.
func CalcScore(findings []Finding) Score {
	s := Score{
		Total:      len(findings),
		BySeverity: make(map[Severity]int),
		BySource:   make(map[Source]int),
	}
	deduction := 0
	for _, f := range findings {
		s.BySeverity[f.Severity]++
		s.BySource[f.Source]++
		switch f.Severity {
		case SevCritical:
			deduction += 25
		case SevHigh, SevError:
			deduction += 10
		case SevMedium, SevWarning:
			deduction += 3
		case SevLow:
			deduction += 1
		}
	}
	s.Value = 100 - deduction
	if s.Value < 0 {
		s.Value = 0
	}
	return s
}

// EvaluateGate decide se findings passam no gate.
func EvaluateGate(findings []Finding, t Threshold) Gate {
	g := Gate{
		Score:   CalcScore(findings),
		Blocked: []Finding{},
	}

	// Source-specific checks (BlockOnSecrets, BlockOnCriticalVuln)
	// rodam ANTES dos thresholds genéricos pra permitir override.
	if t.BlockOnSecrets {
		for _, f := range findings {
			if f.Source == SourceSecrets && (f.Severity == SevCritical || f.Severity == SevHigh) {
				g.Allow = false
				g.Reason = "secret detected"
				g.Blocked = append(g.Blocked, f)
				return g
			}
		}
	}
	if t.BlockOnCriticalVuln {
		for _, f := range findings {
			if f.Source == SourceOSV && f.Severity == SevCritical {
				g.Allow = false
				g.Reason = "critical vulnerability in dependencies"
				g.Blocked = append(g.Blocked, f)
				return g
			}
		}
	}

	counts := g.Score.BySeverity
	if counts[SevCritical] > t.MaxCritical {
		g.Allow = false
		g.Reason = "critical threshold exceeded"
		for _, f := range findings {
			if f.Severity == SevCritical {
				g.Blocked = append(g.Blocked, f)
			}
		}
		return g
	}
	if counts[SevHigh] > t.MaxHigh || counts[SevError] > t.MaxHigh {
		g.Allow = false
		g.Reason = "high threshold exceeded"
		for _, f := range findings {
			if f.Severity == SevHigh || f.Severity == SevError {
				g.Blocked = append(g.Blocked, f)
			}
		}
		return g
	}
	if counts[SevMedium] > t.MaxMedium {
		g.Allow = false
		g.Reason = "medium threshold exceeded"
		for _, f := range findings {
			if f.Severity == SevMedium || f.Severity == SevWarning {
				g.Blocked = append(g.Blocked, f)
			}
		}
		return g
	}
	if counts[SevLow] > t.MaxLow {
		g.Allow = false
		g.Reason = "low threshold exceeded"
		for _, f := range findings {
			if f.Severity == SevLow {
				g.Blocked = append(g.Blocked, f)
			}
		}
		return g
	}
	g.Allow = true
	g.Reason = "all checks passed"
	return g
}

// SortFindings por severity (critical primeiro), source, file.
func SortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
		}
		if findings[i].Source != findings[j].Source {
			return findings[i].Source < findings[j].Source
		}
		if findings[i].FilePath != findings[j].FilePath {
			return findings[i].FilePath < findings[j].FilePath
		}
		return findings[i].Line < findings[j].Line
	})
}

func severityRank(s Severity) int {
	switch s {
	case SevCritical:
		return 0
	case SevHigh, SevError:
		return 1
	case SevMedium, SevWarning:
		return 2
	case SevLow:
		return 3
	}
	return 4
}

// MergeFindings combina múltiplas listas, sem dedup.
func MergeFindings(lists ...[]Finding) []Finding {
	total := 0
	for _, l := range lists {
		total += len(l)
	}
	out := make([]Finding, 0, total)
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}
