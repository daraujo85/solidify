// Package k6 — k6 load test adapter.
//
// SAI-047: priorizar script existente; fallback smoke script
// auto-gerado a partir de endpoints detectados (SAI-046). Sem
// script configurado E sem endpoints = skip.
package k6

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Mode execução.
type Mode string

const (
	ModeContainer Mode = "container" // docker run grafana/k6
	ModeBinary    Mode = "binary"    // k6 binário local
	ModeDisabled  Mode = "disabled"  // skip
)

// ScriptSource origem do script k6.
type ScriptSource string

const (
	SourceExisting ScriptSource = "existing" // user-provided path
	SourceAutoGen  ScriptSource = "auto"     // gerado runtime
)

// Config configura adapter.
type Config struct {
	Mode         Mode          `json:"mode,omitempty"`
	ScriptPath   string        `json:"script_path,omitempty"`    // user-provided
	ThresholdP95 time.Duration `json:"threshold_p95,omitempty"`  // default 500ms
	ThresholdP99 time.Duration `json:"threshold_p99,omitempty"`  // default 1s
	MaxErrorRate float64       `json:"max_error_rate,omitempty"` // default 0.01 (1%)
	VUs          int           `json:"vus,omitempty"`            // default 10
	Duration     time.Duration `json:"duration,omitempty"`       // default 30s
	AllowProd    bool          `json:"allow_prod,omitempty"`     // override
}

// DefaultConfig devolve defaults seguros.
func DefaultConfig() Config {
	return Config{
		Mode:         ModeBinary,
		ThresholdP95: 500 * time.Millisecond,
		ThresholdP99: 1 * time.Second,
		MaxErrorRate: 0.01,
		VUs:          10,
		Duration:     30 * time.Second,
	}
}

// ShouldRun devolve true se adapter tem o que rodar.
func (c Config) ShouldRun() bool {
	if c.Mode == ModeDisabled {
		return false
	}
	if c.ScriptPath != "" {
		if _, err := os.Stat(c.ScriptPath); err == nil {
			return true
		}
	}
	return false // sem script = skip (fallback gerado em outro path)
}

// EndpointsPrioritized lista de endpoints com peso.
type EndpointsPrioritized struct {
	Endpoint string  `json:"endpoint"`         // method+path ou só path
	Weight   float64 `json:"weight,omitempty"` // 0-1; default 1.0
}

// RenderSmokeScript gera k6 script mínimo para endpoints fornecidos.
// Útil quando script user não existe — gera smoke test de baixo risco.
func RenderSmokeScript(target string, eps []EndpointsPrioritized, cfg Config) ([]byte, error) {
	if target == "" {
		return nil, errors.New("k6: target vazio")
	}
	if len(eps) == 0 {
		return nil, errors.New("k6: nenhum endpoint")
	}
	dur := cfg.Duration
	if dur <= 0 {
		dur = 30 * time.Second
	}
	vus := cfg.VUs
	if vus <= 0 {
		vus = 10
	}
	var sb strings.Builder
	sb.WriteString("// Auto-generated smoke script — Solidify\n")
	sb.WriteString("import http from 'k6/http';\n")
	sb.WriteString("import { check, sleep } from 'k6';\n\n")
	fmt.Fprintf(&sb, "export const options = {\n  vus: %d,\n  duration: '%s',\n  thresholds: {\n    http_req_duration: ['p(95)<%d', 'p(99)<%d'],\n    http_req_failed: ['rate<%f'],\n  },\n};\n\n", vus, dur, cfg.ThresholdP95.Milliseconds(), cfg.ThresholdP99.Milliseconds(), cfg.MaxErrorRate)
	fmt.Fprintf(&sb, "const BASE = '%s';\n\n", target)
	sb.WriteString("export default function () {\n")
	for _, ep := range eps {
		w := ep.Weight
		if w <= 0 {
			w = 1.0
		}
		fmt.Fprintf(&sb, "  // weight=%.2f\n", w)
		fmt.Fprintf(&sb, "  http.get(BASE + '%s');\n", ep.Endpoint)
	}
	sb.WriteString("  sleep(1);\n}\n")
	return []byte(sb.String()), nil
}

// LoadScript carrega script user-provided do path.
// Retorna erro se não existe.
func LoadScript(path string) ([]byte, ScriptSource, error) {
	if path == "" {
		return nil, "", errors.New("k6: script path vazio")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("k6: read script: %w", err)
	}
	return data, SourceExisting, nil
}

// ResolveScript resolve script: user-provided OR auto-gerado.
// Sem endpoints E sem script = skip (nil, nil).
func ResolveScript(target, scriptPath string, eps []EndpointsPrioritized, cfg Config) ([]byte, ScriptSource, error) {
	if scriptPath != "" {
		data, src, err := LoadScript(scriptPath)
		if err == nil {
			return data, src, nil
		}
		// Fallthrough pro auto-gen.
	}
	if len(eps) == 0 {
		return nil, "", nil
	}
	data, err := RenderSmokeScript(target, eps, cfg)
	if err != nil {
		return nil, "", err
	}
	return data, SourceAutoGen, nil
}

// RunConfig configura execução.
type RunConfig struct {
	Mode   Mode
	Bin    string // path do k6 binário
	Target string
	Script []byte
	Out    string // path do summary JSON
}

// k6Summary subset do JSON output do k6 --summary-export.
type k6Summary struct {
	Metrics   map[string]k6Metric `json:"metrics"`
	RootGroup struct {
		Checks []k6Check `json:"checks,omitempty"`
	} `json:"root_group,omitempty"`
}

type k6Metric struct {
	Values    map[string]float64 `json:"values"`
	Threshold *struct {
		Sources []struct {
			Name string `json:"name,omitempty"`
			OK   bool   `json:"ok"`
		} `json:"sources,omitempty"`
	} `json:"threshold,omitempty"`
}

type k6Check struct {
	Name   string `json:"name"`
	Passes int    `json:"passes"`
	Fails  int    `json:"fails"`
}

// ParseK6SummaryJSON parseia k6 summary-export JSON.
func ParseK6SummaryJSON(data []byte) (*Summary, error) {
	var raw k6Summary
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("k6: parse: %w", err)
	}
	s := &Summary{}
	if m, ok := raw.Metrics["http_req_duration"]; ok {
		s.P50 = msToDuration(m.Values["p(50)"])
		s.P90 = msToDuration(m.Values["p(90)"])
		s.P95 = msToDuration(m.Values["p(95)"])
		s.P99 = msToDuration(m.Values["p(99)"])
	}
	if m, ok := raw.Metrics["http_req_failed"]; ok {
		s.ErrorRate = m.Values["rate"]
	}
	if m, ok := raw.Metrics["http_reqs"]; ok {
		s.Requests = int(m.Values["count"])
		s.Throughput = m.Values["rate"]
	}
	// Threshold pass/fail.
	for _, m := range raw.Metrics {
		if m.Threshold != nil {
			for _, src := range m.Threshold.Sources {
				if !src.OK {
					s.ThresholdFailed = append(s.ThresholdFailed, src.Name)
				}
			}
		}
	}
	return s, nil
}

// Summary resultado normalizado.
type Summary struct {
	P50             time.Duration `json:"p50_ns"`
	P90             time.Duration `json:"p90_ns"`
	P95             time.Duration `json:"p95_ns"`
	P99             time.Duration `json:"p99_ns"`
	ErrorRate       float64       `json:"error_rate"`
	Requests        int           `json:"requests"`
	Throughput      float64       `json:"throughput_rps"`
	ThresholdFailed []string      `json:"threshold_failed,omitempty"`
}

// PassThresholds devolve true se summary passa nos thresholds.
func (s *Summary) PassThresholds(cfg Config) bool {
	if s.P95 > cfg.ThresholdP95 {
		return false
	}
	if s.P99 > cfg.ThresholdP99 {
		return false
	}
	if s.ErrorRate > cfg.MaxErrorRate {
		return false
	}
	if len(s.ThresholdFailed) > 0 {
		return false
	}
	return true
}

// msToDuration helper — k6 reporta em ms (float).
func msToDuration(ms float64) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms * float64(time.Millisecond))
}

// RunK6 placeholder — execução real requer k6 binário. Esta função
// valida config e retorna command string que caller executa.
func RunK6(ctx context.Context, rc RunConfig) (string, error) {
	if rc.Script == nil {
		return "", errors.New("k6: script nil")
	}
	if rc.Target == "" {
		return "", errors.New("k6: target vazio")
	}
	switch rc.Mode {
	case ModeBinary:
		bin := rc.Bin
		if bin == "" {
			bin = "k6"
		}
		out := rc.Out
		if out == "" {
			out = "k6-summary.json"
		}
		return fmt.Sprintf("%s run --summary-export=%s", bin, out), nil
	case ModeContainer:
		out := rc.Out
		if out == "" {
			out = "/tmp/k6-summary.json"
		}
		return fmt.Sprintf("docker run --rm -i grafana/k6 run --summary-export=%s", out), nil
	}
	return "", fmt.Errorf("k6: mode inválido: %s", rc.Mode)
}

// PrioritizeEndpoints atribui peso — defaults 1.0, change endpoints
// (DiffSet.Added) ganham peso maior.
func PrioritizeEndpoints(all []string, added []string, baseWeight, addedWeight float64) []EndpointsPrioritized {
	if baseWeight <= 0 {
		baseWeight = 1.0
	}
	if addedWeight <= 0 {
		addedWeight = 2.0
	}
	addedMap := make(map[string]bool)
	for _, a := range added {
		addedMap[a] = true
	}
	out := make([]EndpointsPrioritized, 0, len(all))
	for _, ep := range all {
		w := baseWeight
		if addedMap[ep] {
			w = addedWeight
		}
		out = append(out, EndpointsPrioritized{Endpoint: ep, Weight: w})
	}
	return out
}
