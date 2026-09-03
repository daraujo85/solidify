# ADR 0063 — Arbiter executor

Status: Aceito. 2026-08-20.

## Contexto

SAI-069: executar prompt do árbitro (SAI-068) via provider
independente, modelo distinto de A/B por default contractual,
verdict estruturado (resolutions).

## Decisão

`internal/arbiter/executor.go`:

- `Resolution{Topic, Decision, Reasoning}` — decisão por
  tópico divergente.
- `Verdict{RunID, Actor, Verdict, Resolutions, Reasoning}`.
- Decision enum: `accept_peer_a|accept_peer_b|accept_both|
  reject_both`.
- `ExecutorOptions{RunID, Evidence, PeerAOutput, PeerBOutput,
  DivMap, Provider, Model, Timeout, MaxRetries}`.
- `ExecutorResult{Verdict, Prompt, Response, ParsedMap, Model,
  Provider, RetryCount, RepairCount, LatencyMS, StartedAt,
  CompletedAt, Errors, Hash, SchemaVersion}`.
- `NewExecutor` valida provider/model/RunID; default timeout
  60s; MaxRetries clamp [0, 2].
- `Execute` loop: retry → parse → repair 1x → validate.
- Validações: run_id match (parsed == opts.RunID), actor ==
  "arbiter", verdict == "resolved", resolutions ≥ 1.
- `arbiterSchema` map[string]any (Draft 7 subset).
- `repairJSON` 1x via system prompt "JSON repair assistant".
- `computeResultHash` SHA256[:8] do verdict canonical.

## Consequências

- 22 testes: new OK/no-provider/no-model/no-runid, execute OK/
  retry/exhausted/repair, run_id mismatch, actor mismatch,
  verdict != resolved, resolutions vazio, parse OK/invalid,
  build verdict OK/empty-runid/no-resolutions/overrides, hash
  determinístico/nil-safe, schema, decision consts, nil exec,
  latency populated, timeout, max retries clamp, repair propa.
- Repair 1x (não loop infinito) — budget de tokens limitado.
- Retry 0-2 (clamp em 2) — alinha SAI-065.
- Validação de run_id no Execute (não buildVerdict) — separa
  schema vs context.
- Hash canônico (run_id|verdict|topic:decision) — determinístico
  p/ audit log.
- Modelo distinto de A/B é responsabilidade do caller —
  Executor aceita qualquer ai.Provider.

## Trade-offs

- Sem distinção contractual automática de modelo — caller
  passa Provider/Model. ADR seguinte pode adicionar
  `DefaultArbiterModel` derivado de peer models.
- Repair não valida schema pós-conserto — confia no provider.
  ADR seguinte pode adicionar JSON schema validator.
- Verdict sempre "resolved" ou erro — não tem "partial".
  Decisão binária simplifica merge (SAI-070).
- `ParsedMap` mantido no result pra debug — ADR seguinte pode
  omitir em prod mode.
