package peer

import (
	"strings"
	"testing"
)

// Aceitação: NewEvidenceShardSet basic.
func TestNewEvidenceShardSet(t *testing.T) {
	s, err := NewEvidenceShardSet("r1", []EvidenceShard{
		{ID: "s1", Kind: "manifest", Content: "x"},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.RunID != "r1" {
		t.Errorf("runid")
	}
	if s.TotalBytes != 1 {
		t.Errorf("bytes")
	}
}

// Aceitação: NewEvidenceShardSet vazio.
func TestNewEvidenceShardSetEmptyRunID(t *testing.T) {
	if _, err := NewEvidenceShardSet("", nil); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Hash determinístico.
func TestEvidenceHashStable(t *testing.T) {
	s1, _ := NewEvidenceShardSet("r1", []EvidenceShard{{ID: "a", Content: "x"}})
	s2, _ := NewEvidenceShardSet("r1", []EvidenceShard{{ID: "a", Content: "x"}})
	if s1.Hash() != s2.Hash() {
		t.Errorf("stable")
	}
}

// Aceitação: Hash sensível.
func TestEvidenceHashSensitive(t *testing.T) {
	s1, _ := NewEvidenceShardSet("r1", []EvidenceShard{{ID: "a", Content: "x"}})
	s2, _ := NewEvidenceShardSet("r1", []EvidenceShard{{ID: "a", Content: "y"}})
	if s1.Hash() == s2.Hash() {
		t.Errorf("sensitive")
	}
}

// Aceitação: Hash nil safe.
func TestEvidenceHashNil(t *testing.T) {
	var s *EvidenceShardSet
	if s.Hash() != "" {
		t.Errorf("nil hash vazio")
	}
}

// Aceitação: IDs.
func TestEvidenceIDs(t *testing.T) {
	s, _ := NewEvidenceShardSet("r", []EvidenceShard{
		{ID: "a"}, {ID: "b"}, {ID: "c"},
	})
	ids := s.IDs()
	if len(ids) != 3 || ids[0] != "a" {
		t.Errorf("ids")
	}
}

// Aceitação: IDs nil safe.
func TestEvidenceIDsNil(t *testing.T) {
	var s *EvidenceShardSet
	if s.IDs() != nil {
		t.Errorf("nil")
	}
}

// Aceitação: FilterByKind.
func TestEvidenceFilterKind(t *testing.T) {
	s, _ := NewEvidenceShardSet("r", []EvidenceShard{
		{ID: "1", Kind: "manifest"},
		{ID: "2", Kind: "diff"},
		{ID: "3", Kind: "manifest"},
	})
	out := s.FilterByKind("manifest")
	if len(out) != 2 {
		t.Errorf("filter")
	}
}

// Aceitação: FilterByKind nil.
func TestEvidenceFilterNil(t *testing.T) {
	var s *EvidenceShardSet
	if s.FilterByKind("x") != nil {
		t.Errorf("nil")
	}
}

// Aceitação: Build basic.
func TestBuildBasic(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r1", []EvidenceShard{{ID: "a", Content: "x"}})
	b := NewRequestBuilder("run {{run_id}} actor {{actor}} schema {{schema_version}} evidence {{evidence_hash}}", "1")
	r, err := b.Build(RequestOptions{RunID: "r1", Actor: "alice", Evidence: evidence})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.RunID != "r1" || r.Actor != "alice" {
		t.Errorf("ids")
	}
	if !strings.Contains(r.Prompt, "r1") {
		t.Errorf("render")
	}
	if !strings.Contains(r.Prompt, "alice") {
		t.Errorf("actor")
	}
}

// Aceitação: Build missing RunID.
func TestBuildMissingRunID(t *testing.T) {
	b := NewRequestBuilder("x", "1")
	if _, err := b.Build(RequestOptions{}); err == nil {
		t.Errorf("runid vazio")
	}
}

// Aceitação: Build missing Actor.
func TestBuildMissingActor(t *testing.T) {
	b := NewRequestBuilder("x", "1")
	if _, err := b.Build(RequestOptions{RunID: "r"}); err == nil {
		t.Errorf("actor vazio")
	}
}

// Aceitação: Build missing Evidence.
func TestBuildMissingEvidence(t *testing.T) {
	b := NewRequestBuilder("x", "1")
	if _, err := b.Build(RequestOptions{RunID: "r", Actor: "a"}); err == nil {
		t.Errorf("evidence nil")
	}
}

// Aceitação: Build hash populated.
func TestBuildHash(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	b := NewRequestBuilder("test", "1")
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	if r.Hash == "" {
		t.Errorf("hash vazio")
	}
}

// Aceitação: Build schema version override.
func TestBuildSchemaOverride(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	b := NewRequestBuilder("v{{schema_version}}", "1")
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence, SchemaVersion: "2"})
	if !strings.Contains(r.Prompt, "v2") {
		t.Errorf("override")
	}
	if r.Schema != "1" {
		t.Errorf("builder schema preservado")
	}
}

// Aceitação: Build vars extras.
func TestBuildExtraVars(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	b := NewRequestBuilder("branch={{branch}}", "1")
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence, Vars: map[string]string{"branch": "main"}})
	if !strings.Contains(r.Prompt, "main") {
		t.Errorf("extra")
	}
}

// Aceitação: HasPeerAOutput marker.
func TestHasPeerAOutputMarker(t *testing.T) {
	b := NewRequestBuilder("PEER_A_OUTPUT: secret", "1")
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	if !r.HasPeerAOutput("anything") {
		t.Errorf("marker")
	}
}

// Aceitação: HasPeerAOutput substring.
func TestHasPeerAOutputSubstring(t *testing.T) {
	b := NewRequestBuilder("review of {{run_id}} with note 'peer A said this is great code by alice'", "1")
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	if !r.HasPeerAOutput("this is great code by alice") {
		t.Errorf("substring match")
	}
}

// Aceitação: HasPeerAOutput clean.
func TestHasPeerAOutputClean(t *testing.T) {
	b := NewRequestBuilder("review of {{run_id}}", "1")
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	if r.HasPeerAOutput("something else") {
		t.Errorf("clean")
	}
}

// Aceitação: HasPeerAOutput nil safe.
func TestHasPeerAOutputNil(t *testing.T) {
	var r *PeerRequest
	if r.HasPeerAOutput("x") {
		t.Errorf("nil")
	}
}

// Aceitação: VerifyIndependence.
func TestVerifyIndependence(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	b := NewRequestBuilder("review", "1")
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	issues := r.VerifyIndependence("peerA said this is bad")
	if len(issues) != 0 {
		t.Errorf("clean: %v", issues)
	}
}

// Aceitação: VerifyIndependence detecta vazio.
func TestVerifyIndependenceEmptyEvidence(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{})
	b := NewRequestBuilder("x", "1")
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	issues := r.VerifyIndependence("")
	if len(issues) == 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: FormatRequest.
func TestFormatRequest(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	b := NewRequestBuilder("x", "1")
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "alice", Evidence: evidence})
	out := FormatRequest(r)
	if !strings.Contains(out, "r") || !strings.Contains(out, "alice") {
		t.Errorf("format")
	}
	if FormatRequest(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: MarshalJSONBytes.
func TestMarshalJSON(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	b := NewRequestBuilder("x", "1")
	r, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	data, err := r.MarshalJSONBytes()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("empty")
	}
}

// Aceitação: Build determinístico.
func TestBuildDeterministic(t *testing.T) {
	evidence, _ := NewEvidenceShardSet("r", []EvidenceShard{{ID: "a", Content: "x"}})
	b := NewRequestBuilder("test {{run_id}}", "1")
	r1, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	r2, _ := b.Build(RequestOptions{RunID: "r", Actor: "a", Evidence: evidence})
	if r1.Hash != r2.Hash {
		t.Errorf("deterministic")
	}
}

// Aceitação: Default schema version.
func TestDefaultSchemaVersion(t *testing.T) {
	b := NewRequestBuilder("x", "")
	if b.schemaVersion != "1" {
		t.Errorf("default")
	}
}
