# ADR 0066 — Pillar score engine

Status: Aceito. 2026-08-20.

## Contexto

SAI-072: agregar SOLID + Security + Performance +
Maintainability em score global com pesos.

## Decisão

`internal/score/pillar.go`:

- 4 pillars: SOLID (0.40), Security (0.25), Performance (0.20),
  Maintainability (0.15).
- `DefaultWeights` soma 1.0.
- `PillarScore{Pillar, Score, Weight, Applicable, Notes}`.
- `PillarInput{RunID, Pillars, Weights}`.
- `PillarResult{RunID, Pillars, GlobalScore, WeightedSum,
  WeightSum, Missing, Notes}`.

`ComputeGlobalScore(in)`:
- valida RunID
- valida score [0,100] em applicable
- agrega weighted sum
- pillars sem weight → Missing (não falham)
- se weightSum > 0 → global = weightedSum / weightSum
  (renormaliza automaticamente)

`NormalizeWeights(w)` força soma 1.

`GetWeight(pillar, weights)` fallback DefaultWeights.

`ValidatePillarName` membership check.

`RenderResult` textual (✓/N/A, peso entre parênteses, lista
de missing).

## Consequências

- 14 testes: compute basic/empty-runid/all-NA/missing-weight/
  custom-weights/out-of-range, validate name, normalize OK/
  zero/empty, get weight default/custom/unknown, render c/
  missing, default weights somam 1, pillars count.
- Pesos default são opinionated (SOLID domina) — caller pode
  override via Weights.
- Renormalização automática: se 1 pilar é N/A, os outros 3
  dividem o peso total proporcionalmente.
- `Missing` lista pillars aplicáveis sem weight — caller
  decide se falha ou ignora.
- Sem divisão por zero (weightSum=0 → global=0).

## Trade-offs

- Pesos default são hardcoded — política do time, não
  configurável. ADR seguinte pode ler de `~/.solidify.yaml`.
- Sem validação de pesos (negativo, soma > 1) — caller passa
  o que quiser. `NormalizeWeights` corrige soma, não
  sinal.
- Pillar order no render é o de input — caller controla
  ordenação.
- Security/Performance/Maintainability scores ainda não são
  populados por nenhuma pipeline concreta (placeholder).
  ADR seguinte pode plugar ZAP + benchmark + lint.
