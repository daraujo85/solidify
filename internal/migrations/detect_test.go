package migrations

import (
	"sort"
	"strings"
	"testing"
)

func equalStr(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedPaths(ms []Migration) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, string(m.Framework)+":"+m.Path)
	}
	sort.Strings(out)
	return out
}

// Aceitação: cada framework reconhece seu padrão canônico.
func TestDetectEachFramework(t *testing.T) {
	cases := []struct {
		path string
		fw   Framework
	}{
		{"db/migration/V1__init.sql", Flyway},
		{"db/migration/V100__add_users.sql", Flyway},
		{"db/migration/R__repeatable_seed.sql", Flyway},
		{"db/migration/U1__undo_init.sql", Flyway},
		{"db/changelog/db.changelog-master.yaml", Liquibase},
		{"resources/liquibase/changelog-1.xml", Liquibase},
		{"src/Api/Migrations/20240101_InitialMigration.cs", EFCore},
		{"prisma/migrations/20240101_init/migration.sql", Prisma},
		{"migrations/1700000000-AddEmail.ts", TypeORM},
		{"migrations/1700000000-create-users.js", Sequelize},
		{"migrations/20240101_create_users.js", Knex},
		{"app/migrations/0001_initial.py", Django},
		{"db/migrate/20240101120000_create_users.rb", Rails},
		{"database/migrations/2024_01_01_000000_create_users.php", Laravel},
	}
	for _, tc := range cases {
		t.Run(string(tc.fw)+"/"+tc.path, func(t *testing.T) {
			dms := []DetectedMigration{{Path: tc.path, Status: Added}}
			got := Detect(dms, nil)
			if len(got) != 1 {
				t.Fatalf("Detect = %+v, quero 1", got)
			}
			if got[0].Framework != tc.fw {
				t.Errorf("framework = %s, quero %s", got[0].Framework, tc.fw)
			}
		})
	}
}

// Aceitação: versão e descrição extraídos para Flyway.
func TestDetectFlywayVersionDescription(t *testing.T) {
	dms := []DetectedMigration{{Path: "db/migration/V1__init.sql", Status: Added}}
	got := Detect(dms, nil)
	if got[0].Version != "1" {
		t.Errorf("Version = %q, quero 1", got[0].Version)
	}
	if got[0].Description != "init" {
		t.Errorf("Description = %q, quero init", got[0].Description)
	}
}

// Aceitação: Prisma extrai timestamp e descrição.
func TestDetectPrismaVersionDescription(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "prisma/migrations/20240101_add_email/migration.sql", Status: Added},
	}
	got := Detect(dms, nil)
	if got[0].Version != "20240101" {
		t.Errorf("Version = %q", got[0].Version)
	}
	if got[0].Description != "add email" {
		t.Errorf("Description = %q, quero 'add email'", got[0].Description)
	}
}

// Aceitação: Sequelize usa "-" no timestamp.
func TestDetectSequelizeVersionDescription(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "migrations/1700000000-add-email.js", Status: Added},
	}
	got := Detect(dms, nil)
	if got[0].Version != "1700000000" {
		t.Errorf("Version = %q", got[0].Version)
	}
	if got[0].Description != "add email" {
		t.Errorf("Description = %q, quero 'add email'", got[0].Description)
	}
}

// Aceitação: paths não-migration são ignorados.
func TestDetectIgnoresNonMigration(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "apps/api/server.go", Status: Added},
		{Path: "README.md", Status: Added},
		{Path: "src/utils.ts", Status: Added},
	}
	got := Detect(dms, []string{"migrations"})
	if len(got) != 0 {
		t.Fatalf("não-migrations = %+v, quero vazio", got)
	}
}

// Aceitação: generic SQL em diretórios configurados.
func TestDetectGenericSQLInConfiguredDirs(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "migrations/001_init.sql", Status: Added},
		{Path: "db/migrate/002_users.sql", Status: Added},
		{Path: "alembic/versions/003_email.sql", Status: Modified},
		// .sql fora de diretório configurado: ignorado.
		{Path: "scripts/queries.sql", Status: Added},
	}
	got := Detect(dms, []string{"migrations", "db/migrate", "alembic/versions"})
	if len(got) != 3 {
		t.Fatalf("got %d, quero 3: %+v", len(got), sortedPaths(got))
	}
	want := []string{
		"generic-sql:alembic/versions/003_email.sql",
		"generic-sql:db/migrate/002_users.sql",
		"generic-sql:migrations/001_init.sql",
	}
	if !equalStr(sortedPaths(got), want) {
		t.Errorf("got %v, quero %v", sortedPaths(got), want)
	}
	for _, m := range got {
		if m.Framework != GenericSQL {
			t.Errorf("%s: framework = %s, quero generic-sql", m.Path, m.Framework)
		}
	}
}

// Aceitação: status é preservado.
func TestDetectPreservesStatus(t *testing.T) {
	cases := []ChangeStatus{Added, Modified, Deleted, Renamed}
	for _, st := range cases {
		t.Run(string(st), func(t *testing.T) {
			dms := []DetectedMigration{
				{Path: "prisma/migrations/20240101_x/migration.sql", Status: st},
			}
			got := Detect(dms, nil)
			if got[0].Status != st {
				t.Errorf("Status = %s, quero %s", got[0].Status, st)
			}
		})
	}
}

// Aceitação: HasRollback inferido pelo path do rollback.
func TestDetectRollbackFlag(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "prisma/migrations/20240101_x/migration.sql", Status: Added,
			Rollback: "prisma/migrations/20240101_x/rollback.sql"},
		{Path: "prisma/migrations/20240102_y/migration.sql", Status: Added,
			Rollback: ""},
	}
	got := Detect(dms, nil)
	if !got[0].HasRollback {
		t.Errorf("deveria ter rollback")
	}
	if got[0].RollbackOf != "prisma/migrations/20240101_x/rollback.sql" {
		t.Errorf("RollbackOf = %q", got[0].RollbackOf)
	}
	if got[1].HasRollback {
		t.Errorf("não deveria ter rollback")
	}
}

// Aceitação: Rails requer db/migrate/ no path.
func TestDetectRailsRequiresDbMigratePath(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "lib/20240101_create_users.rb", Status: Added},
	}
	got := Detect(dms, nil)
	if len(got) != 0 {
		t.Errorf("path errado casou Rails: %+v", got)
	}
}

// Aceitação: Django migrations/<...>.py.
func TestDetectDjango(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "app/users/migrations/0001_initial.py", Status: Added},
	}
	got := Detect(dms, nil)
	if len(got) != 1 || got[0].Framework != Django {
		t.Fatalf("Django não detectado: %+v", got)
	}
	if got[0].Version != "0001" {
		t.Errorf("Version = %q", got[0].Version)
	}
	if got[0].Description != "initial" {
		t.Errorf("Description = %q", got[0].Description)
	}
}

// Aceitação: Liquibase em resources/.
func TestDetectLiquibaseInResources(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "src/main/resources/liquibase/changelog/001-init.yaml", Status: Added},
	}
	got := Detect(dms, nil)
	if len(got) != 1 || got[0].Framework != Liquibase {
		t.Errorf("Liquibase não detectado: %+v", got)
	}
}

// Aceitação: Prisma migration.sql fora de prisma/migrations/ não casa.
func TestDetectPrismaRequiresPath(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "db/20240101_init/migration.sql", Status: Added},
	}
	got := Detect(dms, nil)
	if len(got) != 0 {
		t.Errorf("path errado casou Prisma: %+v", got)
	}
}

// Aceitação: Knex usa "_" no timestamp.
func TestDetectKnexTimestampPrefix(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "migrations/20240101_create_users.js", Status: Added},
	}
	got := Detect(dms, nil)
	if got[0].Framework != Knex {
		t.Errorf("framework = %s", got[0].Framework)
	}
	if got[0].Version != "20240101" {
		t.Errorf("Version = %q", got[0].Version)
	}
}

// Aceitação: range com múltiplos frameworks.
func TestDetectMultipleFrameworks(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "prisma/migrations/20240101_init/migration.sql", Status: Added},
		{Path: "db/migrate/20240101120000_add_age.rb", Status: Added},
		{Path: "app/migrations/0001_initial.py", Status: Added},
		{Path: "db/migration/V2__seed.sql", Status: Added},
	}
	got := Detect(dms, nil)
	if len(got) != 4 {
		t.Fatalf("got %d, quero 4: %+v", len(got), sortedPaths(got))
	}
	want := []string{"django", "flyway", "prisma", "rails"}
	gotFw := []string{
		string(got[0].Framework),
		string(got[1].Framework),
		string(got[2].Framework),
		string(got[3].Framework),
	}
	sort.Strings(gotFw)
	if !equalStr(gotFw, want) {
		t.Errorf("frameworks = %v, quero %v", gotFw, want)
	}
}

// Sanidade: constantes exportadas.
func TestConstants(t *testing.T) {
	if GenericSQL != "generic-sql" {
		t.Errorf("GenericSQL = %q", GenericSQL)
	}
	if Unknown != "unknown" {
		t.Errorf("Unknown = %q", Unknown)
	}
	if Flyway != "flyway" {
		t.Errorf("Flyway = %q", Flyway)
	}
}

// Aceitação: TypeORM sem migrations/ no path não casa.
func TestDetectTypeORMRequiresMigrationsDir(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "src/entities/User.ts", Status: Added},
	}
	got := Detect(dms, nil)
	if len(got) != 0 {
		t.Errorf("TypeORM casou errado: %+v", got)
	}
}

// Aceitação: Laravel requer database/migrations/.
func TestDetectLaravelRequiresPath(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "app/2024_01_01_create_users.php", Status: Added},
	}
	got := Detect(dms, nil)
	if len(got) != 0 {
		t.Errorf("Laravel casou errado: %+v", got)
	}
}

// Aceitação: versão inválida (sem "_") vira ok=false.
func TestDetectKnexWithoutTimestamp(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "migrations/init.js", Status: Added},
	}
	got := Detect(dms, nil)
	if len(got) != 0 {
		t.Errorf("Knex sem timestamp casou: %+v", got)
	}
}

// Aceitação: ordenado por path, depois framework.
func TestDetectOrderIsDeterministic(t *testing.T) {
	dms := []DetectedMigration{
		{Path: "z/migration.sql", Status: Added},
		{Path: "a/migration.sql", Status: Added},
	}
	got := Detect(dms, []string{".", "z", "a"})
	if !strings.HasPrefix(got[0].Path, "a") {
		t.Errorf("ordenação errada: %+v", got)
	}
}
