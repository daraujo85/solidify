package peer

import (
	"strings"
	"testing"
)

// helpers.
func makeRun(id string, sc float64, gate string, findings []RobustFindingLite) *RunSnapshot {
	return &RunSnapshot{
		RunID:      id,
		Score:      sc,
		GateStatus: gate,
		Findings:   findings,
	}
}

// Aceitação: Compare básico.
func TestCompareBasic(t *testing.T) {
	orig := makeRun("r1", 80, "PASS", []RobustFindingLite{{ID: "a"}, {ID: "b"}})
	sw := makeRun("r2", 81, "PASS", []RobustFindingLite{{ID: "a"}, {ID: "c"}})
	r, err := Compare(orig, sw)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if r.QualityDelta != 1 {
		t.Errorf("delta: %f", r.QualityDelta)
	}
	if !r.GateStable {
		t.Errorf("gate stable")
	}
	if r.Status != "stable" {
		t.Errorf("status: %s", r.Status)
	}
	if len(r.Both) != 1 || r.Both[0] != "a" {
		t.Errorf("both: %v", r.Both)
	}
	if len(r.OnlyOriginal) != 1 || r.OnlyOriginal[0] != "b" {
		t.Errorf("onlyA: %v", r.OnlyOriginal)
	}
	if len(r.OnlySwapped) != 1 || r.OnlySwapped[0] != "c" {
		t.Errorf("onlyB: %v", r.OnlySwapped)
	}
}

// Aceitação: Compare nil.
func TestCompareNil(t *testing.T) {
	if _, err := Compare(nil, nil); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: Gate instability → unstable.
func TestCompareGateUnstable(t *testing.T) {
	orig := makeRun("r1", 80, "PASS", nil)
	sw := makeRun("r2", 81, "FAIL", nil)
	r, _ := Compare(orig, sw)
	if r.GateStable {
		t.Errorf("expected unstable")
	}
	if r.Status != "unstable" {
		t.Errorf("status: %s", r.Status)
	}
}

// Aceitação: Quality delta > 5 → unstable.
func TestCompareBigDelta(t *testing.T) {
	orig := makeRun("r1", 70, "PASS", nil)
	sw := makeRun("r2", 80, "PASS", nil)
	r, _ := Compare(orig, sw)
	if r.Status != "unstable" {
		t.Errorf("delta 10 unstable: %s", r.Status)
	}
}

// Aceitação: Quality delta entre 2-5 → mostly-stable.
func TestCompareMediumDelta(t *testing.T) {
	orig := makeRun("r1", 75, "PASS", nil)
	sw := makeRun("r2", 78, "PASS", nil)
	r, _ := Compare(orig, sw)
	if r.Status != "mostly-stable" {
		t.Errorf("delta 3: %s", r.Status)
	}
}

// Aceitação: Quality delta < 2 → stable.
func TestCompareTinyDelta(t *testing.T) {
	orig := makeRun("r1", 80, "PASS", nil)
	sw := makeRun("r2", 81, "PASS", nil)
	r, _ := Compare(orig, sw)
	if r.Status != "stable" {
		t.Errorf("delta 1: %s", r.Status)
	}
}

// Aceitação: Overlap ratio.
func TestCompareOverlapRatio(t *testing.T) {
	// 3 both, 1 onlyA, 1 onlyB → ratio = 3/5 = 0.6.
	orig := makeRun("r1", 80, "PASS", []RobustFindingLite{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}})
	sw := makeRun("r2", 80, "PASS", []RobustFindingLite{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "e"}})
	r, _ := Compare(orig, sw)
	if r.Overlap < 0.59 || r.Overlap > 0.61 {
		t.Errorf("overlap: %f", r.Overlap)
	}
}

// Aceitação: Overlap vazio.
func TestCompareOverlapEmpty(t *testing.T) {
	orig := makeRun("r1", 80, "PASS", nil)
	sw := makeRun("r2", 80, "PASS", nil)
	r, _ := Compare(orig, sw)
	if r.Overlap != 0 {
		t.Errorf("vazio: %f", r.Overlap)
	}
}

// Aceitação: Overlap 100%.
func TestOverlapFull(t *testing.T) {
	fs := []RobustFindingLite{{ID: "a"}, {ID: "b"}}
	orig := makeRun("r1", 80, "PASS", fs)
	sw := makeRun("r2", 80, "PASS", fs)
	r, _ := Compare(orig, sw)
	if r.Overlap != 1 {
		t.Errorf("overlap: %f", r.Overlap)
	}
}

// Aceitação: negative delta.
func TestCompareNegativeDelta(t *testing.T) {
	orig := makeRun("r1", 90, "PASS", nil)
	sw := makeRun("r2", 85, "PASS", nil)
	r, _ := Compare(orig, sw)
	if r.QualityDelta != -5 {
		t.Errorf("delta: %f", r.QualityDelta)
	}
	if r.Status != "unstable" {
		t.Errorf("status: %s", r.Status)
	}
}

// Aceitação: IsStable helper.
func TestIsStable(t *testing.T) {
	var r *RobustnessResult
	if r.IsStable() {
		t.Errorf("nil")
	}
	r = &RobustnessResult{Status: "stable"}
	if !r.IsStable() {
		t.Errorf("stable")
	}
	r = &RobustnessResult{Status: "unstable"}
	if r.IsStable() {
		t.Errorf("unstable não")
	}
}

// Aceitação: RenderResult.
func TestRenderResult(t *testing.T) {
	r := &RobustnessResult{
		OriginalRunID: "r1", SwappedRunID: "r2",
		Status: "stable", QualityDelta: 1, Overlap: 0.8,
	}
	out := r.RenderResult()
	if !strings.Contains(out, "r1") || !strings.Contains(out, "r2") {
		t.Errorf("runs")
	}
	if !strings.Contains(out, "stable") {
		t.Errorf("status")
	}
	var nilR *RobustnessResult
	if nilR.RenderResult() != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: findings overlap helper.
func TestFindingsOverlap(t *testing.T) {
	onlyA, onlyB, both := findingsOverlap(
		[]RobustFindingLite{{ID: "a"}, {ID: "b"}},
		[]RobustFindingLite{{ID: "b"}, {ID: "c"}},
	)
	if len(both) != 1 || both[0] != "b" {
		t.Errorf("both: %v", both)
	}
	if len(onlyA) != 1 || onlyA[0] != "a" {
		t.Errorf("onlyA: %v", onlyA)
	}
	if len(onlyB) != 1 || onlyB[0] != "c" {
		t.Errorf("onlyB: %v", onlyB)
	}
}

// Aceitação: helpers numéricos.
func TestRobustnessHelpers(t *testing.T) {
	if ftoaR(1.5) != "1.50" {
		t.Errorf("1.50")
	}
	if itoaR(0) != "0" {
		t.Errorf("0")
	}
	if padLeftR("5", 2) != "05" {
		t.Errorf("pad")
	}
}
