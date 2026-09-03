// Package artifacts — filesystem store com atomic writes, SHA-256 e
// manifest de artefatos por run.
//
// Layout:
//
//	<baseDir>/
//	  runs/
//	    <run-id>/
//	      manifest.json
//	      <artifact-name-1>
//	      <artifact-name-2>
//	      ...
//
// Saída é determinística: cada artefato tem SHA-256 e o manifest
// permite auditoria/reproducibilidade.
package artifacts

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// SchemaVersion do manifest. Incrementar em mudanças incompatíveis.
const SchemaVersion = 1

// File/dir permissions.
const (
	dirPerm  = 0o755
	filePerm = 0o644
)

// Artifact representa um arquivo escrito no run dir.
type Artifact struct {
	Name    string    `json:"name"`    // path relativo ao run dir
	Size    int64     `json:"size"`    // bytes
	SHA256  string    `json:"sha256"`  // hex
	Written time.Time `json:"written"` // mtime
}

// Manifest lista todos os artefatos de um run.
type Manifest struct {
	RunID         string     `json:"run_id"`
	CreatedAt     time.Time  `json:"created_at"`
	SchemaVersion int        `json:"schema_version"`
	Artifacts     []Artifact `json:"artifacts"`
	Meta          Meta       `json:"meta"`
}

// Meta são pares chave-valor de proveniência do run (branch, commit,
// base, head, etc).
type Meta map[string]string

// Store é um run dir ativo. Thread-safe.
type Store struct {
	mu       sync.Mutex
	dir      string
	runID    string
	mf       Manifest
	manifest string // path do manifest.json
}

// New cria um novo Store em <baseDir>/runs/<run-id>/. Cria os diretórios
// se necessário. runID pode ser vazio — gera um baseado em timestamp + random.
func New(baseDir, runID string) (*Store, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("artifacts: baseDir vazio")
	}
	if runID == "" {
		runID = newRunID()
	}
	dir := filepath.Join(baseDir, "runs", runID)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("artifacts: mkdir run dir: %w", err)
	}
	s := &Store{
		dir:      dir,
		runID:    runID,
		manifest: filepath.Join(dir, "manifest.json"),
		mf: Manifest{
			RunID:         runID,
			CreatedAt:     time.Now().UTC(),
			SchemaVersion: SchemaVersion,
			Artifacts:     []Artifact{},
			Meta:          Meta{},
		},
	}
	return s, nil
}

// FromDir carrega um Store existente a partir do run dir (lê o manifest
// se existir). Útil pra retomar leitura.
func FromDir(dir string) (*Store, error) {
	mfPath := filepath.Join(dir, "manifest.json")
	s := &Store{
		dir:      dir,
		runID:    filepath.Base(dir),
		manifest: mfPath,
		mf: Manifest{
			RunID:         filepath.Base(dir),
			SchemaVersion: SchemaVersion,
			Artifacts:     []Artifact{},
			Meta:          Meta{},
		},
	}
	data, err := os.ReadFile(mfPath)
	if err == nil {
		if err := json.Unmarshal(data, &s.mf); err != nil {
			return nil, fmt.Errorf("artifacts: parse manifest: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("artifacts: read manifest: %w", err)
	}
	return s, nil
}

// Dir devolve o path absoluto do run dir.
func (s *Store) Dir() string { return s.dir }

// RunID devolve o identificador do run.
func (s *Store) RunID() string { return s.runID }

// SetMeta grava um par chave-valor de proveniência. Pode ser chamado
// múltiplas vezes; o último valor vence.
func (s *Store) SetMeta(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mf.Meta == nil {
		s.mf.Meta = Meta{}
	}
	s.mf.Meta[key] = value
}

// Write grava data como name (path relativo ao run dir). Escrita atômica
// via temp + rename.
func (s *Store) Write(name string, data []byte) (Artifact, error) {
	return s.WriteReader(name, bytesReader(data))
}

// WriteReader grava conteúdo de r em name. Computa SHA-256 durante
// escrita. Usa temp file + rename para atomicidade.
func (s *Store) WriteReader(name string, r io.Reader) (Artifact, error) {
	if name == "" {
		return Artifact{}, fmt.Errorf("artifacts: name vazio")
	}
	if name == "manifest.json" {
		return Artifact{}, fmt.Errorf("artifacts: name reservado: %s", name)
	}
	full := filepath.Join(s.dir, name)
	if !filepath.IsLocal(name) {
		return Artifact{}, fmt.Errorf("artifacts: name não-local: %s", name)
	}
	if err := os.MkdirAll(filepath.Dir(full), dirPerm); err != nil {
		return Artifact{}, fmt.Errorf("artifacts: mkdir: %w", err)
	}

	// Lê tudo (com tee em sha256) para um buffer.
	h := sha256.New()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			h.Write(tmp[:n])
			buf = append(buf, tmp[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return Artifact{}, fmt.Errorf("artifacts: read: %w", err)
		}
	}

	// Atomic write: temp + rename.
	tmpPath, err := tempPath(full)
	if err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(tmpPath, buf, filePerm); err != nil {
		return Artifact{}, fmt.Errorf("artifacts: write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, full); err != nil {
		_ = os.Remove(tmpPath)
		return Artifact{}, fmt.Errorf("artifacts: rename: %w", err)
	}

	art := Artifact{
		Name:    name,
		Size:    int64(len(buf)),
		SHA256:  hex.EncodeToString(h.Sum(nil)),
		Written: time.Now().UTC(),
	}
	s.mu.Lock()
	s.mf.Artifacts = append(s.mf.Artifacts, art)
	s.mu.Unlock()
	return art, nil
}

// Read lê o conteúdo de nome (path relativo). Não inclui manifest.
func (s *Store) Read(name string) ([]byte, error) {
	if name == "" {
		return nil, fmt.Errorf("artifacts: name vazio")
	}
	if !filepath.IsLocal(name) {
		return nil, fmt.Errorf("artifacts: name não-local: %s", name)
	}
	return os.ReadFile(filepath.Join(s.dir, name))
}

// List devolve os artefatos conhecidos (via manifest em memória).
func (s *Store) List() []Artifact {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Artifact, len(s.mf.Artifacts))
	copy(out, s.mf.Artifacts)
	return out
}

// FlushManifest serializa o manifest em manifest.json atomicamente.
// Pode ser chamado múltiplas vezes — vai sempre ler e re-gravar.
func (s *Store) FlushManifest() error {
	s.mu.Lock()
	s.mf.Artifacts = sortArtifacts(s.mf.Artifacts)
	s.mf.SchemaVersion = SchemaVersion
	s.mf.CreatedAt = s.mf.CreatedAt.UTC()
	data, err := json.MarshalIndent(&s.mf, "", "  ")
	path := s.manifest
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("artifacts: marshal: %w", err)
	}

	// Atomic write do manifest.
	tmp, err := tempPath(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, filePerm); err != nil {
		return fmt.Errorf("artifacts: write manifest tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("artifacts: rename manifest: %w", err)
	}
	return nil
}

// SaveManifest é um atalho para gravar + retornar o manifest em JSON.
func (s *Store) SaveManifest() (Manifest, error) {
	if err := s.FlushManifest(); err != nil {
		return Manifest{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := s.mf
	cp.Artifacts = sortArtifacts(cp.Artifacts)
	return cp, nil
}

// newRunID gera um identificador curto (timestamp + 6 hex chars).
func newRunID() string {
	ts := time.Now().UTC().Format("20060102T150405")
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		// fallback determinístico se random falhar.
		return ts + "-000000"
	}
	return fmt.Sprintf("%s-%s", ts, hex.EncodeToString(b[:]))
}

// tempPath devolve <path>.tmp-<random> mesmo diretório.
func tempPath(path string) (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("artifacts: random: %w", err)
	}
	return fmt.Sprintf("%s.tmp-%s", path, hex.EncodeToString(b[:])), nil
}

// bytesReader devolve io.Reader sobre slice.
func bytesReader(b []byte) io.Reader {
	return &sliceReader{b: b}
}

type sliceReader struct {
	b []byte
	i int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

// sortArtifacts devolve cópia ordenada por nome.
func sortArtifacts(in []Artifact) []Artifact {
	out := make([]Artifact, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
