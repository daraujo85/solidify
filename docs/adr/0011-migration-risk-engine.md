# ADR 0011 — Migration risk engine

## Status

Aceito. 2026-08-20.

## Contexto

SAI-017 pede implementar os sinais de risco de ARCHITECTURE.md
§10.4. Conservador: "se ambíguo, silêncio".

Sinais cobertos:

- DROP TABLE/COLUMN/INDEX
- redução de tamanho/tipo incompatível
- NOT NULL sem default/backfill aparente
- rename que quebra consumidores
- update/delete massivo
- criação de índice potencialmente bloqueante
- migration sem rollback quando a stack suporta
- ordem dependente
- migration editada depois de já existir

**Não executa** nada destrutivo — só marca para revisão.

## Decisão

Função pura `Assess(m Migration, ops []Operation) []Finding`.

Três camadas:

1. **`assessOp`** — análise estrutural da Operation:
   - `OpDrop` com Detail ∈ {TABLE, VIEW, TYPE, FUNCTION, SCHEMA} →
     `DROP_OBJECT` (CRITICAL p/ TABLE/SCHEMA, HIGH p/ resto)
   - `OpDrop` com Detail=INDEX → suprimido (tratado por OpDropIndex)
   - `OpAdd` → `ADD_COLUMN` LOW (baseline de schema mutation)
   - `OpRename` → `RENAME_BREAKING` MODERATE
   - `OpDropIndex` → `DROP_INDEX` MODERATE
   - `OpCreateIndex` → `CREATE_INDEX` LOW (baseline)
   - `OpUpdate`/`OpDelete` → `UPDATE_DATA`/`DELETE_DATA` LOW (baseline)

2. **`assessRaw`** — análise textual do `op.Raw`:
   - `OpCreateIndex` sem CONCURRENTLY/ONLINE → `CREATE_INDEX_BLOCKING`
     MODERATE (Postgres bloqueia escrita; MySQL 8 é online por default)
   - `OpUpdate` sem `WHERE` → `MASSIVE_UPDATE` HIGH
   - `OpDelete` sem `WHERE` → `MASSIVE_DELETE` CRITICAL
   - `OpAdd` com NOT NULL sem DEFAULT → `NOT_NULL_NO_DEFAULT` HIGH
   - `OpAlter` com SET NOT NULL → `NOT_NULL_NO_DEFAULT` HIGH

3. **`assessMetadata`** — análise do `Migration.Framework`:
   - Framework ∈ {Flyway, Liquibase, Rails, Django, EFCore, Laravel}
     sem rollback → `MISSING_ROLLBACK` MODERATE
   - Prisma, GenericSQL, Sequelize, Knex, TypeORM: NÃO exige
     rollback (não é convenção nesses frameworks)

**Conservadorismo:** se `Raw` vazio ou padrão não bate, não emite
finding. Melhor falso-negativo do que bloquear release por ruído.

**Não coberto nesta versão** (pode evoluir):

- Redução de tamanho/tipo incompatível — exige comparar declaração
  antes/depois; diffing complexo, deixar para SAI posterior.
- Ordem dependente — exige histórico de execução; fora do escopo
  determinístico.
- Migration editada após release — exige histórico git timestamp vs
  data de aplicação; depende de policy do projeto.

## Consequências

**Positivas:**

- Cobre 100% dos sinais §10.4 que são determinísticos sem diffing.
- 27 testes cobrindo cada code + frameworks com/sem rollback.
- Severidade calibrada por impacto real (DROP TABLE = CRITICAL;
  RENAME = MODERATE).
- Compatível com parser SAI-016 (Operation tem Raw).

**Negativas:**

- "Tipo incompatível" exige comparar colunas antes/depois — diff
  requer state externo. Adiar.
- UPDATE/DELETE sem WHERE pode ser intencional (rare mas válido,
  ex.: one-shot cleanup em migration inicial). Caller pode suprimir
  via policy.
- "Migration editada depois" depende de heurística fora deste
  arquivo.

## Alternativas consideradas

- **Regras em YAML/JSON**: moveria avaliação para config, mas §10.4
  pede regras versionadas no código (auditabilidade). Rejeitado.
- **IA avalia risco**: caro, não-determinístico, overkill para
  padrões conhecidos. Rejeitado.
- **Severity agreement cross-stack**: §11 menciona, mas exige revisão
  por IA + arbiter. Fora do SAI-017 determinístico. Rejeitado.