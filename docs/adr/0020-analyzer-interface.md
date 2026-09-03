# ADR 0020 — Interface Analyzer

Status: Aceito. 2026-08-20.

## Contexto

SAI-026: cada analyzer (lighthouse, sonar, security, tests, load) tem
custo, tempo e detecção diferentes. Precisa interface comum + result
normalizado pra scheduler (SAI-027) poder paralelizar e pra evidência
(SAI-023) agregar.

## Decisão

Interface `Analyzer`:

```go
type Analyzer interface {
    Name() string
    Version() string
    Class() Class
    Detect(ctx context.Context, c Context) (bool, error)
    Run(ctx context.Context, c Context) (Result, error)
}
```

- **Class**: `light` | `cpu` | `browser` | `memory-heavy` |
  `active-network`. Scheduler consome pra definir concurrency.
- **Status**: `ok` | `warn` | `error` | `skip`.
- **Severity**: `info` | `low` | `medium` | `high` | `critical`.
- **Result**: name, version, class, status, findings, metrics,
  started/finished/duration, error. `Detected` flag marca se Detect
  retornou true.

`Registry` thread-safe: Register/Get/All/Names/ByClass. `DetectAll`
roda Detect em todos e devolve slice de detectados. `Run` preenche
metadata padrão se analyzer omitiu.

`Result.Summary()` conta findings por severity. `HighestSeverity()`
devolve a maior severity presente.

## Consequências

- 21 testes (registry, run, detect, summary, class).
- Dependency: zero — só stdlib. Sem acoplamento a outros internal
  packages. Outros packages (scheduler, evidence) consomem via
  interface.
- Field `Detected` é setado tanto em `res` (path detect=skip/error)
  quanto em `out` (path success) — corrigido após test run.
- Ordenação determinística (sort by name) garante reproducibility.
