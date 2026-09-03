# ADR 0061 — Divergence Engine determinístico

Status: Aceito. 2026-08-20.

## Contexto

SAI-067: comparar Peer A vs Peer B de forma determinística —
score deltas, applicable mismatch, severity mismatch, finding
overlap. Output alimenta árbitro (SAI-069).

## Decisão

`DivergenceMap` em `internal/peer/divergence.go` com 4 eixos:

1. `ScoreDelta` — |avg(scores A) - avg(scores B)|.
2. `ApplicableMismatch` — princípios com applicable diferente
   entre A e B (ordenado por princípio).
3. `SeverityMismatch` — findings com mesmo ID mas severidade
   diferente.
4. `FindingOverlap` — onlyA/onlyB/both (sets).

`PeerScores{RunID, Source, Scores, Applicable, Findings}` input.
`FindingLite{ID, Symbol, Severity, Note}` shape normalizada.

`Compute(a, b)` valida RunID match, agrega contagens,
seta `ArbiterRequired = TotalDivergences > 0`.

`RequiresArbiter()` helper nil-safe.

`RenderDivergence(dm)` textual (run_id, score_delta,
applicable/severity/overlap details, arbiter required).

Helpers insertion-sort (`sortMismatches`), itoa/ftoa/padLeft2
locais (evita dep strconv).

## Consequências

- 19 testes: compute OK, RunID vazio/diff, score delta zero/
positivo, applicable mismatch, severity mismatch/missing,
overlap onlyA/onlyB/both, total aggregation, helpers nil-safe,
render com/sem arbiter, sort, helpers numéricos.
- DivergenceMap determinístico — mesma entrada produz mesmo
  output (sort estável, contagens estáveis).
- Field `ArbiterRequired` (adjetivo) + method `RequiresArbiter`
  (verbo) — convenção Go.
- Finding overlap usa sets por ID — peers sem IDs iguais não
  geram spurious overlap.
- Severity mismatch só conta findings presentes em ambos — não
  compara onlyA/onlyB (já contado em overlap).
- Sem dep strconv — helpers próprios (ftoa 2 casas, itoa signed).

## Trade-offs

- ScoreDelta usa média simples (não ponderada por princípio).
  Trade-off: simplicidade > precisão. ADR seguinte pode
  ponderar por coverage/severity.
- FindingOverlap não usa Jaccard index — só contagens. Suficiente
  para needs_arbitration; índice ficaria em métrica separada.
- `FindingLite.Symbol` reservado mas não usado em divergence
  (pertence ao SAI-070 arbitration merge).
- Sem persistência — divergence é computado on-demand a partir
  dos resultados de SAI-064/065.
- TotalDivergences = applicable + severity + onlyA + onlyB —
  finding overlap double-counts onlyA/onlyB vs severity; métrica
  bruta, útil pra trigger de arbiter, não pra SLA.
