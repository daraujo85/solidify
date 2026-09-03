// Divergence Engine (SAI-067).
//
// Determinístico: compara Peer A vs Peer B scores, applicable
// mismatch, severity mismatch, finding overlap.
package peer

import (
	"errors"
	"sort"
	"strings"
)

// DivergenceMap resultado da comparação.
type DivergenceMap struct {
	RunID              string               `json:"run_id"`
	ScoreDelta         float64              `json:"score_delta"`
	ApplicableMismatch []ApplicableMismatch `json:"applicable_mismatch,omitempty"`
	SeverityMismatch   []SeverityMismatch   `json:"severity_mismatch,omitempty"`
	FindingOverlap     FindingOverlap       `json:"finding_overlap"`
	TotalDivergences   int                  `json:"total_divergences"`
	ArbiterRequired    bool                 `json:"arbiter_required"`
}

// ApplicableMismatch princípio com applicable diferente.
type ApplicableMismatch struct {
	Principle string `json:"principle"`
	PeerA     bool   `json:"peer_a"`
	PeerB     bool   `json:"peer_b"`
}

// SeverityMismatch finding com severity diferente.
type SeverityMismatch struct {
	FindingID string `json:"finding_id"`
	PeerA     string `json:"peer_a"`
	PeerB     string `json:"peer_b"`
}

// FindingOverlap estatísticas.
type FindingOverlap struct {
	OnlyA    []string `json:"only_a"`
	OnlyB    []string `json:"only_b"`
	Both     []string `json:"both"`
	OnlyACnt int      `json:"only_a_count"`
	OnlyBCnt int      `json:"only_a_count_b"` // legacy compat
	BothCnt  int      `json:"both_count"`
}

// PeerScores scores por peer.
type PeerScores struct {
	RunID      string             `json:"run_id"`
	Source     string             `json:"source"` // "A"|"B"
	Scores     map[string]float64 `json:"scores"` // SRP/OCP/LSP/ISP/DIP
	Applicable map[string]bool    `json:"applicable"`
	Findings   []FindingLite      `json:"findings,omitempty"`
}

// FindingLite finding leve (symbol+severity+desc).
type FindingLite struct {
	ID       string `json:"id"`
	Symbol   string `json:"symbol"`
	Severity string `json:"severity"` // low|medium|high|critical
	Note     string `json:"note,omitempty"`
}

// Compute divergence map.
func Compute(a, b PeerScores) (*DivergenceMap, error) {
	if a.RunID == "" || b.RunID == "" {
		return nil, errors.New("divergence: RunID vazio")
	}
	if a.RunID != b.RunID {
		return nil, errors.New("divergence: RunIDs diferentes")
	}
	dm := &DivergenceMap{RunID: a.RunID}
	// Score deltas (overall).
	dm.ScoreDelta = scoreDelta(a.Scores, b.Scores)
	// Applicable mismatches.
	dm.ApplicableMismatch = detectApplicableMismatch(a.Applicable, b.Applicable)
	// Severity mismatches.
	dm.SeverityMismatch = detectSeverityMismatch(a.Findings, b.Findings)
	// Finding overlap.
	dm.FindingOverlap = computeOverlap(a.Findings, b.Findings)
	// Total.
	dm.TotalDivergences = len(dm.ApplicableMismatch) + len(dm.SeverityMismatch) +
		dm.FindingOverlap.OnlyACnt + dm.FindingOverlap.OnlyBCnt
	dm.ArbiterRequired = dm.TotalDivergences > 0
	return dm, nil
}

// scoreDelta diferença absoluta entre médias.
func scoreDelta(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	sumA, sumB := 0.0, 0.0
	for _, v := range a {
		sumA += v
	}
	for _, v := range b {
		sumB += v
	}
	avgA := sumA / float64(len(a))
	avgB := sumB / float64(len(b))
	diff := avgA - avgB
	if diff < 0 {
		diff = -diff
	}
	return diff
}

// detectApplicableMismatch diff em applicable.
func detectApplicableMismatch(a, b map[string]bool) []ApplicableMismatch {
	var out []ApplicableMismatch
	keys := make(map[string]bool)
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	for k := range keys {
		va, oka := a[k]
		vb, okb := b[k]
		if !oka || !okb {
			continue
		}
		if va != vb {
			out = append(out, ApplicableMismatch{Principle: k, PeerA: va, PeerB: vb})
		}
	}
	sortMismatches(out)
	return out
}

func sortMismatches(m []ApplicableMismatch) {
	for i := 1; i < len(m); i++ {
		v := m[i]
		j := i - 1
		for j >= 0 && m[j].Principle > v.Principle {
			m[j+1] = m[j]
			j--
		}
		m[j+1] = v
	}
}

// detectSeverityMismatch diff em severity por finding.
func detectSeverityMismatch(a, b []FindingLite) []SeverityMismatch {
	var out []SeverityMismatch
	bMap := indexFindings(b)
	for _, fa := range a {
		fb, ok := bMap[fa.ID]
		if !ok {
			continue
		}
		if fa.Severity != fb.Severity {
			out = append(out, SeverityMismatch{
				FindingID: fa.ID,
				PeerA:     fa.Severity,
				PeerB:     fb.Severity,
			})
		}
	}
	return out
}

// indexFindings by ID.
func indexFindings(findings []FindingLite) map[string]FindingLite {
	out := make(map[string]FindingLite)
	for _, f := range findings {
		out[f.ID] = f
	}
	return out
}

// computeOverlap finding overlap.
func computeOverlap(a, b []FindingLite) FindingOverlap {
	ov := FindingOverlap{}
	bMap := indexFindings(b)
	aMap := indexFindings(a)
	for id, fa := range aMap {
		if _, ok := bMap[id]; ok {
			ov.Both = append(ov.Both, id)
			// check both have same severity?
			_ = fa
		} else {
			ov.OnlyA = append(ov.OnlyA, id)
		}
	}
	for id := range bMap {
		if _, ok := aMap[id]; !ok {
			ov.OnlyB = append(ov.OnlyB, id)
		}
	}
	ov.OnlyACnt = len(ov.OnlyA)
	ov.OnlyBCnt = len(ov.OnlyB)
	ov.BothCnt = len(ov.Both)
	sort.Strings(ov.OnlyA)
	sort.Strings(ov.OnlyB)
	sort.Strings(ov.Both)
	return ov
}

// ArbiterRequired helper.
func (d *DivergenceMap) RequiresArbiter() bool {
	if d == nil {
		return false
	}
	return d.ArbiterRequired
}

// HasMismatch helper.
func (d *DivergenceMap) HasMismatch() bool {
	if d == nil {
		return false
	}
	return d.TotalDivergences > 0
}

// RenderDivergence textual.
func RenderDivergence(d *DivergenceMap) string {
	if d == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("DivergenceMap[")
	sb.WriteString(d.RunID)
	sb.WriteString("]\n")
	sb.WriteString("  score_delta: ")
	sb.WriteString(ftoa(d.ScoreDelta))
	sb.WriteString("\n")
	if len(d.ApplicableMismatch) > 0 {
		sb.WriteString("  applicable_mismatch:\n")
		for _, m := range d.ApplicableMismatch {
			sb.WriteString("    " + m.Principle + ": A=" + btoa(m.PeerA) + " B=" + btoa(m.PeerB) + "\n")
		}
	}
	if len(d.SeverityMismatch) > 0 {
		sb.WriteString("  severity_mismatch:\n")
		for _, m := range d.SeverityMismatch {
			sb.WriteString("    " + m.FindingID + ": A=" + m.PeerA + " B=" + m.PeerB + "\n")
		}
	}
	if d.FindingOverlap.OnlyACnt > 0 || d.FindingOverlap.OnlyBCnt > 0 {
		sb.WriteString("  overlap: onlyA=" + itoa(d.FindingOverlap.OnlyACnt))
		sb.WriteString(" onlyB=" + itoa(d.FindingOverlap.OnlyBCnt))
		sb.WriteString(" both=" + itoa(d.FindingOverlap.BothCnt) + "\n")
	}
	if d.ArbiterRequired {
		sb.WriteString("  → needs arbitration\n")
	}
	return sb.String()
}

func btoa(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := make([]byte, 0, 10)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		out = append(out, byte('0'+n%10))
		n /= 10
	}
	if neg {
		out = append(out, '-')
	}
	// reverse
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

func ftoa(f float64) string {
	// 2 casas decimais.
	intPart := int(f)
	frac := int((f - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	out := itoa(intPart) + "." + padLeft2(itoa(frac))
	return out
}

func padLeft2(s string) string {
	if len(s) >= 2 {
		return s
	}
	return "0" + s
}
