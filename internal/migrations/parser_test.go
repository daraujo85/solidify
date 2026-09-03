package migrations

import (
	"strings"
	"testing"
)

// findFirst devolve a primeira Operation do tipo pedido, ou nil.
func findFirst(ops []Operation, t OperationType) *Operation {
	for i := range ops {
		if ops[i].Type == t {
			return &ops[i]
		}
	}
	return nil
}

func countType(ops []Operation, t OperationType) int {
	n := 0
	for _, o := range ops {
		if o.Type == t {
			n++
		}
	}
	return n
}

// Aceitação: CREATE TABLE básico.
func TestParseCreateTable(t *testing.T) {
	ops := Parse(`CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100));`)
	op := findFirst(ops, OpCreate)
	if op == nil {
		t.Fatalf("não detectou CREATE: %+v", ops)
	}
	if op.Object != "users" {
		t.Errorf("Object = %q, quero users", op.Object)
	}
	if op.Detail != "TABLE" {
		t.Errorf("Detail = %q, quero TABLE", op.Detail)
	}
}

// Aceitação: CREATE TABLE IF NOT EXISTS.
func TestParseCreateTableIfNotExists(t *testing.T) {
	ops := Parse(`CREATE TABLE IF NOT EXISTS sessions (id BIGINT);`)
	op := findFirst(ops, OpCreate)
	if op == nil || op.Object != "sessions" {
		t.Fatalf("Object = %+v", ops)
	}
}

// Aceitação: CREATE VIEW.
func TestParseCreateView(t *testing.T) {
	ops := Parse(`CREATE VIEW active_users AS SELECT * FROM users WHERE active;`)
	op := findFirst(ops, OpCreate)
	if op == nil || op.Object != "active_users" {
		t.Fatalf("Object = %+v", ops)
	}
	if op.Detail != "VIEW" {
		t.Errorf("Detail = %q", op.Detail)
	}
}

// Aceitação: CREATE UNIQUE INDEX.
func TestParseCreateUniqueIndex(t *testing.T) {
	ops := Parse(`CREATE UNIQUE INDEX idx_users_email ON users(email);`)
	op := findFirst(ops, OpCreateIndex)
	if op == nil {
		t.Fatalf("não detectou CREATE_INDEX: %+v", ops)
	}
	if op.Object != "idx_users_email" {
		t.Errorf("Object = %q", op.Object)
	}
	if op.Detail != "UNIQUE" {
		t.Errorf("Detail = %q, quero UNIQUE", op.Detail)
	}
}

// Aceitação: CREATE INDEX IF NOT EXISTS.
func TestParseCreateIndexIfNotExists(t *testing.T) {
	ops := Parse(`CREATE INDEX IF NOT EXISTS idx_orders ON orders(total);`)
	op := findFirst(ops, OpCreateIndex)
	if op == nil || op.Object != "idx_orders" {
		t.Fatalf("Object = %+v", ops)
	}
}

// Aceitação: CREATE OR REPLACE → UNKNOWN (não sabemos se cria ou destrói).
func TestParseCreateOrReplaceIsUnknown(t *testing.T) {
	ops := Parse(`CREATE OR REPLACE VIEW v_users AS SELECT 1;`)
	op := findFirst(ops, OpUnknown)
	if op == nil {
		t.Fatalf("devia ser UNKNOWN: %+v", ops)
	}
	if !strings.Contains(op.Detail, "CREATE OR REPLACE") {
		t.Errorf("Detail = %q", op.Detail)
	}
}

// Aceitação: DROP TABLE.
func TestParseDropTable(t *testing.T) {
	ops := Parse(`DROP TABLE old_users;`)
	op := findFirst(ops, OpDrop)
	if op == nil || op.Object != "old_users" {
		t.Fatalf("Object = %+v", ops)
	}
	if op.Detail != "TABLE" {
		t.Errorf("Detail = %q", op.Detail)
	}
}

// Aceitação: DROP TABLE IF EXISTS.
func TestParseDropTableIfExists(t *testing.T) {
	ops := Parse(`DROP TABLE IF EXISTS temp_table;`)
	op := findFirst(ops, OpDrop)
	if op == nil || op.Object != "temp_table" {
		t.Fatalf("Object = %+v", ops)
	}
}

// Aceitação: DROP INDEX.
func TestParseDropIndex(t *testing.T) {
	ops := Parse(`DROP INDEX idx_old;`)
	op := findFirst(ops, OpDropIndex)
	if op == nil || op.Object != "idx_old" {
		t.Fatalf("Object = %+v", ops)
	}
}

// Aceitação: DROP VIEW.
func TestParseDropView(t *testing.T) {
	ops := Parse(`DROP VIEW v_old;`)
	op := findFirst(ops, OpDrop)
	if op == nil || op.Detail != "VIEW" {
		t.Fatalf("Detail = %+v", ops)
	}
}

// Aceitação: ALTER TABLE ADD COLUMN.
func TestParseAlterAddColumn(t *testing.T) {
	ops := Parse(`ALTER TABLE users ADD COLUMN email VARCHAR(255);`)
	op := findFirst(ops, OpAdd)
	if op == nil {
		t.Fatalf("não detectou ADD: %+v", ops)
	}
	if op.Object != "users" {
		t.Errorf("Object = %q", op.Object)
	}
	if op.Detail != "email" {
		t.Errorf("Detail = %q", op.Detail)
	}
}

// Aceitação: ALTER TABLE ADD (sem COLUMN).
func TestParseAlterAddWithoutColumnKeyword(t *testing.T) {
	ops := Parse(`ALTER TABLE accounts ADD phone VARCHAR(20);`)
	op := findFirst(ops, OpAdd)
	if op == nil || op.Detail != "phone" {
		t.Fatalf("Object = %+v", ops)
	}
}

// Aceitação: ALTER TABLE DROP COLUMN.
func TestParseAlterDropColumn(t *testing.T) {
	ops := Parse(`ALTER TABLE users DROP COLUMN legacy_field;`)
	op := findFirst(ops, OpDrop)
	if op == nil {
		t.Fatalf("não detectou DROP: %+v", ops)
	}
	if op.Detail != "legacy_field" {
		t.Errorf("Detail = %q", op.Detail)
	}
}

// Aceitação: ALTER TABLE RENAME TO.
func TestParseAlterRenameTo(t *testing.T) {
	ops := Parse(`ALTER TABLE old_users RENAME TO users_v2;`)
	op := findFirst(ops, OpRename)
	if op == nil || op.Object != "old_users" {
		t.Fatalf("Object = %+v", ops)
	}
	if op.Detail != "users_v2" {
		t.Errorf("Detail = %q", op.Detail)
	}
}

// Aceitação: ALTER TABLE RENAME COLUMN.
func TestParseAlterRenameColumn(t *testing.T) {
	ops := Parse(`ALTER TABLE users RENAME COLUMN name TO full_name;`)
	op := findFirst(ops, OpRename)
	if op == nil {
		t.Fatalf("não detectou RENAME: %+v", ops)
	}
	if !strings.Contains(op.Detail, "name") || !strings.Contains(op.Detail, "full_name") {
		t.Errorf("Detail = %q", op.Detail)
	}
}

// Aceitação: ALTER TABLE ALTER COLUMN TYPE.
func TestParseAlterAlterColumn(t *testing.T) {
	ops := Parse(`ALTER TABLE users ALTER COLUMN age TYPE BIGINT;`)
	op := findFirst(ops, OpAlter)
	if op == nil || op.Detail != "age" {
		t.Fatalf("Detail = %+v", ops)
	}
}

// Aceitação: ALTER TABLE ADD CONSTRAINT → ALTER genérico.
func TestParseAlterAddConstraintIsGenericAlter(t *testing.T) {
	ops := Parse(`ALTER TABLE users ADD CONSTRAINT fk_org FOREIGN KEY (org_id) REFERENCES orgs(id);`)
	op := findFirst(ops, OpAlter)
	if op == nil {
		t.Fatalf("não detectou ALTER: %+v", ops)
	}
	if op.Object != "users" {
		t.Errorf("Object = %q", op.Object)
	}
}

// Aceitação: UPDATE.
func TestParseUpdate(t *testing.T) {
	ops := Parse(`UPDATE users SET active = true WHERE id = 1;`)
	op := findFirst(ops, OpUpdate)
	if op == nil || op.Object != "users" {
		t.Fatalf("Object = %+v", ops)
	}
}

// Aceitação: DELETE FROM.
func TestParseDeleteFrom(t *testing.T) {
	ops := Parse(`DELETE FROM users WHERE created_at < '2024-01-01';`)
	op := findFirst(ops, OpDelete)
	if op == nil || op.Object != "users" {
		t.Fatalf("Object = %+v", ops)
	}
}

// Aceitação: INSERT INTO → UNKNOWN (não sabemos se destrói).
func TestParseInsertIsUnknown(t *testing.T) {
	ops := Parse(`INSERT INTO users (name) VALUES ('foo');`)
	op := findFirst(ops, OpUnknown)
	if op == nil {
		t.Fatalf("devia ser UNKNOWN: %+v", ops)
	}
	if !strings.Contains(op.Detail, "INSERT") {
		t.Errorf("Detail = %q", op.Detail)
	}
}

// Aceitação: BEGIN/COMMIT/SET → UNKNOWN explícito.
func TestParseControlStatementsAreUnknown(t *testing.T) {
	ops := Parse(`BEGIN; SET search_path TO public; COMMIT;`)
	if countType(ops, OpUnknown) != 3 {
		t.Errorf("UNKNOWN count = %d, quero 3: %+v", countType(ops, OpUnknown), ops)
	}
}

// Aceitação: SELECT puro → UNKNOWN.
func TestParseSelectIsUnknown(t *testing.T) {
	ops := Parse(`SELECT count(*) FROM users;`)
	if findFirst(ops, OpUnknown) == nil {
		t.Errorf("devia ser UNKNOWN: %+v", ops)
	}
}

// Aceitação: TRUNCATE → UNKNOWN (poderia ser DROP, conservador fica UNKNOWN).
func TestParseTruncateIsUnknown(t *testing.T) {
	ops := Parse(`TRUNCATE TABLE logs;`)
	if findFirst(ops, OpUnknown) == nil {
		t.Errorf("devia ser UNKNOWN: %+v", ops)
	}
}

// Aceitação: múltiplos statements em uma migration.
func TestParseMultipleStatements(t *testing.T) {
	sql := `
CREATE TABLE a (id INT);
ALTER TABLE a ADD COLUMN name VARCHAR(50);
CREATE INDEX idx_a_name ON a(name);
DROP TABLE b;
INSERT INTO c VALUES (1);
`
	ops := Parse(sql)
	if len(ops) != 5 {
		t.Fatalf("len = %d, quero 5: %+v", len(ops), ops)
	}
	if findFirst(ops, OpCreate) == nil {
		t.Error("sem CREATE")
	}
	if findFirst(ops, OpAdd) == nil {
		t.Error("sem ADD")
	}
	if findFirst(ops, OpCreateIndex) == nil {
		t.Error("sem CREATE_INDEX")
	}
	if findFirst(ops, OpDrop) == nil {
		t.Error("sem DROP")
	}
	if findFirst(ops, OpUnknown) == nil {
		t.Error("sem UNKNOWN (INSERT)")
	}
}

// Aceitação: comentários linha e bloco são removidos.
func TestParseStripsComments(t *testing.T) {
	sql := `
-- isso é um comentário com DROP TABLE no meio
/* bloco
   CREATE TABLE evil (id INT);
*/
CREATE TABLE good (id INT);
`
	ops := Parse(sql)
	if len(ops) != 1 {
		t.Fatalf("len = %d, quero 1: %+v", len(ops), ops)
	}
	if ops[0].Type != OpCreate || ops[0].Object != "good" {
		t.Errorf("op = %+v", ops[0])
	}
}

// Aceitação: strings literais não vazam como DROP/CREATE.
func TestParseIgnoresKeywordsInStrings(t *testing.T) {
	sql := `INSERT INTO logs (msg) VALUES ('DROP TABLE users;');`
	ops := Parse(sql)
	if len(ops) != 1 {
		t.Fatalf("len = %d, quero 1: %+v", len(ops), ops)
	}
	if ops[0].Type != OpUnknown {
		t.Errorf("op = %+v, devia ser UNKNOWN (INSERT com DROP dentro de string)", ops[0])
	}
}

// Aceitação: identificadores com aspas duplas (Postgres) preservam normalidade.
func TestParseQuotedIdentifiers(t *testing.T) {
	sql := `CREATE TABLE "User" (id INT);`
	ops := Parse(sql)
	op := findFirst(ops, OpCreate)
	if op == nil || op.Object != "user" {
		t.Fatalf("op = %+v", op)
	}
}

// Aceitação: CREATE FUNCTION → UNKNOWN (PL/SQL, fora do escopo).
func TestParseCreateFunctionIsUnknown(t *testing.T) {
	sql := `CREATE FUNCTION notify_user() RETURNS trigger AS $$
BEGIN
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;`
	ops := Parse(sql)
	if findFirst(ops, OpUnknown) == nil {
		t.Errorf("devia ser UNKNOWN: %+v", ops)
	}
}

// Aceitação: migration vazia.
func TestParseEmpty(t *testing.T) {
	if ops := Parse(""); len(ops) != 0 {
		t.Errorf("len = %d, quero 0", len(ops))
	}
	if ops := Parse("-- só comentário\n\n"); len(ops) != 0 {
		t.Errorf("comentário puro: len = %d, quero 0", len(ops))
	}
}

// Aceitação: SQL ofuscado (sintaxe que não reconhecemos) → UNKNOWN.
func TestParseUnknownSyntaxIsUnknown(t *testing.T) {
	sql := `MERGE INTO users USING staging ON ... WHEN MATCHED THEN UPDATE SET ...;`
	ops := Parse(sql)
	if findFirst(ops, OpUnknown) == nil {
		t.Errorf("devia ser UNKNOWN: %+v", ops)
	}
}

// Aceitação: GRANT/REVOKE → UNKNOWN.
func TestParseGrantIsUnknown(t *testing.T) {
	ops := Parse(`GRANT SELECT ON users TO readonly;`)
	if findFirst(ops, OpUnknown) == nil {
		t.Errorf("devia ser UNKNOWN: %+v", ops)
	}
}

// Aceitação: case-insensitive (upper ou lower).
func TestParseIsCaseInsensitive(t *testing.T) {
	sql := `create table lower_case (id int);`
	ops := Parse(sql)
	op := findFirst(ops, OpCreate)
	if op == nil || op.Object != "lower_case" {
		t.Fatalf("op = %+v", op)
	}
}

// Aceitação: ALTER TABLE ONLY (Postgres).
func TestParseAlterTableOnly(t *testing.T) {
	ops := Parse(`ALTER TABLE ONLY users ADD COLUMN slug VARCHAR(50);`)
	op := findFirst(ops, OpAdd)
	if op == nil || op.Object != "users" {
		t.Fatalf("op = %+v", op)
	}
}

// Aceitação: Vários tipos no mesmo migration → contagem certa.
func TestParseCounts(t *testing.T) {
	sql := `
CREATE TABLE a (id INT);
CREATE TABLE b (id INT);
CREATE INDEX x ON a(id);
DROP TABLE c;
ALTER TABLE d ADD COLUMN e INT;
UPDATE f SET g = 1;
DELETE FROM h;
`
	ops := Parse(sql)
	if got := countType(ops, OpCreate); got != 2 {
		t.Errorf("CREATE count = %d, quero 2", got)
	}
	if got := countType(ops, OpCreateIndex); got != 1 {
		t.Errorf("CREATE_INDEX count = %d, quero 1", got)
	}
	if got := countType(ops, OpDrop); got != 1 {
		t.Errorf("DROP count = %d, quero 1", got)
	}
	if got := countType(ops, OpAdd); got != 1 {
		t.Errorf("ADD count = %d, quero 1", got)
	}
	if got := countType(ops, OpUpdate); got != 1 {
		t.Errorf("UPDATE count = %d, quero 1", got)
	}
	if got := countType(ops, OpDelete); got != 1 {
		t.Errorf("DELETE count = %d, quero 1", got)
	}
}

// Aceitação: CREATE TYPE (Postgres enum) → CREATE.
func TestParseCreateType(t *testing.T) {
	ops := Parse(`CREATE TYPE status AS ENUM ('active', 'inactive');`)
	op := findFirst(ops, OpCreate)
	if op == nil || op.Detail != "TYPE" {
		t.Fatalf("op = %+v", op)
	}
}

// Aceitação: semicolon ausente no final → não explode.
func TestParseMissingTrailingSemicolon(t *testing.T) {
	ops := Parse(`CREATE TABLE orphan (id INT)`)
	if findFirst(ops, OpCreate) == nil {
		t.Errorf("devia detectar CREATE mesmo sem ;: %+v", ops)
	}
}

// Aceitação: identifier quoted com nome que parece keyword.
func TestParseQuotedNameIsTreatedAsLiteral(t *testing.T) {
	sql := `CREATE TABLE "select" (id INT);`
	ops := Parse(sql)
	op := findFirst(ops, OpCreate)
	if op == nil || op.Object != "select" {
		t.Fatalf("op = %+v", op)
	}
}

// Aceitação: DROP SCHEMA / DROP FUNCTION / DROP TRIGGER.
func TestParseDropVariants(t *testing.T) {
	cases := []struct {
		sql, obj, kind string
	}{
		{`DROP SCHEMA old CASCADE;`, "old", "SCHEMA"},
		{`DROP FUNCTION old_fn();`, "old_fn()", "FUNCTION"},
		{`DROP TRIGGER old_trg ON users;`, "users", "TRIGGER"},
	}
	for _, tc := range cases {
		ops := Parse(tc.sql)
		op := findFirst(ops, OpDrop)
		if op == nil || op.Detail != tc.kind {
			t.Errorf("%s: op = %+v, quero kind=%s", tc.sql, op, tc.kind)
		}
	}
}
