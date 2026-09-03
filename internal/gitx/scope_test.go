package gitx

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRepo cria um repo git temporário com 3 commits em main e um branch
// `feature` divergente com 1 commit adicional. Devolve (repoDir, baseSHA, headSHA).
//
// Cenário:
//   - C1 → C2 → C3 (main)
//   - C2 → F1 (feature)
//
// O branch point é C2.
func fixtureRepo(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()

	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		var out strings.Builder
		cmd.Stdout = &out
		// Configurar user por invocação (git se recusa a commitar sem).
		cmd.Env = append([]string{},
			"PATH=/usr/bin:/bin",
			"GIT_AUTHOR_NAME=solidify-test",
			"GIT_AUTHOR_EMAIL=test@solidify.local",
			"GIT_COMMITTER_NAME=solidify-test",
			"GIT_COMMITTER_EMAIL=test@solidify.local",
			"HOME="+t.TempDir(), // isola config global
		)
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		return strings.TrimSpace(out.String())
	}

	git("init", "-q", "-b", "main", ".")
	git("config", "commit.gpgsign", "false")
	git("config", "tag.gpgsign", "false")

	// C1
	git("commit", "--allow-empty", "-q", "-m", "C1")

	// C2
	git("commit", "--allow-empty", "-q", "-m", "C2")
	c2 := git("rev-parse", "HEAD")

	// C3
	git("commit", "--allow-empty", "-q", "-m", "C3")
	c3 := git("rev-parse", "HEAD")

	// feature diverge de C2
	if err := exec.Command("git", "-C", dir, "checkout", "-q", c2).Run(); err != nil {
		t.Fatalf("checkout c2: %v", err)
	}
	git("checkout", "-q", "-b", "feature")
	git("commit", "--allow-empty", "-q", "-m", "F1")
	f1 := git("rev-parse", "HEAD")

	// Voltar para main para o teste "esperar o estado estável".
	if err := exec.Command("git", "-C", dir, "checkout", "-q", "main").Run(); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	t.Cleanup(func() { _ = filepath.Base(dir) }) // hook só para clareza
	return dir, c3, f1
}

func TestResolverResolvesRefsToSHAs(t *testing.T) {
	repo, baseSHA, headSHA := fixtureRepo(t)

	// base=main (tip), head=feature (diverge). Refs curtos e longos devem
	// ambos resolver pro mesmo SHA canônico.
	got, err := (&Resolver{Dir: repo}).Resolve("main", "feature")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.BaseSHA != baseSHA {
		t.Errorf("BaseSHA = %s, quero %s", got.BaseSHA, baseSHA)
	}
	if got.HeadSHA != headSHA {
		t.Errorf("HeadSHA = %s, quero %s", got.HeadSHA, headSHA)
	}
	if got.Range != baseSHA+".."+headSHA {
		t.Errorf("Range = %q", got.Range)
	}
	if got.MergeBaseSHA != "" {
		t.Errorf("MergeBaseSHA = %q, quero vazio sem UseMergeBase", got.MergeBaseSHA)
	}

	// Short SHA como base também funciona.
	short := (&Resolver{Dir: repo})
	got, err = short.Resolve(baseSHA[:7], headSHA)
	if err != nil {
		t.Fatalf("Resolve short: %v", err)
	}
	if got.BaseSHA != baseSHA {
		t.Errorf("short: BaseSHA = %s, quero %s", got.BaseSHA, baseSHA)
	}
}

// Divergência: base=main, head=feature. Merge-base volta pro ancestral C2.
func TestResolverMergeBaseOnDivergentBranches(t *testing.T) {
	repo, _, headSHA := fixtureRepo(t)

	// Pega SHA do C2 via main^.
	cmd := exec.Command("git", "-C", repo, "rev-parse", "main^")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse main^: %v", err)
	}
	c2SHA := strings.TrimSpace(string(out))

	got, err := (&Resolver{Dir: repo, UseMergeBase: true}).Resolve("main", "feature")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.MergeBaseSHA != c2SHA {
		t.Errorf("MergeBaseSHA = %s, quero %s (C2)", got.MergeBaseSHA, c2SHA)
	}
	if got.Range != c2SHA+".."+headSHA {
		t.Errorf("Range = %q, quero %s..%s", got.Range, c2SHA, headSHA)
	}
}

// Aceitação da task: "repo temporário com branch divergente".
func TestResolverAcceptanceDivergentBranches(t *testing.T) {
	repo, _, _ := fixtureRepo(t)

	r := &Resolver{Dir: repo, UseMergeBase: true}
	scope, err := r.Resolve("main", "feature")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Validação dos objetos: rodar git log no range deve funcionar sem erro.
	cmd := exec.Command("git", "-C", repo, "log", "--oneline", scope.Range)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git log %s falhou: %v", scope.Range, err)
	}
}

func TestResolverSingleCommitExpandsToParentRange(t *testing.T) {
	repo, _, _ := fixtureRepo(t)

	cmd := exec.Command("git", "-C", repo, "rev-parse", "main")
	mainSHABytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse main: %v", err)
	}
	mainSHA := strings.TrimSpace(string(mainSHABytes))

	cmd = exec.Command("git", "-C", repo, "rev-parse", "main^")
	parentBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse main^: %v", err)
	}
	parentSHA := strings.TrimSpace(string(parentBytes))

	got, err := (&Resolver{Dir: repo, SingleCommit: true}).Resolve("", "main")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.BaseSHA != parentSHA || got.HeadSHA != mainSHA {
		t.Errorf("BaseSHA=%s HeadSHA=%s, quero %s e %s",
			got.BaseSHA, got.HeadSHA, parentSHA, mainSHA)
	}
	if got.Range != parentSHA+".."+mainSHA {
		t.Errorf("Range = %q", got.Range)
	}
}

// Commit raiz (sem parent): single commit precisa falhar com mensagem clara.
func TestResolverSingleCommitRootFails(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append([]string{},
			"PATH=/usr/bin:/bin",
			"GIT_AUTHOR_NAME=solidify-test",
			"GIT_AUTHOR_EMAIL=test@solidify.local",
			"GIT_COMMITTER_NAME=solidify-test",
			"GIT_COMMITTER_EMAIL=test@solidify.local",
			"HOME="+t.TempDir(),
		)
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	git("init", "-q", "-b", "main", ".")
	git("commit", "--allow-empty", "-q", "-m", "root")

	r := &Resolver{Dir: dir, SingleCommit: true}
	_, err := r.Resolve("", "HEAD")
	if err == nil || !strings.Contains(err.Error(), "raiz") {
		t.Fatalf("esperava erro de raiz, veio %v", err)
	}
}

func TestResolverRejectsUnknownRef(t *testing.T) {
	repo, _, headSHA := fixtureRepo(t)

	if _, err := (&Resolver{Dir: repo}).Resolve("branch-inexistente", headSHA); err == nil {
		t.Fatal("base inválida deveria falhar")
	}
	if _, err := (&Resolver{Dir: repo}).Resolve(headSHA, "tag-inexistente"); err == nil {
		t.Fatal("head inválida deveria falhar")
	}
}

func TestResolverRejectsMissingBaseOrHead(t *testing.T) {
	repo, _, headSHA := fixtureRepo(t)

	if _, err := (&Resolver{Dir: repo}).Resolve("", headSHA); err == nil {
		t.Error("base vazio deveria falhar")
	}
	if _, err := (&Resolver{Dir: repo}).Resolve(headSHA, ""); err == nil {
		t.Error("head vazio deveria falhar")
	}
	if _, err := (&Resolver{Dir: repo, SingleCommit: true}).Resolve(headSHA, "HEAD"); err == nil {
		t.Error("single commit + base explícita deveria falhar")
	}
}

// EnsureBinary falha se git não estiver no PATH.
func TestEnsureBinary(t *testing.T) {
	if err := EnsureBinary(); err != nil {
		t.Skipf("git indisponível: %v", err)
	}
}
