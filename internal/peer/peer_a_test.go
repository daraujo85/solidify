// Smoke tests para peer_a dispatcher (SAI-118 + SAI-119 + SAI-120).
//
// Cobre:
//   - SourceTerminalMCP polling lê submission do PeerReviewStore
//   - SourceTerminalMCP sem submission dentro do timeout = skipped
//   - v2 CanonicalPayload (nativo) — caminho feliz
//   - v1 Notes JSON-string — compat legacy
//   - SourceHTTPCombo delega para peer.Executor
//   - SourceAuto resolve para HTTP combo
package peer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/diegoaraujo/solidify/internal/mcpserver"
)

// helper: cria PeerReviewStore + record v2 (CanonicalPayload).
func seedStoreV2(t *testing.T, runID string, payload map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	store, err := mcpserver.NewPeerReviewStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	payloadJSON, _ := json.Marshal(payload)
	sum := sha256.Sum256(payloadJSON)
	in := &mcpserver.SubmitPeerReviewInput{
		RunID:            runID,
		Actor:            "peer_a",
		Schema:           mcpserver.PeerReviewSchemaVersion, // "2"
		EvidenceHash:     hex.EncodeToString(sum[:]),
		Verdict:          "comment",
		CanonicalPayload: payload,
	}
	rec := &mcpserver.PeerReviewRecord{
		ReviewID:         "rec-" + runID,
		RunID:            in.RunID,
		Actor:            in.Actor,
		Schema:           in.Schema,
		EvidenceHash:     in.EvidenceHash,
		Verdict:          in.Verdict,
		CanonicalPayload: in.CanonicalPayload,
		SubmittedAt:      time.Now(),
		ContentHash:      mcpserver.ComputeContentHash(in),
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	return dir
}

// helper: cria PeerReviewStore + record v1 (Notes JSON-string legacy).
func seedStoreV1(t *testing.T, runID string, notes map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	store, err := mcpserver.NewPeerReviewStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	notesJSON, _ := json.Marshal(notes)
	sum := sha256.Sum256([]byte(notesJSON))
	in := &mcpserver.SubmitPeerReviewInput{
		RunID:        runID,
		Actor:        "peer_a",
		Schema:       "1", // v1 legacy
		EvidenceHash: hex.EncodeToString(sum[:]),
		Verdict:      "comment",
		Notes:        string(notesJSON),
	}
	rec := &mcpserver.PeerReviewRecord{
		ReviewID:     "rec-v1-" + runID,
		RunID:        in.RunID,
		Actor:        in.Actor,
		Schema:       in.Schema,
		EvidenceHash: in.EvidenceHash,
		Verdict:      in.Verdict,
		Notes:        in.Notes,
		SubmittedAt:  time.Now(),
		ContentHash:  mcpserver.ComputeContentHash(in),
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	return dir
}

// TestExecuteTerminalMCP_V2_PollFindsRecord valida caminho feliz v2:
// record com CanonicalPayload nativo → parsed corretamente.
func TestExecuteTerminalMCP_V2_PollFindsRecord(t *testing.T) {
	runID := "r-v2-1"
	dir := seedStoreV2(t, runID, map[string]any{
		"run_id":        runID,
		"actor":         "peer_a",
		"verdict":       "comment",
		"quality_score": 85.0,
		"confidence":    0.9,
		"solid": map[string]any{
			"S": map[string]any{"after_score": map[string]any{"value": 85.0}},
			"O": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"L": map[string]any{"after_score": map[string]any{"value": 90.0}},
			"I": map[string]any{"after_score": map[string]any{"value": 85.0}},
			"D": map[string]any{"after_score": map[string]any{"value": 80.0}},
		},
	})
	res, called, err := executeTerminalMCP(context.Background(), PeerAOptions{
		Source:   SourceTerminalMCP,
		RunID:    runID,
		StoreDir: dir,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !called {
		t.Fatalf("expected called=true")
	}
	if !strings.HasPrefix(res.Model, "solidify-mcp://peer_a/") {
		t.Errorf("model prefix errado: %q", res.Model)
	}
	if res.ParsedContent["quality_score"] != 85.0 {
		t.Errorf("ParsedContent não veio do CanonicalPayload nativo: %v", res.ParsedContent)
	}
	if res.QualityScore != 85.0 {
		t.Errorf("quality_score esperado 85, got %v", res.QualityScore)
	}
	if res.ScoreStatus != ScoreStatusAvailable {
		t.Errorf("score_status esperado available, got %q", res.ScoreStatus)
	}
}

// TestExecuteTerminalMCP_V1_Fallback valida compat v1: record com
// schema="1" + Notes JSON-string → ainda funciona (fallback parse).
func TestExecuteTerminalMCP_V1_Fallback(t *testing.T) {
	runID := "r-v1-1"
	dir := seedStoreV1(t, runID, map[string]any{
		"run_id":        runID,
		"actor":         "peer_a",
		"verdict":       "approve",
		"quality_score": 78.0,
		"solid": map[string]any{
			"S": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"O": map[string]any{"after_score": map[string]any{"value": 75.0}},
			"L": map[string]any{"after_score": map[string]any{"value": 85.0}},
			"I": map[string]any{"after_score": map[string]any{"value": 75.0}},
			"D": map[string]any{"after_score": map[string]any{"value": 75.0}},
		},
	})
	res, called, err := executeTerminalMCP(context.Background(), PeerAOptions{
		Source:   SourceTerminalMCP,
		RunID:    runID,
		StoreDir: dir,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !called {
		t.Fatalf("v1 fallback devia retornar called=true")
	}
	if res.QualityScore != 78.0 {
		t.Errorf("v1 quality_score esperado 78, got %v", res.QualityScore)
	}
}

// TestExecuteTerminalMCP_NoSubmission valida skipped quando store vazio.
func TestExecuteTerminalMCP_NoSubmission(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()
	res, called, err := executeTerminalMCP(context.Background(), PeerAOptions{
		Source:   SourceTerminalMCP,
		RunID:    "r-no-sub",
		StoreDir: dir,
		Timeout:  1500 * time.Millisecond,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if called {
		t.Fatalf("expected called=false (skipped)")
	}
	if res != nil {
		t.Errorf("expected nil result, got %+v", res)
	}
	if elapsed < 1*time.Second {
		t.Errorf("voltou rápido demais (%v)", elapsed)
	}
}

// TestExecuteTerminalMCP_RunIDMismatch valida que record de outro
// run é ignorado (independência entre runs).
func TestExecuteTerminalMCP_RunIDMismatch(t *testing.T) {
	dir := seedStoreV2(t, "r-other", map[string]any{
		"run_id": "r-other", "actor": "peer_a", "verdict": "approve",
		"quality_score": 80.0,
		"solid": map[string]any{
			"S": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"O": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"L": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"I": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"D": map[string]any{"after_score": map[string]any{"value": 80.0}},
		},
	})
	_, called, err := executeTerminalMCP(context.Background(), PeerAOptions{
		Source:   SourceTerminalMCP,
		RunID:    "r-mine",
		StoreDir: dir,
		Timeout:  1500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if called {
		t.Errorf("não devia match — run_id diverge")
	}
}

// TestValidatorAcceptsV1AndV2 valida que validator aceita ambos schemas.
func TestValidatorAcceptsV1AndV2(t *testing.T) {
	v := mcpserver.DefaultPeerReviewValidator()
	cases := []struct {
		schema    string
		wantValid bool
	}{
		{"2", true},
		{"1", true}, // v1 compat aceito
		{"3", false},
		{"", false},
	}
	for _, c := range cases {
		in := &mcpserver.SubmitPeerReviewInput{
			RunID:        "r",
			Actor:        "peer_a",
			Schema:       c.schema,
			EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
			Verdict:      "comment",
		}
		errs := v.Validate(in)
		valid := len(errs) == 0
		if valid != c.wantValid {
			t.Errorf("schema=%q valid=%v want=%v (errs=%v)", c.schema, valid, c.wantValid, errs)
		}
	}
}

// TestResolveSource valida mapeamento canônico + fallback gracioso.
func TestResolveSource(t *testing.T) {
	cases := []struct {
		in   string
		want Source
	}{
		{"terminal-mcp", SourceTerminalMCP},
		{"http-combo", SourceHTTPCombo},
		{"auto", SourceAuto},
		{"unknown-source", SourceAuto},
		{"", SourceAuto},
	}
	for _, c := range cases {
		got := ResolveSource(c.in)
		if got != c.want {
			t.Errorf("ResolveSource(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
