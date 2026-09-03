# ADR 0074 — Dashboard UI scaffold (SAI-080..087)

Status: Aceito. 2026-08-20.

## Contexto

SAI-080..086: dashboard frontend (Preact/Vite/TS —
build estático; zero Node no runtime). SAI-087:
`solidify dashboard` CLI serve UI + API.

## Decisão

Frontend (`web/public/`, `web/src/`):

- `index.html` shell mínimo, sem bundler runtime.
- `assets/tokens.css` design tokens (cores, badge
  status, espaçamentos) com fallback dark via
  `prefers-color-scheme`.
- `assets/app.css` layout helpers (grid, table).
- `src/main.js` vanilla JS (sem Preact/Hyperapp) —
  `el(tag, props, ...children)` helper + DOM nativo.
- Hash routing (`#/runs`, `#/run/{id}`, `#/solid`,
  `#/quality`, `#/ai`).
- `innerHTML = ""` substituído por loop de
  `removeChild` — defesa contra XSS em dados do report.

CLI (`cmd/solidify/dashboard.go`):

- `RunDashboard(args, stdout, stderr) int` aceita
  `io.Writer` (não `*os.File`) — testável c/ buffers.
- Flags: `-port`, `-store`, `-static`, `-open`.
- `loadIndex(dir)` caminha diretório, parseia
  `*.json` em `report.Report`. Diretório ausente =
  start vazio (não-erro).
- 2 endpoints: `/api/runs` (summaries ordenados)
  e `/api/run/{id}` (report completo).
- `http.FileServer(http.Dir(static))` serve o
  frontend estático.

XSS: dashboard lê `run_id`, `profile`, `grade`,
`mode`, `provider`, `model_id`, `status`, `id` —
todos via `el()` com `document.createTextNode`
(string) ou `appendChild(el)`. Sem interpolação
de HTML.

## Consequências

- 9 testes: loadIndex lista/missing, summarize,
  list ordenado, get ok/missing, handler /api/runs,
  handler /api/run (200/404/400), dashboard flags
  com port inválida, serveDashboard port vazia.
- Zero dependência externa (sem npm, sem Preact).
  Build = cp -r web/public web/src.
- API fina — todos os dados já estão no JSON
  report; só proxies.
- File server serve `web/public/` + assets. Em
  embed futuro, `embed.FS` pode substituir `http.Dir`.

## Trade-offs

- Sem Preact/Hyperapp — UI vanilla é mais verbose
  mas zero-deps. Trade-off explícito: deps zero >
  DX. ADR futuro pode reintroduzir build step.
- Sem refresh automático ao adicionar report — user
  precisa reload. ADR seguinte pode adicionar
  SSE/websocket ou watcher.
- Sem auth — dashboard assume localhost. Expor
  remote = ADR p/ auth.
- Porta 0 não-discoverable — `http.ListenAndServe`
  retorna erro se porta ocupada; usuário precisa
  verificar stdout.

## Nota de segurança

XSS: Toda escrita no DOM via `textContent` ou
`document.createTextNode` (via helper `el(string)`).
Nunca `innerHTML` com dados do report. Se um report
contém `<script>alert(1)</script>` em algum campo,
o browser renderiza como texto puro, não como HTML.