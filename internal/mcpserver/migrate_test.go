// Tests para migrate v1 → v2 (SAI-122).
package mcpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

// seedV1Record insere record schema v1 no store.
func seedV1Record(t *testing.T, store *PeerReviewStore, runID, actor, notesJSON string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(notesJSON))
	in := &SubmitPeerReviewInput{
		RunID:        runID,
		Actor:        actor,
		Schema:       "1",
		EvidenceHash: hex.EncodeToString(sum[:]),
		Verdict:      "comment",
		Notes:        notesJSON,
	}
	rec := &PeerReviewRecord{
		ReviewID:     "rec-v1-" + runID,
		RunID:        runID,
		Actor:        actor,
		Schema:       "1",
		EvidenceHash: in.EvidenceHash,
		Verdict:      in.Verdict,
		Notes:        in.Notes,
		SubmittedAt:  time.Now(),
		ContentHash:  ComputeContentHash(in),
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("save v1: %v", err)
	}
	return rec.ReviewID
}

// TestMigrate_V1ToV2_Applied valida caminho feliz: record v1 vira v2.
func TestMigrate_V1ToV2_Applied(t *testing.T) {
	store, err := NewPeerReviewStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	payload := map[string]any{
		"run_id":        "r1",
		"actor":         "peer_a",
		"verdict":       "approve",
		"quality_score": 85.0,
	}
	notesJSON, _ := json.Marshal(payload)
	reviewID := seedV1Record(t, store, "r1", "peer_a", string(notesJSON))

	res, err := MigrateV1ToV2(store, MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.Scanned != 1 {
		t.Errorf("scanned esperado 1, got %d", res.Scanned)
	}
	if res.Migrated != 1 {
		t.Errorf("migrated esperado 1, got %d", res.Migrated)
	}
	if res.SkippedV2 != 0 {
		t.Errorf("skipped_v2 esperado 0, got %d", res.SkippedV2)
	}
	if len(res.Errors) != 0 {
		t.Errorf("errors esperado vazio, got %v", res.Errors)
	}

	// Verifica que record no disco é v2 com CanonicalPayload
	rec, err := store.Load(reviewID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.Schema != PeerReviewSchemaVersion {
		t.Errorf("schema esperado %q, got %q", PeerReviewSchemaVersion, rec.Schema)
	}
	if rec.Notes != "" {
		t.Errorf("notes devia ser vazio após migrate, got %q", rec.Notes)
	}
	if rec.CanonicalPayload == nil {
		t.Fatalf("CanonicalPayload vazio após migrate")
	}
	if rec.CanonicalPayload["quality_score"] != 85.0 {
		t.Errorf("CanonicalPayload não preservou dados: %v", rec.CanonicalPayload)
	}
	// ContentHash deve ter sido recomputado (v1 hash ≠ v2 hash)
	equiv := &SubmitPeerReviewInput{
		RunID:            rec.RunID,
		Actor:            rec.Actor,
		Schema:           rec.Schema,
		EvidenceHash:     rec.EvidenceHash,
		Verdict:          rec.Verdict,
		Findings:         rec.Findings,
		CanonicalPayload: rec.CanonicalPayload,
	}
	expectedHash := ComputeContentHash(equiv)
	if rec.ContentHash != expectedHash {
		t.Errorf("ContentHash não foi recomputado: have %q want %q", rec.ContentHash, expectedHash)
	}
}

// TestMigrate_DryRun valida que dry-run não persiste mudanças.
func TestMigrate_DryRun(t *testing.T) {
	store, err := NewPeerReviewStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	notesJSON := `{"run_id":"r","actor":"peer_a","verdict":"approve","quality_score":80}`
	reviewID := seedV1Record(t, store, "r", "peer_a", notesJSON)

	res, err := MigrateV1ToV2(store, MigrateOptions{DryRun: true})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.Migrated != 1 {
		t.Errorf("dry-run devia reportar migrated=1, got %d", res.Migrated)
	}

	// Record no disco deve continuar v1
	rec, err := store.Load(reviewID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.Schema != "1" {
		t.Errorf("dry-run NÃO devia ter mudado schema, got %q", rec.Schema)
	}
	if rec.Notes != notesJSON {
		t.Errorf("dry-run NÃO devia ter limpado notes, got %q", rec.Notes)
	}
	if rec.CanonicalPayload != nil {
		t.Errorf("dry-run NÃO devia ter setado CanonicalPayload, got %v", rec.CanonicalPayload)
	}
}

// TestMigrate_SkipsAlreadyV2 valida que records v2 não são tocados.
func TestMigrate_SkipsAlreadyV2(t *testing.T) {
	store, err := NewPeerReviewStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	// Record v2 (seed direto, sem Notes JSON-string)
	rec := &PeerReviewRecord{
		ReviewID:         "rec-v2-1",
		RunID:            "r2",
		Actor:            "peer_a",
		Schema:           "2",
		EvidenceHash:     "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:          "approve",
		CanonicalPayload: map[string]any{"quality_score": 90.0},
		SubmittedAt:      time.Now(),
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("save v2: %v", err)
	}

	res, err := MigrateV1ToV2(store, MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.SkippedV2 != 1 {
		t.Errorf("skipped_v2 esperado 1, got %d", res.SkippedV2)
	}
	if res.Migrated != 0 {
		t.Errorf("migrated esperado 0 (v2 não conta), got %d", res.Migrated)
	}
}

// TestMigrate_SkipsBadJSON valida que Notes não-JSON é pulado.
func TestMigrate_SkipsBadJSON(t *testing.T) {
	store, err := NewPeerReviewStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	seedV1Record(t, store, "r-bad", "peer_a", "not-valid-json{{{")

	res, err := MigrateV1ToV2(store, MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.SkippedBad != 1 {
		t.Errorf("skipped_bad esperado 1, got %d", res.SkippedBad)
	}
	if res.Migrated != 0 {
		t.Errorf("migrated esperado 0 (Notes inválido), got %d", res.Migrated)
	}
}

// TestMigrate_ActorFilter valida filtro por actor.
func TestMigrate_ActorFilter(t *testing.T) {
	store, err := NewPeerReviewStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	payloadA := `{"run_id":"ra","actor":"peer_a","verdict":"approve","quality_score":80}`
	payloadB := `{"run_id":"rb","actor":"peer_b","verdict":"approve","quality_score":75}`
	seedV1Record(t, store, "ra", "peer_a", payloadA)
	seedV1Record(t, store, "rb", "peer_b", payloadB)

	res, err := MigrateV1ToV2(store, MigrateOptions{Actor: "peer_a"})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.TotalInStore != 2 {
		t.Errorf("total_in_store esperado 2, got %d", res.TotalInStore)
	}
	if res.Scanned != 1 {
		t.Errorf("filtro peer_a: scanned esperado 1, got %d", res.Scanned)
	}
	if res.Migrated != 1 {
		t.Errorf("filtro peer_a: migrated esperado 1, got %d", res.Migrated)
	}

	// peer_b deve continuar v1 (não foi migrado)
	all, _ := store.List()
	for _, r := range all {
		if r.Actor == "peer_b" && r.Schema != "1" {
			t.Errorf("peer_b NÃO devia ter sido migrado: schema=%q", r.Schema)
		}
		if r.Actor == "peer_a" && r.Schema != "2" {
			t.Errorf("peer_a devia ter sido migrado pra v2, got %q", r.Schema)
		}
	}
}

// TestMigrate_EmptyNotes valida que v1 com Notes vazio é pulado (sem payload).
func TestMigrate_EmptyNotes(t *testing.T) {
	store, err := NewPeerReviewStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	seedV1Record(t, store, "r-empty", "peer_a", "")

	res, err := MigrateV1ToV2(store, MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.SkippedBad != 1 {
		t.Errorf("skipped_bad esperado 1 (notes vazio), got %d", res.SkippedBad)
	}
}
