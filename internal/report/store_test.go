package report

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// helpers.
func sampleReport(id string) *Report {
	return &Report{
		SchemaVersion: SchemaVersion,
		Run: RunInfo{
			ID:             id,
			Profile:        "release",
			SolidifyVersion: "1.0",
			StartedAt:      "2026-01-01T00:00:00Z",
			FinishedAt:     "2026-01-01T00:00:01Z",
			ConfigHash:     strings.Repeat("a", 64),
			EvidenceHash:   strings.Repeat("b", 64),
		},
		Git: GitInfo{
			BaseRef: "main", BaseSHA: "abc",
			HeadRef: "feat", HeadSHA: "def",
			Commits:      []Commit{},
			ChangedFiles: []ChangedFile{},
		},
		Components:      []Component{},
		ReleaseNotes:    ReleaseNotes{ExecutiveSummary: "x"},
		Migrations:      []Migration{},
		EnvChanges:      []EnvChange{},
		Analyzers:       []Analyzer{},
		SOLID:           SOLIDBlock{Principles: map[string]Principle{}},
		AIReview:        AIReview{Mode: "peer"},
		Scores:          ScoresBlock{Grade: "A"},
		Risk:            RiskBlock{Level: "LOW"},
		QualityGate:     QualityGate{Status: "PASS"},
		Recommendations: []Recommendation{},
		Limitations:     []string{},
		Artifacts:       []Artifact{},
	}
}

// Aceitação: finalize básico.
func TestFinalizeBasic(t *testing.T) {
	s := NewStore()
	r := sampleReport("r1")
	sr, err := s.Finalize(r, "/tmp/r1.json")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if sr.RunID != "r1" {
		t.Errorf("runid")
	}
	if len(sr.ContentHash) != 64 {
		t.Errorf("hash len")
	}
}

// Aceitação: finalize idempotente (mesmo conteúdo).
func TestFinalizeIdempotent(t *testing.T) {
	s := NewStore()
	r := sampleReport("r1")
	sr1, _ := s.Finalize(r, "/tmp/r1.json")
	sr2, _ := s.Finalize(r, "/tmp/r1.json")
	if sr1.ContentHash != sr2.ContentHash {
		t.Errorf("idempotente")
	}
	if s.Count() != 1 {
		t.Errorf("count: %d", s.Count())
	}
}

// Aceitação: finalize mutado → erro.
func TestFinalizeMutated(t *testing.T) {
	s := NewStore()
	r := sampleReport("r1")
	if _, err := s.Finalize(r, "/tmp/r1.json"); err != nil {
		t.Fatalf("primeiro: %v", err)
	}
	r.Run.SolidifyVersion = "2.0"
	if _, err := s.Finalize(r, "/tmp/r1.json"); err == nil {
		t.Errorf("mutado aceito")
	}
}

// Aceitação: finalize nil.
func TestFinalizeNil(t *testing.T) {
	s := NewStore()
	if _, err := s.Finalize(nil, ""); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: finalize run_id vazio.
func TestFinalizeEmptyID(t *testing.T) {
	s := NewStore()
	r := sampleReport("")
	if _, err := s.Finalize(r, ""); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: finalize bytes.
func TestFinalizeBytes(t *testing.T) {
	s := NewStore()
	data := []byte(`{"hello": "world"}`)
	sr, err := s.FinalizeBytes("r1", data, "/tmp/x.json")
	if err != nil {
		t.Fatalf("bytes: %v", err)
	}
	if sr.SizeBytes != len(data) {
		t.Errorf("size")
	}
}

// Aceitação: finalize bytes idempotente.
func TestFinalizeBytesIdempotent(t *testing.T) {
	s := NewStore()
	data := []byte(`{"x": 1}`)
	if _, err := s.FinalizeBytes("r1", data, ""); err != nil {
		t.Fatalf("first")
	}
	if _, err := s.FinalizeBytes("r1", data, ""); err != nil {
		t.Errorf("idemp")
	}
}

// Aceitação: finalize bytes mutado.
func TestFinalizeBytesMutated(t *testing.T) {
	s := NewStore()
	if _, err := s.FinalizeBytes("r1", []byte(`{"x":1}`), ""); err != nil {
		t.Fatalf("first")
	}
	if _, err := s.FinalizeBytes("r1", []byte(`{"x":2}`), ""); err == nil {
		t.Errorf("mutado")
	}
}

// Aceitação: Get / ByHash.
func TestGetByHash(t *testing.T) {
	s := NewStore()
	r := sampleReport("r1")
	sr, _ := s.Finalize(r, "")
	if got, ok := s.Get("r1"); !ok || got.RunID != sr.RunID || got.ContentHash != sr.ContentHash {
		t.Errorf("get")
	}
	if _, ok := s.Get("r2"); ok {
		t.Errorf("missing")
	}
	ids := s.ByHash(sr.ContentHash)
	if len(ids) != 1 || ids[0] != "r1" {
		t.Errorf("byhash")
	}
}

// Aceitação: VerifyHash bytes.
func TestVerifyHashBytes(t *testing.T) {
	s := NewStore()
	data := []byte(`{"x":1}`)
	if _, err := s.FinalizeBytes("r1", data, ""); err != nil {
		t.Fatalf("finalize")
	}
	if err := s.VerifyHash("r1", data); err != nil {
		t.Errorf("verify ok: %v", err)
	}
	if err := s.VerifyHash("r1", []byte(`{"x":2}`)); err == nil {
		t.Errorf("verify fail")
	}
	if err := s.VerifyHash("r2", data); err == nil {
		t.Errorf("verify missing")
	}
}

// Aceitação: VerifyReport.
func TestVerifyReport(t *testing.T) {
	s := NewStore()
	r := sampleReport("r1")
	if _, err := s.Finalize(r, ""); err != nil {
		t.Fatalf("finalize")
	}
	if err := s.VerifyReport(r); err != nil {
		t.Errorf("verify: %v", err)
	}
	r2 := sampleReport("r2")
	if err := s.VerifyReport(r2); err == nil {
		t.Errorf("missing")
	}
	var nilR *Report
	if err := s.VerifyReport(nilR); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: List ordenado.
func TestListOrdered(t *testing.T) {
	s := NewStore()
	s.Finalize(sampleReport("z"), "")
	s.Finalize(sampleReport("a"), "")
	s.Finalize(sampleReport("m"), "")
	list := s.List()
	if len(list) != 3 {
		t.Errorf("count")
	}
	if list[0].RunID != "a" || list[1].RunID != "m" || list[2].RunID != "z" {
		t.Errorf("ordem")
	}
}

// Aceitação: MarshalSnapshot.
func TestMarshalSnapshot(t *testing.T) {
	s := NewStore()
	s.Finalize(sampleReport("r1"), "/p1")
	s.Finalize(sampleReport("r2"), "/p2")
	b, err := s.MarshalSnapshot()
	if err != nil {
		t.Fatalf("snap: %v", err)
	}
	if !bytes.Contains(b, []byte("r1")) || !bytes.Contains(b, []byte("r2")) {
		t.Errorf("ids")
	}
}

// Aceitação: NewRunID determinístico.
func TestNewRunID(t *testing.T) {
	id := NewRunID("run", time.Unix(1700000000, 0))
	if id != "run-1700000000000000000" {
		t.Errorf("runid: %s", id)
	}
	id2 := NewRunID("", time.Unix(1700000000, 0))
	if !strings.HasPrefix(id2, "run-") {
		t.Errorf("default")
	}
}

// Aceitação: ByHash vazio.
func TestByHashEmpty(t *testing.T) {
	s := NewStore()
	ids := s.ByHash("nonexistent")
	if len(ids) != 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: Get mutável retornar ponteiro mas imutável conceitual.
func TestGetReturnsStored(t *testing.T) {
	s := NewStore()
	r := sampleReport("r1")
	s.Finalize(r, "")
	got, _ := s.Get("r1")
	if got == nil || got.RunID != "r1" {
		t.Errorf("get")
	}
	// mutar retornado não afeta store (caller não deve).
	got.RunID = "hacked"
	got2, _ := s.Get("r1")
	if got2.RunID != "r1" {
		t.Errorf("store mutado")
	}
}
