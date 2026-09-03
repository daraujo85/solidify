# ADR 0098 — ADR index (SAI-115)

Status: Aceito. 2026-08-20.

## Contexto

SAI-115: índice navegável de todas as ADRs do
projeto. 97 ADRs acumuladas ao longo do desenvolvimento.

## Decisão

`docs/adr/README.md`:

- Tabela markdown com 97 entradas.
- Coluna 1: número.
- Coluna 2: título curto.
- Link relativo p/ arquivo `.md`.
- Seção final: convenção (1 ADR/decisão, formato,
  status, cancellation).

## Consequências

- 1 ponto de entrada p/ navegar decisões.
- Sem auto-geração — atualizado manualmente em
  SAI-115 (última task).
- Total: 98 ADRs (1..98).

## Trade-offs

- Manual update — se novas ADRs forem criadas, é
  preciso atualizar índice. Trade-off: simplicidade
  > automação.
- Tabela em markdown — sem filtros/search. Grep
  resolve.
