package peer

import (
	"strings"
	"testing"
)

// Aceitação: Compute basic OK.
func TestComputeOK(t *testing.T) {
	a := PeerScores{
		RunID: "r1", Source: "A",
		Scores:     map[string]float64{"SRP": 80, "OCP": 90},
		Applicable: map[string]bool{"SRP": true, "OCP": true},
		Findings:   []FindingLite{{ID: "f1", Severity: "high"}},
	}
	b := PeerScores{
		RunID: "r1", Source: "B",
		Scores:     map[string]float64{"SRP": 80, "OCP": 90},
		Applicable: map[string]bool{"SRP": true, "OCP": true},
		Findings:   []FindingLite{{ID: "f1", Severity: "high"}},
	}
	dm, err := Compute(a, b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dm.TotalDivergences != 0 {
		t.Errorf("esperado 0: %d", dm.TotalDivergences)
	}
	if dm.ArbiterRequired {
		t.Errorf("não devia precisar")
	}
}

// Aceitação: Compute nil RunID.
func TestComputeNilRunID(t *testing.T) {
	if _, err := Compute(PeerScores{}, PeerScores{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Compute RunIDs diferentes.
func TestComputeDifferentRunIDs(t *testing.T) {
	a := PeerScores{RunID: "r1"}
	b := PeerScores{RunID: "r2"}
	if _, err := Compute(a, b); err == nil {
		t.Errorf("diff runids")
	}
}

// Aceitação: ScoreDelta.
func TestScoreDelta(t *testing.T) {
	a := PeerScores{
		RunID: "r", Source: "A",
		Scores: map[string]float64{"SRP": 100, "OCP": 80},
	}
	b := PeerScores{
		RunID: "r", Source: "B",
		Scores: map[string]float64{"SRP": 60, "OCP": 60},
	}
	dm, _ := Compute(a, b)
	if dm.ScoreDelta <= 0 {
		t.Errorf("delta > 0: %f", dm.ScoreDelta)
	}
}

// Aceitação: ApplicableMismatch.
func TestApplicableMismatch(t *testing.T) {
	a := PeerScores{
		RunID:      "r",
		Applicable: map[string]bool{"SRP": true, "LSP": false},
	}
	b := PeerScores{
		RunID:      "r",
		Applicable: map[string]bool{"SRP": true, "LSP": true},
	}
	dm, _ := Compute(a, b)
	if len(dm.ApplicableMismatch) != 1 {
		t.Errorf("esperado 1: %d", len(dm.ApplicableMismatch))
	}
	if dm.ApplicableMismatch[0].Principle != "LSP" {
		t.Errorf("LSP")
	}
}

// Aceitação: SeverityMismatch.
func TestSeverityMismatch(t *testing.T) {
	a := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f1", Severity: "high"}},
	}
	b := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f1", Severity: "medium"}},
	}
	dm, _ := Compute(a, b)
	if len(dm.SeverityMismatch) != 1 {
		t.Errorf("esperado 1")
	}
	if dm.SeverityMismatch[0].PeerA != "high" || dm.SeverityMismatch[0].PeerB != "medium" {
		t.Errorf("severities")
	}
}

// Aceitação: SeverityMismatch finding missing não conta.
func TestSeverityMismatchMissingFinding(t *testing.T) {
	a := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f1", Severity: "high"}},
	}
	b := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{},
	}
	dm, _ := Compute(a, b)
	if len(dm.SeverityMismatch) != 0 {
		t.Errorf("missing finding não devia mismatch")
	}
}

// Aceitação: Finding overlap onlyA.
func TestOverlapOnlyA(t *testing.T) {
	a := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f1"}, {ID: "f2"}},
	}
	b := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f2"}},
	}
	dm, _ := Compute(a, b)
	if dm.FindingOverlap.OnlyACnt != 1 || dm.FindingOverlap.OnlyBCnt != 0 || dm.FindingOverlap.BothCnt != 1 {
		t.Errorf("overlap: %+v", dm.FindingOverlap)
	}
}

// Aceitação: Finding overlap onlyB.
func TestOverlapOnlyB(t *testing.T) {
	a := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f1"}},
	}
	b := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f1"}, {ID: "f2"}},
	}
	dm, _ := Compute(a, b)
	if dm.FindingOverlap.OnlyACnt != 0 || dm.FindingOverlap.OnlyBCnt != 1 || dm.FindingOverlap.BothCnt != 1 {
		t.Errorf("overlap: %+v", dm.FindingOverlap)
	}
}

// Aceitação: Finding overlap both.
func TestOverlapBoth(t *testing.T) {
	a := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f1"}, {ID: "f2"}},
	}
	b := PeerScores{
		RunID:    "r",
		Findings: []FindingLite{{ID: "f1"}, {ID: "f2"}},
	}
	dm, _ := Compute(a, b)
	if dm.FindingOverlap.BothCnt != 2 {
		t.Errorf("both: %d", dm.FindingOverlap.BothCnt)
	}
}

// Aceitação: Total divergences aggregation.
func TestTotalDivergences(t *testing.T) {
	a := PeerScores{
		RunID:      "r",
		Applicable: map[string]bool{"SRP": true, "LSP": true},
		Findings:   []FindingLite{{ID: "f1", Severity: "high"}, {ID: "f2", Severity: "low"}},
	}
	b := PeerScores{
		RunID:      "r",
		Applicable: map[string]bool{"SRP": true, "LSP": false},
		Findings:   []FindingLite{{ID: "f1", Severity: "medium"}},
	}
	dm, _ := Compute(a, b)
	// 1 applicable + 1 severity + 1 onlyA(f2) + 0 onlyB = 3.
	if dm.TotalDivergences != 3 {
		t.Errorf("total: %d", dm.TotalDivergences)
	}
	if !dm.ArbiterRequired {
		t.Errorf("needs arbitration")
	}
}

// Aceitação: ArbiterRequired helper.
func TestArbiterRequired(t *testing.T) {
	var dm *DivergenceMap
	if dm.RequiresArbiter() {
		t.Errorf("nil")
	}
	dm = &DivergenceMap{}
	if dm.RequiresArbiter() {
		t.Errorf("zero")
	}
	dm.ArbiterRequired = true
	if !dm.RequiresArbiter() {
		t.Errorf("true")
	}
}

// Aceitação: HasMismatch helper.
func TestHasMismatch(t *testing.T) {
	var dm *DivergenceMap
	if dm.HasMismatch() {
		t.Errorf("nil")
	}
	dm = &DivergenceMap{TotalDivergences: 1}
	if !dm.HasMismatch() {
		t.Errorf("has")
	}
}

// Aceitação: RenderDivergence.
func TestRenderDivergence(t *testing.T) {
	dm := &DivergenceMap{
		RunID:              "r1",
		ScoreDelta:         0.5,
		ArbiterRequired:    true,
		ApplicableMismatch: []ApplicableMismatch{{Principle: "SRP", PeerA: true, PeerB: false}},
	}
	out := RenderDivergence(dm)
	if !strings.Contains(out, "r1") {
		t.Errorf("runid")
	}
	if !strings.Contains(out, "SRP") {
		t.Errorf("applicable")
	}
	if !strings.Contains(out, "arbitration") {
		t.Errorf("arb")
	}
	if RenderDivergence(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: itoa/ftoa.
func TestHelpers(t *testing.T) {
	if itoa(0) != "0" {
		t.Errorf("0")
	}
	if itoa(123) != "123" {
		t.Errorf("123")
	}
	if itoa(-5) != "-5" {
		t.Errorf("neg")
	}
	if ftoa(1.5) != "1.50" {
		t.Errorf("f: %s", ftoa(1.5))
	}
	if ftoa(0.25) != "0.25" {
		t.Errorf("f25")
	}
}

// Aceitação: padLeft2.
func TestPadLeft2(t *testing.T) {
	if padLeft2("5") != "05" {
		t.Errorf("pad")
	}
	if padLeft2("55") != "55" {
		t.Errorf("nopad")
	}
}

// Aceitação: SortMismatches.
func TestSortMismatches(t *testing.T) {
	m := []ApplicableMismatch{
		{Principle: "Z"}, {Principle: "A"}, {Principle: "M"},
	}
	sortMismatches(m)
	if m[0].Principle != "A" || m[1].Principle != "M" || m[2].Principle != "Z" {
		t.Errorf("sort: %v", m)
	}
}

// Aceitação: Empty scores = zero delta.
func TestScoreDeltaEmpty(t *testing.T) {
	a := PeerScores{RunID: "r"}
	b := PeerScores{RunID: "r"}
	dm, _ := Compute(a, b)
	if dm.ScoreDelta != 0 {
		t.Errorf("zero")
	}
}
