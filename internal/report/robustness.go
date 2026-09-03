// Robustness integration (SAI-095).
//
// Adiciona bloco de robustness ao Report quando executado
// role-swap. Status stable/mostly-stable/unstable aparece
// no JSON/dashboard/PDF.
package report

import (
	"errors"

	"github.com/diegoaraujo/solidify/internal/peer"
)

// RobustnessBlock entrada no report.
type RobustnessBlock struct {
	Status        string             `json:"status"` // stable|mostly-stable|unstable|not_run
	QualityDelta  float64            `json:"quality_delta"`
	LetterDeltas  map[string]float64 `json:"letter_deltas,omitempty"`
	Overlap       float64            `json:"overlap"`
	GateStable    bool               `json:"gate_stable"`
	OriginalRunID string             `json:"original_run_id,omitempty"`
	SwappedRunID  string             `json:"swapped_run_id,omitempty"`
	OnlyOriginal  []string           `json:"only_original,omitempty"`
	OnlySwapped   []string           `json:"only_swapped,omitempty"`
	Both          []string           `json:"both,omitempty"`
	Notes         []string           `json:"notes,omitempty"`
}

// ErrNoRobustness resultado ausente.
var ErrNoRobustness = errors.New("report: robustness não executado")

// AttachRobustness adiciona bloco ao report a partir de RunSnapshot.
//
// Se r for nil, retorna ErrNoRobustness — caller decide
// se é skip ou erro fatal.
func AttachRobustness(rep *Report, orig, swapped *peer.RunSnapshot) error {
	if rep == nil {
		return errors.New("report: nil")
	}
	if orig == nil || swapped == nil {
		return ErrNoRobustness
	}
	res, err := peer.Compare(orig, swapped)
	if err != nil {
		return err
	}
	if rep.Extra == nil {
		rep.Extra = map[string]any{}
	}
	rep.Extra["robustness"] = RobustnessBlock{
		Status:        res.Status,
		QualityDelta:  res.QualityDelta,
		LetterDeltas:  res.LetterDeltas,
		Overlap:       res.Overlap,
		GateStable:    res.GateStable,
		OriginalRunID: res.OriginalRunID,
		SwappedRunID:  res.SwappedRunID,
		OnlyOriginal:  res.OnlyOriginal,
		OnlySwapped:   res.OnlySwapped,
		Both:          res.Both,
		Notes:         res.Notes,
	}
	return nil
}

// MarkRobustnessNotRun marca como não executado.
//
// Usado quando pipeline roda sem role-swap — placeholder
// p/ UI exibir seção vazia.
func MarkRobustnessNotRun(rep *Report) {
	if rep == nil {
		return
	}
	if rep.Extra == nil {
		rep.Extra = map[string]any{}
	}
	rep.Extra["robustness"] = RobustnessBlock{
		Status: "not_run",
	}
}

// RobustnessFromReport extrai bloco (ou nil se ausente).
func RobustnessFromReport(rep *Report) *RobustnessBlock {
	if rep == nil || rep.Extra == nil {
		return nil
	}
	rb, ok := rep.Extra["robustness"].(RobustnessBlock)
	if !ok {
		return nil
	}
	return &rb
}

// RenderRobustness textual p/ PDF/dashboard.
func (rb *RobustnessBlock) RenderRobustness() string {
	if rb == nil {
		return "Robustness: <nil>\n"
	}
	out := "Robustness["
	out += rb.OriginalRunID
	out += " vs "
	out += rb.SwappedRunID
	out += "]\n  status="
	out += rb.Status
	out += " quality_delta="
	out += ftoaRob(rb.QualityDelta)
	out += " overlap="
	out += ftoaRob(rb.Overlap)
	out += "\n"
	return out
}

func ftoaRob(f float64) string {
	intPart := int(f)
	frac := int((f - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	out := itoaRob(intPart) + "." + padLeftRob(itoaRob(frac), 2)
	return out
}

func itoaRob(n int) string {
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

func padLeftRob(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}
