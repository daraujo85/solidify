# ADR 0094 — E2E hard gate (SAI-111)

Status: Aceito. 2026-08-20.

## Contexto

SAI-111: hard gate non-overridable. Score abaixo do
threshold OU critical finding → FAIL obrigatório.

## Decisão

`internal/hardgate/`:

- `Severity`: low, medium, high, critical.
- `Finding{ID, Severity}`.
- `GateInput{Score, Threshold, Findings}` +
  `GateResult{Status, ScoreOK, NoCritical, HardFail,
  HardFailReas}`.
- `Evaluate(in)`:
  - `ScoreOK` = score ≥ threshold.
  - `NoCritical` = nenhum finding critical.
  - `HardFail` = !ScoreOK || !NoCritical.
  - Status: PASS se !HardFail, senão FAIL.
- `MustPass(in)` retorna `ErrHardGateBlocked`.
- `OverrideAttempt{Actor, Reason, Blocked}` +
  `DetectOverride` — sempre bloqueia (hard gate).

## Consequências

- 8 testes: pass, low score, critical, both fail,
  non-critical OK, MustPass ok/fail, DetectOverride.
- High severity não bloqueia — só critical.
- Override sempre bloqueado — sem backdoor.

## Trade-offs

- Threshold passado via input — sem default global.
  Trade-off: flexibility > convention.
- `OverrideAttempt` é log/detect — sem ação real
  (não há como "desbloquear"). Trade-off: log >
  silent block.
