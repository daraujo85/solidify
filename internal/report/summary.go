// Executive summary input model (SAI-079).
//
// Summary derivado de fatos (scores, gate, risk, arbiter)
// NUNCA altera scores. Se usar IA, é etapa separada —
// input são fatos puros, output é texto livre.
package report

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// SummaryInput fatos p/ gerar summary.
type SummaryInput struct {
	RunID           string
	Profile         string
	GateStatus      string
	QualityScore    float64
	Grade           string
	RiskLevel       string
	RiskFactors     []string
	Confidence      float64
	ConfLevel       string
	Divergences     int
	ArbiterOK       bool
	BreakingChanges int
	Components      []string
	SOLIDDeltas     map[string]float64 // princípio → delta
	Limitations     []string
}

// SummaryResult output.
type SummaryResult struct {
	RunID       string    `json:"run_id"`
	Headline    string    `json:"headline"`
	Body        string    `json:"body"`
	Bullets     []string  `json:"bullets"`
	GeneratedAt time.Time `json:"generated_at"`
	Sources     []string  `json:"sources,omitempty"`
}

// GenerateSummary monta summary determinístico a partir
// de SummaryInput. Sem IA — só template + facts.
func GenerateSummary(in SummaryInput) (*SummaryResult, error) {
	if in.RunID == "" {
		return nil, errors.New("summary: RunID vazio")
	}
	if in.Profile == "" {
		return nil, errors.New("summary: Profile vazio")
	}
	r := &SummaryResult{
		RunID:       in.RunID,
		GeneratedAt: time.Now().UTC(),
	}
	r.Headline = buildHeadline(in)
	r.Bullets = buildBullets(in)
	r.Body = buildBody(in)
	r.Sources = buildSources(in)
	return r, nil
}

func buildHeadline(in SummaryInput) string {
	grade := in.Grade
	if grade == "" {
		grade = "N/A"
	}
	return "Release " + in.Profile + " — grade " + grade +
		", gate " + in.GateStatus + ", risk " + in.RiskLevel
}

func buildBullets(in SummaryInput) []string {
	out := []string{}
	// gate bullet.
	out = append(out, "Quality gate: "+in.GateStatus+
		" (score "+ftoa2(in.QualityScore)+")")
	// risk bullet.
	if len(in.RiskFactors) > 0 {
		out = append(out, "Risk: "+in.RiskLevel+
			" — "+joinSorted(in.RiskFactors, ", "))
	} else {
		out = append(out, "Risk: "+in.RiskLevel+" (sem fatores)")
	}
	// confidence bullet.
	out = append(out, "Confidence: "+in.ConfLevel+
		" ("+ftoa2(in.Confidence*100)+"%)")
	// divergences.
	if in.Divergences > 0 {
		arb := "resolvidas"
		if !in.ArbiterOK {
			arb = "pendentes"
		}
		out = append(out, "Divergências: "+
			itoa2(in.Divergences)+" ("+arb+")")
	} else {
		out = append(out, "Divergências: nenhuma")
	}
	// breaking changes.
	if in.BreakingChanges > 0 {
		out = append(out, "Breaking changes: "+
			itoa2(in.BreakingChanges))
	}
	// SOLID deltas (apenas negativos — destaque).
	if negs := negativeSOLID(in.SOLIDDeltas); len(negs) > 0 {
		out = append(out, "SOLID regressões: "+joinSorted(negs, ", "))
	}
	// limitations.
	if len(in.Limitations) > 0 {
		out = append(out, "Limitações: "+
			itoa2(len(in.Limitations)))
	}
	// components.
	if len(in.Components) > 0 {
		out = append(out, "Componentes afetados: "+
			joinSorted(in.Components, ", "))
	}
	return out
}

func buildBody(in SummaryInput) string {
	var sb strings.Builder
	sb.WriteString("Perfil ")
	sb.WriteString(in.Profile)
	sb.WriteString("; gate ")
	sb.WriteString(in.GateStatus)
	sb.WriteString(". ")
	if in.BreakingChanges > 0 {
		sb.WriteString(itoa2(in.BreakingChanges))
		sb.WriteString(" breaking change(s). ")
	}
	if in.Divergences > 0 {
		sb.WriteString(itoa2(in.Divergences))
		sb.WriteString(" divergência(s) — arbiter ")
		if in.ArbiterOK {
			sb.WriteString("resolveu")
		} else {
			sb.WriteString("pendente")
		}
		sb.WriteString(". ")
	}
	sb.WriteString("Risco ")
	sb.WriteString(in.RiskLevel)
	sb.WriteString("; confiança ")
	sb.WriteString(in.ConfLevel)
	sb.WriteString(".")
	return sb.String()
}

func buildSources(in SummaryInput) []string {
	out := []string{"gate", "scores", "risk", "confidence", "ai_review"}
	if in.Divergences > 0 {
		out = append(out, "divergence_map")
	}
	if in.BreakingChanges > 0 {
		out = append(out, "release_notes")
	}
	return out
}

func negativeSOLID(deltas map[string]float64) []string {
	out := []string{}
	for k, v := range deltas {
		if v < 0 {
			out = append(out, k+ftoa2(v))
		}
	}
	sort.Strings(out)
	return out
}

func joinSorted(items []string, sep string) string {
	cp := append([]string{}, items...)
	sort.Strings(cp)
	return strings.Join(cp, sep)
}

// IAExtendedSummary marca que summary foi extendido por IA.
// NÃO altera scores — só gera texto livre adicional.
type IAExtendedSummary struct {
	RunID       string    `json:"run_id"`
	BaseSummary string    `json:"base_summary"`
	IABullets   []string  `json:"ia_bullets"`
	Provider    string    `json:"provider"`
	ModelID     string    `json:"model_id"`
	GeneratedAt time.Time `json:"generated_at"`
}

// NewIAExtendedSummary helper.
func NewIAExtendedSummary(runID, base, provider, model string, bullets []string) *IAExtendedSummary {
	return &IAExtendedSummary{
		RunID:       runID,
		BaseSummary: base,
		IABullets:   bullets,
		Provider:    provider,
		ModelID:     model,
		GeneratedAt: time.Now().UTC(),
	}
}

// helpers.
func ftoa2(f float64) string {
	intPart := int(f)
	frac := int((f - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	out := itoa2(intPart) + "." + padLeft2(itoa2(frac), 2)
	return out
}

func itoa2(n int) string {
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

func padLeft2(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}
