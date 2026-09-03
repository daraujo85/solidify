package evidence

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/diegoaraujo/solidify/internal/migrations"
)

// Aceitação: New preenche runID, schema e timestamp.
func TestNew(t *testing.T) {
	e := New("r1")
	if e.RunID != "r1" {
		t.Errorf("runID = %s", e.RunID)
	}
	if e.Schema != SchemaVersion {
		t.Errorf("schema = %s", e.Schema)
	}
	if e.GeneratedAt.IsZero() {
		t.Errorf("generatedAt zero")
	}
}

// Aceitação: Marshal serializa JSON válido.
func TestMarshal(t *testing.T) {
	e := New("r1")
	data, err := e.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed["run_id"] != "r1" {
		t.Errorf("run_id = %v", parsed["run_id"])
	}
}

// Aceitação: Marshal nil falha.
func TestMarshalNil(t *testing.T) {
	var e *Evidence
	if _, err := e.Marshal(); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: AddLimit acumula.
func TestAddLimit(t *testing.T) {
	e := New("r1")
	e.AddLimit("scope: 1 analyzer")
	e.AddLimit("scope: dotenv only")
	if len(e.Limits) != 2 {
		t.Errorf("len = %d", len(e.Limits))
	}
}

// Aceitação: AddComponent.
func TestAddComponent(t *testing.T) {
	e := New("r1")
	e.AddComponent(ComponentEvidence{Name: "api", Framework: "spring"})
	e.AddComponent(ComponentEvidence{Name: "worker", Framework: "python"})
	if len(e.Components) != 2 {
		t.Errorf("len = %d", len(e.Components))
	}
}

// Aceitação: AddComponent inicializa Paths vazio.
func TestAddComponentInitPaths(t *testing.T) {
	e := New("r1")
	e.AddComponent(ComponentEvidence{Name: "x"})
	if e.Components[0].Paths == nil {
		t.Errorf("paths nil")
	}
}

// Aceitação: AddMigration.
func TestAddMigration(t *testing.T) {
	e := New("r1")
	e.AddMigration(MigrationEvidence{
		File:       "001_init.sql",
		Framework:  "flyway",
		Operations: []migrations.OperationType{migrations.OpCreate, migrations.OpAdd},
		RiskLevel:  "high",
		Findings:   []string{"DROP TABLE"},
	})
	if len(e.Migrations) != 1 {
		t.Errorf("len = %d", len(e.Migrations))
	}
}

// Aceitação: AddMigration inicializa slices.
func TestAddMigrationInit(t *testing.T) {
	e := New("r1")
	e.AddMigration(MigrationEvidence{File: "x"})
	if e.Migrations[0].Operations == nil {
		t.Errorf("operations nil")
	}
	if e.Migrations[0].Findings == nil {
		t.Errorf("findings nil")
	}
}

// Aceitação: SetEnv dedupe e ordena.
func TestSetEnv(t *testing.T) {
	e := New("r1")
	e.SetEnv(
		[]string{"B", "A", "B"},
		[]string{"X", "X"},
		[]string{"Z", "A"},
		[]string{"Q"},
		[]string{"KEY"},
	)
	if !sortedEqual(e.Env.Used, []string{"A", "B"}) {
		t.Errorf("used = %v", e.Env.Used)
	}
	if !sortedEqual(e.Env.Documented, []string{"X"}) {
		t.Errorf("documented = %v", e.Env.Documented)
	}
	if !sortedEqual(e.Env.Added, []string{"A", "Z"}) {
		t.Errorf("added = %v", e.Env.Added)
	}
	if !sortedEqual(e.Env.Removed, []string{"Q"}) {
		t.Errorf("removed = %v", e.Env.Removed)
	}
	if !sortedEqual(e.Env.Secrets, []string{"KEY"}) {
		t.Errorf("secrets = %v", e.Env.Secrets)
	}
}

// Aceitação: SetEnv com nil inicializa slices vazios.
func TestSetEnvNil(t *testing.T) {
	e := New("r1")
	e.SetEnv(nil, nil, nil, nil, nil)
	if e.Env.Used == nil {
		t.Errorf("used nil")
	}
}

// Aceitação: SetGit.
func TestSetGit(t *testing.T) {
	e := New("r1")
	e.SetGit(GitEvidence{
		BaseSHA:     "abc123",
		HeadSHA:     "def456",
		BaseRef:     "main",
		HeadRef:     "feature/x",
		CommitCount: 5,
	})
	if e.Git.BaseSHA != "abc123" || e.Git.CommitCount != 5 {
		t.Errorf("git = %+v", e.Git)
	}
}

// Aceitação: SetChanges ordena paths.
func TestSetChanges(t *testing.T) {
	e := New("r1")
	e.SetChanges(3, 10, 5,
		[]string{"z.go", "a.go", "m.go"},
		map[string]int{"go": 2, "md": 1})
	if e.Changes.FilesChanged != 3 {
		t.Errorf("files = %d", e.Changes.FilesChanged)
	}
	if e.Changes.Paths[0] != "a.go" {
		t.Errorf("paths = %v", e.Changes.Paths)
	}
	if e.Changes.ByLanguage["go"] != 2 {
		t.Errorf("byLang = %v", e.Changes.ByLanguage)
	}
}

// Aceitação: SetChanges com nil inicializa.
func TestSetChangesNil(t *testing.T) {
	e := New("r1")
	e.SetChanges(0, 0, 0, nil, nil)
	if e.Changes.Paths == nil {
		t.Errorf("paths nil")
	}
	if e.Changes.ByLanguage == nil {
		t.Errorf("byLang nil")
	}
}

// Aceitação: SortedComponents por nome.
func TestSortedComponents(t *testing.T) {
	e := New("r1")
	e.AddComponent(ComponentEvidence{Name: "z"})
	e.AddComponent(ComponentEvidence{Name: "a"})
	e.AddComponent(ComponentEvidence{Name: "m"})
	out := e.SortedComponents()
	want := []string{"a", "m", "z"}
	for i, c := range out {
		if c.Name != want[i] {
			t.Errorf("[%d] = %s, quero %s", i, c.Name, want[i])
		}
	}
}

// Aceitação: SortedMigrations por file.
func TestSortedMigrations(t *testing.T) {
	e := New("r1")
	e.AddMigration(MigrationEvidence{File: "z.sql"})
	e.AddMigration(MigrationEvidence{File: "a.sql"})
	out := e.SortedMigrations()
	if out[0].File != "a.sql" || out[1].File != "z.sql" {
		t.Errorf("sort = %v", out)
	}
}

// Aceitação: JSON marshal inclui todos os campos principais.
func TestMarshalAllFields(t *testing.T) {
	e := New("r1")
	e.SetGit(GitEvidence{BaseSHA: "a", HeadSHA: "b"})
	e.SetChanges(1, 1, 1, []string{"x"}, nil)
	e.AddComponent(ComponentEvidence{Name: "c"})
	e.AddMigration(MigrationEvidence{File: "m", Operations: []migrations.OperationType{migrations.OpCreate}})
	e.SetEnv([]string{"K"}, nil, nil, nil, nil)
	e.AddLimit("scope")
	data, _ := e.Marshal()
	s := string(data)
	for _, want := range []string{
		`"git"`, `"changes"`, `"components"`, `"migrations"`,
		`"env"`, `"limits"`, `"run_id"`, `"schema"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON faltando %s", want)
		}
	}
}

// Aceitação: uniqSorted dedupe e ordena.
func TestUniqSorted(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{[]string{"b", "a", "b"}, []string{"a", "b"}},
		{[]string{}, []string{}},
		{nil, []string{}},
		{[]string{"c"}, []string{"c"}},
	}
	for _, c := range cases {
		got := uniqSorted(c.in)
		if !sortedEqual(got, c.want) {
			t.Errorf("got %v, quero %v", got, c.want)
		}
	}
}

// sortedEqual compara dois slices ordenados.
func sortedEqual(a, b []string) bool {
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
