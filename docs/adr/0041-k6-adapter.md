# ADR 0041 — k6 adapter

Status: Aceito. 2026-08-20.

## Contexto

SAI-047: k6 load test adapter. Prioriza script user-provided;
fallback smoke script auto-gerado a partir de endpoints (SAI-046).
Sem script + sem endpoints = skip (nunca falha release).

## Decisão

`Mode` (container/binary/disabled), `ScriptSource` (existing/auto),
`Config{ScriptPath, ThresholdP95/P99, MaxErrorRate, VUs, Duration,
AllowProd}`.

`ShouldRun()` — Mode=Disabled OR ScriptPath sem file no disk
→ false. Caller decide skip.

`RenderSmokeScript(target, eps, cfg)` gera script k6 mínimo:
vus/duration/thresholds + http.get(BASE+endpoint) por entry.
Thresholds embutidos no script (`p(95)<X p(99)<Y rate<Z`) — k6
avalia sozinho durante execução; failures vêm via `threshold` no
summary.

`LoadScript(path)` lê file. `ResolveScript(target, scriptPath,
eps, cfg)` ordem: user-provided → auto-gen → skip (nil).

`Summary{P50/P90/P95/P99, ErrorRate, Requests, Throughput,
ThresholdFailed}` parseado de `--summary-export` JSON. Tags JSON
`singular "threshold"` (k6 emite singular dentro de metric, não
plural).

`PassThresholds(cfg)` — P95 > threshold OR P99 > threshold OR
ErrorRate > Max OR ThresholdFailed len > 0 → false.

`RunK6(ctx, rc)` retorna command string (não executa). Caller
roda via exec. Mode=binary → `k6 run --summary-export=X`; Mode=
container → `docker run --rm -i grafana/k6 ...`.

`PrioritizeEndpoints(all, added, baseWeight, addedWeight)` —
default 1.0/2.0; endpoints em `DiffSet.Added` (novos) ganham
peso 2x.

## Consequências

- 25 testes (DefaultConfig, ShouldRun casos, RenderSmokeScript
  básico/empty-target/empty-eps/weight, LoadScript missing/empty,
  ResolveScript existing/auto-gen/skip, ParseK6SummaryJSON
  basic/invalid/threshold-fail, PassThresholds threshold-fail,
  RunK6 binary/container/no-script/no-target/invalid-mode,
  PrioritizeEndpoints + defaults, msToDuration, mode constants).
- Threshold failures do k6 são parseados como `ThresholdFailed`,
  preservados na Summary — gate unificado bloqueia se k6
  reporta failure independente de summary values.

## Trade-offs

- `RunK6` retorna command string, não executa — caller integra
  com executor externo (shell/subprocess). Mantém k6 adapter
  test-friendly sem precisar de k6 instalado.
- RenderSmokeScript gera `http.get` apenas — POST/PUT/DELETE em
  endpoints mutantes exigiria script user-provided ou expansão
  futura.
- `p(95)<500` no script é hard-coded em ms — sem unit (k6 não
  aceita "500ms" inline). Aceitável: cfg sempre converte.