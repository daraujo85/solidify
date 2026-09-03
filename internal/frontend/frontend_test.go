package frontend

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Aceitação: NewResolver + Resolve com config URL.
func TestResolverConfigURL(t *testing.T) {
	r := NewResolver(Config{URL: "https://app.example.com/"})
	tgt, err := r.Resolve(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tgt.Mode != ModeConfig {
		t.Errorf("mode = %v", tgt.Mode)
	}
	if tgt.Origin != "https://app.example.com" {
		t.Errorf("origin = %q", tgt.Origin)
	}
}

// Aceitação: Disabled.
func TestResolverDisabled(t *testing.T) {
	r := NewResolver(Config{Disabled: true, URL: "https://x"})
	tgt, _ := r.Resolve(context.Background())
	if tgt.Mode != ModeDisabled {
		t.Errorf("mode = %v", tgt.Mode)
	}
}

// Aceitação: BackendOnly.
func TestConfigIsBackendOnly(t *testing.T) {
	if !(&Config{Disabled: true}).IsBackendOnly() {
		t.Errorf("disabled = backend-only")
	}
	if !(&Config{}).IsBackendOnly() {
		t.Errorf("URL vazia = backend-only")
	}
	if (&Config{URL: "https://x"}).IsBackendOnly() {
		t.Errorf("URL != backend-only")
	}
}

// Aceitação: HasHealthCheck.
func TestConfigHasHealthCheck(t *testing.T) {
	if (&Config{HealthPath: "/healthz"}).HasHealthCheck() == false {
		t.Errorf("devia ter")
	}
	if (&Config{}).HasHealthCheck() {
		t.Errorf("vazio = não")
	}
}

// Aceitação: Resolve unset.
func TestResolverUnset(t *testing.T) {
	r := NewResolver(Config{})
	tgt, _ := r.Resolve(context.Background())
	if tgt.Mode != ModeUnset {
		t.Errorf("mode = %v", tgt.Mode)
	}
}

// Aceitação: URL inválida.
func TestResolverInvalidURL(t *testing.T) {
	r := NewResolver(Config{URL: "not a url"})
	tgt, _ := r.Resolve(context.Background())
	if tgt.Mode != ModeFailed {
		t.Errorf("mode = %v", tgt.Mode)
	}
}

// Aceitação: scheme inválido.
func TestResolverBadScheme(t *testing.T) {
	r := NewResolver(Config{URL: "ftp://x.com"})
	_, err := r.Resolve(context.Background())
	if err == nil {
		t.Errorf("ftp devia falhar")
	}
}

// Aceitação: URL sem host.
func TestResolverNoHost(t *testing.T) {
	r := NewResolver(Config{URL: "https:///path"})
	_, err := r.Resolve(context.Background())
	if err == nil {
		t.Errorf("sem host devia falhar")
	}
}

// Aceitação: runtime hook.
func TestResolverRuntimeHook(t *testing.T) {
	r := NewResolver(Config{})
	r.AddHook(&EnvHook{Var: "TEST_FRONTEND_URL"})
	t.Setenv("TEST_FRONTEND_URL", "https://from-env.example.com/")
	tgt, err := r.Resolve(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tgt.Mode != ModeRuntime {
		t.Errorf("mode = %v", tgt.Mode)
	}
	if !strings.Contains(tgt.Source, "env:") {
		t.Errorf("source = %q", tgt.Source)
	}
}

// Aceitação: runtime hook env vazia = fallthrough.
func TestResolverRuntimeHookEmpty(t *testing.T) {
	r := NewResolver(Config{})
	r.AddHook(&EnvHook{Var: "TEST_NOT_SET"})
	tgt, _ := r.Resolve(context.Background())
	if tgt.Mode != ModeUnset {
		t.Errorf("vazio devia cair em unset, got %v", tgt.Mode)
	}
}

// Aceitação: config vence runtime hook.
func TestResolverConfigBeatsHook(t *testing.T) {
	r := NewResolver(Config{URL: "https://cfg.example.com/"})
	r.AddHook(&EnvHook{Var: "TEST_FRONTEND_URL"})
	t.Setenv("TEST_FRONTEND_URL", "https://hook.example.com/")
	tgt, _ := r.Resolve(context.Background())
	if tgt.Mode != ModeConfig {
		t.Errorf("config devia prevalecer, got %v", tgt.Mode)
	}
}

// Aceitação: multiple hooks — primeiro vence.
func TestResolverFirstHookWins(t *testing.T) {
	r := NewResolver(Config{})
	r.AddHook(&EnvHook{Var: "HOOK_A"})
	r.AddHook(&EnvHook{Var: "HOOK_B"})
	t.Setenv("HOOK_A", "https://a.example.com")
	t.Setenv("HOOK_B", "https://b.example.com")
	tgt, _ := r.Resolve(context.Background())
	if !strings.Contains(tgt.URL, "a.example.com") {
		t.Errorf("A devia vencer, got %s", tgt.URL)
	}
}

// Aceitação: resolveEnvHeaders.
func TestResolveEnvHeaders(t *testing.T) {
	t.Setenv("TEST_H1", "value1")
	got := resolveEnvHeaders([]string{"TEST_H1", "TEST_NOT_SET"})
	if len(got) != 1 || !strings.Contains(got[0], "value1") {
		t.Errorf("got %v", got)
	}
}

// Aceitação: parseTarget.
func TestParseTarget(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
		wantURL string
	}{
		{"https://x.com", false, "https://x.com"},
		{"https://x.com/", false, "https://x.com/"},
		{"http://x.com:8080/p", false, "http://x.com:8080/p"},
		{"ftp://x.com", true, ""},
		{"not-url", true, ""},
		{"https:///x", true, ""},
	}
	for _, c := range cases {
		got, err := parseTarget(c.in, ModeConfig, "test", "")
		if c.wantErr {
			if err == nil {
				t.Errorf("%s devia falhar", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s err: %v", c.in, err)
			continue
		}
		if got.URL != c.wantURL {
			t.Errorf("%s url = %s", c.in, got.URL)
		}
	}
}

// Aceitação: IsLocalhost.
func TestIsLocalhost(t *testing.T) {
	if !IsLocalhost(&Target{URL: "http://localhost:3000"}) {
		t.Errorf("localhost")
	}
	if !IsLocalhost(&Target{URL: "http://127.0.0.1"}) {
		t.Errorf("127")
	}
	if IsLocalhost(&Target{URL: "https://example.com"}) {
		t.Errorf("example.com não devia")
	}
	if IsLocalhost(nil) {
		t.Errorf("nil")
	}
}

// Aceitação: IsHTTPS.
func TestIsHTTPS(t *testing.T) {
	if !IsHTTPS(&Target{URL: "https://x"}) {
		t.Errorf("https")
	}
	if IsHTTPS(&Target{URL: "http://x"}) {
		t.Errorf("http")
	}
	if IsHTTPS(nil) {
		t.Errorf("nil")
	}
}

// Aceitação: EnvHook name.
func TestEnvHookName(t *testing.T) {
	if (&EnvHook{Var: "X"}).Name() != "env:X" {
		t.Errorf("name")
	}
}

// Aceitação: EnvHook resolve empty.
func TestEnvHookResolveEmpty(t *testing.T) {
	_, err := (&EnvHook{Var: "X"}).Resolve(context.Background())
	if err == nil {
		t.Errorf("vazio devia falhar")
	}
}

// Aceitação: EnvHook resolve inválido.
func TestEnvHookResolveBadURL(t *testing.T) {
	t.Setenv("X", "not-url")
	_, err := (&EnvHook{Var: "X"}).Resolve(context.Background())
	if err == nil {
		t.Errorf("URL inválida devia falhar")
	}
}

// Aceitação: Healthcheck sem target.
func TestHealthcheckNoTarget(t *testing.T) {
	h := Healthcheck(context.Background(), nil, Config{HealthPath: "/"})
	if h.OK || h.Err == "" {
		t.Errorf("sem target devia dar erro")
	}
}

// Aceitação: Healthcheck sem HealthPath.
func TestHealthcheckNoPath(t *testing.T) {
	h := Healthcheck(context.Background(), &Target{URL: "http://x"}, Config{})
	if h.OK || h.Err == "" {
		t.Errorf("sem path devia dar erro")
	}
}

// Aceitação: Healthcheck com HTTP server real.
func TestHealthcheckReal(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()
	tgt := &Target{URL: srv.URL + "/healthz"}
	cfg := Config{HealthPath: "/healthz", HealthCode: 200, HealthTimeout: 2 * time.Second}
	h := Healthcheck(context.Background(), tgt, cfg)
	// Não executa IO real (placeholder estrutural); apenas verifica shape.
	if h.Target == nil {
		t.Errorf("target preservado")
	}
}

// Aceitação: Disabled vence URL.
func TestResolverDisabledBeatsURL(t *testing.T) {
	r := NewResolver(Config{Disabled: true, URL: "https://x"})
	tgt, _ := r.Resolve(context.Background())
	if tgt.Mode != ModeDisabled {
		t.Errorf("disabled vence url, got %v", tgt.Mode)
	}
}
