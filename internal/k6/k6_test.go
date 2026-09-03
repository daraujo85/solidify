package k6

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Aceitação: DefaultConfig.
func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.ThresholdP95 <= 0 || c.ThresholdP99 <= 0 {
		t.Errorf("thresholds zero")
	}
	if c.MaxErrorRate <= 0 {
		t.Errorf("max err")
	}
	if c.VUs <= 0 || c.Duration <= 0 {
		t.Errorf("vu/duration")
	}
}

// Aceitação: ShouldRun.
func TestShouldRun(t *testing.T) {
	if (&Config{Mode: ModeDisabled}).ShouldRun() {
		t.Errorf("disabled não roda")
	}
	if (&Config{Mode: ModeBinary}).ShouldRun() {
		t.Errorf("sem script não roda")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "test.js")
	os.WriteFile(script, []byte("// test"), 0644)
	if !(&Config{Mode: ModeBinary, ScriptPath: script}).ShouldRun() {
		t.Errorf("com script devia rodar")
	}
	if (&Config{Mode: ModeBinary, ScriptPath: "/nope/missing"}).ShouldRun() {
		t.Errorf("script inexistente = não roda")
	}
}

// Aceitação: RenderSmokeScript.
func TestRenderSmokeScript(t *testing.T) {
	eps := []EndpointsPrioritized{{Endpoint: "/a"}, {Endpoint: "/b"}}
	cfg := DefaultConfig()
	script, err := RenderSmokeScript("http://localhost", eps, cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	s := string(script)
	if !strings.Contains(s, "BASE = 'http://localhost'") {
		t.Errorf("BASE = target")
	}
	if !strings.Contains(s, "/a") || !strings.Contains(s, "/b") {
		t.Errorf("endpoints")
	}
	if !strings.Contains(s, "http.get") {
		t.Errorf("http.get call")
	}
}

// Aceitação: RenderSmokeScript empty target.
func TestRenderSmokeScriptNoTarget(t *testing.T) {
	if _, err := RenderSmokeScript("", []EndpointsPrioritized{{Endpoint: "/a"}}, DefaultConfig()); err == nil {
		t.Errorf("sem target devia falhar")
	}
}

// Aceitação: RenderSmokeScript empty eps.
func TestRenderSmokeScriptNoEps(t *testing.T) {
	if _, err := RenderSmokeScript("http://x", nil, DefaultConfig()); err == nil {
		t.Errorf("sem eps devia falhar")
	}
}

// Aceitação: RenderSmokeScript weight.
func TestRenderSmokeScriptWeight(t *testing.T) {
	eps := []EndpointsPrioritized{{Endpoint: "/a", Weight: 2.5}}
	_, _ = RenderSmokeScript("http://x", eps, DefaultConfig())
	// Smoke test: não falha.
}

// Aceitação: LoadScript.
func TestLoadScript(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "test.js")
	os.WriteFile(script, []byte("// test content"), 0644)
	data, src, err := LoadScript(script)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if src != SourceExisting {
		t.Errorf("source")
	}
	if !strings.Contains(string(data), "test content") {
		t.Errorf("content")
	}
}

// Aceitação: LoadScript missing.
func TestLoadScriptMissing(t *testing.T) {
	if _, _, err := LoadScript("/nope/missing"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: LoadScript empty path.
func TestLoadScriptEmpty(t *testing.T) {
	if _, _, err := LoadScript(""); err == nil {
		t.Errorf("vazio devia falhar")
	}
}

// Aceitação: ResolveScript existing.
func TestResolveScriptExisting(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "test.js")
	os.WriteFile(script, []byte("// user"), 0644)
	data, src, err := ResolveScript("http://x", script, nil, DefaultConfig())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if src != SourceExisting {
		t.Errorf("existing devia vencer, got %v", src)
	}
	if len(data) == 0 {
		t.Errorf("data vazio")
	}
}

// Aceitação: ResolveScript auto-gen fallback.
func TestResolveScriptAutoGen(t *testing.T) {
	eps := []EndpointsPrioritized{{Endpoint: "/a"}}
	data, src, err := ResolveScript("http://x", "/missing", eps, DefaultConfig())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if src != SourceAutoGen {
		t.Errorf("auto-gen, got %v", src)
	}
	if len(data) == 0 {
		t.Errorf("data vazio")
	}
}

// Aceitação: ResolveScript skip se sem nada.
func TestResolveScriptSkip(t *testing.T) {
	data, src, err := ResolveScript("http://x", "", nil, DefaultConfig())
	if err != nil {
		t.Errorf("skip não devia dar erro: %v", err)
	}
	if data != nil || src != "" {
		t.Errorf("skip = nil, src='', got data=%v src=%v", data, src)
	}
}

// Aceitação: ParseK6SummaryJSON.
func TestParseK6SummaryJSON(t *testing.T) {
	data := []byte(`{
		"metrics":{
			"http_req_duration":{"values":{"p(50)":50,"p(90)":150,"p(95)":400,"p(99)":800}},
			"http_req_failed":{"values":{"rate":0.001}},
			"http_reqs":{"values":{"count":1000,"rate":33.3}}
		}
	}`)
	s, err := ParseK6SummaryJSON(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.P95 != 400*time.Millisecond {
		t.Errorf("p95 = %v", s.P95)
	}
	if s.P99 != 800*time.Millisecond {
		t.Errorf("p99 = %v", s.P99)
	}
	if s.Requests != 1000 {
		t.Errorf("reqs = %d", s.Requests)
	}
	if s.Throughput < 33.0 || s.Throughput > 33.5 {
		t.Errorf("rps = %f", s.Throughput)
	}
}

// Aceitação: ParseK6SummaryJSON inválido.
func TestParseK6SummaryJSONInvalid(t *testing.T) {
	if _, err := ParseK6SummaryJSON([]byte("garbage")); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ParseK6SummaryJSON threshold fail.
func TestParseK6SummaryJSONThresholdFail(t *testing.T) {
	data := []byte(`{
		"metrics":{
			"http_req_duration":{
				"values":{"p(95)":500},
				"threshold":{"sources":[{"name":"p(95)<500","ok":false}]}
			},
			"http_req_failed":{"values":{"rate":0}},
			"http_reqs":{"values":{"count":10,"rate":1}}
		}
	}`)
	s, err := ParseK6SummaryJSON(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(s.ThresholdFailed) == 0 {
		t.Errorf("threshold failed não populado")
	}
}

// Aceitação: PassThresholds.
func TestPassThresholds(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ThresholdP95 = 500 * time.Millisecond
	cfg.ThresholdP99 = 1 * time.Second
	cfg.MaxErrorRate = 0.01
	good := &Summary{P95: 400 * time.Millisecond, P99: 800 * time.Millisecond, ErrorRate: 0.001}
	if !good.PassThresholds(cfg) {
		t.Errorf("bom devia passar")
	}
	bad := &Summary{P95: 600 * time.Millisecond, P99: 800 * time.Millisecond, ErrorRate: 0.001}
	if bad.PassThresholds(cfg) {
		t.Errorf("P95 alto devia falhar")
	}
	badErr := &Summary{P95: 100 * time.Millisecond, P99: 200 * time.Millisecond, ErrorRate: 0.5}
	if badErr.PassThresholds(cfg) {
		t.Errorf("err alto devia falhar")
	}
}

// Aceitação: PassThresholds threshold failed.
func TestPassThresholdsThresholdFailed(t *testing.T) {
	s := &Summary{ThresholdFailed: []string{"p(95)"}}
	if s.PassThresholds(DefaultConfig()) {
		t.Errorf("threshold failed = bloqueia")
	}
}

// Aceitação: RunK6.
func TestRunK6Binary(t *testing.T) {
	cmd, err := RunK6(context.Background(), RunConfig{
		Mode:   ModeBinary,
		Script: []byte("//"),
		Target: "http://x",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(cmd, "k6 run") {
		t.Errorf("cmd = %q", cmd)
	}
}

// Aceitação: RunK6 container.
func TestRunK6Container(t *testing.T) {
	cmd, err := RunK6(context.Background(), RunConfig{
		Mode:   ModeContainer,
		Script: []byte("//"),
		Target: "http://x",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(cmd, "docker run") {
		t.Errorf("cmd = %q", cmd)
	}
}

// Aceitação: RunK6 no script.
func TestRunK6NoScript(t *testing.T) {
	if _, err := RunK6(context.Background(), RunConfig{Mode: ModeBinary, Target: "x"}); err == nil {
		t.Errorf("nil script devia falhar")
	}
}

// Aceitação: RunK6 no target.
func TestRunK6NoTarget(t *testing.T) {
	if _, err := RunK6(context.Background(), RunConfig{Mode: ModeBinary, Script: []byte("//")}); err == nil {
		t.Errorf("no target devia falhar")
	}
}

// Aceitação: RunK6 invalid mode.
func TestRunK6InvalidMode(t *testing.T) {
	if _, err := RunK6(context.Background(), RunConfig{Mode: "x", Script: []byte("//"), Target: "x"}); err == nil {
		t.Errorf("mode inválido devia falhar")
	}
}

// Aceitação: PrioritizeEndpoints.
func TestPrioritizeEndpoints(t *testing.T) {
	eps := PrioritizeEndpoints([]string{"/a", "/b"}, []string{"/a"}, 1.0, 2.0)
	if eps[0].Weight != 2.0 {
		t.Errorf("added /a weight = 2.0, got %f", eps[0].Weight)
	}
	if eps[1].Weight != 1.0 {
		t.Errorf("base /b weight = 1.0, got %f", eps[1].Weight)
	}
}

// Aceitação: PrioritizeEndpoints default weights.
func TestPrioritizeEndpointsDefaults(t *testing.T) {
	eps := PrioritizeEndpoints([]string{"/a"}, nil, 0, 0)
	if eps[0].Weight != 1.0 {
		t.Errorf("default base = 1.0")
	}
}

// Aceitação: msToDuration.
func TestMsToDuration(t *testing.T) {
	if msToDuration(0) != 0 {
		t.Errorf("0")
	}
	if msToDuration(-1) != 0 {
		t.Errorf("-1")
	}
	if msToDuration(500) != 500*time.Millisecond {
		t.Errorf("500ms")
	}
}

// Aceitação: Mode constants.
func TestModeConstants(t *testing.T) {
	if ModeContainer == "" || ModeBinary == "" || ModeDisabled == "" {
		t.Errorf("constants")
	}
	if SourceExisting == "" || SourceAutoGen == "" {
		t.Errorf("sources")
	}
}
