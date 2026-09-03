# ADR 0048 — Agent setup docs

Status: Aceito. 2026-08-20.

## Contexto

SAI-054: gerar instruções para registrar MCP Solidify em
Claude Code / Codex sem hardcode de home path destrutivo.
Caller decide onde aplicar (print, save, copy).

## Decisão

`SetupTarget` enum: `claude-code`/`codex`/`generic`.

`SetupDoc{Target, Binary, BinaryResolve, Steps, Snippet, Notes,
AdvancedHints}`.

`SetupOptions{BinaryPath, Target, ExtraNotes}`.

`GenerateSetupDoc(opts)` retorn `SetupDoc`:
- `BinaryResolve` dica OS-aware (`which`/`where`/`Get-Command`)
- Steps numerados (1, 2, 3, …)
- Snippet JSON formatado por target
- Notes (target-specific)
- AdvancedHints (shared)

`Markdown()` renderiza como markdown.
`PlainText()` renderiza plain text (default).

`AllTargets()` retorna `[claude-code, codex, generic]`.

Targets:
- **claude-code**: `~/.claude.json` `mcpServers.solidify`. Args
  `["mcp", "serve"]`. Não escreve nada sozinho; caller copia.
- **codex**: `~/.codex/mcp.json` `mcp.servers.solidify`.
- **generic**: snippet simples `{command, args}`.

## Consequências

- 13 testes (DefaultSetupOptions, ClaudeCode/Codex/Generic
  gen, Markdown/PlainText, empty defaults, unknown target,
  ExtraNotes append, AllTargets, no destructive path, resolve
  hint, advanced hints, BinaryResolve populated).
- Setup nunca escreve arquivo — caller decide (segurança).
- Versão `solidify` (sem path absoluto) obriga caller rodar
  `which` primeiro → elimina hardcode destrutivo.

## Trade-offs

- "não-destrutivo" = não modificar homedir automaticamente;
  instrução explícita pro user. Trade-off: copy-paste manual
  (vs auto-install). Aceitável: install scripts podem ser
  opt-in SAI futuro.
- `buildResolveHint` OS-aware via `runtime.GOOS` — simples,
  test-friendly.
- Advanced hints cobrem casos extremos (systemd, launchd,
  CI) — bundled sem target-specific.
- Snippet JSON hard-coded (não template) — mudanças no
  schema SDK exigem update aqui; aceitável, versionado.
