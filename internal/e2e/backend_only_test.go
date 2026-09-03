package e2e

import "testing"

// Aceitação: scenario backend_only passa.
func TestBackendOnly(t *testing.T) {
	s := StandardScenarios[0]
	r := CheckScenario(s)
	if !r.Passed {
		t.Errorf("backend_only: %+v", r)
	}
}

// Aceitação: scenario frontend_only passa.
func TestFrontendOnly(t *testing.T) {
	s := StandardScenarios[1]
	r := CheckScenario(s)
	if !r.Passed {
		t.Errorf("frontend_only: %+v", r)
	}
}

// Aceitação: scenario fullstack passa.
func TestFullstack(t *testing.T) {
	s := StandardScenarios[2]
	r := CheckScenario(s)
	if !r.Passed {
		t.Errorf("fullstack: %+v", r)
	}
}

// Aceitação: scenario minimal → INCOMPLETE gate mas sem PDF.
func TestMinimalIncomplete(t *testing.T) {
	s := StandardScenarios[3]
	r := CheckScenario(s)
	// minimal: gate=INCOMPLETE ok, mas PDF inválido (sem conteúdo).
	if r.Passed {
		t.Errorf("minimal deve falhar pdf: %+v", r)
	}
	if !r.GateOK {
		t.Errorf("gate: %+v", r)
	}
	if s.ExpectedGate != "INCOMPLETE" {
		t.Errorf("expected incomplete")
	}
}

// Aceitação: scenario custom mismatch.
func TestMismatch(t *testing.T) {
	s := Scenario{
		Name: "bad", HasBackend: true,
		ExpectedGate: "FAIL", // não bate com has_backend
		ExpectedLighthse: true, // não bate
	}
	r := CheckScenario(s)
	if r.Passed {
		t.Errorf("expected fail: %+v", r)
	}
	if len(r.Reasons) == 0 {
		t.Errorf("reasons vazio")
	}
}

// Aceitação: VerifyBackendOnly PDF válido.
func TestVerifyBackendOnlyOK(t *testing.T) {
	pdf := []byte("%PDF-1.4\n...content...")
	json := `{"run":{"id":"r1"}}`
	out, err := VerifyBackendOnly(pdf, json)
	if err != nil {
		t.Errorf("verify: %v", err)
	}
	if !out.LighthouseNA || !out.BackendActive || !out.PDFValidMagic || !out.JSONParses {
		t.Errorf("flags: %+v", out)
	}
}

// Aceitação: VerifyBackendOnly PDF inválido.
func TestVerifyBackendOnlyBadPDF(t *testing.T) {
	pdf := []byte("not a pdf")
	_, err := VerifyBackendOnly(pdf, "{}")
	if err == nil {
		t.Errorf("expected pdf error")
	}
}

// Aceitação: VerifyBackendOnly JSON vazio.
func TestVerifyBackendOnlyEmptyJSON(t *testing.T) {
	pdf := []byte("%PDF-1.4\n")
	_, err := VerifyBackendOnly(pdf, "")
	if err == nil {
		t.Errorf("expected json error")
	}
}

// Aceitação: VerifyFrontendOnly OK.
func TestVerifyFrontendOnlyOK(t *testing.T) {
	pdf := []byte("%PDF-1.4\n...")
	json := `{"lighthouse":{"performance":90}}`
	out, err := VerifyFrontendOnly(pdf, json)
	if err != nil {
		t.Errorf("verify: %v", err)
	}
	if !out.LighthouseActive || !out.BackendNA || !out.HasPerformance {
		t.Errorf("flags: %+v", out)
	}
}

// Aceitação: VerifyFrontendOnly bad PDF.
func TestVerifyFrontendOnlyBadPDF(t *testing.T) {
	if _, err := VerifyFrontendOnly([]byte("nope"), "{}"); err == nil {
		t.Errorf("expected error")
	}
}

// Aceitação: VerifyFrontendOnly sem perf.
func TestVerifyFrontendOnlyNoPerf(t *testing.T) {
	pdf := []byte("%PDF-1.4\n")
	out, err := VerifyFrontendOnly(pdf, "{}")
	if err != nil {
		t.Errorf("verify: %v", err)
	}
	if out.HasPerformance {
		t.Errorf("no perf esperado")
	}
}

// Aceitação: containsCI helper.
func TestContainsCI(t *testing.T) {
	if !containsCI("HELLO", "hello") {
		t.Errorf("ci")
	}
	if !containsCI("hello world", "WORLD") {
		t.Errorf("ci 2")
	}
	if containsCI("abc", "xyz") {
		t.Errorf("no match")
	}
	if !containsCI("anything", "") {
		t.Errorf("empty needle")
	}
}

// Aceitação: VerifyContractualPeer OK.
func TestVerifyContractualOK(t *testing.T) {
	in := PeersInput{Peers: 3, Arbiters: 1, RobustnessStatus: "stable", SwapApplied: true}
	out, err := VerifyContractualPeer(in)
	if err != nil {
		t.Errorf("verify: %v", err)
	}
	if out.PeersCount != 3 || out.Robustness != "stable" {
		t.Errorf("flags: %+v", out)
	}
}

// Aceitação: < 2 peers falha.
func TestVerifyContractualFewPeers(t *testing.T) {
	in := PeersInput{Peers: 1, Arbiters: 1}
	if _, err := VerifyContractualPeer(in); err == nil {
		t.Errorf("expected error")
	}
}

// Aceitação: arbiter != 1 falha.
func TestVerifyContractualBadArbiter(t *testing.T) {
	in := PeersInput{Peers: 2, Arbiters: 2}
	if _, err := VerifyContractualPeer(in); err == nil {
		t.Errorf("expected error")
	}
}
