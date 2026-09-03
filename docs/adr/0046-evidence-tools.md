# ADR 0046 — Evidence retrieval tools

Status: Aceito. 2026-08-20.

## Contexto

SAI-052: tools de evidence retrieval via MCP — manifest, diff
chunk, context, symbol search, analyzer result, release
inventory. Budget-aware pagination.

## Decisão

`Kind*` constants. `DefaultLimit=50`, `MaxLimit=500`.

`ResolveStore(baseDir, runID)` cria `artifacts.Store` (path
`<baseDir>/runs/<runID>/`).

`DefaultEvidenceHandler(baseDir)` retorna `EvidenceGetHandler`
que valida runID + limit/offset + dispatch:

- `manifest` → `evidence.json` (full)
- `diff` → `diff/<path>` (chunk, truncated if > 100KB)
- `context` → `context/<path>` (paginated por linhas,
  Offset+Limit)
- `symbols` → `symbols.json` (substring search por linha)
- `analyzer` → `analyzer/<path>.json`
- `release` → `release-inventory.json`

Pagination: `Items[]`, `Total`, `Truncated`, `NextOffset`,
`Budget` map[string]any.

`EvidenceError` tipo pré-fixo `evidence:`. `sprintf` minimal
substitui `%s`/`%v`/`%d` por string (sem `fmt`).

`rawJSON(data)` → first char `{` ou `[` retorna
`map[string]any{"raw": string(data)}` (placeholder; SDK vai
parse no cliente), else plain string.

## Consequências

- 17 testes (ResolveStore empty/create, splitLines, trim,
  rawJSON object/string/empty, anySlice, DefaultEvidenceHandler
  basic/empty/no-kind/limits/limit-cap, EvidenceError format,
  DiffChunk missing path, SymbolSearch missing query, Manifest
  missing).
- `artifacts.New` path: `<baseDir>/runs/<runID>/` — tests
  ajustados.
- `sprintf` minimal substitui fmt.Sprintf pra evitar import
  desnecessário — `%s`/`%v`/`%d` cobrem Errorf cases.

## Trade-offs

- `rawJSON` retorna `map[string]any{"raw": string(data)}` em
  vez de parse real — evita import duplo `encoding/json` no
  handler; ponta-cliente ainda recebe JSON parse-ável.
- Symbol search é substring puramente linear (sem índice)
  — aceitável pra symbols.json típico (< 10k entries).
- Diff chunk é arquivo único; > 100KB truncated sem
  NextOffset + per-line slicing. Adequado pra review chunks
  pré-paginados.
- `sprintf` minimal aceita só `%s`/`%v`/`%d` — outros verbos
  não suportados (escape via `%s` redundancy).
