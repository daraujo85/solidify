# ADR 0088 — Fixture fullstack sample (SAI-105)

Status: Aceito. 2026-08-20.

## Contexto

SAI-105: repo sintético p/ E2E backend+frontend
com migration, env, tests. Base p/ SAI-106..111.

## Decisão

`fixtures/fullstack-sample/`:

- `README.md` — descrição.
- `src/api/users.go` — backend Go (handler).
- `src/web/index.html` — frontend HTML.
- `db/001_users.sql` — migration CREATE TABLE.
- `tests/users_test.go` — testes Go.
- `.env.example` — env vars (DB_*, API_*).
- `package.json` — frontend deps placeholder.
- `solidify.yaml` — profile release + analyzers
  (go-test on, lighthouse off).

## Consequências

- Cobertura completa: backend + frontend +
  migration + env + tests.
- `solidify.yaml` desabilita lighthouse —
  SAI-106 espera `Lighthouse N/A`.
- Sem dependências reais — fixture leve,
  self-contained.
- `tests/users_test.go` exercita o handler —
  pode rodar em CI real se necessário.

## Trade-offs

- Fixture não roda end-to-end real — sem
  DB/servidor. Validação só sintática. Trade-off:
  leveza > fidelity.
- Sem package-lock.json — placeholder mínimo.
