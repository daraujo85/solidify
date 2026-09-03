package gate

import (
	"strings"
	"testing"
)

// Aceitação: PASS básico.
func TestPass(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore:    85,
		IndependenceOK: true, ArbiterResolved: true, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusPass {
		t.Errorf("esperado PASS: %s", r.Status)
	}
}

// Aceitação: WARN entre threshold e threshold+5.
func TestWarn(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore:    77, // 75 + 2
		IndependenceOK: true, ArbiterResolved: true, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusWarn {
		t.Errorf("esperado WARN: %s", r.Status)
	}
}

// Aceitação: FAIL abaixo do threshold.
func TestFail(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore:    50,
		IndependenceOK: true, ArbiterResolved: true, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusFail {
		t.Errorf("esperado FAIL: %s", r.Status)
	}
}

// Aceitação: BLOCKED por immutable critical.
func TestBlockedImmutable(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore:       95,
		ImmutableCritical: 1,
		IndependenceOK:    true, ArbiterResolved: true, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusBlocked {
		t.Errorf("esperado BLOCKED: %s", r.Status)
	}
}

// Aceitação: BLOCKED por independence fail.
func TestBlockedIndependence(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore:    95,
		IndependenceOK: false, ArbiterResolved: true, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusBlocked {
		t.Errorf("esperado BLOCKED: %s", r.Status)
	}
}

// Aceitação: INCOMPLETE por evidence.
func TestIncompleteEvidence(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore:    95,
		IndependenceOK: true, ArbiterResolved: true, EvidenceComplete: false,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusIncomplete {
		t.Errorf("esperado INCOMPLETE: %s", r.Status)
	}
}

// Aceitação: INCOMPLETE por arbiter não resolveu.
func TestIncompleteArbiter(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore:    95,
		IndependenceOK: true, ArbiterResolved: false, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusIncomplete {
		t.Errorf("esperado INCOMPLETE: %s", r.Status)
	}
}

// Aceitação: RunID vazio.
func TestEmptyRunID(t *testing.T) {
	if _, err := Evaluate(GateInput{Profile: ProfileRelease}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Profile vazio.
func TestEmptyProfile(t *testing.T) {
	if _, err := Evaluate(GateInput{RunID: "r1"}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Profile inválido.
func TestInvalidProfile(t *testing.T) {
	if _, err := Evaluate(GateInput{RunID: "r1", Profile: "weird"}); err == nil {
		t.Errorf("inválido")
	}
}

// Aceitação: Perfil quick threshold 60.
func TestProfileQuick(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileQuick,
		GlobalScore:    70,
		IndependenceOK: true, ArbiterResolved: true, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusPass {
		t.Errorf("quick 70: %s", r.Status)
	}
}

// Aceitação: Perfil contractual threshold 85.
func TestProfileContractual(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileContractual,
		GlobalScore:    80,
		IndependenceOK: true, ArbiterResolved: true, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusFail {
		t.Errorf("contractual 80: %s", r.Status)
	}
}

// Aceitação: IsBlocking.
func TestIsBlocking(t *testing.T) {
	var r *GateResult
	if !r.IsBlocking() {
		t.Errorf("nil bloqueia")
	}
	r = &GateResult{Status: StatusPass}
	if r.IsBlocking() {
		t.Errorf("pass")
	}
	r = &GateResult{Status: StatusWarn}
	if r.IsBlocking() {
		t.Errorf("warn não bloqueia")
	}
	r = &GateResult{Status: StatusFail}
	if !r.IsBlocking() {
		t.Errorf("fail")
	}
	r = &GateResult{Status: StatusBlocked}
	if !r.IsBlocking() {
		t.Errorf("blocked")
	}
	r = &GateResult{Status: StatusIncomplete}
	if !r.IsBlocking() {
		t.Errorf("incomplete")
	}
}

// Aceitação: IsPassing.
func TestIsPassing(t *testing.T) {
	var r *GateResult
	if r.IsPassing() {
		t.Errorf("nil")
	}
	r = &GateResult{Status: StatusPass}
	if !r.IsPassing() {
		t.Errorf("pass")
	}
	r = &GateResult{Status: StatusWarn}
	if !r.IsPassing() {
		t.Errorf("warn")
	}
	r = &GateResult{Status: StatusFail}
	if r.IsPassing() {
		t.Errorf("fail não")
	}
}

// Aceitação: ValidProfile.
func TestValidProfile(t *testing.T) {
	for _, p := range []string{ProfileQuick, ProfileRelease, ProfileContractual} {
		if !ValidProfile(p) {
			t.Errorf("%s inválido", p)
		}
	}
	if ValidProfile("random") {
		t.Errorf("random")
	}
}

// Aceitação: RenderGate.
func TestRenderGate(t *testing.T) {
	r := &GateResult{
		RunID: "r1", Profile: ProfileRelease,
		Status: StatusPass, Reason: "ok",
		Score: 85, Threshold: 75,
	}
	out := RenderGate(r)
	if !strings.Contains(out, "r1") {
		t.Errorf("runid")
	}
	if !strings.Contains(out, StatusPass) {
		t.Errorf("status")
	}
	if RenderGate(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: BLOCKED precedence sobre FAIL.
func TestBlockedPrecedence(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore: 30, ImmutableCritical: 1,
		IndependenceOK: true, ArbiterResolved: true, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusBlocked {
		t.Errorf("BLOCKED > FAIL: %s", r.Status)
	}
}

// Aceitação: INCOMPLETE precedence sobre FAIL.
func TestIncompletePrecedence(t *testing.T) {
	in := GateInput{
		RunID: "r1", Profile: ProfileRelease,
		GlobalScore:    30,
		IndependenceOK: true, ArbiterResolved: false, EvidenceComplete: true,
	}
	r, _ := Evaluate(in)
	if r.Status != StatusIncomplete {
		t.Errorf("INCOMPLETE > FAIL: %s", r.Status)
	}
}

// Aceitação: helpers numéricos.
func TestHelpers(t *testing.T) {
	if ftoa5(1.5) != "1.50" {
		t.Errorf("1.50")
	}
	if itoa5(0) != "0" {
		t.Errorf("0")
	}
	if padLeft5("5", 2) != "05" {
		t.Errorf("pad")
	}
}
