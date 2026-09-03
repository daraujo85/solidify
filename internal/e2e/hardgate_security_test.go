// E2E hard gate security — SAI-111 acceptance.
package e2e

import (
	"errors"
	"testing"

	"github.com/diegoaraujo/solidify/internal/hardgate"
)

// Aceitação: finding de segurança critical força FAIL mesmo com score
// alto — hard gate non-overridable.
func TestE2E_SecurityCriticalForcesHardFail(t *testing.T) {
	in := hardgate.GateInput{
		Score: 95, Threshold: 60,
		Findings: []hardgate.Finding{
			{ID: "SEC-001", Severity: hardgate.SeverityHigh},
			{ID: "SEC-002", Severity: hardgate.SeverityCritical}, // ex: secret exposto / SQLi
		},
	}
	r := hardgate.Evaluate(in)
	if r.Status != "FAIL" || !r.HardFail {
		t.Fatalf("esperado FAIL: %+v", r)
	}
	if r.NoCritical {
		t.Errorf("NoCritical deveria ser false")
	}
	if err := hardgate.MustPass(in); !errors.Is(err, hardgate.ErrHardGateBlocked) {
		t.Errorf("MustPass deveria bloquear: %v", err)
	}
}

// Aceitação: tentativa de override do hard gate é sempre bloqueada, sem
// exceção mesmo com actor/reason justificados.
func TestE2E_SecurityHardGateOverrideAlwaysBlocked(t *testing.T) {
	att := hardgate.OverrideAttempt{Actor: "release-manager", Reason: "hotfix urgente"}
	out := hardgate.DetectOverride(att)
	if !out.Blocked {
		t.Fatal("override nunca deveria passar (hard gate non-overridable)")
	}
}

// Aceitação: score abaixo do threshold sozinho (sem critical) também é
// hard fail — duas causas independentes de bloqueio.
func TestE2E_SecurityLowScoreAloneHardFails(t *testing.T) {
	r := hardgate.Evaluate(hardgate.GateInput{Score: 40, Threshold: 75})
	if r.Status != "FAIL" || !r.NoCritical {
		t.Fatalf("score baixo sem critical: %+v", r)
	}
}
