# ADR 0077 — Robustness comparator (SAI-094)

Status: Aceito. 2026-08-20.

## Contexto

SAI-094: comparar 2 runs (orig vs swap) e extrair
métricas que provam robustez do pipeline frente a
variação de papéis.

## Decisão

`internal/peer/robustness.go`:

- `RunSnapshot{RunID, Score, Grade, GateStatus,
  Pillars, SOLID, Findings}`.
- `RobustFindingLite{ID, Symbol, Severity, Note}`
  (renomeada de `FindingLite` p/ evitar colisão com
  `divergence.go`).
- `RobustnessResult` com 9 campos:
  QualityDelta, LetterDeltas (S/O/L/I/D),
  OnlyOriginal, OnlySwapped, Both, Overlap (jaccard),
  GateStable, Status, Notes.
- `Compare(orig, swapped) (*RobustnessResult, error)`.
  Erro: `ErrEmptySnapshot` se orig/swapped nil.
- `letterDeltas(a, b)` — usa `score.Principles` loop
  + `score.LetterByPrinciple(snap.Letters, p)` →
  `vb.Score - va.Score`.
- `findingsOverlap(a, b)` — sets + diff (both/onlyA/
  onlyB), sort determinístico.
- Jaccard: `|both| / (|onlyA| + |onlyB| + |both|)`.
- `computeRobustnessStatus`:
  - `unstable`: gate unstable OR |Δ| ≥ 5
  - `stable`: gate stable + |Δ| < 2
  - `mostly-stable`: demais
- `RenderResult` textual (debug/log).
- `IsStable()` nil-safe.

## Consequências

- 14 testes: básico, nil, gate unstable, big delta,
  medium delta, tiny delta, overlap ratio (3/5=0.6),
  overlap vazio, overlap 100%, negative delta,
  IsStable (nil/true/false), RenderResult (incl nil),
  findingsOverlap, helpers numéricos.
- `RobustFindingLite` separado de `FindingLite`
  (divergence.go) — diferentes propósitos:
  comparator (lite) vs divergence (full).
- Thresholds hard-coded (2/5). Tune via config fica
  p/ ADR futuro se necessário.

## Trade-offs

- Thresholds hard-coded (2/5) — sem tuning runtime.
  Trade-off: zero-config > flex.
- `ftoaR`/`itoaR`/`padLeftR` reimplementados — sem
  `strconv`/`fmt` no comparador p/ zero import noise.
  Trade-off: reuso de helpers já existentes em outros
  packages ficaria melhor.
- Letter deltas usam `Score` (numérico), não `After`
  (campo inexistente em `LetterScore`).
