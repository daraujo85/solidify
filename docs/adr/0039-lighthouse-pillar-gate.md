# ADR 0039 — Lighthouse pillar gate

Status: Aceito. 2026-08-20.

## Contexto

SAI-045: score pillar frontend precisa virar gate. SEO
informativo default (weight=0) — não bloqueia release. Pilares
chave (perf/a11y/bp) precisam ≥ 0.9 conservative.

## Decisão

`PillarThreshold` carrega min por pilar + aggregate. Default =
conservador (perf/a11y/bp ≥ 0.9, SEO=0, aggregate ≥ 0.8).
Lenient = 0.5 em tudo (dev/test).

`EvaluatePillarGate(rep, th)`:
- nil report → Skipped+Allow (não bloqueia)
- Mode=Disabled → Skipped+Allow (backend-only)
- Senão: checa cada pilar cuja `Min* > 0` E `Pillar.* > 0` E
  `Pillar.* < Min*` → failed. Aggregate falha se `MinAggregate > 0`
  E `AggregateScore < MinAggregate`.

Pattern `MinPillar > 0 && Pillar > 0 && Pillar < MinPillar` —
threshold=0 desabilita checagem, Pillar=0 (não mediu) também.

`PillarGateResult{Allow, Reason, Failed, Skipped, SkippedReason}`.
`GateDefault(rep)` / `GateLenient(rep)` helpers.

`WorstPillar(p)` acha pilar com menor score (não-zero).
`AggregatePercent(rep)` helper 0-1 → 0-100.

`joinStrings` local helper (evita import "strings" conflito).

## Consequências

- 16 testes (default/lent thresholds, perfect/fail/lenient/disabled/
  nil/seo-informative/seo-strict gates, helpers, WorstPillar
  cases, AggregatePercent, multi-fail, joinStrings).
- Gate padrão permite release sem frontend (Skipped + Allow).
- Frontend strict opt-in via `MinSEO > 0`.

## Trade-offs

- Aggregate não é derivado dos pilares checados — pode dar
  Allow=true mesmo se aggregate < MinAggregate mas pilares
  individuais OK. Caller deve sempre olhar Aggregate.
- `Failed` é lista — caller pode mapear pra human-readable
  reasons por pilar.
- `WorstPillar` retorna zero se tudo zero — caller trata como
  "no data".