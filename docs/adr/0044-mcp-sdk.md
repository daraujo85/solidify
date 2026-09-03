# ADR 0044 — MCP SDK Go pin

Status: Aceito. 2026-08-20.

## Contexto

SAI-050: adicionar SDK MCP oficial Go (`github.com/modelcontextprotocol/go-sdk`)
pinado em versão estável, transporte stdio. Wrapper thin
re-exporta tipos do SDK e adiciona identidade Solidify.

## Decisão

`SDKVersion = "v1.0.0"` (pin estável). `ServerName = "solidify"`,
`ServerVersion = "0.1.0"`.

`internal/mcp/mcp.go` re-exporta `Server`, `ServerOptions`,
`Transport`, `ToolHandler`, `Tool`, `Session` (type aliases).

`NewServer()` constrói `mcp.Server` com `Implementation{
Name: "solidify", Version: "0.1.0"}`.

`StdioTransport()` retorna `&mcp.StdioTransport{}`.

`AddTool(s, name, description, handler)` helper que monta
`*mcp.Tool{Name, Description}` + `s.AddTool`.

`VersionInfo{SDK, Server, Build}` + `CurrentVersion()`.

## Consequências

- 6 testes (SDKVersion pinned, ServerName, ServerVersion,
  NewServer, CurrentVersion, StdioTransport).
- `go.mod` ganha `github.com/modelcontextprotocol/go-sdk v1.0.0`
  + transitives (`hashicorp/golang-lru/v2`, `golang/gc/v2`/`v3`).
- Setup Stdio pronto pra SAI-051/053 tools.

## Trade-offs

- Pin major (`v1.0.0`) sem caret — sem updates automáticos;
  upgrade conscious (segurança).
- Wrapper thin evita callers dependem diretamente do SDK;
  upgrade centralizado em `internal/mcp/`.
- `AddTool` helper: SDK aceita `*Tool` com fields adicionais
  (InputSchema, Annotations) — wrapper aceita só básicos,
  mais simples. Schema via RawInputSchema se preciso.
