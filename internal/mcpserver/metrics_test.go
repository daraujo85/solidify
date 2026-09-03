// Tests para telemetry + v1 cutoff (SAI-121).
package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withTempMetrics(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.jsonl")
	MetricsPath = path
	t.Cleanup(func() { MetricsPath = "" })
	return path
}

// TestAppendMetric_AndRead valida o round-trip JSONL.
func TestAppendMetric_AndRead(t *testing.T) {
	withTempMetrics(t)
	now := time.Now()
	for i := 0; i < 3; i++ {
		if err := AppendMetric(Metric{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Schema:    "2",
			Actor:     "peer_a",
			ReviewID:  "rec-1",
			Verdict:   "comment",
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	events, err := ReadMetrics()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("esperado 3 events, got %d", len(events))
	}
	if events[0].Schema != "2" {
		t.Errorf("schema errado: %q", events[0].Schema)
	}
}

// TestReadMetrics_MissingFile valida que arquivo ausente = lista vazia (não erro).
func TestReadMetrics_MissingFile(t *testing.T) {
	withTempMetrics(t)
	events, err := ReadMetrics()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("esperado vazio, got %d", len(events))
	}
}

// TestReadMetrics_SkipsCorrupt valida best-effort: linha corrompida pula.
func TestReadMetrics_SkipsCorrupt(t *testing.T) {
	path := withTempMetrics(t)
	// linha boa + linha corrompida + linha boa
	raw := `{"ts":"2026-08-21T10:00:00Z","schema":"2","actor":"peer_a","review_id":"a","verdict":"approve"}
not-json-line-here
{"ts":"2026-08-21T10:00:01Z","schema":"1","actor":"peer_a","review_id":"b","verdict":"comment"}
`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	events, err := ReadMetrics()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("esperado 2 (skip corrupt), got %d", len(events))
	}
}

// TestSummarizeMetrics valida agregação por schema/actor/verdict.
func TestSummarizeMetrics(t *testing.T) {
	events := []Metric{
		{Timestamp: time.Now(), Schema: "1", Actor: "peer_a", Verdict: "approve"},
		{Timestamp: time.Now().Add(time.Second), Schema: "1", Actor: "peer_a", Verdict: "comment"},
		{Timestamp: time.Now().Add(2 * time.Second), Schema: "2", Actor: "peer_a", Verdict: "approve"},
		{Timestamp: time.Now().Add(3 * time.Second), Schema: "2", Actor: "peer_b", Verdict: "approve"},
	}
	s := SummarizeMetrics(events, time.Time{}, time.Time{})
	if s.Total != 4 {
		t.Errorf("total esperado 4, got %d", s.Total)
	}
	if s.V1Count != 2 {
		t.Errorf("v1_count esperado 2, got %d", s.V1Count)
	}
	if s.V2Count != 2 {
		t.Errorf("v2_count esperado 2, got %d", s.V2Count)
	}
	if s.V1Fraction != 0.5 {
		t.Errorf("v1_fraction esperado 0.5, got %v", s.V1Fraction)
	}
	if s.BySchema["1"] != 2 || s.BySchema["2"] != 2 {
		t.Errorf("by_schema errado: %v", s.BySchema)
	}
	if s.ByActor["peer_a"] != 3 || s.ByActor["peer_b"] != 1 {
		t.Errorf("by_actor errado: %v", s.ByActor)
	}
}

// TestSummarizeMetrics_SinceFilter valida filtro temporal.
func TestSummarizeMetrics_SinceFilter(t *testing.T) {
	base := time.Now()
	events := []Metric{
		{Timestamp: base, Schema: "1"},
		{Timestamp: base.Add(time.Hour), Schema: "2"},
		{Timestamp: base.Add(2 * time.Hour), Schema: "2"},
	}
	s := SummarizeMetrics(events, base.Add(30*time.Minute), time.Time{})
	if s.Total != 2 {
		t.Errorf("filtro since: esperado 2, got %d", s.Total)
	}
	if s.V1Count != 0 {
		t.Errorf("filtro since: v1 deveria ser 0 (evento anterior), got %d", s.V1Count)
	}
}

// TestV1Cutoff_ZeroMeansNoCutoff valida default sem cutoff.
func TestV1Cutoff_ZeroMeansNoCutoff(t *testing.T) {
	V1Cutoff = time.Time{}
	if v1AfterCutoff("1") {
		t.Errorf("cutoff zero deveria aceitar v1 (warning-only)")
	}
	if !v1AfterCutoff("2") == false {
		t.Errorf("v2 não passa por cutoff")
	}
	if v1AfterCutoff("2") {
		t.Errorf("v2 não devia cair em cutoff")
	}
}

// TestV1Cutoff_AfterDate_Rejects valida hard fail pós-cutoff.
func TestV1Cutoff_AfterDate_Rejects(t *testing.T) {
	V1Cutoff = time.Now().Add(-1 * time.Hour)
	defer func() { V1Cutoff = time.Time{} }()
	if !v1AfterCutoff("1") {
		t.Errorf("cutoff vencido devia rejeitar v1")
	}
	if v1AfterCutoff("2") {
		t.Errorf("v2 não afetado por cutoff")
	}
}

// TestSetV1Cutoff_Parse valida parsing RFC3339 + reset vazio.
func TestSetV1Cutoff_Parse(t *testing.T) {
	defer func() { V1Cutoff = time.Time{} }()
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	if err := SetV1Cutoff(future); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if V1Cutoff.IsZero() {
		t.Errorf("cutoff devia ter sido setado")
	}
	if v1AfterCutoff("1") {
		t.Errorf("cutoff futuro devia aceitar v1")
	}
	if err := SetV1Cutoff(""); err != nil {
		t.Errorf("reset: %v", err)
	}
	if !V1Cutoff.IsZero() {
		t.Errorf("reset devia zerar cutoff")
	}
	if err := SetV1Cutoff("not-a-date"); err == nil {
		t.Errorf("string inválida devia dar erro")
	}
}

// TestMetricJSONShape valida shape do JSONL pra grep/awk friendly.
func TestMetricJSONShape(t *testing.T) {
	m := Metric{
		Timestamp: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
		Schema:    "2",
		Actor:     "peer_a",
		ReviewID:  "rec123",
		Verdict:   "comment",
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// campos chave devem estar presentes (grep-friendly)
	want := []string{`"ts":"2026-08-21T10:00:00Z"`, `"schema":"2"`, `"actor":"peer_a"`, `"review_id":"rec123"`}
	s := string(b)
	for _, w := range want {
		if !contains(s, w) {
			t.Errorf("JSONL faltando %q em %s", w, s)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
