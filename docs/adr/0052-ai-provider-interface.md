# ADR 0052 — AI Provider Interface

Status: Aceito. 2026-08-20.

## Contexto

SAI-058: interface para provedores de IA. Solidify precisa
abstrair OpenAI-compat, 9Router, e futuros provedores. Adapters
implementam `Provider`.

## Decisão

`Provider` interface em `internal/ai/provider.go`:

```go
type Provider interface {
  Name() string
  ListModels(ctx context.Context) ([]ModelInfo, error)
  CompleteJSON(ctx context.Context, opts CompleteOptions) (*CompleteResult, error)
  Metadata() ProviderMetadata
}
```

Tipos auxiliares:
- `Role` (system/user/assistant), `Message{Role, Content}`
- `ModelInfo{ID, Name, Provider, ContextSize, MaxOutput, SupportsJSON, SupportsVision, SupportsAudio}`
- `CompleteOptions{Model, Messages, Temperature*, MaxTokens*, JSONSchema, Stop}`
- `CompleteResult{Content, Model, FinishReason, PromptTokens, CompletionTokens, TotalTokens}`
- `ProviderMetadata{Name, Version, BaseURL, RequiresKey, Streaming}`

Helpers:
- `ValidateOptions(opts)` — model vazio, messages vazias, role/content vazios
- `FilterByCapability(models, json, vision, audio)` — filtro
- `FindModel`, `SupportsJSON` — lookup
- `BuildMessages(system, user)` — conveniência
- `TruncateContent(s, max)` — corta com ellipsis
- `StripCodeFences(s)` — remove ```json wrapper

Errors sentinelizados:
- `ErrModelNotFound`, `ErrEmptyMessages`, `ErrProviderNotImpl`, `ErrResponseInvalid`

`MockProvider` em mesmo arquivo: 2 modelos, CompleteJSON
devolve JSON estático. `ListModels` retorna **cópia defensiva**
para evitar mutação externa do estado interno.

## Consequências

- 24 testes (mock name/list/complete/missing/empty, validate
  model vazio/role vazio/content vazio/ok, filter JSON/vision,
  find model, supports json helper, role valid, build messages,
  truncate, strip fences, metadata, immutable list, ctx cancel).
- `JSONSchema` field em CompleteOptions abre caminho para
  adapters implementarem structured output (OpenAI json_schema,
  Anthropic tool_use, etc).
- Adapters futuros implementam só 4 métodos — testáveis
  isoladamente com MockProvider como dependência injetada.

## Trade-offs

- Interface retorna `[]ModelInfo` em vez de canal — caller
  itera normal. Streaming fica via flag `Metadata.Streaming` +
  método opcional futuro (não na v1).
- `Temperature*` / `MaxTokens*` pointers para distinguir
  "não setado" de "zero" — verboso mas correto.
- Sem retry/backoff na interface — adapters decidem. ADR
  seguinte pode adicionar wrapper `RetryProvider` decorator.
- MockProvider no mesmo package — não em `testing/`. Aceitável:
  outros packages podem usar MockProvider como stub em testes
  próprios.
