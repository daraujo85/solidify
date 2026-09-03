# ADR 0016 — SQLite storage

## Status

Aceito. 2026-08-20.

## Contexto

SAI-022 pede persistência relacional local:

- `runs`, `components`, `analyzer_results`, `reviews`, `models`,
  `findings`, `artifacts`
- Para queries, agregações e auditoria
- Sem processo externo (DB embutido)
- `CGO_ENABLED=0` para manter build estático

## Decisão

Driver: `modernc.org/sqlite` — SQLite transpilado para Go puro.

DB: arquivo único em `<baseDir>/solidify.db`.

DSN: `file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)`

Schema versionado em `schema_version` (v1). `CREATE IF NOT EXISTS`
— sem runtime migration engine (escala para uma versão; adicionar
quando passar para v2).

API: `Store` com helpers tipados por domínio (`InsertRun`,
`ListFindingsByRun`, etc). Tipos são ponteiros nulos (`*time.Time`,
`*int64`) para opcionais.

Helpers para SQL dinâmico:
- `QuoteIdentifier` — whitelist alfanum + underscore.
- `QuoteValue` — escape de aspas (preferir `?` parametrizado).

## Consequências

**Positivas:**

- CGO_ENABLED=0 OK (modernc é pure-Go).
- Sem processo externo (sem sqlite3 binary).
- WAL mode para reduzir locking (concurrent reads OK).
- Foreign keys enforced.
- 27 testes (requisitos, NULL handling, concurrent, WAL, FK).
- Schema idempotente.

**Negativas:**

- modernc adiciona ~10MB ao binário (compilação do SQLite pure-Go).
- Migrations são "CREATE IF NOT EXISTS" — não há evolução de schema
  para v1. Quando passar para v2, adicionar `ALTER TABLE` idempotente.
- Sem full-text search — para queries complexas, `LIKE` é suficiente
  no MVP.
- Sem replicação — DB local. Para multi-host, sync via artifact
  store (filesystem shared).

## Alternativas consideradas

- **mattn/go-sqlite3**: requer CGO. Rejeitado (build complexity).
- **BoltDB / bbolt**: KV store. Sem SQL. Agregações seriam
  client-side. Rejeitado.
- **Postgres embutido**: usa processo externo. Rejeitado.
- **JSON files**: zero SQL. Queries triviais. SAI-022 prefere
  relacional. Rejeitado.
- **Migrations engine (goose, migrate)**: overhead para 1 schema.
  Adiar até v2. Rejeitado.
