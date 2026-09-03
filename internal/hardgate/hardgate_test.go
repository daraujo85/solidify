package hardgate

import "testing"

// Aceitação: PASS sem findings.
func TestEvaluatePass(t *testing.T) {
	r := Evaluate(GateInput{Score: 90, Threshold: 75})
	if r.Status != "PASS" || r.HardFail {
		t.Errorf("expected pass: %+v", r)
	}
}

// Aceitação: FAIL score baixo.
func TestEvaluateLowScore(t *testing.T) {
	r := Evaluate(GateInput{Score: 50, Threshold: 75})
	if r.Status != "FAIL" || !r.HardFail {
		t.Errorf("expected fail: %+v", r)
	}
	if r.ScoreOK {
		t.Errorf("score ok")
	}
}

// Aceitação: FAIL critical finding.
func TestEvaluateCritical(t *testing.T) {
	r := Evaluate(GateInput{
		Score: 90, Threshold: 75,
		Findings: []Finding{{ID: "f1", Severity: SeverityCritical}},
	})
	if r.Status != "FAIL" || !r.HardFail {
		t.Errorf("expected fail: %+v", r)
	}
	if r.NoCritical {
		t.Errorf("no critical")
	}
	if len(r.HardFailReas) == 0 {
		t.Errorf("reasons vazio")
	}
}

// Aceitação: FAIL ambos.
func TestEvaluateBothFail(t *testing.T) {
	r := Evaluate(GateInput{
		Score: 50, Threshold: 75,
		Findings: []Finding{{ID: "c", Severity: SeverityCritical}},
	})
	if !r.HardFail {
		t.Errorf("expected fail")
	}
	if len(r.HardFailReas) < 2 {
		t.Errorf("expected 2 reasons: %+v", r)
	}
}

// Aceitação: findings não-críticos não bloqueiam.
func TestEvaluateNonCritical(t *testing.T) {
	r := Evaluate(GateInput{
		Score: 90, Threshold: 75,
		Findings: []Finding{
			{ID: "a", Severity: SeverityLow},
			{ID: "b", Severity: SeverityHigh},
		},
	})
	if r.HardFail {
		t.Errorf("high não deve bloquear: %+v", r)
	}
}

// Aceitação: MustPass PASS.
func TestMustPassOK(t *testing.T) {
	if err := MustPass(GateInput{Score: 90, Threshold: 75}); err != nil {
		t.Errorf("must pass: %v", err)
	}
}

// Aceitação: MustPass FAIL.
func TestMustPassFail(t *testing.T) {
	if err := MustPass(GateInput{Score: 50, Threshold: 75}); err == nil {
		t.Errorf("expected error")
	}
}

// Aceitação: DetectOverride bloqueia.
func TestDetectOverride(t *testing.T) {
	att := DetectOverride(OverrideAttempt{Actor: "dev", Reason: "urgent"})
	if !att.Blocked {
		t.Errorf("not blocked")
	}
}
