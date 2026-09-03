# ADR 0090 — E2E frontend-only release (SAI-107)

Status: Aceito. 2026-08-20.

## Contexto

SAI-107: validar comportamento E2E p/ fixture
frontend-only. Backend analyzers N/A, lighthouse
ativo, métricas perf presentes.

## Decisão

`internal/e2e/` (extensão do SAI-106):

- `FrontendOnlyExpected{LighthouseActive, BackendNA,
  PDFValidMagic, JSONParses, HasPerformance}`.
- `VerifyFrontendOnly(pdf, json)`:
  - PDF magic `%PDF`.
  - JSON não vazio.
  - `HasPerformance` = contém "lighthouse" ou
    "performance" (case-insensitive).
- `containsCI(haystack, needle)` — substring match
  case-insensitive sem `strings.ToLower` (zero
  alloc).

## Consequências

- 4 testes adicionais: verify OK, bad PDF, no perf,
  containsCI helper.
- Total e2e: 12 testes.
- Heurística de performance é simples — confia em
  campo textual. Detecção robusta fica p/ schema
  validation.

## Trade-offs

- `containsCI` reimplementado — não usa
  `strings.ToLower`. Trade-off: zero alloc >
  stdlib.
- `HasPerformance` é binário — sem score/grade.
  Trade-off: smoke check > métricas exatas.
- Sem parsing JSON estrutural — caller pode
  inspecionar campos via API real.
