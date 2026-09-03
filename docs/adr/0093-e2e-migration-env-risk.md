# ADR 0093 — E2E migration/env risk (SAI-110)

Status: Aceito. 2026-08-20.

## Contexto

SAI-110: detectar mudanças em migrations e env vars;
calcular risk score; bloquear release se > threshold.

## Decisão

`internal/migenv/`:

- `Change{Kind, Path, Added, Removed, Modified,
  RiskLevel}`.
- `ChangeKind`: migration, env, config.
- `RiskWeights`:
  - migration: low=5, medium=15, high=30.
  - env: low=3, medium=10, high=25.
  - config: low=1, medium=5, high=10.
- `RiskScore{Total, Breaks, Reasons}` +
  `CalculateRisk(changes, threshold)` (default 50).
- `MigrationCheck{HasUp, HasDown, HasBaseline}` +
  `ValidateMigration` (todos obrigatórios).
- `EnvCheck{Added, Removed, HasDefaults, HasSecret}`
  + `ValidateEnv` (warnings).

## Consequências

- 10 testes: risk empty/default/below, migration
  ok/no-up/no-down/no-baseline, env ok/warnings.
- Migration tem 3 obrigatórios (up/down/baseline) —
  release sem um falha validação.
- Risk threshold = 50. Migration high (30) +
  env high (25) = 55 → bloqueia.

## Trade-offs

- Threshold hard-coded (50) — tune via config
  fica p/ ADR futuro.
- `RiskWeights` é tabela — sem pesos por projeto.
  Trade-off: simplicidade > customização.
- Env warnings são binários (present/absent) —
  sem severity grading.
