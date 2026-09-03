# ADR 0119 — MCP transport real (SAI-119)

Status: Aceito. 2026-08-21.

## Contexto

SAI-118 fechou `peer_a` no orquestrador com transporte
**stub** (`terminal-mcp` lia `<artifact_dir>/peer_a_response.json`).
Stub cumpre o contrato de `ExecutorResult` mas não usa o
protocolo MCP — agente (Claude Code / OpenCode) teria que
escrever o arquivo de review direto na mão.

ADR-0118 já listava como diferido:
> MCP transport real (SAI-119): SDK choice
> (depende SAI-044 SDK choice). Stub atual lê arquivo;
> transporte oficial substitui.

SAI-044 foi resolvido em ADR-0044 (SDK oficial Go pinado
em v1.0.0). `internal/mcpserver/` já tem o server completo:

- `solidify_begin_review` (run + contract refs)
- `solidify_submit_peer_review` (validação + atomic save)
- `solidify_evidence_get` (manifest / diff / context / symbols / analyzer / release)
- `solidify_setup` (gera doc não-destrutivo pra Claude Code / Codex)

Falta o wire no lado orquestrador + CLI subcommand.

## Decisão

### 1. Storage compartilhado via `PeerReviewStore`

`internal/mcpserver/peer.go::PeerReviewStore` já persiste
records atomicamente em `<dir>/<review_id>.json`. ADR-0119
mantém o storage existente e padroniza o `dir`:

- Default: `~/.solidify/peer_reviews/`
- Override: `ai.peer_a.store_dir` no config

O store é o canal de comunicação entre agente (que escreve
via `solidify_submit_peer_review`) e orquestrador (que lê
via polling).

### 2. CLI subcommand `solidify mcp serve`

Adiciona em `internal/app/mcp.go` (novo) e dispatcher
`app.go`. Stdio transport + tools:

- `solidify_submit_peer_review` → persiste em PeerReviewStore
- `solidify_evidence_get` → bound ao `artifact_dir` corrente
- `solidify_begin_review` → cria run se necessário
- `solidify_setup` → apenas doc (não escreve config)

Claude Code registra o server via snippet do ADR-0044:

```json
{
  "mcpServers": {
    "solidify": {
      "command": "solidify",
      "args": ["mcp", "serve"]
    }
  }
}
```

### 3. `peer_a.go::executeTerminalMCP` polling

Substitui o file-read stub:

```go
func executeTerminalMCP(ctx, opts) (*ExecutorResult, bool, error) {
    store, err := NewPeerReviewStore(resolveStoreDir(opts))
    if err != nil { return nil, false, err }
    deadline := time.Now().Add(opts.Timeout)
    for time.Now().Before(deadline) {
        records := store.List()           // novo helper
        for _, rec := range records {
            if rec.RunID == opts.RunID && rec.Actor == "peer_a" {
                return recordToExecutorResult(rec), true, nil
            }
        }
        select {
        case <-ctx.Done(): return nil, false, ctx.Err()
        case <-time.After(2 * time.Second):
        }
    }
    return nil, false, nil // timeout = skipped
}
```

Polling de 2s é o suficiente — agente humano demora
segundos-a-minutos, overhead de 2s é invisível. Pode virar
file-watch (inotify/kqueue) depois se virar gargalo.

### 4. Schema do submission

Agente envia canonical schema em `Notes` (string JSON) e
mantém `Verdict` ∈ {approve, request_changes, comment}
(requisito do validator existente). Orquestrador parseia
Notes como canonical:

```json
{
  "run_id": "r20260820-143012",
  "actor": "peer_a",
  "schema": "1",
  "evidence_hash": "<sha256>",
  "verdict": "comment",
  "notes": "{\"solid\":{...},\"quality_score\":85.0,\"confidence\":0.9}",
  "findings": {}
}
```

`notes` carrega o canonical review completo;
`verdict` satisfaz o validator legado. SAI-120 futuro pode
mover `notes` pra campo próprio (`canonical_payload`).

### 5. Actor identity

`actor_id` no `ExecutorResult.Model` vira
`"solidify-mcp://peer_a/<review_id>"` (auditável: bate com
o `review_id` persistido, operador pode rastrear o agente
que submeteu).

### 6. `ai.peer_a.source` enum ganha semântica clara

| Source | Comportamento |
|---|---|
| `terminal-mcp` | Polls PeerReviewStore (ADR-0119). Requer Claude Code / OpenCode com server Solidify registrado E submission do agente. Sem submission no timeout → skipped. |
| `http-combo` | HTTP via 9Router (SAI-118). |
| `auto` | `http-combo` na Fase 1. SAI-120+ pode virar `terminal-mcp` quando MCP SDK estiver universalmente registrado. |

Default config mantém `terminal-mcp` (ADR-0118 decisão);
mas `solidify run` em CI sem MCP server = skipped → gate
BLOCKED → operador sabe que precisa de MCP. Erro
explícito > fallback silencioso (princípio SAI-116).

## Não-objetivos

- File-watch (inotify/kqueue) em vez de polling 2s.
- Streaming de progresso do agente via MCP resources.
- MCP transport não-stdio (HTTP/SSE) — Phase 2.
- Auto-aprovação / submission via outro modelo sem agente.
- Schema versionamento (`canonical_payload` próprio) —
  SAI-120.

## Consequências

- `peer_a` agora funciona end-to-end quando Claude Code
  registra o server MCP. Antes: stub inerte.
- Independência real: terminal MCP é o agente humano/IA,
  que tem contexto e critério diferente do peer_b (modelo
  HTTP). Não é mais só "modelo diferente".
- Latência: agente demora segundos-a-minutos pra revisar;
  timeout configurável (default 90s, ajustável).
- Operador que não registra MCP server vê BLOCKED
  consistente (não INCOMPLETE silencioso).
- Setup não-destrutivo: `solidify mcp setup` mostra
  snippet pronto; usuário cola no `~/.claude.json`.

## Próximo passo

1. ~~Implementar `PeerReviewStore.List()` helper~~ ✅
2. ~~`internal/app/mcp.go` com `mcp serve` + `mcp setup`~~ ✅
3. ~~Refatorar `peer_a.go::executeTerminalMCP` polling~~ ✅
4. ~~Smoke: `internal/peer/peer_a_test.go` cobre poll
   finds record, no submission = skipped, run_id
   mismatch, legacy stub fallback, ResolveSource.~~ ✅
5. SAI-120: schema versionamento + `canonical_payload`
   field próprio (remover `Notes` JSON-string, criar
   campo nativo).