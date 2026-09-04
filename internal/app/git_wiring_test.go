package app

import (
	"testing"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/gitx"
)

// TestBuildChangedFiles: binário não recebe added/deleted lines fabricados.
func TestBuildChangedFiles(t *testing.T) {
	changes := []gitx.FileChange{
		{Path: "a.go", Status: gitx.ChangeModified, Add: 10, Del: 2},
		{Path: "img.png", Status: gitx.ChangeAdded, IsBinary: true},
	}
	out := buildChangedFiles(changes)
	if len(out) != 2 {
		t.Fatalf("esperava 2 changed files, veio %d", len(out))
	}
	if out[0].AddedLines == nil || *out[0].AddedLines != 10 {
		t.Errorf("a.go AddedLines = %v, queria 10", out[0].AddedLines)
	}
	if out[1].AddedLines != nil || out[1].DeletedLines != nil {
		t.Errorf("binário não pode ter contagem de linhas fabricada: %+v", out[1])
	}
}

// TestBuildMigrations_RollbackOnlyWhenChecked: migration Flyway nova com
// sibling U no diff -> RollbackPresent=true; sem sibling -> RollbackPresent
// permanece nil (desconhecido), nunca "false" fabricado.
func TestBuildMigrations_RollbackOnlyWhenChecked(t *testing.T) {
	changes := []gitx.FileChange{
		{Path: "db/migration/V1__init.sql", Status: gitx.ChangeAdded},
		{Path: "db/migration/U1__init.sql", Status: gitx.ChangeAdded},
		{Path: "prisma/migrations/20240101_x/migration.sql", Status: gitx.ChangeModified},
	}
	cfg := config.Detectors{MigrationPaths: []string{"db/migration"}}
	out := buildMigrations(t.TempDir(), changes, cfg)

	var flyway, prisma *bool
	for i := range out {
		switch out[i].Path {
		case "db/migration/V1__init.sql":
			flyway = out[i].RollbackPresent
		case "prisma/migrations/20240101_x/migration.sql":
			prisma = out[i].RollbackPresent
		}
	}
	if flyway == nil || !*flyway {
		t.Errorf("Flyway V1 com sibling U1 no diff deveria ter RollbackPresent=true, veio %v", flyway)
	}
	if prisma != nil {
		t.Errorf("migration modificada sem convenção checável deveria ficar RollbackPresent=nil, veio %v", *prisma)
	}
}
