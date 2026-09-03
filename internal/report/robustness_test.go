package report

import (
	"testing"

	"github.com/diegoaraujo/solidify/internal/peer"
)

func makeSnap(id string, sc float64, gate string, fids []string) *peer.RunSnapshot {
	fs := []peer.RobustFindingLite{}
	for _, fid := range fids {
		fs = append(fs, peer.RobustFindingLite{ID: fid})
	}
	return &peer.RunSnapshot{
		RunID: id, Score: sc, GateStatus: gate, Findings: fs,
	}
}

// Aceitação: AttachRobustness popula Extra.
func TestAttachRobustness(t *testing.T) {
	rep := &Report{SchemaVersion: SchemaVersion}
	orig := makeSnap("r1", 80, "PASS", []string{"a", "b"})
	sw := makeSnap("r2", 81, "PASS", []string{"a", "c"})
	if err := AttachRobustness(rep, orig, sw); err != nil {
		t.Fatalf("attach: %v", err)
	}
	rb := RobustnessFromReport(rep)
	if rb == nil {
		t.Fatalf("no block")
	}
	if rb.Status != "stable" {
		t.Errorf("status: %s", rb.Status)
	}
	if rb.QualityDelta != 1 {
		t.Errorf("delta: %f", rb.QualityDelta)
	}
}

// Aceitação: AttachRobustness nil snapshot.
func TestAttachRobustnessNil(t *testing.T) {
	rep := &Report{SchemaVersion: SchemaVersion}
	if err := AttachRobustness(rep, nil, nil); err != ErrNoRobustness {
		t.Errorf("expected ErrNoRobustness: %v", err)
	}
}

// Aceitação: AttachRobustness rep nil.
func TestAttachRobustnessNilRep(t *testing.T) {
	if err := AttachRobustness(nil, makeSnap("a", 80, "PASS", nil), makeSnap("b", 81, "PASS", nil)); err == nil {
		t.Errorf("expected error")
	}
}

// Aceitação: MarkRobustnessNotRun.
func TestMarkNotRun(t *testing.T) {
	rep := &Report{SchemaVersion: SchemaVersion}
	MarkRobustnessNotRun(rep)
	rb := RobustnessFromReport(rep)
	if rb == nil || rb.Status != "not_run" {
		t.Errorf("not_run: %+v", rb)
	}
}

// Aceitação: MarkRobustnessNotRun nil rep.
func TestMarkNotRunNil(t *testing.T) {
	MarkRobustnessNotRun(nil) // no panic
}

// Aceitação: RobustnessFromReport nil/empty.
func TestFromReportNil(t *testing.T) {
	if RobustnessFromReport(nil) != nil {
		t.Errorf("nil")
	}
	rep := &Report{}
	if RobustnessFromReport(rep) != nil {
		t.Errorf("empty")
	}
}

// Aceitação: RenderRobustness textual.
func TestRenderRobustness(t *testing.T) {
	rb := &RobustnessBlock{
		OriginalRunID: "r1", SwappedRunID: "r2",
		Status: "stable", QualityDelta: 1, Overlap: 0.8,
	}
	out := rb.RenderRobustness()
	if out == "" || out == "Robustness: <nil>\n" {
		t.Errorf("render: %s", out)
	}
	var nilRb *RobustnessBlock
	if nilRb.RenderRobustness() != "Robustness: <nil>\n" {
		t.Errorf("nil render")
	}
}

// Aceitação: helpers numéricos.
func TestRobustnessHelpers(t *testing.T) {
	if ftoaRob(1.5) != "1.50" {
		t.Errorf("1.50")
	}
	if itoaRob(0) != "0" {
		t.Errorf("0")
	}
	if itoaRob(-5) != "-5" {
		t.Errorf("-5")
	}
	if padLeftRob("5", 2) != "05" {
		t.Errorf("pad")
	}
}
