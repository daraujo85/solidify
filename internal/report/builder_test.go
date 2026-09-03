package report

import (
	"strings"
	"testing"
	"time"
)

// Aceitação: build básico.
func TestBuildBasic(t *testing.T) {
	in := BuilderInput{
		RunID: "r1", Profile: "release",
		SolidifyVersion: "1.0",
		StartedAt:      time.Now(),
		FinishedAt:     time.Now().Add(time.Second),
		ConfigHash:     strings.Repeat("a", 64),
		EvidenceHash:   strings.Repeat("b", 64),
	}
	r, err := NewBuilder(in).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if r.SchemaVersion != SchemaVersion {
		t.Errorf("schema version")
	}
	if r.Run.ID != "r1" {
		t.Errorf("run id")
	}
}

// Aceitação: RunID vazio.
func TestBuildEmptyRunID(t *testing.T) {
	if _, err := NewBuilder(BuilderInput{Profile: "release"}).Build(); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Profile vazio.
func TestBuildEmptyProfile(t *testing.T) {
	if _, err := NewBuilder(BuilderInput{RunID: "r1"}).Build(); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: slices default → vazios.
func TestBuildDefaultSlices(t *testing.T) {
	r, _ := NewBuilder(BuilderInput{
		RunID: "r1", Profile: "release",
		StartedAt: time.Now(), FinishedAt: time.Now(),
		ConfigHash:   strings.Repeat("a", 64),
		EvidenceHash: strings.Repeat("b", 64),
	}).Build()
	if r.Components == nil {
		t.Errorf("components nil")
	}
	if r.Migrations == nil {
		t.Errorf("migrations nil")
	}
	if r.EnvChanges == nil {
		t.Errorf("env nil")
	}
	if r.Analyzers == nil {
		t.Errorf("analyzers nil")
	}
	if r.Recommendations == nil {
		t.Errorf("rec nil")
	}
	if r.Limitations == nil {
		t.Errorf("lim nil")
	}
	if r.Artifacts == nil {
		t.Errorf("art nil")
	}
	if r.SOLID.Principles == nil {
		t.Errorf("princ nil")
	}
}

// Aceitação: hash determinístico.
func TestHashDeterministic(t *testing.T) {
	in := BuilderInput{
		RunID: "r1", Profile: "release",
		StartedAt:    time.Unix(1700000000, 0).UTC(),
		FinishedAt:   time.Unix(1700000060, 0).UTC(),
		ConfigHash:   strings.Repeat("a", 64),
		EvidenceHash: strings.Repeat("b", 64),
	}
	r1, _ := NewBuilder(in).Build()
	r2, _ := NewBuilder(in).Build()
	h1, _ := r1.HashContent()
	h2, _ := r2.HashContent()
	if h1 != h2 {
		t.Errorf("hash: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("sha256 len")
	}
}

// Aceitação: hash nil.
func TestHashNil(t *testing.T) {
	if _, err := (*Report)(nil).HashContent(); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: schema version sempre constante.
func TestMarshalSchemaVersion(t *testing.T) {
	r := &Report{}
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), SchemaVersion) {
		t.Errorf("schema version no json")
	}
}

// Aceitação: marshal + unmarshal round-trip.
func TestMarshalRoundTrip(t *testing.T) {
	r, _ := NewBuilder(BuilderInput{
		RunID: "r1", Profile: "release",
		StartedAt:    time.Unix(1700000000, 0).UTC(),
		FinishedAt:   time.Unix(1700000060, 0).UTC(),
		ConfigHash:   strings.Repeat("a", 64),
		EvidenceHash: strings.Repeat("b", 64),
	}).Build()
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Report
	if err := jsonUnmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Run.ID != "r1" {
		t.Errorf("roundtrip")
	}
}

// Aceitação: build com SOLID/AI preenchidos.
func TestBuildWithContent(t *testing.T) {
	in := BuilderInput{
		RunID: "r1", Profile: "release",
		StartedAt: time.Now(), FinishedAt: time.Now(),
		ConfigHash:   strings.Repeat("a", 64),
		EvidenceHash: strings.Repeat("b", 64),
		Git: GitInfo{
			BaseRef: "main", BaseSHA: "abc",
			HeadRef: "feat", HeadSHA: "def",
			Commits:      []Commit{{SHA: "abc", ShortSHA: "abc", Subject: "x", Type: "feat"}},
			ChangedFiles: []ChangedFile{{Path: "x.go", Status: "M"}},
		},
		Analyzers:   []Analyzer{{ID: "test", Version: "1", Applicability: "APPLICABLE", DurationMS: 100}},
		QualityGate: QualityGate{Status: "PASS"},
		Risk:        RiskBlock{Level: "LOW"},
	}
	r, _ := NewBuilder(in).Build()
	if len(r.Git.Commits) != 1 {
		t.Errorf("commits")
	}
	if len(r.Analyzers) != 1 {
		t.Errorf("analyzers")
	}
	if r.QualityGate.Status != "PASS" {
		t.Errorf("gate")
	}
}

// Aceitação: canonical marshal ordena chaves.
func TestCanonicalKeys(t *testing.T) {
	m := map[string]any{"b": 2, "a": 1, "c": 3}
	b, err := marshalCanonical(m)
	if err != nil {
		t.Fatalf("canon: %v", err)
	}
	s := string(b)
	idxA := strings.Index(s, `"a"`)
	idxB := strings.Index(s, `"b"`)
	idxC := strings.Index(s, `"c"`)
	if !(idxA < idxB && idxB < idxC) {
		t.Errorf("ordem: %s", s)
	}
}

// Aceitação: canonical array.
func TestCanonicalArray(t *testing.T) {
	a := []any{1, 2, "x"}
	b, _ := marshalCanonical(a)
	if string(b) != `[1,2,"x"]` {
		t.Errorf("array: %s", b)
	}
}

// Aceitação: canonical nested.
func TestCanonicalNested(t *testing.T) {
	m := map[string]any{"x": map[string]any{"y": 2, "a": 1}, "z": []any{1, 2}}
	b, _ := marshalCanonical(m)
	s := string(b)
	if !strings.Contains(s, `"a":1`) || !strings.Contains(s, `"y":2`) {
		t.Errorf("nested: %s", s)
	}
}
