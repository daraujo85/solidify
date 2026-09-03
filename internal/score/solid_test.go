package score

import (
	"strings"
	"testing"
)

// Aceitação: ComputeScore basic.
func TestComputeScoreBasic(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Score: 80, Applicable: true},
		{Principle: "OCP", Score: 90, Applicable: true},
		{Principle: "LSP", Score: 100, Applicable: true},
	}
	s := ComputeScore(letters)
	if s != 90 {
		t.Errorf("esperado 90: %f", s)
	}
}

// Aceitação: N/A letters excluídas.
func TestComputeScoreNA(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Score: 80, Applicable: true},
		{Principle: "LSP", Applicable: false}, // N/A
		{Principle: "OCP", Score: 100, Applicable: true},
	}
	s := ComputeScore(letters)
	if s != 90 {
		t.Errorf("N/A deve ser excluída: %f", s)
	}
}

// Aceitação: todas N/A → score 0 (sem divisão por zero).
func TestComputeScoreAllNA(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Applicable: false},
		{Principle: "LSP", Applicable: false},
	}
	s := ComputeScore(letters)
	if s != 0 {
		t.Errorf("vazio: %f", s)
	}
}

// Aceitação: letras vazias → 0.
func TestComputeScoreEmpty(t *testing.T) {
	if s := ComputeScore(nil); s != 0 {
		t.Errorf("nil")
	}
	if s := ComputeScore([]LetterScore{}); s != 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: ComputeDelta improved.
func TestComputeDeltaImproved(t *testing.T) {
	b := Snapshot{RunID: "r", Label: "before", Letters: []LetterScore{
		{Principle: "SRP", Score: 50, Applicable: true},
		{Principle: "OCP", Score: 60, Applicable: true},
	}}
	a := Snapshot{RunID: "r", Label: "after", Letters: []LetterScore{
		{Principle: "SRP", Score: 90, Applicable: true},
		{Principle: "OCP", Score: 80, Applicable: true},
	}}
	d, err := ComputeDelta(b, a)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !d.Improved {
		t.Errorf("improved")
	}
	if d.Unchanged || d.Regressed {
		t.Errorf("flags")
	}
	if d.BeforeScore != 55 || d.AfterScore != 85 {
		t.Errorf("scores: %f / %f", d.BeforeScore, d.AfterScore)
	}
	if d.ScoreDelta != 30 {
		t.Errorf("delta: %f", d.ScoreDelta)
	}
}

// Aceitação: ComputeDelta regressed.
func TestComputeDeltaRegressed(t *testing.T) {
	b := Snapshot{RunID: "r", Letters: []LetterScore{
		{Principle: "SRP", Score: 90, Applicable: true},
	}}
	a := Snapshot{RunID: "r", Letters: []LetterScore{
		{Principle: "SRP", Score: 50, Applicable: true},
	}}
	d, _ := ComputeDelta(b, a)
	if !d.Regressed {
		t.Errorf("regressed")
	}
}

// Aceitação: ComputeDelta unchanged.
func TestComputeDeltaUnchanged(t *testing.T) {
	b := Snapshot{RunID: "r", Letters: []LetterScore{
		{Principle: "SRP", Score: 80, Applicable: true},
	}}
	a := Snapshot{RunID: "r", Letters: []LetterScore{
		{Principle: "SRP", Score: 80, Applicable: true},
	}}
	d, _ := ComputeDelta(b, a)
	if !d.Unchanged {
		t.Errorf("unchanged")
	}
	if d.Improved || d.Regressed {
		t.Errorf("flags")
	}
}

// Aceitação: ComputeDelta RunIDs diferentes.
func TestComputeDeltaDifferentRunIDs(t *testing.T) {
	b := Snapshot{RunID: "r1"}
	a := Snapshot{RunID: "r2"}
	if _, err := ComputeDelta(b, a); err == nil {
		t.Errorf("diff")
	}
}

// Aceitação: ComputeDelta RunID vazio.
func TestComputeDeltaEmptyRunID(t *testing.T) {
	if _, err := ComputeDelta(Snapshot{}, Snapshot{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: ValidateLetters OK.
func TestValidateLettersOK(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Score: 80, Applicable: true},
		{Principle: "OCP", Score: 90, Applicable: true},
	}
	if err := ValidateLetters(letters); err != nil {
		t.Errorf("err: %v", err)
	}
}

// Aceitação: ValidateLetters principle vazio.
func TestValidateLettersEmpty(t *testing.T) {
	if err := ValidateLetters([]LetterScore{{Score: 80, Applicable: true}}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: ValidateLetters duplicado.
func TestValidateLettersDuplicated(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Score: 80, Applicable: true},
		{Principle: "SRP", Score: 90, Applicable: true},
	}
	if err := ValidateLetters(letters); err == nil {
		t.Errorf("dup")
	}
}

// Aceitação: ValidateLetters out of range.
func TestValidateLettersOutOfRange(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Score: 150, Applicable: true},
	}
	if err := ValidateLetters(letters); err == nil {
		t.Errorf("fora range")
	}
}

// Aceitação: ValidateLetters score negativo.
func TestValidateLettersNegative(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Score: -10, Applicable: true},
	}
	if err := ValidateLetters(letters); err == nil {
		t.Errorf("neg")
	}
}

// Aceitação: CanonicalLetters ordem.
func TestCanonicalLetters(t *testing.T) {
	in := []LetterScore{
		{Principle: "DIP"}, {Principle: "SRP"}, {Principle: "OCP"},
	}
	out := CanonicalLetters(in)
	if out[0].Principle != "SRP" || out[1].Principle != "OCP" || out[2].Principle != "DIP" {
		t.Errorf("ordem: %v", out)
	}
}

// Aceitação: LetterByPrinciple.
func TestLetterByPrinciple(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Score: 80},
		{Principle: "OCP", Score: 90},
	}
	if l, ok := LetterByPrinciple(letters, "SRP"); !ok || l.Score != 80 {
		t.Errorf("not found")
	}
	if _, ok := LetterByPrinciple(letters, "DIP"); ok {
		t.Errorf("DIP devia não existir")
	}
}

// Aceitação: IsPrincipleValid.
func TestIsPrincipleValid(t *testing.T) {
	for _, p := range Principles {
		if !IsPrincipleValid(p) {
			t.Errorf("%s inválido", p)
		}
	}
	if IsPrincipleValid("XYZ") {
		t.Errorf("XYZ inválido")
	}
}

// Aceitação: RenderSnapshot.
func TestRenderSnapshot(t *testing.T) {
	s := Snapshot{
		RunID: "r1", Label: "before",
		Letters: []LetterScore{
			{Principle: "SRP", Score: 80, Applicable: true},
			{Principle: "LSP", Applicable: false},
		},
	}
	out := RenderSnapshot(s)
	if !strings.Contains(out, "r1") {
		t.Errorf("runid")
	}
	if !strings.Contains(out, "N/A") {
		t.Errorf("NA")
	}
}

// Aceitação: RenderDelta.
func TestRenderDelta(t *testing.T) {
	d := &Delta{RunID: "r", Improved: true, ScoreDelta: 10}
	out := RenderDelta(d)
	if !strings.Contains(out, "improved") {
		t.Errorf("improved")
	}
	if RenderDelta(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: ftoa1.
func TestFtoa1(t *testing.T) {
	if ftoa1(1.5) != "1.5" {
		t.Errorf("1.5")
	}
	if ftoa1(0.0) != "0.0" {
		t.Errorf("0")
	}
}

// Aceitação: itoa2.
func TestItoa2(t *testing.T) {
	if itoa2(0) != "0" {
		t.Errorf("0")
	}
	if itoa2(42) != "42" {
		t.Errorf("42")
	}
	if itoa2(-5) != "-5" {
		t.Errorf("neg")
	}
}

// Aceitação: countApplicable.
func TestCountApplicable(t *testing.T) {
	letters := []LetterScore{
		{Principle: "SRP", Applicable: true},
		{Principle: "OCP", Applicable: false},
		{Principle: "LSP", Applicable: true},
	}
	if n := countApplicable(letters); n != 2 {
		t.Errorf("count: %d", n)
	}
}

// Aceitação: applicable counts delta.
func TestDeltaApplicableCounts(t *testing.T) {
	b := Snapshot{RunID: "r", Letters: []LetterScore{
		{Principle: "SRP", Score: 50, Applicable: true},
		{Principle: "LSP", Applicable: false},
	}}
	a := Snapshot{RunID: "r", Letters: []LetterScore{
		{Principle: "SRP", Score: 80, Applicable: true},
		{Principle: "LSP", Score: 70, Applicable: true},
	}}
	d, _ := ComputeDelta(b, a)
	if d.ApplicableBefore != 1 || d.ApplicableAfter != 2 {
		t.Errorf("applicable counts: %d/%d", d.ApplicableBefore, d.ApplicableAfter)
	}
}
