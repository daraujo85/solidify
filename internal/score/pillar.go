// Pillar score engine (SAI-072).
//
// Agrega SOLID + Security + Performance + Maintainability em
// score global com pesos.
package score

import (
	"errors"
	"strings"
)

// Pillar enum.
const (
	PillarSOLID           = "solid"
	PillarSecurity        = "security"
	PillarPerformance     = "performance"
	PillarMaintainability = "maintainability"
)

// Pillars canônicos (ordem default).
var Pillars = []string{PillarSOLID, PillarSecurity, PillarPerformance, PillarMaintainability}

// DefaultWeights default weighting.
var DefaultWeights = map[string]float64{
	PillarSOLID:           0.40,
	PillarSecurity:        0.25,
	PillarPerformance:     0.20,
	PillarMaintainability: 0.15,
}

// PillarScore um pilar.
type PillarScore struct {
	Pillar     string  `json:"pillar"`
	Score      float64 `json:"score"` // 0..100
	Weight     float64 `json:"weight"`
	Applicable bool    `json:"applicable"`
	Notes      string  `json:"notes,omitempty"`
}

// PillarInput input.
type PillarInput struct {
	RunID   string             `json:"run_id"`
	Pillars []PillarScore      `json:"pillars"`
	Weights map[string]float64 `json:"weights,omitempty"`
}

// PillarResult saída.
type PillarResult struct {
	RunID       string        `json:"run_id"`
	Pillars     []PillarScore `json:"pillars"`
	GlobalScore float64       `json:"global_score"`
	WeightedSum float64       `json:"weighted_sum"`
	WeightSum   float64       `json:"weight_sum"`
	Missing     []string      `json:"missing,omitempty"`
	Notes       string        `json:"notes,omitempty"`
}

// ComputeGlobalScore agrega pilares com pesos.
// Pillars não-aplicáveis excluídas; pesos renormalizados.
// Se nenhuma pilar aplicável → 0.
func ComputeGlobalScore(in PillarInput) (*PillarResult, error) {
	if in.RunID == "" {
		return nil, errors.New("pillar: RunID vazio")
	}
	weights := in.Weights
	if weights == nil {
		weights = DefaultWeights
	}
	res := &PillarResult{
		RunID:   in.RunID,
		Pillars: in.Pillars,
	}
	wSum := 0.0
	weightedSum := 0.0
	missing := make(map[string]bool)
	for _, p := range in.Pillars {
		if !p.Applicable {
			continue
		}
		if p.Score < 0 || p.Score > 100 {
			return nil, errors.New("pillar: score fora range: " + p.Pillar)
		}
		w, ok := weights[p.Pillar]
		if !ok {
			missing[p.Pillar] = true
			continue
		}
		weightedSum += p.Score * w
		wSum += w
	}
	for k := range missing {
		res.Missing = append(res.Missing, k)
	}
	res.WeightedSum = weightedSum
	res.WeightSum = wSum
	if wSum > 0 {
		res.GlobalScore = weightedSum / wSum
	}
	return res, nil
}

// ValidatePillarName check.
func ValidatePillarName(p string) bool {
	for _, k := range Pillars {
		if k == p {
			return true
		}
	}
	return false
}

// NormalizeWeights renormaliza pra somar 1.
// Se soma = 0 → retorna map vazio.
func NormalizeWeights(w map[string]float64) map[string]float64 {
	if len(w) == 0 {
		return map[string]float64{}
	}
	sum := 0.0
	for _, v := range w {
		sum += v
	}
	if sum == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(w))
	for k, v := range w {
		out[k] = v / sum
	}
	return out
}

// GetWeight helper com fallback DefaultWeights.
func GetWeight(pillar string, weights map[string]float64) float64 {
	if w, ok := weights[pillar]; ok {
		return w
	}
	if w, ok := DefaultWeights[pillar]; ok {
		return w
	}
	return 0
}

// RenderResult textual.
func RenderResult(r *PillarResult) string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("PillarResult[")
	sb.WriteString(r.RunID)
	sb.WriteString("] global=")
	sb.WriteString(ftoa1(r.GlobalScore))
	sb.WriteString("\n")
	for _, p := range r.Pillars {
		marker := "✓"
		if !p.Applicable {
			marker = "N/A"
		}
		sb.WriteString("  ")
		sb.WriteString(p.Pillar)
		sb.WriteString(": ")
		sb.WriteString(marker)
		if p.Applicable {
			sb.WriteString(" ")
			sb.WriteString(ftoa1(p.Score))
			sb.WriteString(" (w=")
			sb.WriteString(ftoa1(p.Weight))
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
	if len(r.Missing) > 0 {
		sb.WriteString("  missing weights: ")
		sb.WriteString(strings.Join(r.Missing, ","))
		sb.WriteString("\n")
	}
	return sb.String()
}
