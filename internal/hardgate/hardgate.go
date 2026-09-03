// E2E hard gate (SAI-111).
//
// Hard gate é non-overridable: score abaixo do threshold
// OU critical finding → FAIL obrigatório, sem WARN.
package hardgate

import "errors"

// Severity severidade do finding.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Finding um finding.
type Finding struct {
	ID       string
	Severity Severity
}

// GateInput entrada.
type GateInput struct {
	Score     float64 // 0..100
	Threshold float64 // mínimo p/ PASS
	Findings  []Finding
}

// GateResult saída.
type GateResult struct {
	Status       string // PASS|FAIL
	ScoreOK      bool
	NoCritical   bool
	HardFail     bool
	HardFailReas []string
}

// Evaluate hard gate.
//
// Regras:
// 1. Score >= Threshold.
// 2. Sem finding de severity=critical.
//
// Falha em qualquer = FAIL + HardFail=true.
func Evaluate(in GateInput) GateResult {
	r := GateResult{}
	if in.Score >= in.Threshold {
		r.ScoreOK = true
	} else {
		r.HardFailReas = append(r.HardFailReas,
			"score abaixo do threshold")
	}
	r.NoCritical = true
	for _, f := range in.Findings {
		if f.Severity == SeverityCritical {
			r.NoCritical = false
			r.HardFailReas = append(r.HardFailReas,
				"critical finding: "+f.ID)
		}
	}
	r.HardFail = !r.ScoreOK || !r.NoCritical
	if r.HardFail {
		r.Status = "FAIL"
	} else {
		r.Status = "PASS"
	}
	return r
}

// ErrHardGateBlocked erro quando hard gate bloqueia.
var ErrHardGateBlocked = errors.New("hardgate: release bloqueado")

// MustPass valida e retorna erro se bloqueado.
func MustPass(in GateInput) error {
	r := Evaluate(in)
	if r.HardFail {
		return ErrHardGateBlocked
	}
	return nil
}

// OverrideAttempt marcador — log/detect tentativas
// de bypass.
type OverrideAttempt struct {
	Actor   string
	Reason  string
	Blocked bool
}

// DetectOverride checa se tentativa é válida.
//
// Hard gate não permite override. Sempre bloqueia.
func DetectOverride(att OverrideAttempt) OverrideAttempt {
	att.Blocked = true
	return att
}
