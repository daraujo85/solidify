// Confidence engine (SAI-074).
//
// Evidence completeness, peer agreement, independence,
// analyzer completeness, arbiter confidence.
package confidence

import (
	"errors"
	"strings"
)

// Signal enum.
const (
	SignalEvidence     = "evidence_completeness"
	SignalAgreement    = "peer_agreement"
	SignalIndependence = "independence"
	SignalAnalyzer     = "analyzer_completeness"
	SignalArbiter      = "arbiter_confidence"
)

// DefaultWeights pesos default.
var DefaultWeights = map[string]float64{
	SignalEvidence:     0.30,
	SignalAgreement:    0.25,
	SignalIndependence: 0.20,
	SignalAnalyzer:     0.15,
	SignalArbiter:      0.10,
}

// SignalScore um sinal.
type SignalScore struct {
	Signal string  `json:"signal"`
	Score  float64 `json:"score"` // 0..100
	Weight float64 `json:"weight"`
	Notes  string  `json:"notes,omitempty"`
}

// ConfidenceInput input.
type ConfidenceInput struct {
	RunID   string             `json:"run_id"`
	Signals []SignalScore      `json:"signals"`
	Weights map[string]float64 `json:"weights,omitempty"`
}

// ConfidenceResult saída.
type ConfidenceResult struct {
	RunID       string        `json:"run_id"`
	Signals     []SignalScore `json:"signals"`
	Score       float64       `json:"score"` // 0..100
	Level       string        `json:"level"` // low|medium|high
	WeightedSum float64       `json:"weighted_sum"`
	WeightSum   float64       `json:"weight_sum"`
	Missing     []string      `json:"missing,omitempty"`
}

// Compute agrega signals em confidence score.
func Compute(in ConfidenceInput) (*ConfidenceResult, error) {
	if in.RunID == "" {
		return nil, errors.New("confidence: RunID vazio")
	}
	weights := in.Weights
	if weights == nil {
		weights = DefaultWeights
	}
	res := &ConfidenceResult{RunID: in.RunID, Signals: in.Signals}
	wSum := 0.0
	wScore := 0.0
	missing := make(map[string]bool)
	for _, s := range in.Signals {
		if s.Score < 0 || s.Score > 100 {
			return nil, errors.New("confidence: signal fora range: " + s.Signal)
		}
		w, ok := weights[s.Signal]
		if !ok {
			missing[s.Signal] = true
			continue
		}
		wScore += s.Score * w
		wSum += w
	}
	for k := range missing {
		res.Missing = append(res.Missing, k)
	}
	res.WeightedSum = wScore
	res.WeightSum = wSum
	if wSum > 0 {
		res.Score = wScore / wSum
	}
	res.Level = computeLevel(res.Score)
	return res, nil
}

// computeLevel: <50 low, <75 medium, ≥75 high.
func computeLevel(s float64) string {
	switch {
	case s >= 75:
		return "high"
	case s >= 50:
		return "medium"
	default:
		return "low"
	}
}

// IsHighConfidence helper.
func (r *ConfidenceResult) IsHighConfidence() bool {
	if r == nil {
		return false
	}
	return r.Level == "high"
}

// RenderResult textual.
func RenderResult(r *ConfidenceResult) string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("Confidence[")
	sb.WriteString(r.RunID)
	sb.WriteString("] score=")
	sb.WriteString(ftoa4(r.Score))
	sb.WriteString(" level=")
	sb.WriteString(r.Level)
	sb.WriteString("\n")
	for _, s := range r.Signals {
		sb.WriteString("  ")
		sb.WriteString(s.Signal)
		sb.WriteString(": ")
		sb.WriteString(ftoa4(s.Score))
		sb.WriteString(" (w=")
		sb.WriteString(ftoa4(s.Weight))
		sb.WriteString(")")
		if s.Notes != "" {
			sb.WriteString(" — ")
			sb.WriteString(s.Notes)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// DefaultWeightsSum helper p/ testes.
func DefaultWeightsSum() float64 {
	sum := 0.0
	for _, w := range DefaultWeights {
		sum += w
	}
	return sum
}

// helpers locais.
func ftoa4(f float64) string {
	intPart := int(f)
	frac := int((f - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	out := itoa4(intPart) + "." + padLeft4(itoa4(frac), 2)
	return out
}

func itoa4(n int) string {
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

func padLeft4(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}
