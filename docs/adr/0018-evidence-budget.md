# ADR 0018 — Evidence budget e truncation

Status: Aceito. 2026-08-20.

## Contexto
SAI-024: char/line budgets, priorities, reason-for-context, truncation metadata. 10MB diff não explode memória.

## Decisão
`budget.Builder` agrega items com Priority (low/medium/high) e aplica budget. Items high-priority incluídos sempre; low dropar quando budget estoura; medium/high truncados via head+tail. Truncation metadata inclui original/kept chars/lines + SHA-256 + reason.

## Consequências
- 17 testes (priority order, drop, truncate, large diff).
- Diff 10MB cabe em 10000 chars sem panicar.
- Ordem determinística (sort por priority desc, source asc).
