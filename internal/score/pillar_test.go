package score

import (
	"strings"
	"testing"
)

// Aceitação: ComputeGlobalScore basic.
func TestComputeGlobalBasic(t *testing.T) {
	in := PillarInput{
		RunID: "r1",
		Pillars: []PillarScore{
			{Pillar: PillarSOLID, Score: 80, Applicable: true},
			{Pillar: PillarSecurity, Score: 90, Applicable: true},
			{Pillar: PillarPerformance, Score: 70, Applicable: true},
			{Pillar: PillarMaintainability, Score: 100, Applicable: true},
		},
	}
	r, err := ComputeGlobalScore(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// 80*0.4 + 90*0.25 + 70*0.2 + 100*0.15 = 32 + 22.5 + 14 + 15 = 83.5
	if r.GlobalScore < 83 || r.GlobalScore > 84 {
		t.Errorf("global: %f", r.GlobalScore)
	}
}

// Aceitação: ComputeGlobalScore RunID vazio.
func TestComputeGlobalEmptyRunID(t *testing.T) {
	if _, err := ComputeGlobalScore(PillarInput{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: pilar N/A excluída.
func TestComputeGlobalNA(t *testing.T) {
	in := PillarInput{
		RunID: "r1",
		Pillars: []PillarScore{
			{Pillar: PillarSOLID, Score: 80, Applicable: true},
			{Pillar: PillarSecurity, Applicable: false},
		},
	}
	r, _ := ComputeGlobalScore(in)
	// SOLID: 80*0.4 / 0.4 = 80
	if r.GlobalScore != 80 {
		t.Errorf("global: %f", r.GlobalScore)
	}
}

// Aceitação: nenhuma pilar aplicável → 0.
func TestComputeGlobalAllNA(t *testing.T) {
	in := PillarInput{
		RunID: "r1",
		Pillars: []PillarScore{
			{Pillar: PillarSOLID, Applicable: false},
		},
	}
	r, _ := ComputeGlobalScore(in)
	if r.GlobalScore != 0 {
		t.Errorf("vazio: %f", r.GlobalScore)
	}
}

// Aceitação: score fora de range.
func TestComputeGlobalOutOfRange(t *testing.T) {
	in := PillarInput{
		RunID: "r1",
		Pillars: []PillarScore{
			{Pillar: PillarSOLID, Score: 150, Applicable: true},
		},
	}
	if _, err := ComputeGlobalScore(in); err == nil {
		t.Errorf("fora range")
	}
}

// Aceitação: pilar sem weight → missing.
func TestComputeGlobalMissingWeight(t *testing.T) {
	in := PillarInput{
		RunID: "r1",
		Pillars: []PillarScore{
			{Pillar: "custom", Score: 80, Applicable: true},
		},
	}
	r, _ := ComputeGlobalScore(in)
	if len(r.Missing) != 1 {
		t.Errorf("missing: %v", r.Missing)
	}
}

// Aceitação: custom weights.
func TestComputeGlobalCustomWeights(t *testing.T) {
	in := PillarInput{
		RunID: "r1",
		Weights: map[string]float64{
			PillarSOLID: 1.0,
		},
		Pillars: []PillarScore{
			{Pillar: PillarSOLID, Score: 75, Applicable: true},
		},
	}
	r, _ := ComputeGlobalScore(in)
	if r.GlobalScore != 75 {
		t.Errorf("custom: %f", r.GlobalScore)
	}
}

// Aceitação: ValidatePillarName.
func TestValidatePillarName(t *testing.T) {
	for _, p := range Pillars {
		if !ValidatePillarName(p) {
			t.Errorf("%s", p)
		}
	}
	if ValidatePillarName("random") {
		t.Errorf("random")
	}
}

// Aceitação: NormalizeWeights.
func TestNormalizeWeights(t *testing.T) {
	w := map[string]float64{"a": 2.0, "b": 1.0, "c": 1.0}
	out := NormalizeWeights(w)
	if out["a"] != 0.5 {
		t.Errorf("a: %f", out["a"])
	}
	if out["b"] != 0.25 {
		t.Errorf("b: %f", out["b"])
	}
}

// Aceitação: NormalizeWeights soma zero.
func TestNormalizeWeightsZero(t *testing.T) {
	w := map[string]float64{"a": 0, "b": 0}
	out := NormalizeWeights(w)
	if len(out) != 0 {
		t.Errorf("zero sum")
	}
}

// Aceitação: NormalizeWeights vazio.
func TestNormalizeWeightsEmpty(t *testing.T) {
	out := NormalizeWeights(map[string]float64{})
	if len(out) != 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: GetWeight.
func TestGetWeight(t *testing.T) {
	if w := GetWeight(PillarSOLID, nil); w != 0.40 {
		t.Errorf("default solid: %f", w)
	}
	if w := GetWeight(PillarSOLID, map[string]float64{PillarSOLID: 0.5}); w != 0.5 {
		t.Errorf("custom: %f", w)
	}
	if w := GetWeight("unknown", nil); w != 0 {
		t.Errorf("unknown: %f", w)
	}
}

// Aceitação: RenderResult.
func TestRenderResult(t *testing.T) {
	r := &PillarResult{
		RunID:       "r1",
		GlobalScore: 85,
		Pillars: []PillarScore{
			{Pillar: PillarSOLID, Score: 80, Applicable: true, Weight: 0.4},
			{Pillar: PillarSecurity, Applicable: false},
		},
		Missing: []string{"foo"},
	}
	out := RenderResult(r)
	if !strings.Contains(out, "r1") {
		t.Errorf("runid")
	}
	if !strings.Contains(out, PillarSOLID) {
		t.Errorf("solid")
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("missing")
	}
	if RenderResult(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: DefaultWeights somam 1.
func TestDefaultWeightsSum(t *testing.T) {
	sum := 0.0
	for _, w := range DefaultWeights {
		sum += w
	}
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("sum: %f", sum)
	}
}

// Aceitação: Pillars 4 entries.
func TestPillarsCount(t *testing.T) {
	if len(Pillars) != 4 {
		t.Errorf("count: %d", len(Pillars))
	}
}
