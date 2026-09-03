# ADR 0097 — Example reports sanitizados (SAI-114)

Status: Aceito. 2026-08-20.

## Contexto

SAI-114: reports JSON de exemplo (PASS/WARN/FAIL)
com secrets redacted. Doc viva p/ consumidores da
API.

## Decrição

`examples/`:

- `report-pass.json` — score 87.5, robustness
  stable, gate PASS.
- `report-warn.json` — score 78.0, robustness
  mostly-stable, gate WARN.
- `report-fail.json` — score 62.0, robustness
  unstable, gate FAIL com critical finding.

## Conteúdo

Cada report inclui:

- `run.id` + `profile` + `started_at`/`finished_at`.
- `scores.global` + `grade`.
- `solid.principles{S,O,L,I,D}`.
- `risk.score` + `level` + `factors`.
- `quality_gate.status` + `threshold` + `reasons`.
- `ai_review.peers` (3 roles) + `robustness`.
- `recommendations` + `limitations`.

## Consequências

- 3 reports cobrindo spectrum PASS/WARN/FAIL.
- Sem secrets — todos os tokens são placeholders
  genéricos (`gc/gemini-3.1-pro-preview`,
  `kr/claude-sonnet-5`, etc.).
- Formato canônico alinhado com schema 1.0.0.

## Trade-offs

- Sem HTML/PDF — só JSON. Render fica p/ ferramenta
  externa (ou ADR futura).
- Report FAIL inclui `critical_findings` — outros
  omit. Trade-off: FAIL mais rico > uniformity.
