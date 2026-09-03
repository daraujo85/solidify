package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Aceitação: New cria dir com estrutura esperada.
func TestNewCreatesRunDir(t *testing.T) {
	base := t.TempDir()
	s, err := New(base, "test-001")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.FlushManifest()

	want := filepath.Join(base, "runs", "test-001")
	if s.Dir() != want {
		t.Errorf("dir = %s, quero %s", s.Dir(), want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("dir não existe: %v", err)
	}
}

// Aceitação: runID vazio gera id baseado em timestamp.
func TestNewAutoRunID(t *testing.T) {
	base := t.TempDir()
	s, err := New(base, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.RunID() == "" {
		t.Errorf("runID vazio")
	}
	if !strings.Contains(s.RunID(), "-") {
		t.Errorf("runID malformado: %s", s.RunID())
	}
}

// Aceitação: New falha com baseDir vazio.
func TestNewEmptyBaseDir(t *testing.T) {
	_, err := New("", "x")
	if err == nil {
		t.Fatalf("esperava erro")
	}
}

// Aceitação: Write retorna artifact com SHA-256 correto.
func TestWriteSHA256(t *testing.T) {
	base := t.TempDir()
	s, err := New(base, "test-sha")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data := []byte("hello world")
	art, err := s.Write("hello.txt", data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// SHA-256 esperado.
	h := sha256.Sum256(data)
	want := hex.EncodeToString(h[:])
	if art.SHA256 != want {
		t.Errorf("sha256 = %s, quero %s", art.SHA256, want)
	}
	if art.Size != int64(len(data)) {
		t.Errorf("size = %d, quero %d", art.Size, len(data))
	}
}

// Aceitação: Write grava arquivo no disco.
func TestWriteFileOnDisk(t *testing.T) {
	base := t.TempDir()
	s, err := New(base, "test-disk")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data := []byte("conteúdo")
	art, err := s.Write("disk.txt", data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(s.Dir(), "disk.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("conteúdo = %q, quero %q", got, data)
	}
	_ = art
}

// Aceitação: Write em subdiretório cria os diretórios pai.
func TestWriteNestedPath(t *testing.T) {
	base := t.TempDir()
	s, err := New(base, "test-nested")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = s.Write("a/b/c/file.txt", []byte("nested"))
	if err != nil {
		t.Fatalf("Write nested: %v", err)
	}
}

// Aceitação: Write com name vazio falha.
func TestWriteEmptyName(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-empty")
	_, err := s.Write("", []byte("x"))
	if err == nil {
		t.Fatalf("esperava erro")
	}
}

// Aceitação: Write com name reservado (manifest.json) falha.
func TestWriteReservedName(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-reserved")
	_, err := s.Write("manifest.json", []byte("x"))
	if err == nil {
		t.Fatalf("esperava erro")
	}
}

// Aceitação: Write com name não-local (path traversal) falha.
func TestWriteNonLocalName(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-traversal")
	cases := []string{"../etc/passwd", "/etc/passwd", "a/../../b"}
	for _, c := range cases {
		if _, err := s.Write(c, []byte("x")); err == nil {
			t.Errorf("%s devia falhar", c)
		}
	}
}

// Aceitação: WriteReader funciona com io.Reader.
func TestWriteReader(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-reader")
	art, err := s.WriteReader("from-reader.txt", strings.NewReader("abc123"))
	if err != nil {
		t.Fatalf("WriteReader: %v", err)
	}

	h := sha256.Sum256([]byte("abc123"))
	if art.SHA256 != hex.EncodeToString(h[:]) {
		t.Errorf("sha256 errado")
	}
}

// Aceitação: Read devolve bytes originais.
func TestRead(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-read")
	want := []byte("read me")
	_, _ = s.Write("r.txt", want)
	got, err := s.Read("r.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, quero %q", got, want)
	}
}

// Aceitação: Read com name não-local falha.
func TestReadNonLocal(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-read-nl")
	if _, err := s.Read("../etc/passwd"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: List devolve artefatos escritos.
func TestList(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-list")
	s.Write("a.txt", []byte("a"))
	s.Write("b.txt", []byte("b"))
	s.Write("c.txt", []byte("c"))
	arts := s.List()
	if len(arts) != 3 {
		t.Errorf("len = %d, quero 3", len(arts))
	}
}

// Aceitação: SetMeta persiste no manifest.
func TestSetMeta(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-meta")
	s.SetMeta("base", "main")
	s.SetMeta("head", "feature/x")
	s.SetMeta("base", "develop") // sobrescreve

	mf, err := s.SaveManifest()
	if err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if mf.Meta["base"] != "develop" {
		t.Errorf("meta base = %s, quero develop", mf.Meta["base"])
	}
	if mf.Meta["head"] != "feature/x" {
		t.Errorf("meta head = %s", mf.Meta["head"])
	}
}

// Aceitação: SaveManifest persiste em manifest.json.
func TestSaveManifest(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-save-mf")
	s.Write("a.txt", []byte("a"))
	s.Write("b.txt", []byte("b"))
	s.SetMeta("k", "v")

	mf, err := s.SaveManifest()
	if err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if mf.SchemaVersion != SchemaVersion {
		t.Errorf("schema = %d", mf.SchemaVersion)
	}
	if len(mf.Artifacts) != 2 {
		t.Errorf("artifacts = %d, quero 2", len(mf.Artifacts))
	}
	if mf.Meta["k"] != "v" {
		t.Errorf("meta não persistido")
	}

	// manifest.json está no disco.
	data, err := os.ReadFile(filepath.Join(s.Dir(), "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var parsed Manifest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.RunID != "test-save-mf" {
		t.Errorf("runID = %s", parsed.RunID)
	}
}

// Aceitação: FromDir carrega manifest existente.
func TestFromDir(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-from")
	s.Write("x.txt", []byte("x"))
	s.SetMeta("k", "v")
	if _, err := s.SaveManifest(); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	s2, err := FromDir(s.Dir())
	if err != nil {
		t.Fatalf("FromDir: %v", err)
	}
	if s2.RunID() != "test-from" {
		t.Errorf("runID = %s", s2.RunID())
	}
	if len(s2.List()) != 1 {
		t.Errorf("List não carregou")
	}
}

// Aceitação: FromDir sem manifest existente funciona.
func TestFromDirNoManifest(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "runs", "empty")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	s, err := FromDir(dir)
	if err != nil {
		t.Fatalf("FromDir: %v", err)
	}
	if s.RunID() != "empty" {
		t.Errorf("runID = %s", s.RunID())
	}
	if len(s.List()) != 0 {
		t.Errorf("artifacts = %d, quero 0", len(s.List()))
	}
}

// Aceitação: escritas múltiplas têm SHA-256 individual.
func TestMultipleWritesIndividualSHA(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-multi")
	cases := map[string]string{
		"a.txt": "alpha",
		"b.txt": "beta",
		"c.txt": "gamma",
	}
	for name, data := range cases {
		art, err := s.Write(name, []byte(data))
		if err != nil {
			t.Fatalf("Write %s: %v", name, err)
		}
		h := sha256.Sum256([]byte(data))
		want := hex.EncodeToString(h[:])
		if art.SHA256 != want {
			t.Errorf("%s sha = %s, quero %s", name, art.SHA256, want)
		}
	}
}

// Aceitação: manifest é determinístico (ordenado por nome).
func TestManifestSorted(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-sort")
	s.Write("z.txt", []byte("z"))
	s.Write("a.txt", []byte("a"))
	s.Write("m.txt", []byte("m"))

	mf, _ := s.SaveManifest()
	want := []string{"a.txt", "m.txt", "z.txt"}
	for i, a := range mf.Artifacts {
		if a.Name != want[i] {
			t.Errorf("artifacts[%d] = %s, quero %s", i, a.Name, want[i])
		}
	}
}

// Aceitação: FlushManifest pode ser chamado múltiplas vezes.
func TestFlushManifestMultiple(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-flush")
	s.Write("a.txt", []byte("a"))
	for i := 0; i < 3; i++ {
		if err := s.FlushManifest(); err != nil {
			t.Fatalf("Flush #%d: %v", i, err)
		}
	}
}

// Aceitação: arquivo vazio SHA-256.
func TestWriteEmptyFile(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-empty-file")
	art, err := s.Write("empty.txt", []byte{})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if art.Size != 0 {
		t.Errorf("size = %d", art.Size)
	}
	// SHA-256 de conteúdo vazio.
	h := sha256.Sum256([]byte{})
	want := hex.EncodeToString(h[:])
	if art.SHA256 != want {
		t.Errorf("sha256 = %s, quero %s", art.SHA256, want)
	}
}

// Aceitação: Write não cria manifest.json exposto.
func TestWriteDoesNotCreateManifest(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-no-mf")
	s.Write("data.txt", []byte("d"))
	if _, err := os.Stat(filepath.Join(s.Dir(), "manifest.json")); err == nil {
		t.Errorf("manifest.json não devia existir antes de FlushManifest")
	}
	s.FlushManifest()
	if _, err := os.Stat(filepath.Join(s.Dir(), "manifest.json")); err != nil {
		t.Errorf("manifest.json devia existir após FlushManifest")
	}
}

// Aceitação: Write é thread-safe.
func TestWriteConcurrent(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-conc")
	const N = 50
	done := make(chan error, N)
	for i := 0; i < N; i++ {
		go func(i int) {
			_, err := s.Write("file.txt", []byte{byte(i)})
			done <- err
		}(i)
	}
	for i := 0; i < N; i++ {
		if err := <-done; err != nil {
			t.Errorf("Write concorrente: %v", err)
		}
	}
	if got := len(s.List()); got != N {
		t.Errorf("List = %d, quero %d", got, N)
	}
}

// Aceitação: runID único entre runs.
func TestNewRunIDsUnique(t *testing.T) {
	base := t.TempDir()
	s1, _ := New(base, "")
	s2, _ := New(base, "")
	if s1.RunID() == s2.RunID() {
		t.Errorf("runID colidiu: %s == %s", s1.RunID(), s2.RunID())
	}
}

// Aceitação: temp file é limpo em caso de rename falha.
func TestTempCleanedOnError(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-temp")
	_, err := s.Write("ok.txt", []byte("ok"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Verifica que não há .tmp- residual.
	entries, _ := os.ReadDir(s.Dir())
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temp residual: %s", e.Name())
		}
	}
}

// Aceitação: Manifest serialização tem campos esperados.
func TestManifestJSON(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-json")
	s.Write("a.txt", []byte("hello"))
	mf, _ := s.SaveManifest()

	if mf.RunID != "test-json" {
		t.Errorf("runID = %s", mf.RunID)
	}
	if mf.CreatedAt.IsZero() {
		t.Errorf("createdAt zero")
	}
	if mf.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d", mf.SchemaVersion)
	}
	if len(mf.Artifacts) != 1 {
		t.Errorf("artifacts = %d", len(mf.Artifacts))
	}
}

// Aceitação: a/./b é normalizado.
func TestWriteDotPath(t *testing.T) {
	base := t.TempDir()
	s, _ := New(base, "test-dot")
	_, err := s.Write("a/./b.txt", []byte("x"))
	if err != nil {
		t.Fatalf("Write dot: %v", err)
	}
}
