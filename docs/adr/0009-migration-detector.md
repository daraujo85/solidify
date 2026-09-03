# ADR 0009 — Migration detector framework-aware

## Status

Aceito. 2026-08-19.

## Contexto

SAI-015 pede detectar migrations em 9 frameworks
(Flyway, Liquibase, EF Core, Prisma, TypeORM, Sequelize, Knex,
Django, Rails, Laravel) + generic SQL em diretórios configuráveis.
Saída: `Migration{Framework, Path, Version, Description, Status,
HasRollback, RollbackOf}` normalizado.

## Decisão

Pattern matching por path, sem ler conteúdo do arquivo. Cada framework
tem um `match(path) (version, desc, ok)` próprio. Regras com prioridade:

| Framework  | Priority | Pattern                                   |
|------------|----------|-------------------------------------------|
| Flyway     | 90       | V<ver>__<desc>.sql / R__<desc>.sql / U<ver>.sql |
| EFCore     | 85       | */Migrations/*Migration.cs                |
| Prisma     | 85       | prisma/migrations/<ts>_<name>/migration.sql |
| Django     | 85       | */migrations/<NNNN>_<name>.py             |
| Rails      | 85       | db/migrate/<ts>_<name>.rb                 |
| Laravel    | 85       | database/migrations/<ts>_<name>.php       |
| Liquibase  | 80       | *liquibase*/* ou *changelog*/*            |
| TypeORM    | 70       | **/migrations/*.ts                        |
| Sequelize  | 70       | migrations/<ts>-<name>.js                 |
| Knex       | 70       | migrations/<ts>_<name>.js                 |

Maior prioridade vence. Generic SQL é fallback: `.sql` num diretório
configurado (`migrations`, `db/migrate`, `alembic/versions`, …).

`descNormalize` troca `_` e `-` por espaço — `add_email` / `add-email`
vira `add email` para apresentação.

**Não faço parse de conteúdo** (SQL, EF modelSnapshot, etc.) — isso é
SAI-016 (parser) e SAI-017 (risk engine).

## Consequências

**Positivas:**

- Path-only: O(N) stat-free, super rápido mesmo em monorepos com
  milhares de migrations.
- Framework inferido antes de qualquer IA — entrada estável para o
  parser SQL na fase seguinte.
- Status (added/modified/deleted/renamed) preservado do SAI-007.

**Negativas:**

- Path-only erra quando framework é ambíguo (ex.: projeto usa
  `migrations/` mas não é TypeORM). Aceitável — caller pode passar
  genericDirs para refinar.
- Versão para migrations sem padrão numérico (ex.: Django com nome
  `initial.py`) vira "0001" do prefixo mas descrição pode ficar crua.
- TypeORM era inicialmente muito permissivo (`src/*` casava); corrigido
  para exigir `migrations/` em qualquer segmento do path.

## Alternativas consideradas

- **AST/syntax parsing do conteúdo**: caro, sem benefício claro pro
  SAI-015. SAI-016 já lê o arquivo para detectar operações. Rejeitado.
- **Mapa único pattern→framework**: ordenação implícita via ordem do
  switch é frágil. Rejeitado — array ordenado por prioridade é
  explícito.