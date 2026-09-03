// Robustness comparator (SAI-094).
//
// Compara 2 runs (original vs swap) e extrai:
// - score delta (global)
// - letter deltas (S/O/L/I/D)
// - finding overlap (intersecção + só-A + só-B)
// - gate stability (mesmo status?)
package peer

import (
	"errors"
	"sort"
	"strings"

	"github.com/diegoaraujo/solidify/internal/score"
)

// RobustFindingLite simplificado p/ comparador.
type RobustFindingLite struct {
	ID       string
	Symbol   string
	Severity string
	Note     string
}

// RunSnapshot entrada do comparador.
type RunSnapshot struct {
	RunID      string
	Score      float64
	Grade      string
	GateStatus string
	Pillars    []string
	SOLID      score.Snapshot
	Findings   []RobustFindingLite
}

// RobustnessResult saída.
type RobustnessResult struct {
	OriginalRunID string             `json:"original_run_id"`
	SwappedRunID  string             `json:"swapped_run_id"`
	QualityDelta  float64            `json:"quality_delta"`
	LetterDeltas  map[string]float64 `json:"letter_deltas"`
	OnlyOriginal  []string           `json:"only_original,omitempty"`
	OnlySwapped   []string           `json:"only_swapped,omitempty"`
	Both          []string           `json:"both,omitempty"`
	Overlap       float64            `json:"overlap"`
	GateStable    bool               `json:"gate_stable"`
	Status        string             `json:"status"` // stable|mostly-stable|unstable
	Notes         []string           `json:"notes,omitempty"`
}

// ErrEmptySnapshot sem run.
var ErrEmptySnapshot = errors.New("robustness: snapshot vazio")

// Compare dois runs.
func Compare(orig, swapped *RunSnapshot) (*RobustnessResult, error) {
	if orig == nil || swapped == nil {
		return nil, ErrEmptySnapshot
	}
	r := &RobustnessResult{
		OriginalRunID: orig.RunID,
		SwappedRunID:  swapped.RunID,
		LetterDeltas:  map[string]float64{},
	}
	// quality delta.
	r.QualityDelta = swapped.Score - orig.Score
	// letter deltas: para cada princípio, diferença.
	r.LetterDeltas = letterDeltas(orig.SOLID, swapped.SOLID)
	// findings overlap.
	onlyA, onlyB, both := findingsOverlap(orig.Findings, swapped.Findings)
	r.OnlyOriginal = onlyA
	r.OnlySwapped = onlyB
	r.Both = both
	// overlap ratio (jaccard).
	total := len(onlyA) + len(onlyB) + len(both)
	if total > 0 {
		r.Overlap = float64(len(both)) / float64(total)
	}
	// gate stability.
	r.GateStable = orig.GateStatus == swapped.GateStatus
	// status binning.
	r.Status = computeRobustnessStatus(r)
	return r, nil
}

func letterDeltas(a, b score.Snapshot) map[string]float64 {
	out := map[string]float64{}
	for _, p := range score.Principles {
		va, _ := score.LetterByPrinciple(a.Letters, p)
		vb, _ := score.LetterByPrinciple(b.Letters, p)
		out[p] = vb.Score - va.Score
	}
	return out
}

// FindingsOverlapPublic wrapper público p/ benchmarks.
func FindingsOverlapPublic(a, b []RobustFindingLite) (onlyA, onlyB, both []string) {
	return findingsOverlap(a, b)
}

func findingsOverlap(a, b []RobustFindingLite) (onlyA, onlyB, both []string) {
	setA := map[string]bool{}
	setB := map[string]bool{}
	for _, f := range a {
		setA[f.ID] = true
	}
	for _, f := range b {
		setB[f.ID] = true
	}
	// both.
	for id := range setA {
		if setB[id] {
			both = append(both, id)
		} else {
			onlyA = append(onlyA, id)
		}
	}
	for id := range setB {
		if !setA[id] {
			onlyB = append(onlyB, id)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	sort.Strings(both)
	return
}

// computeRobustnessStatus: stable|mostly-stable|unstable.
//
// stable:     gate stable + |quality_delta| < 2
// mostly-stable: gate stable + |quality_delta| < 5
// unstable:   gate unstable OR |quality_delta| ≥ 5
func computeRobustnessStatus(r *RobustnessResult) string {
	absDelta := r.QualityDelta
	if absDelta < 0 {
		absDelta = -absDelta
	}
	if !r.GateStable {
		return "unstable"
	}
	if absDelta >= 5 {
		return "unstable"
	}
	if absDelta < 2 {
		return "stable"
	}
	return "mostly-stable"
}

// IsStable helper.
func (r *RobustnessResult) IsStable() bool {
	if r == nil {
		return false
	}
	return r.Status == "stable"
}

// RenderResult textual.
func (r *RobustnessResult) RenderResult() string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("Robustness[")
	sb.WriteString(r.OriginalRunID)
	sb.WriteString(" vs ")
	sb.WriteString(r.SwappedRunID)
	sb.WriteString("]\n  status=")
	sb.WriteString(r.Status)
	sb.WriteString(" gate_stable=")
	if r.GateStable {
		sb.WriteString("yes")
	} else {
		sb.WriteString("no")
	}
	sb.WriteString(" quality_delta=")
	sb.WriteString(ftoaR(r.QualityDelta))
	sb.WriteString(" overlap=")
	sb.WriteString(ftoaR(r.Overlap))
	sb.WriteString("\n")
	if len(r.LetterDeltas) > 0 {
		sb.WriteString("  letters: ")
		keys := make([]string, 0, len(r.LetterDeltas))
		for k := range r.LetterDeltas {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(ftoaR(r.LetterDeltas[k]))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func ftoaR(f float64) string {
	intPart := int(f)
	frac := int((f - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	out := itoaR(intPart) + "." + padLeftR(itoaR(frac), 2)
	return out
}

func itoaR(n int) string {
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

func padLeftR(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}
