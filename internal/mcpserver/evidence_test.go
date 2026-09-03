package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Aceitação: ResolveStore dir/runID vazio.
func TestResolveStoreEmpty(t *testing.T) {
	if _, err := ResolveStore("", "r1"); err == nil {
		t.Errorf("base_dir vazio")
	}
	if _, err := ResolveStore("/tmp", ""); err == nil {
		t.Errorf("run_id vazio")
	}
}

// Aceitação: ResolveStore criação.
func TestResolveStore(t *testing.T) {
	dir := t.TempDir()
	s, err := ResolveStore(dir, "r1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s == nil {
		t.Errorf("nil")
	}
}

// Aceitação: splitLines.
func TestSplitLines(t *testing.T) {
	if len(splitLines("")) != 0 {
		t.Errorf("empty")
	}
	if len(splitLines("a\nb")) != 2 {
		t.Errorf("split")
	}
}

// Aceitação: trim.
func TestTrim(t *testing.T) {
	if s := trim([]byte("  abc  ")); string(s) != "abc" {
		t.Errorf("trim = %s", s)
	}
	if len(trim([]byte(""))) != 0 {
		t.Errorf("empty")
	}
}

// Aceitação: rawJSON object.
func TestRawJSONObject(t *testing.T) {
	v := rawJSON([]byte(`{"a":1}`))
	if _, ok := v.(map[string]any); !ok {
		t.Errorf("objeto = map")
	}
}

// Aceitação: rawJSON string.
func TestRawJSONString(t *testing.T) {
	v := rawJSON([]byte("plain text"))
	if s, ok := v.(string); !ok || s != "plain text" {
		t.Errorf("string")
	}
}

// Aceitação: rawJSON vazio.
func TestRawJSONEmpty(t *testing.T) {
	if rawJSON([]byte("")) != nil {
		t.Errorf("empty = nil")
	}
}

// Aceitação: anySlice.
func TestAnySlice(t *testing.T) {
	out := anySlice([]string{"a", "b"})
	if len(out) != 2 {
		t.Errorf("len")
	}
	if out[0].(string) != "a" {
		t.Errorf("val")
	}
}

// Aceitação: DefaultEvidenceHandler básico.
func TestDefaultEvidenceHandler(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "runs", "r1"), 0755)
	os.WriteFile(filepath.Join(dir, "runs", "r1", "evidence.json"), []byte(`{"run_id":"r1"}`), 0644)
	h := DefaultEvidenceHandler(dir)
	out, err := h(context.Background(), &EvidenceGetInput{RunID: "r1", Kind: KindManifest})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.Total != 1 {
		t.Errorf("total")
	}
}

// Aceitação: DefaultEvidenceHandler run_id vazio.
func TestDefaultEvidenceHandlerEmptyRunID(t *testing.T) {
	h := DefaultEvidenceHandler("/tmp")
	if _, err := h(context.Background(), &EvidenceGetInput{Kind: KindManifest}); err == nil {
		t.Errorf("run_id vazio")
	}
}

// Aceitação: DefaultEvidenceHandler kind unknown.
func TestDefaultEvidenceHandlerUnknownKind(t *testing.T) {
	h := DefaultEvidenceHandler("/tmp")
	if _, err := h(context.Background(), &EvidenceGetInput{RunID: "r1", Kind: "x"}); err == nil {
		t.Errorf("kind unknown")
	}
}

// Aceitação: DefaultEvidenceHandler limits aplicados.
func TestDefaultEvidenceHandlerLimits(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "runs", "r1", "context"), 0755)
	os.WriteFile(filepath.Join(dir, "runs", "r1", "context", "f.txt"), []byte("a\nb\nc\nd\ne"), 0644)
	h := DefaultEvidenceHandler(dir)
	out, err := h(context.Background(), &EvidenceGetInput{RunID: "r1", Kind: KindContext, Path: "f.txt", Limit: 2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.Items) != 2 {
		t.Errorf("limit = %d", len(out.Items))
	}
	if !out.Truncated {
		t.Errorf("truncated")
	}
}

// Aceitação: DefaultEvidenceHandler limit cap.
func TestDefaultEvidenceHandlerLimitCap(t *testing.T) {
	h := DefaultEvidenceHandler("/tmp")
	// verificação via criar evidence + offset grande:
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "runs", "r1", "context"), 0755)
	os.WriteFile(filepath.Join(dir, "runs", "r1", "context", "f.txt"), []byte("x"), 0644)
	h = DefaultEvidenceHandler(dir)
	in := &EvidenceGetInput{RunID: "r1", Kind: KindContext, Path: "f.txt", Limit: 10000}
	_, err := h(context.Background(), in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if in.Limit > MaxLimit {
		t.Errorf("cap in.Limit")
	}
}

// Aceitação: ErrEvidence type.
func TestEvidenceError(t *testing.T) {
	e := errEvidence("test %s", "x")
	if e.Error() != "evidence: test x" {
		t.Errorf("format")
	}
}

// Aceitação: DiffChunk missing path.
func TestDiffChunkMissingPath(t *testing.T) {
	h := DefaultEvidenceHandler("/tmp")
	if _, err := h(context.Background(), &EvidenceGetInput{RunID: "r1", Kind: KindDiff}); err == nil {
		t.Errorf("path obrigatório")
	}
}

// Aceitação: Symbol search missing query.
func TestSymbolSearchMissingQuery(t *testing.T) {
	h := DefaultEvidenceHandler("/tmp")
	if _, err := h(context.Background(), &EvidenceGetInput{RunID: "r1", Kind: KindSymbols}); err == nil {
		t.Errorf("query obrigatória")
	}
}

// Aceitação: Manifest missing.
func TestManifestMissing(t *testing.T) {
	h := DefaultEvidenceHandler("/tmp")
	if _, err := h(context.Background(), &EvidenceGetInput{RunID: "r1", Kind: KindManifest}); err == nil {
		t.Errorf("manifest missing")
	}
}
