# ADR 0079 — Benchmark suite realista (SAI-096)

Status: Aceito. 2026-08-20.

## Contexto

SAI-096: medir custo de hot paths com fixtures
realistas — 1k files, 10MB diff, monorepo (10k files).

## Decisão

`internal/bench/`:

- `bench.go` (fixtures):
  - Constantes: `SmallFiles=100`, `MediumFiles=1000`,
    `LargeFiles=5000`, `Monorepo=10000`,
    `Diff10MB=10MB`.
  - `StandardProfiles{1k_files, 10MB_diff, monorepo}`.
  - `GeneratePillarInput(n, seed)` — N pillars
    randômicos.
  - `GenerateSOLIDSnapshot(n, seed)` — letters p/
    cada princípio.
  - `GenerateFindingsLite(n, seed)`.
  - `GenerateRunSnapshot(id, seed)`.
  - `GenerateDiffBytes(n, seed)`.
- `bench_test.go` (alvos quentes):
  - `BenchmarkPillarScore_1k` / `_10k`.
  - `BenchmarkRobustnessCompare_100`.
  - `BenchmarkFindingsOverlap_500`.
  - `BenchmarkGenerateDiff_10MB`.

## Consequências

- Benchmarks rodam em linux/arm64:
  - PillarScore_1k: ~9µs/op.
  - PillarScore_10k: ~66µs/op.
  - RobustnessCompare_100: ~20µs/op.
  - FindingsOverlap_500: ~98µs/op.
  - GenerateDiff_10MB: ~47ms/op.
- Hot paths: `ComputeGlobalScore` (pillar),
  `Compare` (robustness), `findingsOverlap` (overlap).
- `peer.FindingsOverlapPublic` wrapper p/ expor
  helper interno p/ benchmarks sem aumentar API
  pública.

## Trade-offs

- Benchmarks não persistem histórico — execução
  pontual. Trend tracking fica p/ integração com
  benchstat (futuro).
- Sem `b.ReportAllocs()` por padrão — caller ativa
  se quiser. Trade-off: zero noise default.
- `Generate*` funções são determinísticas (seed fixo)
  — reprodutibilidade > randomness real.
