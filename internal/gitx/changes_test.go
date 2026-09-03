package gitx

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitCmdInDir é um helper que monta um exec.Cmd com PATH/user/git isolados.
func gitCmdInDir(dir string, args ...string) *exec.Cmd {
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = []string{
		"PATH=/usr/bin:/bin",
		"GIT_AUTHOR_NAME=solidify-test",
		"GIT_AUTHOR_EMAIL=test@solidify.local",
		"GIT_COMMITTER_NAME=solidify-test",
		"GIT_COMMITTER_EMAIL=test@solidify.local",
	}
	return c
}

// runGit roda git em dir e devolve stdout trimado; falha o teste se errar.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := gitCmdInDir(dir, args...)
	var out bytes.Buffer
	c.Stdout = &out
	if err := c.Run(); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out.String())
}

// writeFile grava body em dir/name com permissões 0o644 (sem shell).
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("escrever %s: %v", name, err)
	}
}

// changesFixture sobe um repo com vários tipos de mudança entre HEAD~1 e HEAD:
//   - adiciona c.txt (added)
//   - modifica a.txt (modified)
//   - deleta d.txt (deleted)
//   - renomeia b.txt → b-renamed.txt (renamed)
//   - binário: adiciona bin.dat (added binary)
func changesFixture(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	git("config", "commit.gpgsign", "false")

	writeFile(t, dir, "a.txt", "a\n")
	writeFile(t, dir, "b.txt", "b\n")
	writeFile(t, dir, "d.txt", "d\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q", "-m", "initial")
	c1 := git("rev-parse", "HEAD")

	// a.txt: modificado
	writeFile(t, dir, "a.txt", "aa\n")

	// d.txt: deletado
	if err := os.Remove(filepath.Join(dir, "d.txt")); err != nil {
		t.Fatalf("remover d.txt: %v", err)
	}
	runGitQuiet(t, dir, "rm", "-q", "d.txt")

	// b.txt → b-renamed.txt: renomeado via git mv (faz rename no disco + index)
	runGitQuiet(t, dir, "mv", "b.txt", "b-renamed.txt")

	// c.txt: adicionado
	writeFile(t, dir, "c.txt", "c\nc2\n")

	// bin.dat: binário (bytes não-text)
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, 0o644); err != nil {
		t.Fatalf("escrever bin.dat: %v", err)
	}

	git("add", ".")
	git("commit", "--allow-empty", "-q", "-m", "mutations")
	c2 := git("rev-parse", "HEAD")

	return dir, c1 + ".." + c2, c2
}

// runGitQuiet roda git e não captura stdout (apenas verifica erro).
func runGitQuiet(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := gitCmdInDir(dir, args...)
	if err := c.Run(); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}

// Encontra a FileChange para um path (case-sensitive, exato).
func findByPath(changes []FileChange, path string) *FileChange {
	for i := range changes {
		if changes[i].Path == path {
			return &changes[i]
		}
	}
	return nil
}

// Acceptance: rename aparece como renamed, não delete + add.
func TestChangesAcceptanceRenameIsNotDeleteAdd(t *testing.T) {
	repo, rng, _ := changesFixture(t)

	got, err := Changes(repo, rng, ChangesOpts{RenameDetection: true, CopyDetection: false})
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}

	rename := findByPath(got, "b-renamed.txt")
	if rename == nil {
		t.Fatalf("rename não encontrado em %+v", got)
	}
	if rename.Status != ChangeRenamed {
		t.Errorf("status = %v, quero renamed", rename.Status)
	}
	if rename.OldPath != "b.txt" {
		t.Errorf("OldPath = %q, quero b.txt", rename.OldPath)
	}
	for _, fc := range got {
		if fc.Status == ChangeDeleted && fc.Path == "b.txt" {
			t.Errorf("delete de b.txt apareceu junto com o rename; deveria ter sido fundido")
		}
	}
}

func TestChangesAllKinds(t *testing.T) {
	repo, rng, _ := changesFixture(t)

	got, err := Changes(repo, rng, ChangesOpts{RenameDetection: true, CopyDetection: false})
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}

	must := func(path string, want ChangeKind) {
		t.Helper()
		fc := findByPath(got, path)
		if fc == nil {
			t.Errorf("path %q ausente", path)
			return
		}
		if fc.Status != want {
			t.Errorf("%s status = %v, quero %v", path, fc.Status, want)
		}
	}
	must("a.txt", ChangeModified)
	must("c.txt", ChangeAdded)
	must("d.txt", ChangeDeleted)
	must("b-renamed.txt", ChangeRenamed)
	must("bin.dat", ChangeAdded)
}

func TestChangesNumstatLines(t *testing.T) {
	repo, rng, _ := changesFixture(t)

	got, err := Changes(repo, rng, ChangesOpts{RenameDetection: true})
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}

	a := findByPath(got, "a.txt")
	if a == nil || a.Add != 1 || a.Del != 1 {
		t.Errorf("a.txt: add=%d del=%d, quero 1/1 (aa substituiu a)", a.Add, a.Del)
	}
	bin := findByPath(got, "bin.dat")
	if bin == nil {
		t.Fatalf("bin.dat ausente")
	}
	if !bin.IsBinary || bin.Add != -1 || bin.Del != -1 {
		t.Errorf("bin.dat: binary=%v add=%d del=%d, quero true/-1/-1",
			bin.IsBinary, bin.Add, bin.Del)
	}
}

func TestChangesModeChange(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	writeFile(t, dir, "exec.sh", "#!/bin/sh\necho hi\n")
	runGitQuiet(t, dir, "add", ".")
	git("commit", "--allow-empty", "-q", "-m", "first")
	c1sha := git("rev-parse", "HEAD")

	if err := os.Chmod(filepath.Join(dir, "exec.sh"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	runGitQuiet(t, dir, "add", ".")
	git("commit", "--allow-empty", "-q", "-m", "chmod")
	c2sha := git("rev-parse", "HEAD")

	got, err := Changes(dir, c1sha+".."+c2sha, ChangesOpts{RenameDetection: false})
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	fc := findByPath(got, "exec.sh")
	if fc == nil {
		t.Fatalf("exec.sh ausente em %+v", got)
	}
	if !fc.ModeChange {
		t.Errorf("ModeChange = false, quero true (modes: %s -> %s)",
			fc.SourceMode, fc.DestMode)
	}
}

func TestChangesCopyDetection(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	git("config", "diff.renames", "copies")
	writeFile(t, dir, "a.txt", "hello\n")
	runGitQuiet(t, dir, "add", ".")
	git("commit", "--allow-empty", "-q", "-m", "first")
	c1sha := git("rev-parse", "HEAD")

	dst, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatalf("ler a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a-copy.txt"), dst, 0o644); err != nil {
		t.Fatalf("cp a.txt -> a-copy.txt: %v", err)
	}
	runGitQuiet(t, dir, "add", ".")
	git("commit", "--allow-empty", "-q", "-m", "copy")
	c2sha := git("rev-parse", "HEAD")

	got, err := Changes(dir, c1sha+".."+c2sha, ChangesOpts{RenameDetection: true, CopyDetection: true})
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	cp := findByPath(got, "a-copy.txt")
	if cp == nil {
		t.Fatalf("a-copy.txt ausente em %+v", got)
	}
	if cp.Status != ChangeCopied && cp.Status != ChangeAdded {
		t.Errorf("a-copy.txt status = %v, quero copied ou added", cp.Status)
	}
}

func TestChangesEmptyRangeIsAllowed(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	git("commit", "--allow-empty", "-q", "-m", "single")
	sha := git("rev-parse", "HEAD")

	got, err := Changes(dir, sha+".."+sha, ChangesOpts{})
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("range vazio devolveu %d entradas: %+v", len(got), got)
	}
}

func TestChangesRejectsEmptyRange(t *testing.T) {
	if _, err := Changes("/tmp", "", ChangesOpts{}); err == nil {
		t.Fatal("range vazio deveria falhar")
	}
}
