// Tests para doctor canary checks (SAI-123).
package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/diegoaraujo/solidify/internal/mcpserver"
)

// withCanaryFixture redireciona MetricsPath + SOLIDIFY_PEER_REVIEW_STORE_DIR
// pra paths em tmp dir, semeia eventos e/ou records, e restaura no fim.
type canaryFixture struct {
	t            *testing.T
	metricsPath  string
	storeDir     string
	originalM    string
	originalSD   string
}

func newCanaryFixture(t *testing.T) *canaryFixture {
	t.Helper()
	dir := t.TempDir()
	mp := filepath.Join(dir, "metrics.jsonl")
	sd := filepath.Join(dir, "peer_reviews")
	os.MkdirAll(sd, 0755)
	origM := mcpserver.MetricsPath
	origSD := os.Getenv("SOLIDIFY_PEER_REVIEW_STORE_DIR")
	mcpserver.MetricsPath = mp
	os.Setenv("SOLIDIFY_PEER_REVIEW_STORE_DIR", sd)
	t.Cleanup(func() {
		mcpserver.MetricsPath = origM
		if origSD != "" {
			os.Setenv("SOLIDIFY_PEER_REVIEW_STORE_DIR", origSD)
		} else {
			os.Unsetenv("SOLIDIFY_PEER_REVIEW_STORE_DIR")
		}
	})
	return &canaryFixture{t: t, metricsPath: mp, storeDir: sd, originalM: origM, originalSD: origSD}
}

func (f *canaryFixture) writeMetrics(events []mcpserver.Metric) {
	f.t.Helper()
	var lines [][]byte
	for _, e := range events {
		b, _ := json.Marshal(e)
		lines = append(lines, b)
	}
	data := joinLines(lines)
	if err := os.WriteFile(f.metricsPath, data, 0644); err != nil {
		f.t.Fatalf("write metrics: %v", err)
	}
}

func (f *canaryFixture) writeV1Record(runID string) {
	f.t.Helper()
	store, err := mcpserver.NewPeerReviewStore(f.storeDir)
	if err != nil {
		f.t.Fatalf("store: %v", err)
	}
	notes := `{"run_id":"` + runID + `","actor":"peer_a","verdict":"approve","quality_score":80}`
	rec := &mcpserver.PeerReviewRecord{
		ReviewID:     "rec-" + runID,
		RunID:        runID,
		Actor:        "peer_a",
		Schema:       "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "approve",
		Notes:        notes,
		SubmittedAt:  time.Now(),
	}
	if err := store.Save(rec); err != nil {
		f.t.Fatalf("save: %v", err)
	}
}

func joinLines(lines [][]byte) []byte {
	out := []byte{}
	for i, l := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, l...)
	}
	return out
}

// TestCanary_NoMetrics valida OK quando não há dados.
func TestCanary_NoMetrics(t *testing.T) {
	newCanaryFixture(t)
	r := checkPeerReviewCanary()
	if !r.OK {
		t.Errorf("esperado OK sem dados: %s", r.Message)
	}
	if !strings.Contains(r.Message, "sem submissões") {
		t.Errorf("mensagem devia indicar 'sem submissões', got %q", r.Message)
	}
}

// TestCanary_TooFewEvents valida OK com "dados insuficientes".
func TestCanary_TooFewEvents(t *testing.T) {
	f := newCanaryFixture(t)
	now := time.Now()
	f.writeMetrics([]mcpserver.Metric{
		{Timestamp: now, Schema: "2", Actor: "peer_a", Verdict: "approve"},
		{Timestamp: now, Schema: "1", Actor: "peer_a", Verdict: "approve"},
	})
	r := checkPeerReviewCanary()
	if !r.OK {
		t.Errorf("poucos events devia ser OK: %s", r.Message)
	}
	if !strings.Contains(r.Message, "dados insuficientes") {
		t.Errorf("mensagem devia indicar 'dados insuficientes', got %q", r.Message)
	}
}

// TestCanary_LowV1Fraction valida OK com sugestão de cutoff.
func TestCanary_LowV1Fraction(t *testing.T) {
	f := newCanaryFixture(t)
	now := time.Now()
	events := []mcpserver.Metric{}
	// 50 v2 + 1 v1 = v1_fraction = 1/51 ≈ 2% (< 5%)
	for i := 0; i < 50; i++ {
		events = append(events, mcpserver.Metric{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			Schema:    "2",
			Actor:     "peer_a",
			Verdict:   "approve",
		})
	}
	events = append(events, mcpserver.Metric{
		Timestamp: now, Schema: "1", Actor: "peer_a", Verdict: "comment",
	})
	f.writeMetrics(events)
	r := checkPeerReviewCanary()
	if !r.OK {
		t.Errorf("v1_fraction baixa devia ser OK: %s", r.Message)
	}
	if !strings.Contains(r.Message, "considere setar") {
		t.Errorf("mensagem devia sugerir cutoff, got %q", r.Message)
	}
	if !strings.Contains(r.Message, "v1_cutoff=") {
		t.Errorf("mensagem devia incluir v1_cutoff=, got %q", r.Message)
	}
}

// TestCanary_HighV1Fraction valida FAIL quando v1 ainda em uso.
func TestCanary_HighV1Fraction(t *testing.T) {
	f := newCanaryFixture(t)
	now := time.Now()
	events := []mcpserver.Metric{}
	// 5 v2 + 10 v1 = v1_fraction = 10/15 ≈ 67%
	for i := 0; i < 5; i++ {
		events = append(events, mcpserver.Metric{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			Schema:    "2",
			Actor:     "peer_a",
		})
	}
	for i := 0; i < 10; i++ {
		events = append(events, mcpserver.Metric{
			Timestamp: now.Add(-time.Duration(i+10) * time.Minute),
			Schema:    "1",
			Actor:     "peer_a",
		})
	}
	f.writeMetrics(events)
	r := checkPeerReviewCanary()
	if r.OK {
		t.Errorf("v1_fraction alta devia ser FAIL: %s", r.Message)
	}
	if !strings.Contains(r.Message, "rollout v2") {
		t.Errorf("mensagem devia indicar rollout pendente, got %q", r.Message)
	}
}

// TestCanary_OldEventsIgnored valida que eventos fora da janela não contam.
func TestCanary_OldEventsIgnored(t *testing.T) {
	f := newCanaryFixture(t)
	old := time.Now().Add(-60 * 24 * time.Hour) // 60 dias atrás
	recent := time.Now().Add(-1 * time.Hour)
	// 100 v1 antigas (fora da janela) + 20 v2 recentes
	events := []mcpserver.Metric{}
	for i := 0; i < 100; i++ {
		events = append(events, mcpserver.Metric{Timestamp: old.Add(time.Duration(i) * time.Minute), Schema: "1"})
	}
	for i := 0; i < 20; i++ {
		events = append(events, mcpserver.Metric{Timestamp: recent.Add(time.Duration(i) * time.Minute), Schema: "2"})
	}
	f.writeMetrics(events)
	r := checkPeerReviewCanary()
	if !r.OK {
		t.Errorf("eventos antigos ignorados, devia ser OK: %s", r.Message)
	}
	if !strings.Contains(r.Message, "considere setar") {
		t.Errorf("mensagem devia sugerir cutoff (v1_fraction≈0 na janela), got %q", r.Message)
	}
}

// TestStoreV1_Empty valida OK quando store vazio.
func TestStoreV1_Empty(t *testing.T) {
	newCanaryFixture(t)
	r := checkPeerReviewStoreV1()
	if !r.OK {
		t.Errorf("store vazio devia ser OK: %s", r.Message)
	}
}

// TestStoreV1_HasV1Records valida FAIL + sugestão migrate.
func TestStoreV1_HasV1Records(t *testing.T) {
	f := newCanaryFixture(t)
	f.writeV1Record("r1")
	f.writeV1Record("r2")
	r := checkPeerReviewStoreV1()
	if r.OK {
		t.Errorf("2 records v1 devia ser FAIL: %s", r.Message)
	}
	if !strings.Contains(r.Message, "peer-reviews migrate") {
		t.Errorf("mensagem devia sugerir migrate, got %q", r.Message)
	}
	if !strings.Contains(r.Message, "2 records") {
		t.Errorf("mensagem devia indicar count=2, got %q", r.Message)
	}
}

// TestStoreV1_OnlyV2 valida OK quando store tem só v2.
func TestStoreV1_OnlyV2(t *testing.T) {
	f := newCanaryFixture(t)
	store, _ := mcpserver.NewPeerReviewStore(f.storeDir)
	rec := &mcpserver.PeerReviewRecord{
		ReviewID:         "rec-v2",
		RunID:            "r-v2",
		Actor:            "peer_a",
		Schema:           "2",
		EvidenceHash:     "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:          "approve",
		CanonicalPayload: map[string]any{"quality_score": 80.0},
		SubmittedAt:      time.Now(),
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	r := checkPeerReviewStoreV1()
	if !r.OK {
		t.Errorf("só v2 devia ser OK: %s", r.Message)
	}
	if !strings.Contains(r.Message, "clean") {
		t.Errorf("mensagem devia indicar 'clean', got %q", r.Message)
	}
}
