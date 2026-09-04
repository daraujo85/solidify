package migrations

import (
	"strings"
	"testing"
)

// findByCode devolve o primeiro Finding com o code pedido, ou nil.
func findByCode(fs []Finding, code string) *Finding {
	for i := range fs {
		if fs[i].Code == code {
			return &fs[i]
		}
	}
	return nil
}

func countCode(fs []Finding, code string) int {
	n := 0
	for _, f := range fs {
		if f.Code == code {
			n++
		}
	}
	return n
}

// Aceitação: DROP TABLE gera DROP_OBJECT critical.
func TestRiskDropTableIsCritical(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V1__drop.sql"}
	ops := []Operation{{Type: OpDrop, Object: "users", Detail: "TABLE", Raw: "DROP TABLE users;"}}
	fs := Assess(m, ops)
	f := findByCode(fs, "DROP_OBJECT")
	if f == nil {
		t.Fatalf("sem DROP_OBJECT: %+v", fs)
	}
	if f.Level != RiskCritical {
		t.Errorf("Level = %s, quero CRITICAL", f.Level)
	}
	if !strings.Contains(f.Message, "users") {
		t.Errorf("Message = %q", f.Message)
	}
}

// Aceitação: DROP COLUMN gera DROP_OBJECT high.
func TestRiskDropColumnIsHigh(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V2__drop_col.sql"}
	ops := []Operation{
		{Type: OpAlter, Object: "users", Detail: "legacy", Raw: "ALTER TABLE users DROP COLUMN legacy"},
		{Type: OpDrop, Object: "users", Detail: "legacy", Raw: "ALTER TABLE users DROP COLUMN legacy"},
	}
	fs := Assess(m, ops)
	f := findByCode(fs, "DROP_OBJECT")
	if f == nil {
		t.Fatalf("sem DROP_OBJECT: %+v", fs)
	}
	if f.Level != RiskHigh {
		t.Errorf("Level = %s, quero HIGH", f.Level)
	}
}

// Aceitação: DROP INDEX gera DROP_INDEX moderate.
func TestRiskDropIndexIsModerate(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V3__drop_idx.sql"}
	ops := []Operation{{Type: OpDropIndex, Object: "idx_old", Raw: "DROP INDEX idx_old"}}
	fs := Assess(m, ops)
	f := findByCode(fs, "DROP_INDEX")
	if f == nil {
		t.Fatalf("sem DROP_INDEX: %+v", fs)
	}
	if f.Level != RiskModerate {
		t.Errorf("Level = %s, quero MODERATE", f.Level)
	}
}

// Aceitação: CREATE INDEX sem CONCURRENTLY gera CREATE_INDEX_BLOCKING.
func TestRiskCreateIndexWithoutConcurrently(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V4__idx.sql"}
	ops := []Operation{{Type: OpCreateIndex, Object: "idx_x", Raw: "CREATE INDEX idx_x ON users(email)"}}
	fs := Assess(m, ops)
	f := findByCode(fs, "CREATE_INDEX_BLOCKING")
	if f == nil {
		t.Fatalf("sem CREATE_INDEX_BLOCKING: %+v", fs)
	}
	if f.Level != RiskModerate {
		t.Errorf("Level = %s", f.Level)
	}
}

// Aceitação: CREATE INDEX CONCURRENTLY NÃO gera blocking finding.
func TestRiskCreateIndexConcurrentlyNoBlocking(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V5__idx.sql"}
	ops := []Operation{{Type: OpCreateIndex, Object: "idx_x",
		Raw: "CREATE INDEX CONCURRENTLY idx_x ON users(email)"}}
	fs := Assess(m, ops)
	if findByCode(fs, "CREATE_INDEX_BLOCKING") != nil {
		t.Errorf("CONCURRENTLY deveria suprimir blocking: %+v", fs)
	}
}

// Aceitação: ADD COLUMN com NOT NULL sem DEFAULT gera NOT_NULL_NO_DEFAULT.
func TestRiskAddColumnNotNullNoDefault(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V6__notnull.sql"}
	ops := []Operation{{Type: OpAdd, Object: "users", Detail: "email",
		Raw: "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL"}}
	fs := Assess(m, ops)
	f := findByCode(fs, "NOT_NULL_NO_DEFAULT")
	if f == nil {
		t.Fatalf("sem NOT_NULL_NO_DEFAULT: %+v", fs)
	}
	if f.Level != RiskHigh {
		t.Errorf("Level = %s, quero HIGH", f.Level)
	}
}

// Aceitação: ADD COLUMN com NOT NULL E DEFAULT NÃO gera finding.
func TestRiskAddColumnNotNullWithDefaultOK(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V7__safe.sql"}
	ops := []Operation{{Type: OpAdd, Object: "users", Detail: "email",
		Raw: "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT ''"}}
	fs := Assess(m, ops)
	if findByCode(fs, "NOT_NULL_NO_DEFAULT") != nil {
		t.Errorf("com DEFAULT não devia marcar: %+v", fs)
	}
}

// Aceitação: ALTER COLUMN SET NOT NULL (sem backfill) gera finding.
func TestRiskAlterColumnSetNotNull(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V8__setnotnull.sql"}
	ops := []Operation{{Type: OpAlter, Object: "users", Detail: "email",
		Raw: "ALTER TABLE users ALTER COLUMN email SET NOT NULL"}}
	fs := Assess(m, ops)
	f := findByCode(fs, "NOT_NULL_NO_DEFAULT")
	if f == nil {
		t.Fatalf("devia marcar SET NOT NULL: %+v", fs)
	}
}

// Aceitação: UPDATE sem WHERE gera MASSIVE_UPDATE high.
func TestRiskUpdateWithoutWhereIsHigh(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V9__upd.sql"}
	ops := []Operation{{Type: OpUpdate, Object: "users",
		Raw: "UPDATE users SET active = false"}}
	fs := Assess(m, ops)
	f := findByCode(fs, "MASSIVE_UPDATE")
	if f == nil {
		t.Fatalf("sem MASSIVE_UPDATE: %+v", fs)
	}
	if f.Level != RiskHigh {
		t.Errorf("Level = %s, quero HIGH", f.Level)
	}
}

// Aceitação: UPDATE com WHERE NÃO gera MASSIVE_UPDATE.
func TestRiskUpdateWithWhereOK(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V10__upd.sql"}
	ops := []Operation{{Type: OpUpdate, Object: "users",
		Raw: "UPDATE users SET active = false WHERE created_at < '2024-01-01'"}}
	fs := Assess(m, ops)
	if findByCode(fs, "MASSIVE_UPDATE") != nil {
		t.Errorf("com WHERE não devia marcar: %+v", fs)
	}
}

// Aceitação: DELETE sem WHERE gera MASSIVE_DELETE critical.
func TestRiskDeleteWithoutWhereIsCritical(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V11__del.sql"}
	ops := []Operation{{Type: OpDelete, Object: "logs",
		Raw: "DELETE FROM logs"}}
	fs := Assess(m, ops)
	f := findByCode(fs, "MASSIVE_DELETE")
	if f == nil {
		t.Fatalf("sem MASSIVE_DELETE: %+v", fs)
	}
	if f.Level != RiskCritical {
		t.Errorf("Level = %s, quero CRITICAL", f.Level)
	}
}

// Aceitação: DELETE com WHERE NÃO gera MASSIVE_DELETE.
func TestRiskDeleteWithWhereOK(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V12__del.sql"}
	ops := []Operation{{Type: OpDelete, Object: "logs",
		Raw: "DELETE FROM logs WHERE created_at < '2024-01-01'"}}
	fs := Assess(m, ops)
	if findByCode(fs, "MASSIVE_DELETE") != nil {
		t.Errorf("com WHERE não devia marcar: %+v", fs)
	}
}

// Aceitação: RENAME gera RENAME_BREAKING moderate.
func TestRiskRenameIsModerate(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V13__ren.sql"}
	ops := []Operation{{Type: OpRename, Object: "users", Detail: "members"}}
	fs := Assess(m, ops)
	f := findByCode(fs, "RENAME_BREAKING")
	if f == nil {
		t.Fatalf("sem RENAME_BREAKING: %+v", fs)
	}
	if f.Level != RiskModerate {
		t.Errorf("Level = %s, quero MODERATE", f.Level)
	}
}

// Aceitação: migration Flyway sem rollback gera MISSING_ROLLBACK.
func TestRiskMissingRollbackForFlyway(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V14__x.sql", HasRollback: false}
	fs := Assess(m, nil)
	f := findByCode(fs, "MISSING_ROLLBACK")
	if f == nil {
		t.Fatalf("Flyway sem rollback devia marcar: %+v", fs)
	}
}

// Aceitação: migration Flyway COM rollback não gera MISSING_ROLLBACK.
func TestRiskHasRollbackNoFinding(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V15__x.sql", HasRollback: true}
	fs := Assess(m, nil)
	if findByCode(fs, "MISSING_ROLLBACK") != nil {
		t.Errorf("com rollback não devia marcar: %+v", fs)
	}
}

// Aceitação: migration Prisma sem rollback NÃO gera finding (Prisma não tem rollback built-in).
func TestRiskNoRollbackForPrismaOK(t *testing.T) {
	m := Migration{Framework: Prisma, Path: "prisma/migrations/x/migration.sql", HasRollback: false}
	fs := Assess(m, nil)
	if findByCode(fs, "MISSING_ROLLBACK") != nil {
		t.Errorf("Prisma não devia exigir rollback: %+v", fs)
	}
}

// Aceitação: migration Rails sem rollback gera MISSING_ROLLBACK.
func TestRiskMissingRollbackForRails(t *testing.T) {
	m := Migration{Framework: Rails, Path: "db/migrate/x.rb", HasRollback: false}
	fs := Assess(m, nil)
	if findByCode(fs, "MISSING_ROLLBACK") == nil {
		t.Errorf("Rails sem rollback devia marcar")
	}
}

// Aceitação: migration Django sem rollback gera MISSING_ROLLBACK.
func TestRiskMissingRollbackForDjango(t *testing.T) {
	m := Migration{Framework: Django, Path: "app/migrations/x.py", HasRollback: false}
	fs := Assess(m, nil)
	if findByCode(fs, "MISSING_ROLLBACK") == nil {
		t.Errorf("Django sem rollback devia marcar")
	}
}

// Aceitação: migration EFCore sem rollback gera MISSING_ROLLBACK.
func TestRiskMissingRollbackForEFCore(t *testing.T) {
	m := Migration{Framework: EFCore, Path: "src/Migrations/x.cs", HasRollback: false}
	fs := Assess(m, nil)
	if findByCode(fs, "MISSING_ROLLBACK") == nil {
		t.Errorf("EFCore sem rollback devia marcar")
	}
}

// Aceitação: migration Laravel sem rollback gera MISSING_ROLLBACK.
func TestRiskMissingRollbackForLaravel(t *testing.T) {
	m := Migration{Framework: Laravel, Path: "database/migrations/x.php", HasRollback: false}
	fs := Assess(m, nil)
	if findByCode(fs, "MISSING_ROLLBACK") == nil {
		t.Errorf("Laravel sem rollback devia marcar")
	}
}

// Aceitação: migration Liquibase sem rollback gera MISSING_ROLLBACK.
func TestRiskMissingRollbackForLiquibase(t *testing.T) {
	m := Migration{Framework: Liquibase, Path: "changelog.xml", HasRollback: false}
	fs := Assess(m, nil)
	if findByCode(fs, "MISSING_ROLLBACK") == nil {
		t.Errorf("Liquibase sem rollback devia marcar")
	}
}

// Aceitação: migration Generic SQL sem rollback NÃO gera finding.
func TestRiskNoRollbackForGenericSQLOK(t *testing.T) {
	m := Migration{Framework: GenericSQL, Path: "db/001.sql", HasRollback: false}
	fs := Assess(m, nil)
	if findByCode(fs, "MISSING_ROLLBACK") != nil {
		t.Errorf("GenericSQL não devia exigir rollback: %+v", fs)
	}
}

// Aceitação: CREATE TABLE puro não gera DROP, mas CREATE_INDEX e CREATE_OBJECT low.
func TestRiskCreateTableNoDataFindings(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V16__create.sql"}
	ops := []Operation{{Type: OpCreate, Object: "users", Detail: "TABLE", Raw: "CREATE TABLE users"}}
	fs := Assess(m, ops)
	for _, f := range fs {
		if f.Code == "DROP_OBJECT" || f.Code == "MASSIVE_UPDATE" {
			t.Errorf("criação pura não devia gerar finding destrutivo: %+v", f)
		}
	}
}

// Aceitação: assessRaw não emite nada se Raw vazio.
func TestRiskEmptyRawNoCrash(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V17__x.sql"}
	ops := []Operation{{Type: OpCreateIndex, Object: "idx", Raw: ""}}
	fs := Assess(m, ops)
	// CREATE_INDEX base (sem Raw) ainda emite CREATE_INDEX low, mas não blocking.
	if findByCode(fs, "CREATE_INDEX_BLOCKING") != nil {
		t.Errorf("Raw vazio não devia gerar blocking: %+v", fs)
	}
}

// Aceitação: ADD COLUMN puro (sem NOT NULL) só gera ADD_COLUMN low.
func TestRiskAddColumnSafeOnlyLow(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V18__add.sql"}
	ops := []Operation{{Type: OpAdd, Object: "users", Detail: "phone",
		Raw: "ALTER TABLE users ADD COLUMN phone VARCHAR(20)"}}
	fs := Assess(m, ops)
	if findByCode(fs, "ADD_COLUMN") == nil {
		t.Errorf("devia marcar ADD_COLUMN low: %+v", fs)
	}
	if findByCode(fs, "NOT_NULL_NO_DEFAULT") != nil {
		t.Errorf("ADD sem NOT NULL não devia marcar NOT_NULL: %+v", fs)
	}
}

// Aceitação: UPDATE com WHERE contendo backticks/aspas simples não confunde.
func TestRiskUpdateWhereInStringIgnored(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V19__upd.sql"}
	ops := []Operation{{Type: OpUpdate, Object: "users",
		Raw: "UPDATE users SET note = 'no where here' WHERE id = 1"}}
	fs := Assess(m, ops)
	if findByCode(fs, "MASSIVE_UPDATE") != nil {
		t.Errorf("devia detectar WHERE: %+v", fs)
	}
}

// Aceitação: vários findings na mesma migration.
func TestRiskMultipleFindingsInOneMigration(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V20__multi.sql", HasRollback: false}
	ops := []Operation{
		{Type: OpDrop, Object: "old_table", Detail: "TABLE", Raw: "DROP TABLE old_table"},
		{Type: OpAdd, Object: "users", Detail: "age",
			Raw: "ALTER TABLE users ADD COLUMN age INT NOT NULL"},
	}
	fs := Assess(m, ops)
	if findByCode(fs, "DROP_OBJECT") == nil {
		t.Error("sem DROP_OBJECT")
	}
	if findByCode(fs, "NOT_NULL_NO_DEFAULT") == nil {
		t.Error("sem NOT_NULL_NO_DEFAULT")
	}
	if findByCode(fs, "MISSING_ROLLBACK") == nil {
		t.Error("sem MISSING_ROLLBACK")
	}
	if findByCode(fs, "ADD_COLUMN") == nil {
		t.Error("sem ADD_COLUMN")
	}
}

// Aceitação: Migration com 0 ops não gera finding (a não ser MISSING_ROLLBACK).
func TestRiskEmptyOpsOnlyRollback(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V21__x.sql", HasRollback: false}
	fs := Assess(m, nil)
	if len(fs) != 1 || fs[0].Code != "MISSING_ROLLBACK" {
		t.Errorf("esperava só MISSING_ROLLBACK, got %+v", fs)
	}
}

// Aceitação: UNKNOWN op não gera findings estruturais.
func TestRiskUnknownOpNoFindings(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V22__x.sql"}
	ops := []Operation{{Type: OpUnknown, Object: "users", Detail: "INSERT",
		Raw: "INSERT INTO users VALUES (1)"}}
	fs := Assess(m, ops)
	for _, f := range fs {
		if f.Operation == string(OpUnknown) {
			t.Errorf("UNKNOWN não devia gerar finding: %+v", f)
		}
	}
}

// Aceitação: CREATE TABLE LOW apenas como baseline (não DROP, não destrutivo).
func TestRiskNoFindingsForHarmlessCreateTable(t *testing.T) {
	m := Migration{Framework: Flyway, Path: "db/migration/V23__create.sql", HasRollback: true}
	ops := []Operation{{Type: OpCreate, Object: "logs", Detail: "TABLE",
		Raw: "CREATE TABLE logs (id BIGINT)"}}
	fs := Assess(m, ops)
	if len(fs) != 0 {
		t.Errorf("criação inofensiva não devia gerar findings: %+v", fs)
	}
}

// Aceitação: ImpactForLevel/ImpactForFindings — enum de 3 níveis §13.
func TestImpactForLevel(t *testing.T) {
	cases := []struct {
		level RiskLevel
		want  Impact
	}{
		{RiskLow, ImpactLow},
		{RiskModerate, ImpactMedium},
		{RiskHigh, ImpactHigh},
		{RiskCritical, ImpactHigh},
		{RiskLevel("UNKNOWN"), Impact("")},
	}
	for _, c := range cases {
		if got := ImpactForLevel(c.level); got != c.want {
			t.Errorf("ImpactForLevel(%q) = %q, want %q", c.level, got, c.want)
		}
	}
}

func TestImpactForFindingsNoFindingsIsEmpty(t *testing.T) {
	if got := ImpactForFindings(nil); got != Impact("") {
		t.Errorf("sem findings devia ser \"\" (sem fabricar default), got %q", got)
	}
}

func TestImpactForFindingsAddColumnIsBaixo(t *testing.T) {
	// "add column nullable" = ADD_COLUMN LOW -> Baixo (contrato §13).
	m := Migration{Framework: GenericSQL, Path: "db/migrations/0032_add_query_index.sql"}
	ops := []Operation{{Type: OpAdd, Object: "orders", Detail: "note",
		Raw: "ALTER TABLE orders ADD COLUMN note TEXT"}}
	fs := Assess(m, ops)
	if got := ImpactForFindings(fs); got != ImpactLow {
		t.Errorf("ADD COLUMN nullable devia ser Baixo, got %q (findings=%+v)", got, fs)
	}
}

func TestImpactForFindingsDropTableIsAlto(t *testing.T) {
	// DROP TABLE = DROP_OBJECT CRITICAL -> Alto (pior finding vence).
	m := Migration{Framework: GenericSQL, Path: "db/migrations/0040_drop_legacy.sql"}
	ops := []Operation{{Type: OpDrop, Object: "legacy", Detail: "TABLE",
		Raw: "DROP TABLE legacy"}}
	fs := Assess(m, ops)
	if got := ImpactForFindings(fs); got != ImpactHigh {
		t.Errorf("DROP TABLE devia ser Alto, got %q (findings=%+v)", got, fs)
	}
}

func TestImpactForFindingsRenameIsMedio(t *testing.T) {
	// RENAME = RENAME_BREAKING MODERATE -> Médio.
	m := Migration{Framework: GenericSQL, Path: "db/migrations/0041_rename.sql"}
	ops := []Operation{{Type: OpRename, Object: "orders", Detail: "old_orders",
		Raw: "ALTER TABLE old_orders RENAME TO orders"}}
	fs := Assess(m, ops)
	if got := ImpactForFindings(fs); got != ImpactMedium {
		t.Errorf("RENAME devia ser Médio, got %q (findings=%+v)", got, fs)
	}
}
