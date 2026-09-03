# ADR 0056 — Capability Probe

Status: Aceito. 2026-08-20.

## Contexto

SAI-062: probe de capabilities observáveis por modelo. JSON
compliance + latency + availability. Cache. Spec é explícita:
**não ranquear inteligência por nome**.

## Decisão

`ProbeResult{Model, Provider, JSONCompliant, LatencyMS, Available,
ErrorMessage, ProbedAt, ResponseSize}` em `internal/ai/probe.go`.

`ProbeSummary{Model, Provider, JSONCompliant, Available,
AvgLatencyMS, P95LatencyMS, SampleCount, SuccessCount}`.

`ProbeCache` in-memory (map[model][]ProbeResult) thread-safe.

`Probe(ctx, p, opts)`:
- valida p/model
- system prompt força JSON
- `DefaultProbePrompt = "Respond with valid JSON of the form
  {\"ok\":true,\"answer\":\"yes\"} and nothing else."`
- timeout 30s default
- timer wall-clock → latency
- error → `Available:false, ErrorMessage`
- sucesso → `StripCodeFences(content)` + `IsLikelyJSON` → JSONCompliant

`ProbeMany(ctx, p, opts, samples)` executa N paralelo (default 3).

`Summarize(results)` agrega: SampleCount, SuccessCount,
Available, JSONCompliant (OR), Avg/P95 latency via percentile.

`RankByCapability(summaries)`:
- score = Available(+1000) + JSONCompliant(+500) + latency-bonus(cap 500)
- ordena desc por score
- **não** ranqueia por "inteligência" / nome / brand

Helpers:
- `IsLikelyJSON(s)` — strip fences + parse
- `IsLikelyJSONObject(s)` — só objetos
- `percentile(values, p)` — insertion sort
- `sortInt64` — ordenação in-place

## Consequências

- 18 testes (json likely ok/fail/object, percentile p0/50/100/
  nil, cache put/get/missing/reset/all, probe ok/missing/nil
  provider/empty model, probe many default, summarize empty,
  rank by capability, default prompt, latency, ctx cancel,
  provider override, custom timeout, json compliance).
- Critérios de ranking auditáveis — score é aditivo, sem ML,
  sem peso subjetivo.
- Cache thread-safe via mutex — múltiplas goroutines podem
  probe paralelas sem race.

## Trade-offs

- Latência medida em 1 sample pode ser ruidosa — `ProbeMany`
  mitiga com N samples + P95.
- `IsLikelyJSON` aceita JSON parcial (objeto/array) — modelos
  que adicionam prosa antes/depois do JSON passam. Spec não
  exige strict mode.
- Score simples aditivo — não captura trade-offs (latência vs
  custo, etc). ADR seguinte pode adicionar weighting.
- Probe usa `CompleteJSON` real — provider pode rate-limit.
  ADR seguinte pode adicionar backoff.
- `DefaultProbePrompt` em inglês — bias cultural possível.
  Caller pode passar `PromptJSON` custom.
