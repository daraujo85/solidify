# ADR 0075 — Print + PDF pipeline (SAI-088..092)

Status: Aceito. 2026-08-20.

## Contexto

SAI-088 print stylesheet (já no `tokens.css` — `@media
print` esconde nav, força cores claras, ajusta
largura). SAI-089 detector de browser headless. SAI-090
comando PDF. SAI-091 fallback Docker. SAI-092 validação
de conteúdo.

## Decisão

`internal/print/browser.go`:

- `BrowserCandidate{Name, Path, Args}` — paths
  por plataforma (darwin/windows/linux).
- `CommonCandidates()` retorna lista ordenada por
  preferência: chrome → chromium → edge.
- `DetectBrowser()` tenta paths absolutos primeiro,
  cai pra `exec.LookPath`.
- `HasBrowser()` boolean helper.

`internal/print/pdf.go`:

- `PDFOptions{URL, HTMLPath, Output, Width, Height,
  MarginTop/Bottom/Left/Right, Wait, DockerImage}`.
- Defaults: A4 (210x297mm), 10mm margins.
- `GeneratePDF(opts)`:
  1. `DetectBrowser()` → `runLocal()` se achou.
  2. Senão → `runDocker()` com imagem configurável
     (default `chromium-headless:latest`).
- `runLocal` invoca `--print-to-pdf=<out>` + flags
  paper/margin/virtual-time-budget.
- `runDocker` monta volumes (`-v`): HTML dir como
  `/src:ro`, output dir como `/out`.
- `IsPDF(path)` checa magic bytes `%PDF`.
- `PDFSize(path)` wrapper de `os.Stat`.

`internal/print/validate.go`:

- `ExtractPDFText(path)` heurística: varre bytes,
  extrai conteúdo entre `(...)` (PDF literal strings)
  e `<...>` (PDF hex strings). Heurística simples
  sem lib PDF — zero deps.
- `ValidatePDF(path, required)` checa seções
  obrigatórias (gate, score, SOLID, risk, run_id)
  contra o texto extraído.
- `DefaultRequiredSections` listas 5 marcadores
  mínimos p/ release contractual.

Print stylesheet já presente em `tokens.css`:
- `@media print { .bar { display: none } }`
- cards com `break-inside: avoid` (não corta página).

## Consequências

- 12 testes: candidates, HasBrowser, extractPDF (não-
  PDF/vazio/ok), validate missing/ok, generate sem
  source/output, IsPDF magic/vazio/missing, PDFSize,
  printable, WithTimeout.
- Browser detection é rápido (stat only); falha
  silenciosa → Docker fallback.
- PDF sem lib externa — heurística textual basta
  p/ validar seções obrigatórias. Se PDF for
  encriptado ou comprimido, extração vira vazia —
  logado mas não-bloqueante (return erro só se
  path não existe / não-PDF).
- Default `chromium-headless:latest` precisa existir
  no registry. ADR pode trocar pra imagem pinned
  (ex: `chromium:128`) p/ reprodutibilidade.

## Trade-offs

- Heurística `ExtractPDFText` não cobre streams
  comprimidos (FlateDecode). PDFs gerados por
  chromium-headless não são comprimidos por default;
  se outro produtor comprimir, validação falha.
  ADR pode adicionar decoder.
- Sem paralelismo no browser — uma render por vez.
  Trade-off: simplicidade > throughput.
- `exec.Command` herda stdin/stdout do chamador —
  browser output vai pro console. Útil p/ debug;
  caller pode redirecionar.
- Docker fallback assume `docker` no PATH. Em
  CI sem Docker, falha com mensagem clara
  (`ErrNoBrowser` antes do fallback).