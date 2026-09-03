// Package frontend — frontend target resolver + healthcheck.
//
// SAI-043: Resolve URL via config ou runtime hooks; healthcheck
// antes de scan pesado (Lighthouse). Suporta backend-only fixture
// (sem frontend) sem ser invocado.
package frontend

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Mode indica como target frontend foi resolvido.
type Mode string

const (
	ModeConfig   Mode = "config"   // URL explícita em .solidify/config
	ModeRuntime  Mode = "runtime"  // runtime hook dinâmico
	ModeDisabled Mode = "disabled" // backend-only fixture
	ModeFailed   Mode = "failed"   // tentativa de resolver falhou
	ModeUnset    Mode = "unset"    // nada configurado
)

// Target representa URL frontend resolvida.
type Target struct {
	URL     string   `json:"url"`
	Origin  string   `json:"origin"`
	Path    string   `json:"path"`
	Mode    Mode     `json:"mode"`
	Reason  string   `json:"reason,omitempty"`
	Headers []string `json:"headers,omitempty"`
	Source  string   `json:"source,omitempty"` // de onde veio
}

// Config configura resolver.
type Config struct {
	URL           string        `json:"url,omitempty"`
	HeadersEnv    []string      `json:"headers_env,omitempty"`    // nomes de env vars a passar como headers
	Disabled      bool          `json:"disabled,omitempty"`       // backend-only
	HealthPath    string        `json:"health_path,omitempty"`    // ex.: "/healthz"
	HealthCode    int           `json:"health_code,omitempty"`    // default 200
	HealthTimeout time.Duration `json:"health_timeout,omitempty"` // default 5s
}

// IsBackendOnly devolve true se backend-only.
func (c *Config) IsBackendOnly() bool {
	return c.Disabled || c.URL == ""
}

// HasHealthCheck devolve true se HealthPath != "".
func (c *Config) HasHealthCheck() bool {
	return c.HealthPath != ""
}

// Resolver é a interface principal.
type Resolver struct {
	cfg   Config
	hooks []RuntimeHook
}

// RuntimeHook permite resolver target dinâmico (lambda, k8s API, etc).
type RuntimeHook interface {
	Name() string
	Resolve(ctx context.Context) (*Target, error)
}

// NewResolver cria resolver a partir de config.
func NewResolver(cfg Config) *Resolver {
	return &Resolver{cfg: cfg}
}

// AddHook adiciona runtime hook (ordem = ordem de adição).
func (r *Resolver) AddHook(h RuntimeHook) {
	r.hooks = append(r.hooks, h)
}

// Resolve tenta: config → runtime hooks → unset.
// Não-fatal: devolve Target com Mode=ModeUnset se nada resolver.
func (r *Resolver) Resolve(ctx context.Context) (*Target, error) {
	if r.cfg.Disabled {
		return &Target{Mode: ModeDisabled, Reason: "frontend disabled em config (backend-only)"}, nil
	}
	if u := r.cfg.URL; u != "" {
		t, err := parseTarget(u, ModeConfig, "config.url", "")
		if err != nil {
			return &Target{Mode: ModeFailed, Reason: err.Error()}, err
		}
		t.Headers = resolveEnvHeaders(r.cfg.HeadersEnv)
		return t, nil
	}
	for _, h := range r.hooks {
		t, err := h.Resolve(ctx)
		if err != nil {
			continue
		}
		if t != nil && t.URL != "" {
			return t, nil
		}
	}
	return &Target{Mode: ModeUnset, Reason: "nenhuma fonte de URL configurada"}, nil
}

// parseTarget valida URL.
func parseTarget(raw string, mode Mode, source, reason string) (*Target, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("frontend: parse url: %w", err)
	}
	if u.Host == "" {
		return nil, errors.New("frontend: url sem host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("frontend: scheme inválido: %s", u.Scheme)
	}
	origin := u.Scheme + "://" + u.Host
	path := u.Path
	if path == "" {
		path = "/"
	}
	return &Target{
		URL:    raw,
		Origin: origin,
		Path:   path,
		Mode:   mode,
		Source: source,
		Reason: reason,
	}, nil
}

// resolveEnvHeaders transforma env var names → "Name: Value".
// Erros de env (não setada) são silenciosos (header skip).
func resolveEnvHeaders(names []string) []string {
	var out []string
	for _, n := range names {
		v := os.Getenv(n)
		if v == "" {
			continue
		}
		out = append(out, n+": "+v)
	}
	return out
}

// HealthcheckResult estado.
type HealthcheckResult struct {
	OK     bool          `json:"ok"`
	Code   int           `json:"code"`
	Time   time.Duration `json:"time_ns"`
	Err    string        `json:"err,omitempty"`
	Target *Target       `json:"target"`
}

// Healthcheck testa Target.HealthPath via HEAD/GET; default code=200.
func Healthcheck(ctx context.Context, t *Target, cfg Config) HealthcheckResult {
	h := HealthcheckResult{Target: t}
	if t == nil || t.URL == "" {
		h.Err = "target vazio"
		return h
	}
	if cfg.HealthPath == "" {
		h.Err = "healthcheck não configurado"
		return h
	}
	timeout := cfg.HealthTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	wantCode := cfg.HealthCode
	if wantCode == 0 {
		wantCode = 200
	}
	h.Code = -1 // unreachable marker
	h.Time = 0
	hcCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	h.Time = time.Since(start)
	// Implementação real: HTTP HEAD/GET (placeholder estrutural).
	// Não executa IO aqui — caller passa result via InjectHealth.
	_ = hcCtx
	_ = wantCode
	return h
}

// IsLocalhost devolve true se target é localhost.
func IsLocalhost(t *Target) bool {
	if t == nil {
		return false
	}
	check := t.Origin
	if check == "" {
		check = t.URL
	}
	return strings.HasPrefix(check, "http://localhost") ||
		strings.HasPrefix(check, "http://127.0.0.1") ||
		strings.HasPrefix(check, "http://[::1]")
}

// IsHTTPS devolve true se scheme é https.
func IsHTTPS(t *Target) bool {
	if t == nil {
		return false
	}
	return strings.HasPrefix(t.URL, "https://")
}

// EnvHook — runtime hook simples baseado em env var.
type EnvHook struct {
	Var string
}

// Name devolve nome do hook.
func (h *EnvHook) Name() string { return "env:" + h.Var }

// Resolve lê env var, parseia como target.
func (h *EnvHook) Resolve(ctx context.Context) (*Target, error) {
	v := os.Getenv(h.Var)
	if v == "" {
		return nil, errors.New("env vazia")
	}
	return parseTarget(v, ModeRuntime, "env:"+h.Var, "")
}
