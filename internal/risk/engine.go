// Release Risk engine (SAI-073).
//
// Migration/env/breaking/security/blast-radius/review disagreement.
package risk

import (
	"errors"
	"sort"
	"strings"
)

// RiskKind tipos de risco.
const (
	KindMigration = "migration"
	KindEnv       = "env"
	KindBreaking  = "breaking"
	KindSecurity  = "security"
	KindBlast     = "blast_radius"
	KindDisagree  = "review_disagreement"
)

// Severity levels.
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// SeverityRank ordem numérica.
var SeverityRank = map[string]int{
	SeverityLow:      1,
	SeverityMedium:   2,
	SeverityHigh:     3,
	SeverityCritical: 4,
}

// RiskFactor um fator de risco.
type RiskFactor struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Evidence    string `json:"evidence,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
}

// RiskInput input.
type RiskInput struct {
	RunID   string       `json:"run_id"`
	Factors []RiskFactor `json:"factors"`
}

// RiskScore saída.
type RiskScore struct {
	RunID      string         `json:"run_id"`
	Factors    []RiskFactor   `json:"factors"`
	RawScore   int            `json:"raw_score"`
	Normalized float64        `json:"normalized"` // 0..100
	Level      string         `json:"level"`      // low|medium|high|critical
	TopFactors []RiskFactor   `json:"top_factors,omitempty"`
	Breakdown  map[string]int `json:"breakdown"` // por kind
	Notes      string         `json:"notes,omitempty"`
}

// MaxRawScore cap superior (saturado).
const MaxRawScore = 100

// Weights por kind.
var Weights = map[string]int{
	KindMigration: 10,
	KindEnv:       8,
	KindBreaking:  15,
	KindSecurity:  20,
	KindBlast:     12,
	KindDisagree:  10,
}

// Compute agrega fatores em score 0..100.
// Cada fator: weight * severity_rank.
func Compute(in RiskInput) (*RiskScore, error) {
	if in.RunID == "" {
		return nil, errors.New("risk: RunID vazio")
	}
	rs := &RiskScore{
		RunID:     in.RunID,
		Factors:   in.Factors,
		Breakdown: make(map[string]int),
	}
	raw := 0
	for _, f := range in.Factors {
		w, ok := Weights[f.Kind]
		if !ok {
			w = 5 // unknown kind = leve
		}
		sev, ok := SeverityRank[f.Severity]
		if !ok {
			sev = 1
		}
		contrib := w * sev
		raw += contrib
		rs.Breakdown[f.Kind] += contrib
	}
	if raw > MaxRawScore {
		raw = MaxRawScore
	}
	rs.RawScore = raw
	rs.Normalized = float64(raw) * 100.0 / float64(MaxRawScore)
	rs.Level = computeLevel(rs.Normalized)
	rs.TopFactors = topFactors(in.Factors, 5)
	return rs, nil
}

// computeLevel severity based on normalized.
func computeLevel(n float64) string {
	switch {
	case n >= 75:
		return SeverityCritical
	case n >= 50:
		return SeverityHigh
	case n >= 25:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// topFactors ordena por contribuição descendente.
func topFactors(factors []RiskFactor, n int) []RiskFactor {
	type scored struct {
		f   RiskFactor
		raw int
	}
	var all []scored
	for _, f := range factors {
		w := Weights[f.Kind]
		if w == 0 {
			w = 5
		}
		sev := SeverityRank[f.Severity]
		if sev == 0 {
			sev = 1
		}
		all = append(all, scored{f: f, raw: w * sev})
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].raw > all[j].raw
	})
	if len(all) > n {
		all = all[:n]
	}
	out := make([]RiskFactor, len(all))
	for i, s := range all {
		out[i] = s.f
	}
	return out
}

// ValidateKind check.
func ValidateKind(k string) bool {
	_, ok := Weights[k]
	return ok
}

// RenderScore textual.
func RenderScore(r *RiskScore) string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("RiskScore[")
	sb.WriteString(r.RunID)
	sb.WriteString("] raw=")
	sb.WriteString(itoa3(r.RawScore))
	sb.WriteString(" normalized=")
	sb.WriteString(ftoa3(r.Normalized))
	sb.WriteString(" level=")
	sb.WriteString(r.Level)
	sb.WriteString("\n")
	for k, v := range r.Breakdown {
		sb.WriteString("  ")
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(itoa3(v))
		sb.WriteString("\n")
	}
	if len(r.TopFactors) > 0 {
		sb.WriteString("  top:\n")
		for _, f := range r.TopFactors {
			sb.WriteString("    - [")
			sb.WriteString(f.Kind)
			sb.WriteString("/")
			sb.WriteString(f.Severity)
			sb.WriteString("] ")
			sb.WriteString(f.Description)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// IsShippable helper: low/medium → ok; high/critical → não.
func (r *RiskScore) IsShippable() bool {
	if r == nil {
		return false
	}
	return r.Level == SeverityLow || r.Level == SeverityMedium
}

// BreakdownByKind helper.
func (r *RiskScore) BreakdownByKind(kind string) int {
	if r == nil {
		return 0
	}
	return r.Breakdown[kind]
}

// helpers.
func itoa3(n int) string {
	if n == 0 {
		return "0"
	}
	out := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	if neg {
		out = append([]byte{'-'}, out...)
	}
	return string(out)
}

func ftoa3(f float64) string {
	intPart := int(f)
	frac := int((f - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	out := itoa3(intPart) + "." + padLeft(itoa3(frac), 2)
	return out
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}
