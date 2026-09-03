package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/diegoaraujo/solidify/internal/doctor"
	"github.com/diegoaraujo/solidify/internal/mcpserver"
	"github.com/diegoaraujo/solidify/internal/report"
)

func ptr(v float64) *float64 { return &v }

func writeReport(t *testing.T, dir, id string) {
	t.Helper()
	r := &report.Report{
		SchemaVersion: report.SchemaVersion,
		Run: report.RunInfo{
			ID: id, Profile: "release",
			SolidifyVersion: "1",
			StartedAt:       time.Now().UTC().Format(time.RFC3339),
			FinishedAt:      time.Now().UTC().Format(time.RFC3339),
			ConfigHash:      strings.Repeat("a", 64),
			EvidenceHash:    strings.Repeat("b", 64),
		},
		Scores:       report.ScoresBlock{Quality: ptr(85), Grade: "A", Confidence: 0.9, ConfidenceLevel: "HIGH"},
		Risk:         report.RiskBlock{Level: "LOW"},
		QualityGate:  report.QualityGate{Status: "PASS"},
		SOLID:        report.SOLIDBlock{Principles: map[string]report.Principle{}},
		AIReview:     report.AIReview{Mode: "peer"},
		Git:          report.GitInfo{BaseRef: "main", BaseSHA: "x", HeadRef: "f", HeadSHA: "y"},
		Components:   []report.Component{},
		ReleaseNotes: report.ReleaseNotes{ExecutiveSummary: "x"},
		Migrations:   []report.Migration{},
		EnvChanges:   []report.EnvChange{},
		Analyzers:    []report.Analyzer{},
	}
	b, _ := json.Marshal(r)
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadIndex(t *testing.T) {
	dir := t.TempDir()
	writeReport(t, dir, "r1")
	writeReport(t, dir, "r2")
	idx, err := loadIndex(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(idx.runs) != 2 {
		t.Errorf("runs: %d", len(idx.runs))
	}
}

func TestLoadIndexMissing(t *testing.T) {
	idx, err := loadIndex("/nonexistent/path/xyz")
	if err != nil {
		t.Errorf("esperado ok p/ dir ausente: %v", err)
	}
	if len(idx.runs) != 0 {
		t.Errorf("vazio")
	}
}

func TestSummarize(t *testing.T) {
	r := &report.Report{
		Run:         report.RunInfo{ID: "r1", Profile: "release", FinishedAt: "x"},
		Scores:      report.ScoresBlock{Quality: ptr(85), Grade: "A"},
		Risk:        report.RiskBlock{Level: "LOW"},
		QualityGate: report.QualityGate{Status: "PASS"},
	}
	s := summarize(r)
	if s.RunID != "r1" || s.Quality == nil || *s.Quality != 85 {
		t.Errorf("summary")
	}
}

func TestIndexList(t *testing.T) {
	dir := t.TempDir()
	writeReport(t, dir, "z")
	writeReport(t, dir, "a")
	idx, _ := loadIndex(dir)
	list := idx.list()
	if list[0].RunID != "a" || list[1].RunID != "z" {
		t.Errorf("ordem: %v", list)
	}
}

func TestIndexGet(t *testing.T) {
	dir := t.TempDir()
	writeReport(t, dir, "r1")
	idx, _ := loadIndex(dir)
	r, err := idx.get("r1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if r.Run.ID != "r1" {
		t.Errorf("get")
	}
	if _, err := idx.get("nope"); err == nil {
		t.Errorf("missing")
	}
}

func TestHandlerRuns(t *testing.T) {
	dir := t.TempDir()
	writeReport(t, dir, "r1")
	idx, _ := loadIndex(dir)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(idx.list())
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/runs", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("code: %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("r1")) {
		t.Errorf("body")
	}
}

func TestHandlerRun(t *testing.T) {
	dir := t.TempDir()
	writeReport(t, dir, "r1")
	idx, _ := loadIndex(dir)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/run/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/run/")
		if id == "" {
			http.Error(w, "vazio", 400)
			return
		}
		rep, err := idx.get(id)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rep)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/run/r1", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("code: %d", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/api/run/nope", nil)
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != 404 {
		t.Errorf("404")
	}
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/api/run/", nil)
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != 400 {
		t.Errorf("400")
	}
}

func TestDashboardFlags(t *testing.T) {
	dir := t.TempDir()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	done := make(chan int, 1)
	go func() {
		done <- Run([]string{
			"-port", "abc-not-a-port",
			"-store", dir,
			"-static", "/tmp",
		}, out, errOut)
	}()
	select {
	case code := <-done:
		if code != 1 {
			t.Errorf("esperado 1: %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Errorf("timeout dashboard")
	}
}

func TestServeDashboardEmptyPort(t *testing.T) {
	err := serve(Config{Port: ""}, &bytes.Buffer{})
	if err == nil {
		t.Errorf("port vazia")
	}
}

// SAI-125: trend endpoint retorna série válida mesmo sem dados.
func TestHandlerPeerReviewsTrend(t *testing.T) {
	withTempMetricsPath(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/peer-reviews/trend", func(w http.ResponseWriter, r *http.Request) {
		events, err := mcpserver.ReadMetrics()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		trend := mcpserver.ComputeTrend(events, 30, time.Time{}, doctor.CanaryFractionThreshold)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(trend)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/peer-reviews/trend", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code: %d body=%s", rec.Code, rec.Body.String())
	}
	var trend mcpserver.PeerReviewTrend
	if err := json.Unmarshal(rec.Body.Bytes(), &trend); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	if trend.WindowDays != 30 {
		t.Errorf("window: %d", trend.WindowDays)
	}
	if len(trend.Points) != 30 {
		t.Errorf("points len: %d", len(trend.Points))
	}
}

// SAI-125: trend endpoint com eventos reais no JSONL.
func TestHandlerPeerReviewsTrend_WithEvents(t *testing.T) {
	path := withTempMetricsPath(t)
	now := time.Now().UTC()
	for i := 0; i < 12; i++ {
		ev := mcpserver.Metric{
			Timestamp: now.Add(-time.Duration(i) * time.Hour),
			Schema:    "2",
			Actor:     "peer_a",
			Verdict:   "approve",
		}
		if err := mcpserver.AppendMetric(ev); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	// path usado por referência pra silenciar unused
	_ = path

	mux := http.NewServeMux()
	mux.HandleFunc("/api/peer-reviews/trend", func(w http.ResponseWriter, r *http.Request) {
		events, _ := mcpserver.ReadMetrics()
		trend := mcpserver.ComputeTrend(events, 30, time.Time{}, doctor.CanaryFractionThreshold)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(trend)
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/peer-reviews/trend", nil))
	if rec.Code != 200 {
		t.Fatalf("code: %d", rec.Code)
	}
	var trend mcpserver.PeerReviewTrend
	if err := json.Unmarshal(rec.Body.Bytes(), &trend); err != nil {
		t.Fatalf("json: %v", err)
	}
	// últimos buckets devem ter Total>0 (eventos das últimas horas)
	last := trend.Points[len(trend.Points)-1]
	if last.Total == 0 {
		t.Errorf("último ponto devia ter eventos: %+v", last)
	}
}

// withTempMetricsPath redireciona mcpserver.MetricsPath pra tmp.
func withTempMetricsPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/metrics.jsonl"
	orig := mcpserver.MetricsPath
	mcpserver.MetricsPath = path
	t.Cleanup(func() { mcpserver.MetricsPath = orig })
	return path
}

func TestServeEphemeral(t *testing.T) {
	dir := t.TempDir()
	writeReport(t, dir, "r1")
	cfg := Config{
		Port:     "8080",
		StoreDir: dir,
	}
	addr, shutdown, err := ServeEphemeral(cfg)
	if err != nil {
		t.Fatalf("ServeEphemeral: %v", err)
	}

	resp, err := http.Get("http://" + addr + "/api/runs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestServeEmbeddedAssets(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Port:      "8080",
		StoreDir:  dir,
		StaticDir: "", // Triggers embedded FS
	}
	mux, _, err := buildMux(cfg)
	if err != nil {
		t.Fatalf("buildMux: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("status: %d", rec.Code)
	}
}
