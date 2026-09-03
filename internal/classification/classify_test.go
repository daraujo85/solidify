package classification

import (
	"testing"

	"github.com/diegoaraujo/solidify/internal/gitx"
)

// makeCommit instancia um gitx.Commit só com Type e Breaking — os outros
// campos não importam para classificação.
func makeCommit(typeName string, breaking bool) gitx.Commit {
	return gitx.Commit{
		Conventional: gitx.Conventional{
			Type:     typeName,
			Breaking: breaking,
		},
	}
}

// Aceitação: Conventional Commit type → categoria canônica.
func TestClassifyFromConventionalType(t *testing.T) {
	cases := []struct {
		typeName string
		want     Category
	}{
		{"feat", CategoryFeature},
		{"fix", CategoryBugfix},
		{"refactor", CategoryRefactor},
		{"perf", CategoryPerformance},
		{"security", CategorySecurity},
		{"test", CategoryTest},
		{"docs", CategoryDocs},
		{"build", CategoryBuild},
		{"ci", CategoryCI},
		{"chore", CategoryChore},
		{"feature", CategoryFeature}, // alias
		{"bugfix", CategoryBugfix},   // alias
		{"performance", CategoryPerformance},
		{"unknown_type", CategoryUnknown},
	}
	for _, tc := range cases {
		got := ClassifyCommit(makeCommit(tc.typeName, false), nil)
		if got != tc.want {
			t.Errorf("type=%q quero=%q veio=%q", tc.typeName, tc.want, got)
		}
	}
}

// Aceitação: breaking detectado em Conventional sempre vira breaking-change.
func TestClassifyBreakingWinsOverConventionalType(t *testing.T) {
	for _, typeName := range []string{"feat", "fix", "refactor", "perf", "chore"} {
		got := ClassifyCommit(makeCommit(typeName, true), nil)
		if got != CategoryBreakingChange {
			t.Errorf("type=%q breaking=true veio=%q, quero breaking-change", typeName, got)
		}
	}
}

// Aceitação: path hints quando Conventional não casa.
func TestClassifyFromPathHints(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  Category
	}{
		{"migration sql", []string{"migrations/001_init.sql"}, CategoryMigration},
		{"migration prisma", []string{"prisma/migrations/20240101_init/migration.sql"}, CategoryMigration},
		{"migration db dir", []string{"db/migrate/001_init.sql"}, CategoryMigration},
		{"migration bare sql", []string{"foo.sql"}, CategoryMigration},
		{"migration alias", []string{"alembic/versions/001_init.py"}, CategoryMigration},
		{"config Dockerfile", []string{"Dockerfile"}, CategoryConfiguration},
		{"config yaml", []string{"config/app.yaml"}, CategoryConfiguration},
		{"config toml", []string{"configs/db.toml"}, CategoryConfiguration},
		{"config env", []string{".env.production"}, CategoryConfiguration},
		{"config json", []string{"config/i18n/en.json"}, CategoryConfiguration},
		{"docs readme", []string{"README.md"}, CategoryDocs},
		{"docs CHANGELOG", []string{"CHANGELOG.md"}, CategoryDocs},
		{"docs dir", []string{"docs/guide/intro.md"}, CategoryDocs},
		{"test go", []string{"main_test.go"}, CategoryTest},
		{"test js", []string{"foo.test.ts"}, CategoryTest},
		{"test dir", []string{"tests/foo.py"}, CategoryTest},
		{"ci workflow", []string{".github/workflows/ci.yml"}, CategoryCI},
		{"ci gitlab", []string{".gitlab-ci.yml"}, CategoryCI},
		{"ci jenkins", []string{"Jenkinsfile"}, CategoryCI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyCommit(makeCommit("", false), tc.paths)
			if got != tc.want {
				t.Errorf("paths=%v quero=%q veio=%q", tc.paths, tc.want, got)
			}
		})
	}
}

// Conventional type tem prioridade sobre path hints (mesmo que paths
// sugiram outra categoria).
func TestClassifyConventionalBeatsPathHints(t *testing.T) {
	// Feat em commit que toca .github/workflows.
	got := ClassifyCommit(makeCommit("feat", false),
		[]string{".github/workflows/ci.yml"})
	if got != CategoryFeature {
		t.Errorf("feat + ci path deve ser feature (Conventional vence), veio %q", got)
	}
	// Refactor em commit que toca migrations.
	got = ClassifyCommit(makeCommit("refactor", false),
		[]string{"migrations/001_init.sql"})
	if got != CategoryRefactor {
		t.Errorf("refactor + migrations deve ser refactor, veio %q", got)
	}
}

// Paths sem Conventional → categoria do path mais frequente / prioritário.
func TestClassifyMigrationBeatsConfigInTie(t *testing.T) {
	// Migration é mais específico que config — empate vai pra migration.
	paths := []string{"migrations/001.sql", "config.yaml"}
	got := ClassifyCommit(makeCommit("", false), paths)
	if got != CategoryMigration {
		t.Errorf("paths=%v quero migration, veio %q", paths, got)
	}
}

// Sem Conventional type nem path hint → unknown.
func TestClassifyUnknown(t *testing.T) {
	if got := ClassifyCommit(makeCommit("", false), nil); got != CategoryUnknown {
		t.Errorf("empty: %q", got)
	}
	if got := ClassifyCommit(makeCommit("other", false), []string{"foo.go", "bar.go"}); got != CategoryUnknown {
		t.Errorf("type=other + paths de código: %q", got)
	}
}

// Path de teste não é migration nem config.
func TestClassifyTestPathIsNotMigration(t *testing.T) {
	paths := []string{"tests/migrations/foo.sql"}
	got := ClassifyCommit(makeCommit("", false), paths)
	if got != CategoryTest {
		t.Errorf("SQL em tests/ deve ser test, veio %q", got)
	}
}

// Edge: paths vazios com breaking não-Conventional (footer) ainda classifica.
func TestClassifyBreakingFromFooter(t *testing.T) {
	c := gitx.Commit{
		Conventional: gitx.Conventional{
			Type:     "refactor",
			Breaking: true, // detectado via footer
		},
	}
	got := ClassifyCommit(c, nil)
	if got != CategoryBreakingChange {
		t.Errorf("refactor com breaking footer: %q", got)
	}
}

// Sanity: cobertura das constantes — todas as 14 categorias existem.
func TestAllCanonicalCategories(t *testing.T) {
	want := []Category{
		CategoryFeature, CategoryBugfix, CategoryRefactor, CategoryPerformance,
		CategorySecurity, CategoryTest, CategoryDocs, CategoryBuild,
		CategoryCI, CategoryChore, CategoryMigration, CategoryConfiguration,
		CategoryBreakingChange, CategoryUnknown,
	}
	if len(want) != 14 {
		t.Fatalf("categorias canônicas devem ser 14, são %d", len(want))
	}
	seen := map[Category]bool{}
	for _, c := range want {
		seen[c] = true
	}
	for _, c := range want {
		if !seen[c] {
			t.Errorf("categoria %q não está na lista", c)
		}
	}
}
