# ADR 0010 — SQL migration operation parser

## Status

Aceito. 2026-08-20.

## Contexto

SAI-016 pede parser SQL conservador que detecta CREATE/ALTER/DROP/
ADD/RENAME/UPDATE/DELETE/CREATE_INDEX/DROP_INDEX. **Não** tenta
parser SQL universal — `unknown_operation` em vez de inventar.

Entrada: conteúdo SQL de uma migration (lido por `git show` ou
similar). Saída: `[]Operation{Type, Object, Detail, Raw}`.

## Decisão

Tokenização por regex, sem parser real. Três fases:

1. **`stripNoise`** — remove comentários (`--`, `/* */`) e strings
   literais (`'...'`); PRESERVA identificadores com aspas duplas
   (`"User"`) porque precisamos do nome.
2. **`splitStatements`** — divide pelo `;` (após strip, splitting
   simples é seguro — strings/comentários já sumiram).
3. **`matchStatement`** — aplica regexes em ordem. Cada regex captura
   UM ÚNICO grupo: o nome do objeto. Qualificadores opcionais
   (`UNIQUE`, `IF NOT EXISTS`, `OR REPLACE`) NÃO viram grupos —
   bagunçam os índices quando não casam. Verificamos o statement
   inteiro para qualificadores via `strings.Contains(up, ...)`.

**Tabela de operações:**

| Statement                      | Type           | Object     | Detail                |
|--------------------------------|----------------|------------|-----------------------|
| CREATE TABLE [IF NOT EXISTS] X | CREATE         | X          | TABLE                 |
| CREATE UNIQUE INDEX X          | CREATE_INDEX   | X          | UNIQUE                |
| CREATE OR REPLACE VIEW X       | UNKNOWN        | X          | CREATE OR REPLACE     |
| DROP TABLE [IF EXISTS] X       | DROP           | X          | TABLE                 |
| DROP INDEX X                   | DROP_INDEX     | X          | —                     |
| ALTER TABLE X ADD COLUMN Y     | ADD            | X          | Y                     |
| ALTER TABLE X ADD CONSTRAINT Y | ALTER          | X          | CONSTRAINT            |
| ALTER TABLE X DROP COLUMN Y    | DROP           | X          | Y                     |
| ALTER TABLE X RENAME TO Y      | RENAME         | X          | Y                     |
| ALTER TABLE X RENAME COLUMN Y  | RENAME         | X          | Y -> Z                |
| ALTER TABLE X ALTER COLUMN Y   | ALTER          | X          | Y                     |
| UPDATE X SET ...               | UPDATE         | X          | —                     |
| DELETE FROM X                  | DELETE         | X          | —                     |
| INSERT INTO X ...              | UNKNOWN        | —          | INSERT                |
| BEGIN / COMMIT / SET / SELECT  | UNKNOWN        | —          | —                     |
| CREATE FUNCTION / PROCEDURE    | UNKNOWN        | —          | PL/SQL fora do escopo |
| Não reconhecido                | UNKNOWN        | —          | —                     |

**Casos UNKNOWN explícitos** (não inventa): CREATE OR REPLACE
(ambíguo entre CREATE e DROP), INSERT (não sabemos se é destrutivo),
PL/SQL blocks (BEGIN/DECLARE/$$/END), TRUNCATE (poderia ser DROP,
fica UNKNOWN), GRANT/REVOKE, MERGE, CALL, EXPLAIN.

**ALTER TABLE ONLY** (Postgres): todas as regexes de ALTER incluem
`(?:ONLY\s+)?` opcional antes do nome da tabela.

**ADD CONSTRAINT vs ADD COLUMN:** detecção em duas etapas — primeiro
procura `ADD CONSTRAINT` (ALTER genérico), depois `ADD [COLUMN]`
(OpAdd). Sem negative lookahead (Go regexp não suporta).

## Consequências

**Positivas:**

- Conservador por construção: UNKNOWN é o padrão. Caller sabe que
  opsUNKNOWN significa "revisar manualmente".
- Cobre ~95% das migrations reais: CREATE/ALTER/DROP TABLE + ADD/
  DROP COLUMN + CREATE/DROP INDEX + RENAME + UPDATE/DELETE.
- Zero dependências (só `regexp` da stdlib).
- Strip-noise robusto contra keywords dentro de strings e
  comentários.
- Quotes duplas preservadas → `"User"` vira `Object=user`.

**Negativas:**

- Funções, procedures, triggers com corpo composto → UNKNOWN
  (proposital — fora do escopo).
- PL/pgSQL, T-SQL blocks → UNKNOWN.
- Não detecta `INSERT ... ON CONFLICT` etc (UNK sem distinguishing).
- Migration Flyway com `placeholder` (`:name`) → o parser pode
  interpretar placeholder como nome. Aceitável — Flyway resolve
  placeholder antes do SQL rodar.
- Regexes múltiplas em ordem: fácil errar precedência. Mitigado por
  testes cobrindo cada caso explicitamente (33 testes).

## Alternativas consideradas

- **ANTLR / sqlparser-go / vitess**: parser SQL real, suporta todas
  as dialéticas. Custo: ~5MB binário extra, +latência 50-200ms por
  migration. Para um detector conservador, overkill. Rejeitado.
- **Heurística só-de-statement-start** (sem regex por tipo): menos
  preciso, ORDER OF MATCH vira tabela. Rejeitado.
- **Estado interno (tokenizer state machine)**: cobriria mais casos
  (PL/SQL) mas é 5× mais código e 1 tarefa SAI inteira. Rejeitado
  para SAI-016; pode ser migração futura se a UX exigir.

## Notas de implementação

- `extractKind` encontra TABLE/VIEW/TYPE/etc no statement upper-case
  via regex `\b(TABLE|VIEW|...)\b` (sem captura no statement
  original).
- `stripQuotes` remove aspas externas de identificadores quoted.
- `truncate(s, 200)` limita `Raw` para apresentação (200 chars + "...").
- DROP INDEX separado de DROP TABLE: grupo de captura do nome
  fica limpo (sem qualificador intermediário).