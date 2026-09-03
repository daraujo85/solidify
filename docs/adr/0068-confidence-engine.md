# ADR 0068 — Confidence engine

Status: Aceito. 2026-08-20.

## Contexto

SAI-074: agregar signals (evidence completeness, peer
agreement, independence, analyzer completeness, arbiter
confidence) em score de confiança 0..100 + level.

## Decisão

`internal/confidence/engine.go`:

- 5 signals: evidence_completeness (0.30), peer_agreement
  (0.25), independence (0.20), analyzer_completeness (0.15),
  arbiter_confidence (0.10).
- Pesos somam 1.0.
- `SignalScore{Signal, Score, Weight, Notes}`.
- `ConfidenceInput{RunID, Signals, Weights}`.
- `ConfidenceResult{RunID, Signals, Score, Level, WeightedSum,
  WeightSum, Missing}`.

`Compute(in)`:
- valida RunID
- valida score [0,100] em cada signal
- agrega weighted average
- signals sem weight → Missing
- sem signals → score 0

`computeLevel`: <50 low, <75 medium, ≥75 high.

`IsHighConfidence()` helper.

## Consequências

- 14 testes: compute basic/empty-runid/out-of-range/missing-
  weight/no-signals, level low/medium/high, is high conf,
  render, default weights sum, custom weights, helpers,
  negative score, score max.
- Estrutura paralela ao Pillar engine (SAI-072) — mesma
  forma (signals, weights, weighted avg, level).
- Score out-of-range falha rápido — caller não recebe lixo.

## Trade-offs

- Pesos opinionados — evidence domina (0.30) seguido de
  agreement. ADR seguinte pode ler de config.
- Sem renormalização explícita quando 1 signal missing — score
  é média dos pesos presentes (não dos 5 originais). Trade-off:
  simplicidade > estabilidade absoluta.
- Level bins hardcoded (50/75) — caller pode querer custom
  thresholds.
- Confidence é ortogonal a Risk (SAI-073) — pode haver release
  com risk high mas confidence high (decisão clara mesmo que
  arriscada). ADR seguinte pode combinar os dois.
