# ADR 0065 — SOLID Score engine

Status: Aceito. 2026-08-20.

## Contexto

SAI-071: média das letras aplicáveis; before/after/delta;
N/A correto (letras não-aplicáveis excluídas do cálculo).

## Decisão

`internal/score/solid.go`:

- `Principles` const canônico (ordem S→O→L→I→D).
- `LetterScore{Principle, Score, Applicable, Reasoning}`.
- `Snapshot{RunID, Label, Letters}`.
- `Delta{RunID, BeforeScore, AfterScore, ScoreDelta, Improved,
  Regressed, Unchanged, ApplicableBefore, ApplicableAfter,
  Notes}`.

`ComputeScore(letters)`:
- soma só letras com `Applicable=true`
- ignora scores negativos
- se count=0 → 0 (sem divisão por zero)

`ComputeDelta(before, after)`:
- valida RunID match
- deriva `Improved`/`Regressed`/`Unchanged` por sinal do delta
- conta applicable before/after

`ValidateLetters` checa: principle não vazio, sem dup, score
em [0,100].

`CanonicalLetters` ordena por Principles.

`LetterByPrinciple` lookup.

`IsPrincipleValid` membership check.

`RenderSnapshot`/`RenderDelta` textual (✓/N/A, ↑↓=).

## Consequências

- 19 testes: compute basic/NA/all-NA/empty, delta improved/
  regressed/unchanged/diff-runid/empty-runid, validate OK/
  empty/dup/out-of-range/negative, canonical order, letter
  lookup, is valid, render snapshot/delta, ftoa1, itoa2,
  count applicable, applicable counts delta.
- N/A letters são **excluídas** — princípio não conta na média.
- Sem divisão por zero — `count=0` retorna 0 explícito.
- CanonicalLetters idempotente — caller pode aplicar sempre.
- ScoreDelta signed (after - before) — Improved se > 0,
  Regressed se < 0.

## Trade-offs

- Score negativo ignorado silenciosamente — caller pode ter
  bug e não detectar. ADR seguinte pode validar score >= 0
  em ValidateLetters.
- `ApplicableBefore`/`ApplicableAfter` contam todas as
  aplicáveis (não as que mudaram) — útil pra "quantas letras
  foram consideradas" no relatório.
- Delta em float — comparação com zero usa `>` / `<` exatos.
  Em scores iguais via float ops pode dar Unchanged mesmo
  com pequenas diferenças. ADR seguinte pode usar epsilon.
- Render textual sem cores — terminal-friendly.
