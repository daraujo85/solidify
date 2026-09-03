package gitx

import (
	"regexp"
	"strings"
	"testing"
)

// logFixture sobe um repo com 7 commits cobrindo Conventional Commits
// (feat, fix, refactor, perf, security, chore) + 2 breaking variants.
func logFixture(t *testing.T) (dir, rng string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	git("config", "commit.gpgsign", "false")

	// C1 — feat sem scope.
	writeFile(t, dir, "a.txt", "v1\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q",
		"-m", "feat: add a")
	c1 := git("rev-parse", "HEAD")

	// C2 — fix com scope.
	writeFile(t, dir, "a.txt", "v2\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q",
		"-m", "fix(api): handle nil")

	// C3 — refactor.
	writeFile(t, dir, "a.txt", "v3\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q",
		"-m", "refactor: extract helper")

	// C4 — perf.
	writeFile(t, dir, "a.txt", "v4\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q",
		"-m", "perf: cache lookup")

	// C5 — security.
	writeFile(t, dir, "a.txt", "v5\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q",
		"-m", "security: sanitize input")

	// C6 — chore.
	writeFile(t, dir, "a.txt", "v6\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q",
		"-m", "chore: bump deps")

	// C7 — breaking via '!'.
	writeFile(t, dir, "a.txt", "v7\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q",
		"-m", "feat(api)!: drop /v1")

	// C8 — breaking via footer.
	writeFile(t, dir, "a.txt", "v8\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q", "-m",
		"refactor: rename config\n\nBREAKING CHANGE: v1 toml schema gone")
	c8 := git("rev-parse", "HEAD")

	return dir, c1 + ".." + c8
}

// Aceitação: tipos feat/fix/refactor/perf/security/chore parseiam certo.
func TestLogParsesConventionalTypes(t *testing.T) {
	dir, rng := logFixture(t)

	commits, err := Log(dir, rng, LogOpts{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	// 8 commits em ordem newest-first.
	if len(commits) != 8 {
		t.Fatalf("commits = %d, quero 8", len(commits))
	}

	wantTypes := []string{
		"refactor", // C8 — footer não muda type
		"feat",     // C7 — feat!
		"chore",    // C6
		"security", // C5
		"perf",     // C4
		"refactor", // C3
		"fix",      // C2
		"feat",     // C1
	}
	for i, want := range wantTypes {
		if got := commits[i].Conventional.Type; got != want {
			t.Errorf("commit[%d] type = %q, quero %q (subject=%q)",
				i, got, want, commits[i].Subject)
		}
	}
}

// Aceitação: scope parseia quando presente.
func TestLogParsesScope(t *testing.T) {
	dir, rng := logFixture(t)

	commits, err := Log(dir, rng, LogOpts{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	// commits[1] é feat(api)!: drop /v1 — scope "api"
	// commits[6] é fix(api): handle nil — scope "api"
	if got := commits[1].Conventional.Scope; got != "api" {
		t.Errorf("commit[1] scope = %q, quero api", got)
	}
	if got := commits[6].Conventional.Scope; got != "api" {
		t.Errorf("commit[6] scope = %q, quero api", got)
	}
	// commits[0] (refactor) sem scope
	if got := commits[0].Conventional.Scope; got != "" {
		t.Errorf("commit[0] scope = %q, quero vazio", got)
	}
}

// Aceitação: breaking via '!' e via footer.
func TestLogDetectsBreakingChange(t *testing.T) {
	dir, rng := logFixture(t)

	commits, err := Log(dir, rng, LogOpts{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	// commits[0] = refactor + BREAKING CHANGE footer → breaking
	// commits[1] = feat(api)! → breaking
	// commits[2..7] = não-breaking
	if !commits[0].Conventional.Breaking {
		t.Errorf("commit[0] (refactor+footer) não marcado breaking")
	}
	if !commits[1].Conventional.Breaking {
		t.Errorf("commit[1] (feat!) não marcado breaking")
	}
	for i := 2; i < 8; i++ {
		if commits[i].Conventional.Breaking {
			t.Errorf("commit[%d] (%s) marcado breaking, não deveria",
				i, commits[i].Subject)
		}
	}
}

// Aceitação: issue keys extraídos por regex configurável.
func TestLogExtractsIssueKeys(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	git("config", "commit.gpgsign", "false")
	writeFile(t, dir, "a.txt", "v1\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q", "-m", "feat: PROJ-123 add thing")
	git("commit", "--allow-empty", "-q", "-m", "fix: typo\n\nRefs PROJ-456")
	git("commit", "--allow-empty", "-q", "-m", "chore: PROJ-123 again")
	head := git("rev-parse", "HEAD")
	oldest := git("rev-parse", git("rev-list", "--max-parents=0", "HEAD"))

	patterns := []*regexp.Regexp{regexp.MustCompile(`PROJ-\d+`)}
	commits, err := Log(dir, oldest+".."+head, LogOpts{IssueKeyPatterns: patterns})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	// 3 commits; newest->oldest.
	//   [0] = chore: PROJ-123 again → [PROJ-123]
	//   [1] = fix: typo\nRefs PROJ-456 → [PROJ-456]
	//   [2] = feat: PROJ-123 add thing → [PROJ-123]
	if len(commits) != 3 {
		t.Fatalf("commits = %d, quero 3", len(commits))
	}
	if got := commits[0].IssueKeys; len(got) != 1 || got[0] != "PROJ-123" {
		t.Errorf("commit[0] keys = %v, quero [PROJ-123]", got)
	}
	if got := commits[1].IssueKeys; len(got) != 1 || got[0] != "PROJ-456" {
		t.Errorf("commit[1] keys = %v, quero [PROJ-456]", got)
	}
	if got := commits[2].IssueKeys; len(got) != 1 || got[0] != "PROJ-123" {
		t.Errorf("commit[2] keys = %v, quero [PROJ-123]", got)
	}
}

// MaxCommits limita a quantidade devolvida.
func TestLogMaxCommits(t *testing.T) {
	dir, rng := logFixture(t)

	commits, err := Log(dir, rng, LogOpts{MaxCommits: 3})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 3 {
		t.Errorf("commits = %d, quero 3", len(commits))
	}
}

// Metadados (SHA, autor, datas) são extraídos.
func TestLogParsesMetadata(t *testing.T) {
	dir, rng := logFixture(t)

	commits, err := Log(dir, rng, LogOpts{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	c := commits[0]
	if len(c.SHA) != 40 {
		t.Errorf("SHA = %q, quero 40 hex chars", c.SHA)
	}
	if c.AuthorName != "solidify-test" {
		t.Errorf("AuthorName = %q, quero solidify-test", c.AuthorName)
	}
	if c.AuthorEmail != "test@solidify.local" {
		t.Errorf("AuthorEmail = %q, quero test@solidify.local", c.AuthorEmail)
	}
	if c.AuthorDate == "" {
		t.Error("AuthorDate vazio")
	}
	if c.Subject == "" {
		t.Error("Subject vazio")
	}
	if c.Body == "" {
		t.Error("Body vazio")
	}
}

// Range vazio é rejeitado.
func TestLogRejectsEmptyRange(t *testing.T) {
	if _, err := Log("/tmp", "", LogOpts{}); err == nil {
		t.Fatal("range vazio deveria falhar")
	}
}

// Sem IssueKeyPatterns, campo IssueKeys fica vazio (não popula).
func TestLogWithoutIssuePatterns(t *testing.T) {
	dir, rng := logFixture(t)
	commits, err := Log(dir, rng, LogOpts{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	for i, c := range commits {
		if len(c.IssueKeys) != 0 {
			t.Errorf("commit[%d] keys = %v, quero []", i, c.IssueKeys)
		}
	}
}

// Subject que não bate Conventional vira Type="other".
func TestLogNonConventionalCommit(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	git("config", "commit.gpgsign", "false")
	git("commit", "--allow-empty", "-q", "-m", "WIP stuff")
	head := git("rev-parse", "HEAD")

	commits, err := Log(dir, head, LogOpts{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("commits = %d, quero 1", len(commits))
	}
	if commits[0].Conventional.Type != "other" {
		t.Errorf("Type = %q, quero other", commits[0].Conventional.Type)
	}
	if commits[0].Subject == "" {
		t.Error("Subject vazio")
	}
}

// parseConventional (sem git): valida a função pura.
func TestParseConventional(t *testing.T) {
	cases := []struct {
		in       string
		outType  string
		outScope string
		outBreak bool
		outDesc  string
	}{
		{"feat: add", "feat", "", false, "add"},
		{"fix(api): null", "fix", "api", false, "null"},
		{"feat(api)!: drop", "feat", "api", true, "drop"},
		{"chore: bump", "chore", "", false, "bump"},
		{"random thing", "other", "", false, "random thing"},
	}
	for _, tc := range cases {
		c := parseConventional(tc.in, "")
		if c.Type != tc.outType || c.Scope != tc.outScope ||
			c.Breaking != tc.outBreak || c.Description != tc.outDesc {
			t.Errorf("parseConventional(%q) = %+v, quero type=%q scope=%q break=%v desc=%q",
				tc.in, c, tc.outType, tc.outScope, tc.outBreak, tc.outDesc)
		}
	}
}

// Footer "BREAKING-CHANGE:" (com hífen) também marca breaking.
func TestParseConventionalBreakingFooterVariants(t *testing.T) {
	body := "refactor: rename\n\nBREAKING-CHANGE: schema v2 only"
	c := parseConventional("refactor: rename", body)
	if !c.Breaking {
		t.Error("BREAKING-CHANGE: (com hífen) não marcou breaking")
	}
}

// Smoke: em fixture com múltiplos commits, ordem é newest-first.
func TestLogOrderIsNewestFirst(t *testing.T) {
	dir, rng := logFixture(t)
	commits, err := Log(dir, rng, LogOpts{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if commits[0].Subject != "refactor: rename config" {
		t.Errorf("primeiro commit = %q, quero o refactor mais recente",
			commits[0].Subject)
	}
	if commits[len(commits)-1].Subject != "feat: add a" {
		t.Errorf("último commit = %q, quero o feat inicial",
			commits[len(commits)-1].Subject)
	}
}

// Sanidade: body multilinha é preservado (Subject = primeira linha).
func TestLogPreservesBody(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }
	git("init", "-q", "-b", "main", ".")
	git("config", "commit.gpgsign", "false")
	git("commit", "--allow-empty", "-q", "-m",
		"feat: thing\n\nLong body with details.\n\nMore.")
	head := git("rev-parse", "HEAD")

	commits, err := Log(dir, head, LogOpts{})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("commits = %d, quero 1", len(commits))
	}
	if !strings.Contains(commits[0].Body, "Long body with details.") {
		t.Errorf("body sem segunda linha: %q", commits[0].Body)
	}
	if commits[0].Subject != "feat: thing" {
		t.Errorf("subject = %q, quero feat: thing", commits[0].Subject)
	}
}
