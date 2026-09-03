# ADR 0053 — OpenAI-Compatible HTTP Adapter

Status: Aceito. 2026-08-20.

## Contexto

SAI-059: adapter para provedores OpenAI-compatíveis sem SDK
pesado. `net/http` + structs mínimas. Cobre OpenAI, Together,
Groq, OpenRouter, 9Router — todos expõem `/v1/chat/completions`
+ `/v1/models`.

## Decisão

`OpenAIProvider` em `internal/ai/openai.go`:

```go
type OpenAIProvider struct {
  BaseURL string
  APIKey  string
  OrgID string
  HTTPClient *http.Client
  StaticModels []ModelInfo
  ProviderName string
}
```

`NewOpenAIProvider(baseURL, apiKey)` faz trim de trailing slash
+ cria `http.Client` com timeout 60s.

Builders fluentes: `WithHTTPClient`, `WithStaticModels`,
`WithProviderName`.

`ListModels`:
- StaticModels se setado → cópia defensiva
- Senão `GET {BaseURL}/v1/models` com `Authorization: Bearer ...`
- parse `oaiModelsResponse{Data: [{ID, Object, OwnedBy}]}`
- devolve ModelInfo (capability `SupportsJSON: true` conservador)

`CompleteJSON`:
- validate opts
- `POST {BaseURL}/v1/chat/completions`
- payload: `{model, messages, temperature*, max_tokens*, stop, response_format?}`
- `response_format.type=json_object` quando JSONSchema setado
- parse `{choices[0].message.content, usage{prompt_tokens, completion_tokens, total_tokens}, finish_reason}`

`applyAuth(req)` injeta Authorization + opcional OpenAI-Organization.

Errors:
- `BaseURL` vazio → error
- non-200 status → error com body
- empty choices → `ErrResponseInvalid`

## Consequências

- 21 testes (defaults, metadata, provider name, static models,
  HTTP list/status/parse/network, complete HTTP, json_schema
  response_format, status error, empty choices, no URL, validate
  fail, org header, http client nil-safe, no auth header,
  trailing slash, temperature, stop).
- `httptest.NewServer` para testes HTTP sem framework — handler
  inline verifica path/auth/body e devolve fixture.
- Capability `SupportsJSON=true` conservador para todos modelos
  OpenAI-compat; adapters específicos (vision/audio) podem
  sobrescrever via lista estática.
- Sem streaming na v1 — flag em Metadata indica capacidade futura.

## Trade-offs

- Sem retry/backoff na interface — caller controla via
  HTTPClient customizado ou wrapper. ADR seguinte pode adicionar
  `RetryProvider` decorator.
- Sem tool_use/function_calling — escopo v1 é chat completion
  + json_object. Tools podem ser ADR futuro.
- Error messages expõem body do provider (potencial info
  disclosure em logs). Aceitável: debug > secrecy para v1.
- `ProviderName` default "openai" — caller usa `WithProviderName`
  pra distinguir provedores no agregado.
- Timeout fixo 60s — caller pode sobrescrever via WithHTTPClient.
