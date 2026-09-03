# ADR 0083 — Cache verification (SAI-100)

Status: Aceito. 2026-08-20.

## Contexto

SAI-100: re-run idêntico reaproveita analyzer
results; mudança de tool/config invalida. Cache já
existe em `internal/cache`. Falta verificação
explícita do comportamento.

## Decisão

`internal/cacheverify/`:

- `Scenario{Name, Key cache.Key, ExpectHit,
  MutationReason}`.
- `Result{Scenario, Hit, KeyHash, CacheHits, Misses,
  Passed, Reason}`.
- `Verify(sc, prevHash)` — compara hash novo vs
  prev; passa se hit==ExpectHit.
- `VerifyPair{RunA, RunB, ExpectSameHash,
  Description}` + `PairResult`.
- `VerifyHashPair(p)` — hashA == hashB ==
  ExpectSameHash.
- Cenários prontos:
  - `SameRunScenario` — tudo igual → hit.
  - `ChangedToolScenario` — analyzer name mudou →
    miss.
  - `ChangedConfigScenario` — ConfigHash mudou →
    miss.
  - `ChangedInputScenario` — InputHash mudou → miss.
  - `ChangedBaseScenario` — Base SHA mudou → miss.
- `Aggregate{Total, Passed, Failed, Hits, Misses}` +
  `AggregateResults(rs)`.

## Consequências

- 10 testes: verify hit/miss-empty/miss-different,
  same/scenarios (4 mutations), pair same/different,
  aggregate.
- Testes independentes de filesystem — operam só
  sobre cache.Key.Hash().
- MutationReason é descritiva — log/dashboard
  mostra por que cache miss aconteceu.

## Trade-offs

- Sem storage real (FS/disk) — testes são puros.
  Integração com cache real fica p/ E2E (SAI-110+).
- Sem TTL test — `cache` package já tem TTL,
  verify só checa hit/miss por hash. Trade-off:
  simplicidade > cobertura total.
- `Aggregate` é simples — sem percentuais. Trade-off:
  zero dep > métricas.
