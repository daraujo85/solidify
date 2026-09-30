package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diegoaraujo/solidify/internal/config"
)

func logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestJEVRegression_DisabledSkips(t *testing.T) {
	cfg := config.Default() // JEVRegression.Enabled=false
	got := runJEVRegressionAnalyzer(context.Background(), cfg, "diff", nil, logger())
	if got.ID != "jev_regression" {
		t.Fatalf("ID = %s", got.ID)
	}
	if len(got.Limitations) == 0 {
		t.Error("esperava limitation de disabled")
	}
}

func TestJEVRegression_NoKeySkips(t *testing.T) {
	cfg := config.Default()
	cfg.Analyzers.JEVRegression.Enabled = true
	cfg.Analyzers.JEVRegression.APIKeyEnv = "JEV_API_KEY"
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")

	got := runJEVRegressionAnalyzer(context.Background(), cfg, "diff", nil, logger())
	if len(got.Limitations) == 0 {
		t.Error("esperava limitation de no_key")
	}
}

func TestJEVRegression_EmptyDiffSkips(t *testing.T) {
	cfg := config.Default()
	cfg.Analyzers.JEVRegression.Enabled = true
	cfg.Analyzers.JEVRegression.APIKeyEnv = "JEV_API_KEY"
	t.Setenv("JEV_API_KEY", "k")

	got := runJEVRegressionAnalyzer(context.Background(), cfg, "", nil, logger())
	if len(got.Limitations) == 0 {
		t.Error("esperava limitation de empty_diff")
	}
}

func TestJEVRegression_DetectsFindings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev","answers":{
			"breaks_public_contract":{"type":"noul","noul":true,"probability":0.95},
			"weakens_existing_logic":{"type":"noul","noul":false,"probability":0.1},
			"weakens_tests":{"type":"noul","noul":false,"probability":0.05},
			"shared_state_side_effect":{"type":"noul","noul":false,"probability":0.1}
		}}`))
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.Analyzers.JEVRegression.Enabled = true
	cfg.Analyzers.JEVRegression.APIKeyEnv = "JEV_API_KEY"
	cfg.Analyzers.JEVRegression.Endpoint = srv.URL
	t.Setenv("JEV_API_KEY", "k")

	got := runJEVRegressionAnalyzer(context.Background(), cfg, "diff that breaks contract", []string{"a.go"}, logger())
	if len(got.Findings) != 1 {
		t.Fatalf("findings = %d, quero 1", len(got.Findings))
	}
	if got.Findings[0]["rule"] != "breaks_public_contract" {
		t.Errorf("rule = %v", got.Findings[0]["rule"])
	}
	if got.Metrics["signals_detected"] != 1 {
		t.Errorf("signals_detected = %v", got.Metrics["signals_detected"])
	}
}

func TestJEVRegression_ServerErrorSkips(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.Analyzers.JEVRegression.Enabled = true
	cfg.Analyzers.JEVRegression.APIKeyEnv = "JEV_API_KEY"
	cfg.Analyzers.JEVRegression.Endpoint = srv.URL
	t.Setenv("JEV_API_KEY", "k")

	got := runJEVRegressionAnalyzer(context.Background(), cfg, "diff", nil, logger())
	if len(got.Limitations) == 0 {
		t.Error("esperava limitation de jev_error")
	}
	if len(got.Findings) != 0 {
		t.Errorf("findings = %d, quero 0 (fail-open)", len(got.Findings))
	}
}
