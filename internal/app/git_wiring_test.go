package app

import (
	"strings"
	"testing"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/gitx"
	"github.com/diegoaraujo/solidify/internal/report"
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

// TestBuildEnvChanges: var nova com default -> added+HasDefault=true; var
// removida -> removed, HasDefault nil (nunca avaliado pra remoção); nome
// com SECRET -> LikelySecret=true; var referenciada em outro arquivo do
// mesmo diff -> aparece em Components/References; fora dos arquivos de doc
// -> ignorada (nunca fabrica env change de qualquer .env aleatório).
func TestBuildEnvChanges(t *testing.T) {
	diffFiles := []gitx.DiffFile{
		{
			Path: ".env.example",
			Hunks: []gitx.Hunk{{Content: strings.Join([]string{
				"+API_SECRET_TOKEN=changeme",
				"+FEATURE_FLAG_X=",
				"-OLD_VAR=1",
				" UNCHANGED_VAR=keep",
			}, "\n")}},
		},
		{
			Path: "app/config.go",
			Hunks: []gitx.Hunk{{Content: "+cfg := os.Getenv(\"API_SECRET_TOKEN\")"}},
		},
		{
			Path: ".env.production", // não é arquivo de doc -> ignorado
			Hunks: []gitx.Hunk{{Content: "+PROD_ONLY=1"}},
		},
	}
	cfg := config.Detectors{EnvDocumentationFiles: []string{".env.example"}}
	out := buildEnvChanges(diffFiles, cfg)

	byName := map[string]report.EnvChange{}
	for _, ec := range out {
		byName[ec.Name] = ec
	}
	if len(out) != 3 {
		t.Fatalf("esperava 3 env changes (API_SECRET_TOKEN, FEATURE_FLAG_X, OLD_VAR), veio %d: %+v", len(out), out)
	}
	if _, ok := byName["PROD_ONLY"]; ok {
		t.Errorf("PROD_ONLY não é de arquivo de doc, não devia aparecer")
	}

	tok := byName["API_SECRET_TOKEN"]
	if tok.Status != "added" || tok.HasDefault == nil || !*tok.HasDefault {
		t.Errorf("API_SECRET_TOKEN = %+v, queria added+HasDefault=true", tok)
	}
	if !tok.LikelySecret {
		t.Errorf("API_SECRET_TOKEN deveria ser LikelySecret")
	}
	if len(tok.Components) != 1 || tok.Components[0] != "app/config.go" {
		t.Errorf("API_SECRET_TOKEN.Components = %v, queria [app/config.go]", tok.Components)
	}

	flag := byName["FEATURE_FLAG_X"]
	if flag.HasDefault == nil || *flag.HasDefault {
		t.Errorf("FEATURE_FLAG_X sem valor deveria ter HasDefault=false, veio %v", flag.HasDefault)
	}

	old := byName["OLD_VAR"]
	if old.Status != "removed" {
		t.Errorf("OLD_VAR.Status = %q, queria removed", old.Status)
	}
	if old.HasDefault != nil {
		t.Errorf("remoção nunca avalia HasDefault, veio %v", *old.HasDefault)
	}
}
