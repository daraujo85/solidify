# ADR 0057 — Model Selector

Status: Aceito. 2026-08-20.

## Contexto

SAI-063: seleção de modelos para Peer A/B com modos configuráveis.
- Pinned: config fixa (reprodutibilidade total)
- Discover: probe + rank (robustez, encontra melhor)
- Hybrid: pinned primário + discover fallback

Distinctness policy garante independência Peer A vs Peer B
(provider/model/family distintos).

## Decisão

`SelectorMode`: pinned, discover, hybrid.
`DistinctnessPolicy`: none, provider, model, family.

`SelectorOptions{Mode, Pinned, Distinctness, MinModels,
RequireJSON, RequireVision, RequireAudio}`.

Default: `Mode=discover, Distinctness=provider, MinModels=2`.

`SelectorDecision{Model, Provider, Reason, Source, Rank, Score}`:
audit trail por decisão.

`SelectorResult{Decisions, Mode, Policy, Reason}`.

`Selector{opts, probes}`:
- `selectPinned`: busca cada ID na lista de providers; missing →
  decision com Reason "não tem modelo"
- `selectDiscover`: probe todos → rank → distinctness filter
- `selectHybrid`: pinned primeiro; se < MinModels, completa com
  discover (Source=fallback)

`collectSummaries`: itera providers × models, filtra por caps,
usa cache (ProbeCache), senão `ProbeMany(samples=3)`.

`applyDistinctness`:
- none: inclui todos
- provider: dedup por `Summary.Provider`
- model: dedup por `Model`
- family: `ModelFamily(model)` (gpt/claude/gemini/llama/mistral/other)

`ModelFamily(model)` heurística por prefixo (case-insensitive).

`FormatResult(r)` render human-readable.

## Consequências

- 18 testes (defaults, nil cache, sem providers, pinned OK,
  pinned missing, discover OK, distinct providers/models/family,
  distinct none inclui todos, hybrid com fallback, hybrid pinned
  suficiente, mode inválido, model family 7 cases + case
  insensitive, format result, require json, min zero, decisions
  rank/score).
- `Reason` populated em todas decisões — caller sabe por que
  cada modelo foi escolhido.
- `Source` distingue pinned/discover/fallback — UI pode
  destacar "manual override" vs "automático".
- `ProbeCache` reusa resultados de probes anteriores — selector
  re-rápido após warmup.

## Trade-offs

- `ModelFamily` heurístico (prefix match) — modelos com nomes
  exóticos caem em "other". Aceitável: spec não define taxonomia
  canônica; family grouping é best-effort.
- Hybrid: se pinned falha discovery (todos indisponíveis), retorna
  pinned com decision vazia de Provider/Reason="não tem modelo".
  Caller decide se isso é erro ou warning.
- `RequireJSON` filter aplicado antes de probe — não validamos
  que capability declarada bate com real (probe pode detectar
  JSON compliance real).
- Sem versionamento de `SelectorOptions` schema — adicionar
  campo requer migration de config files existentes.
