# ADR 0042 — Load result normalization

Status: Aceito. 2026-08-20.

## Contexto

SAI-048: normalizar resultados k6 (SAI-047) em estrutura
transversal `LoadResult` com fingerprint de configuração
(script+target+vus+duration) para SAI-049 baseline comparator.

## Decisão

`LoadResult{script, target, VUs, duration, P50/P90/P95/P99,
error_rate, requests, throughput, fingerprint, timestamp,
threshold_ok, threshold_failed}`.

`Normalize(in NormalizeInput)` rejeita target vazio.

`ComputeFingerprint(script, target, vus, dur)` →
sha256(JSON{...}) — NÃO inclui métricas; identifica "tipo
de run" (config/setup), não valores.

`CompareToBaseline(cur, base, th)`:
- fingerprints iguais (otherwise error)
- baseline P50/P95/P99 > 0 (otherwise error)
- percentChange por métrica
- Regression = true se P95>th.MaxP95Increase OR P99... OR
  err > MaxErrIncrease OR throughput < -MaxDrop.

`DefaultRegressionThreshold` P95+20%, P99+30%, err+2%,
throughput-15%.

`compatibleWith`, `EqualMetadata`, `Summary`,
`SortByP95`, `AggregateStats` (median P95), `String`,
`MarshalJSON2` (timestamp_iso RFC3339), `IsCompatibleWith`.

`percentChange[T ~int64|~float64]` generic — funciona com
`time.Duration` (int64 base) e float64.

## Consequências

- 22 testes (Normalize basic/no-target, ComputeFingerprint
  stable/sensitive, CompatibleWith, CompareToBaseline
  no-regression/P95-regression/err-regression/throughput-drop
  /incompatible/no-metrics, percentChange, DefaultThreshold,
  EqualMetadata, Summary, SortByP95, AggregateStats,
  AggregateStatsEmpty, String, IsCompatibleWith, Timestamp,
  ThresholdFail, MarshalJSON2).
- Errrorf `%%` escapes — Go vet rejeita `%` órfão antes de
  espaço/número.
- Fixtures CompareToBaseline exigem P50 não-zero (anchor
  percentChange via baseline.P50).

## Trade-offs

- `MarshalJSON2` (não `MarshalJSON`) — convenção paralela
  pra override explícita; `timestamp_iso` adicional em UTC
  preservado separado.
- `IsCompatibleWith` alias de `CompatibleWith` — caller
  prefere nome semântico; ambos disponíveis.
- AggregateStats retorna `*Delta` com P95Delta = median em
  ms (raw); throughput/etc ainda não usados. Aceitável:
  campo semantic ambiguity, documented.
