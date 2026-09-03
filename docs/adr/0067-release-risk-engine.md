# ADR 0067 — Release Risk engine

Status: Aceito. 2026-08-20.

## Contexto

SAI-073: agregar risk factors (migration/env/breaking/
security/blast_radius/review_disagreement) em score 0..100 +
nível.

## Decisão

`internal/risk/engine.go`:

- 6 kinds: migration, env, breaking, security, blast_radius,
  review_disagreement.
- Weights: security(20), breaking(15), blast_radius(12),
  migration(10), review_disagreement(10), env(8).
- Severity rank: low=1, medium=2, high=3, critical=4.
- `RiskFactor{Kind, Description, Severity, Evidence, Symbol}`.
- `RiskInput{RunID, Factors}`.
- `RiskScore{RunID, Factors, RawScore, Normalized, Level,
  TopFactors, Breakdown, Notes}`.

`Compute(in)`:
- soma `weight * severity_rank` por fator
- satura `raw` em `MaxRawScore=100`
- `normalized = raw * 100 / MaxRawScore` (idêntico a raw aqui)
- `level`: <25 low, <50 medium, <75 high, ≥75 critical

`IsShippable()`: low/medium → true; high/critical → false.

`topFactors(n=5)` ordena por contribuição desc.

`BreakdownByKind(kind)` accessor.

Unknown kind → peso default 5. Unknown severity → rank 1.

## Consequências

- 16 testes: compute basic/empty-runid/saturate, level low/
  medium/high/critical, breakdown por kind, top ordenação/limit,
  validate kind, unknown kind/severity, render, IsShippable,
  weights defined, helpers numéricos, severity rank, empty
  factors.
- Saturação em 100 evita runaway scoring quando muitos
  fatores convergem.
- Unknown kind tratado (não falha) — caller pode ter kinds
  custom sem modificar o engine.
- Breakdown por kind sempre disponível pra report.

## Trade-offs

- Pesos hardcoded — política fixa. ADR seguinte pode ler de
  config.
- Security > Breaking > Blast > Migration/Disagree > Env —
  opinionated, reflete risco de produção.
- `normalized == raw` aqui (mesma escala). Mantido pra
  futuras expansões onde pesos mudem.
- `IsShippable` é heurística — caller pode override com
  política custom (ex: zero security risk sempre).
- Unknown severity vira low (rank 1) — não é erro. Trade-off:
  caller's typo silencioso. ADR seguinte pode validar
  severity no Compute.
- Top factors limita a 5 — caller não vê além. ADR seguinte
  pode parametrizar.
