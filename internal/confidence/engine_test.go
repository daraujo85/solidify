package confidence

import (
	"strings"
	"testing"
)

// Aceitação: Compute basic.
func TestComputeBasic(t *testing.T) {
	in := ConfidenceInput{
		RunID: "r1",
		Signals: []SignalScore{
			{Signal: SignalEvidence, Score: 90},
			{Signal: SignalAgreement, Score: 80},
			{Signal: SignalIndependence, Score: 100},
			{Signal: SignalAnalyzer, Score: 70},
			{Signal: SignalArbiter, Score: 85},
		},
	}
	r, err := Compute(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Score < 80 || r.Score > 90 {
		t.Errorf("score: %f", r.Score)
	}
	if r.Level != "high" {
		t.Errorf("level: %s", r.Level)
	}
}

// Aceitação: RunID vazio.
func TestComputeEmptyRunID(t *testing.T) {
	if _, err := Compute(ConfidenceInput{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: signal fora range.
func TestComputeSignalOutOfRange(t *testing.T) {
	in := ConfidenceInput{
		RunID:   "r1",
		Signals: []SignalScore{{Signal: SignalEvidence, Score: 150}},
	}
	if _, err := Compute(in); err == nil {
		t.Errorf("fora range")
	}
}

// Aceitação: signal missing weight.
func TestComputeMissingWeight(t *testing.T) {
	in := ConfidenceInput{
		RunID: "r1",
		Signals: []SignalScore{
			{Signal: "custom_signal", Score: 80},
		},
	}
	r, _ := Compute(in)
	if len(r.Missing) != 1 {
		t.Errorf("missing: %v", r.Missing)
	}
}

// Aceitação: nenhum signal → score 0.
func TestComputeNoSignals(t *testing.T) {
	in := ConfidenceInput{RunID: "r1"}
	r, _ := Compute(in)
	if r.Score != 0 {
		t.Errorf("vazio: %f", r.Score)
	}
}

// Aceitação: level low.
func TestLevelLow(t *testing.T) {
	if computeLevel(30) != "low" {
		t.Errorf("low")
	}
}

// Aceitação: level medium.
func TestLevelMedium(t *testing.T) {
	if computeLevel(60) != "medium" {
		t.Errorf("medium")
	}
}

// Aceitação: level high.
func TestLevelHigh(t *testing.T) {
	if computeLevel(85) != "high" {
		t.Errorf("high")
	}
}

// Aceitação: IsHighConfidence.
func TestIsHighConfidence(t *testing.T) {
	var r *ConfidenceResult
	if r.IsHighConfidence() {
		t.Errorf("nil")
	}
	r = &ConfidenceResult{Level: "high"}
	if !r.IsHighConfidence() {
		t.Errorf("high")
	}
	r = &ConfidenceResult{Level: "medium"}
	if r.IsHighConfidence() {
		t.Errorf("medium não")
	}
}

// Aceitação: RenderResult.
func TestRenderResult(t *testing.T) {
	r := &ConfidenceResult{
		RunID: "r1", Score: 85, Level: "high",
		Signals: []SignalScore{{Signal: SignalEvidence, Score: 90, Weight: 0.3, Notes: "complete"}},
	}
	out := RenderResult(r)
	if !strings.Contains(out, "r1") {
		t.Errorf("runid")
	}
	if !strings.Contains(out, SignalEvidence) {
		t.Errorf("signal")
	}
	if !strings.Contains(out, "complete") {
		t.Errorf("notes")
	}
	if RenderResult(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: DefaultWeights somam 1.
func TestDefaultWeightsSum(t *testing.T) {
	sum := DefaultWeightsSum()
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("sum: %f", sum)
	}
}

// Aceitação: Custom weights.
func TestCustomWeights(t *testing.T) {
	in := ConfidenceInput{
		RunID: "r1",
		Weights: map[string]float64{
			SignalEvidence: 1.0,
		},
		Signals: []SignalScore{
			{Signal: SignalEvidence, Score: 70},
		},
	}
	r, _ := Compute(in)
	if r.Score != 70 {
		t.Errorf("custom: %f", r.Score)
	}
}

// Aceitação: helpers numéricos.
func TestHelpers(t *testing.T) {
	if itoa4(0) != "0" {
		t.Errorf("0")
	}
	if itoa4(42) != "42" {
		t.Errorf("42")
	}
	if ftoa4(1.5) != "1.50" {
		t.Errorf("1.50")
	}
	if padLeft4("5", 2) != "05" {
		t.Errorf("pad")
	}
}

// Aceitação: Score negativo.
func TestNegativeScore(t *testing.T) {
	in := ConfidenceInput{
		RunID:   "r1",
		Signals: []SignalScore{{Signal: SignalEvidence, Score: -10}},
	}
	if _, err := Compute(in); err == nil {
		t.Errorf("neg")
	}
}

// Aceitação: Score exatamente 100.
func TestScoreMax(t *testing.T) {
	in := ConfidenceInput{
		RunID: "r1",
		Signals: []SignalScore{
			{Signal: SignalEvidence, Score: 100},
			{Signal: SignalAgreement, Score: 100},
			{Signal: SignalIndependence, Score: 100},
			{Signal: SignalAnalyzer, Score: 100},
			{Signal: SignalArbiter, Score: 100},
		},
	}
	r, _ := Compute(in)
	if r.Score != 100 {
		t.Errorf("max: %f", r.Score)
	}
}
