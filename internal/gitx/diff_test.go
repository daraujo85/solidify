package gitx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diffFixture sobe um repo com dois commits onde o segundo introduz:
//   - adição de c.txt
//   - modificação de a.txt (1 hunk)
//   - deleção de d.txt
//   - rename de b.txt → b-renamed.txt (sem mudança de conteúdo)
func diffFixture(t *testing.T) (dir, rng string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	git("config", "commit.gpgsign", "false")

	writeFile(t, dir, "a.txt", "line1\nline2\nline3\nline4\nline5\n")
	writeFile(t, dir, "b.txt", "shared\n")
	writeFile(t, dir, "d.txt", "to delete\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q", "-m", "first")
	c1 := git("rev-parse", "HEAD")

	// a.txt: muda linha 3
	writeFile(t, dir, "a.txt", "line1\nline2\nLINE3\nline4\nline5\n")
	// d.txt: deletado
	if err := os.Remove(filepath.Join(dir, "d.txt")); err != nil {
		t.Fatalf("remover d.txt: %v", err)
	}
	runGitQuiet(t, dir, "rm", "-q", "d.txt")
	// b.txt → b-renamed.txt: rename sem mudança
	runGitQuiet(t, dir, "mv", "b.txt", "b-renamed.txt")
	// c.txt: adicionado
	writeFile(t, dir, "c.txt", "new file\n")
	git("add", ".")
	git("commit", "--allow-empty", "-q", "-m", "second")
	c2 := git("rev-parse", "HEAD")

	return dir, c1 + ".." + c2
}

// rmFile remove um arquivo do disco. Falha o teste se der erro.
func rmFile(t *testing.T, dir, name string) error {
	t.Helper()
	return os.Remove(filepath.Join(dir, name))
}

// TestDiffSimpleModifiedFile exercita o caminho principal: modificação com
// 1 hunk. Aceitação da task.
func TestDiffSimpleModifiedFile(t *testing.T) {
	dir, rng := diffFixture(t)

	files, err := Diff(dir, rng, DiffOpts{ContextLines: 1})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	var a *DiffFile
	for i := range files {
		if files[i].Path == "a.txt" {
			a = &files[i]
		}
	}
	if a == nil {
		t.Fatalf("a.txt ausente em %+v", files)
	}
	if a.Status != ChangeModified {
		t.Errorf("status = %v, quero modified", a.Status)
	}
	if len(a.Hunks) != 1 {
		t.Errorf("hunks = %d, quero 1", len(a.Hunks))
	}
	h := a.Hunks[0]
	// Com ContextLines=1 e mudança na linha 3, git emite @@ -2,3 +2,3 @@
	// (o hunk começa na primeira linha de contexto, não na primeira alterada).
	if h.OldStart != 2 || h.NewStart != 2 {
		t.Errorf("coords = old=%d new=%d, quero 2/2", h.OldStart, h.NewStart)
	}
	if !strings.Contains(h.Content, "-line3") {
		t.Errorf("hunk não tem linha removida: %q", h.Content)
	}
	if !strings.Contains(h.Content, "+LINE3") {
		t.Errorf("hunk não tem linha adicionada: %q", h.Content)
	}
	if h.Hash == "" || !strings.HasPrefix(h.Hash, "hunk:") {
		t.Errorf("hash inválido: %q", h.Hash)
	}
}

func TestDiffAddedAndDeleted(t *testing.T) {
	dir, rng := diffFixture(t)
	files, err := Diff(dir, rng, DiffOpts{ContextLines: 1})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	hasStatus := func(path string, want ChangeKind) bool {
		for _, f := range files {
			if f.Path == path && f.Status == want {
				return true
			}
		}
		return false
	}

	if !hasStatus("c.txt", ChangeAdded) {
		t.Error("c.txt não foi classificado como added")
	}
	if !hasStatus("d.txt", ChangeDeleted) {
		t.Error("d.txt não foi classificado como deleted")
	}
	if !hasStatus("b-renamed.txt", ChangeRenamed) {
		t.Error("b-renamed.txt não foi classificado como renamed")
	}
}

// Hash estável: rodar duas vezes sobre o mesmo range devolve o mesmo hash.
func TestDiffHashIsStable(t *testing.T) {
	dir, rng := diffFixture(t)

	a, err := Diff(dir, rng, DiffOpts{ContextLines: 1})
	if err != nil {
		t.Fatalf("Diff 1: %v", err)
	}
	b, err := Diff(dir, rng, DiffOpts{ContextLines: 3})
	if err != nil {
		t.Fatalf("Diff 2: %v", err)
	}

	hashOf := func(files []DiffFile, path string) string {
		for _, f := range files {
			if f.Path == path && len(f.Hunks) > 0 {
				return f.Hunks[0].Hash
			}
		}
		return ""
	}
	// Context lines diferente NÃO deve mudar o hash do core diff.
	if hashOf(a, "a.txt") != hashOf(b, "a.txt") {
		t.Errorf("hash mudou com context lines: %s vs %s",
			hashOf(a, "a.txt"), hashOf(b, "a.txt"))
	}
	if hashOf(a, "a.txt") == "" {
		t.Error("hash vazio")
	}
}

// Hash muda quando o conteúdo do hunk muda.
func TestDiffHashChangesWithContent(t *testing.T) {
	dir, rng := diffFixture(t)
	a, err := Diff(dir, rng, DiffOpts{ContextLines: 1})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	hashA := ""
	for _, f := range a {
		if f.Path == "a.txt" && len(f.Hunks) > 0 {
			hashA = f.Hunks[0].Hash
		}
	}
	if hashA == "" {
		t.Fatal("hash vazio")
	}

	// Nova edição em a.txt: muda o conteúdo e o hash.
	writeFile(t, dir, "a.txt", "line1\nline2\nLINE3\nline4\nLINE5\n")
	runGitQuiet(t, dir, "add", ".")
	runGitQuiet(t, dir, "commit", "--allow-empty", "-q", "-m", "third")
	c2 := mustRunGit(t, dir, "rev-parse", "HEAD")
	c3 := mustRunGit(t, dir, "rev-parse", "HEAD~1")
	b, err := Diff(dir, c3+".."+c2, DiffOpts{ContextLines: 1})
	if err != nil {
		t.Fatalf("Diff 2: %v", err)
	}
	for _, f := range b {
		if f.Path == "a.txt" && len(f.Hunks) > 0 {
			if f.Hunks[0].Hash == hashA {
				t.Error("hash não mudou após editar o arquivo")
			}
			return
		}
	}
	t.Error("a.txt ausente no segundo diff")
}

func TestDiffEmptyRangeProducesNoFiles(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }

	git("init", "-q", "-b", "main", ".")
	git("commit", "--allow-empty", "-q", "-m", "single")
	sha := git("rev-parse", "HEAD")

	files, err := Diff(dir, sha+".."+sha, DiffOpts{})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("range vazio devolveu %d arquivos: %+v", len(files), files)
	}
}

func TestDiffRejectsEmptyRange(t *testing.T) {
	if _, err := Diff("/tmp", "", DiffOpts{}); err == nil {
		t.Fatal("range vazio deveria falhar")
	}
}

// MaxHunkBytes trunca o conteúdo sem afetar o hash (que usa o diff completo).
func TestDiffMaxHunkBytesTruncatesContent(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string { return runGit(t, dir, args...) }
	git("init", "-q", "-b", "main", ".")

	// Cria arquivo com 100 linhas.
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("line\n")
	}
	writeFile(t, dir, "big.txt", b.String())
	git("add", ".")
	git("commit", "--allow-empty", "-q", "-m", "first")
	c1 := git("rev-parse", "HEAD")

	// Modifica tudo.
	var b2 strings.Builder
	for i := 0; i < 100; i++ {
		b2.WriteString("LINE\n")
	}
	writeFile(t, dir, "big.txt", b2.String())
	git("add", ".")
	git("commit", "--allow-empty", "-q", "-m", "second")
	c2 := git("rev-parse", "HEAD")

	// Com limite pequeno.
	files, err := Diff(dir, c1+".."+c2, DiffOpts{ContextLines: 1, MaxHunkBytes: 50})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(files) != 1 || len(files[0].Hunks) == 0 {
		t.Fatalf("esperava 1 arquivo com 1 hunk, veio %+v", files)
	}
	if len(files[0].Hunks[0].Content) > 200 {
		t.Errorf("conteúdo não foi truncado: %d bytes", len(files[0].Hunks[0].Content))
	}
	if files[0].Hunks[0].Hash == "" {
		t.Error("hash vazio após truncar")
	}
}

func TestBlobAt(t *testing.T) {
	dir, _ := diffFixture(t)

	body, err := BlobAt(dir, "HEAD", "c.txt")
	if err != nil {
		t.Fatalf("BlobAt: %v", err)
	}
	if body != "new file\n" {
		t.Errorf("conteúdo = %q, quero %q", body, "new file\n")
	}
}

func TestBlobAtMissingFile(t *testing.T) {
	dir, _ := diffFixture(t)
	if _, err := BlobAt(dir, "HEAD", "fantasma.txt"); err == nil {
		t.Fatal("BlobAt em path inexistente deveria falhar")
	}
}

// mustRunGit é runGit que devolve string sem checar (deve ser usado após
// garantir que o comando vai funcionar; aqui serve para encadear).
func mustRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := gitCmdInDir(dir, args...)
	var sb strings.Builder
	c.Stdout = &sb
	if err := c.Run(); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(sb.String())
}
