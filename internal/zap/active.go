package zap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// APIScanConfig configura scan ativo contra OpenAPI spec.
type APIScanConfig struct {
	Timeout         time.Duration // timeout total; default 10min
	MaxEndpoints    int           // limite de endpoints a scanear; default 100
	Concurrent      int           // requests paralelos; default 5
	PassAuthHeaders []string      // headers ex.: "Authorization: Bearer X" (NÃO logado)
	UserAgent       string        // UA customizado (não identificar Solidify)
}

// DefaultAPIScanConfig devolve defaults seguros.
func DefaultAPIScanConfig() APIScanConfig {
	return APIScanConfig{
		Timeout:      10 * time.Minute,
		MaxEndpoints: 100,
		Concurrent:   5,
		UserAgent:    "Solidify-ZAP/1.0 (release-quality-gate)",
	}
}

// Endpoint é um endpoint da spec OpenAPI.
type Endpoint struct {
	Method  string `json:"method"`  // GET/POST/PUT/DELETE/PATCH
	Path    string `json:"path"`    // /users/{id}
	Summary string `json:"summary"` // descrição opcional
	OpID    string `json:"op_id"`   // operationId (opcional)
}

// OpenAPISpec parsed subset.
type OpenAPISpec struct {
	Title     string     `json:"title"`
	Version   string     `json:"version"`
	BaseURL   string     `json:"base_url"`
	Endpoints []Endpoint `json:"endpoints"`
}

// openAPI3Schema subset pra parsing JSON.
type openAPI3 struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

// swagger2Schema subset pra parsing JSON (fallback).
type swagger2 struct {
	Swagger string `json:"swagger"`
	Info    struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Host        string                                `json:"host"`
	BasePath    string                                `json:"basePath"`
	Schemes     []string                              `json:"schemes"`
	Paths       map[string]map[string]json.RawMessage `json:"paths"`
	Definitions map[string]any                        `json:"-"`
}

// ParseOpenAPISpecJSON parseia OpenAPI 3.x ou Swagger 2.0 JSON.
// Sem YAML support — caller converte antes se precisar.
func ParseOpenAPISpecJSON(data []byte) (*OpenAPISpec, error) {
	// Detectar OpenAPI 3.x.
	var v3 openAPI3
	if err := json.Unmarshal(data, &v3); err == nil && strings.HasPrefix(v3.OpenAPI, "3.") {
		return convertOpenAPI3(&v3), nil
	}
	// Detectar Swagger 2.0.
	var v2 swagger2
	if err := json.Unmarshal(data, &v2); err == nil && strings.HasPrefix(v2.Swagger, "2.") {
		return convertSwagger2(&v2), nil
	}
	return nil, errors.New("zap: spec não é OpenAPI 3.x nem Swagger 2.0")
}

func convertOpenAPI3(s *openAPI3) *OpenAPISpec {
	out := &OpenAPISpec{Title: s.Info.Title, Version: s.Info.Version}
	if len(s.Servers) > 0 {
		out.BaseURL = s.Servers[0].URL
	}
	for path, methods := range s.Paths {
		for method := range methods {
			m := strings.ToUpper(method)
			switch m {
			case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
				out.Endpoints = append(out.Endpoints, Endpoint{Method: m, Path: path})
			}
		}
	}
	return out
}

func convertSwagger2(s *swagger2) *OpenAPISpec {
	out := &OpenAPISpec{Title: s.Info.Title, Version: s.Info.Version}
	scheme := "https"
	if len(s.Schemes) > 0 {
		scheme = s.Schemes[0]
	}
	base := s.BasePath
	if base == "" {
		base = "/"
	}
	out.BaseURL = scheme + "://" + s.Host + base
	for path, methods := range s.Paths {
		for method := range methods {
			m := strings.ToUpper(method)
			switch m {
			case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
				out.Endpoints = append(out.Endpoints, Endpoint{Method: m, Path: path})
			}
		}
	}
	return out
}

// EndpointsCount conta endpoints no spec.
func (s *OpenAPISpec) EndpointsCount() int {
	return len(s.Endpoints)
}

// EndpointsByMethod agrupa.
func (s *OpenAPISpec) EndpointsByMethod() map[string]int {
	out := make(map[string]int)
	for _, e := range s.Endpoints {
		out[e.Method]++
	}
	return out
}

// HasPath checa existência de path.
func (s *OpenAPISpec) HasPath(path string) bool {
	for _, e := range s.Endpoints {
		if e.Path == path {
			return true
		}
	}
	return false
}

// APIScanResult resultado.
type APIScanResult struct {
	Spec     *OpenAPISpec  `json:"spec"`
	Report   *Report       `json:"report"`
	Duration time.Duration `json:"duration_ns"`
	Scanned  int           `json:"scanned"` // endpoints efetivamente scaneados
	Skipped  int           `json:"skipped"` // endpoints pulados (cap MaxEndpoints)
	Err      string        `json:"err,omitempty"`
}

// RunAPIScan valida target + parse spec + simula scan.
// Sem execução real (não instala ZAP); gera findings heurísticos baseados
// na spec (ex.: PUT/DELETE sem auth marker = high).
func RunAPIScan(ctx context.Context, targetURL string, spec []byte, cfg APIScanConfig, guard *TargetGuard) (*APIScanResult, error) {
	if guard == nil {
		return nil, errors.New("zap: TargetGuard obrigatório")
	}
	if err := guard.Validate(targetURL); err != nil {
		return nil, err
	}
	if cfg.Timeout <= 0 {
		cfg = DefaultAPIScanConfig()
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	start := time.Now()
	parsed, err := ParseOpenAPISpecJSON(spec)
	if err != nil {
		return nil, fmt.Errorf("zap: parse spec: %w", err)
	}

	class, _ := ClassifyTarget(targetURL)
	rep := &Report{Mode: "api", Target: targetURL, Class: class}

	total := len(parsed.Endpoints)
	scanned := total
	skipped := 0
	if total > cfg.MaxEndpoints {
		scanned = cfg.MaxEndpoints
		skipped = total - cfg.MaxEndpoints
	}

	for i := 0; i < scanned; i++ {
		select {
		case <-ctx.Done():
			return &APIScanResult{
				Spec: parsed, Report: rep,
				Duration: time.Since(start),
				Scanned:  i, Skipped: skipped,
				Err: ctx.Err().Error(),
			}, ctx.Err()
		default:
		}
		ep := parsed.Endpoints[i]
		for _, f := range scanEndpointHeuristics(ep) {
			rep.AddFinding(f)
		}
	}

	return &APIScanResult{
		Spec: parsed, Report: rep,
		Duration: time.Since(start),
		Scanned:  scanned, Skipped: skipped,
	}, nil
}

// scanEndpointHeuristics analisa um endpoint e devolve findings heurísticos.
func scanEndpointHeuristics(ep Endpoint) []Finding {
	var out []Finding
	// Escrita sem path param (id): possível mass-assignment / falta de escopo.
	writeMethod := ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH"
	hasIDParam := strings.Contains(ep.Path, "{")
	if writeMethod && !hasIDParam {
		out = append(out, Finding{
			Name:     "API-WRITE-NO-RESOURCE-ID",
			Severity: SevMedium,
			Risk:     "medium",
			URL:      ep.Path,
			Method:   ep.Method,
			Solution: "Adicionar path param {id} ou escopo de recurso",
			Source:   "zap-api",
		})
	}
	// DELETE sem path param: provavelmente errado.
	if ep.Method == "DELETE" && !hasIDParam {
		out = append(out, Finding{
			Name:     "API-DELETE-NO-RESOURCE-ID",
			Severity: SevHigh,
			Risk:     "high",
			URL:      ep.Path,
			Method:   ep.Method,
			Solution: "DELETE sempre deve ter path param {id}",
			Source:   "zap-api",
		})
	}
	// GET admin-like paths.
	if ep.Method == "GET" && (strings.HasPrefix(ep.Path, "/admin") || strings.Contains(ep.Path, "/admin/")) {
		out = append(out, Finding{
			Name:     "API-ADMIN-EXPOSED",
			Severity: SevHigh,
			Risk:     "high",
			URL:      ep.Path,
			Method:   ep.Method,
			Solution: "Validar auth + autorização admin",
			Source:   "zap-api",
		})
	}
	return out
}

// AuthHeaderRedact devolve header redacted pra logs.
func AuthHeaderRedact(h string) string {
	if h == "" {
		return ""
	}
	idx := strings.Index(h, ":")
	if idx < 0 {
		return "***"
	}
	return strings.TrimSpace(h[:idx+1]) + " ***REDACTED***"
}

// FilterFindingsBySeverity filtra findings por severity min.
func FilterFindingsBySeverity(rep *Report, minSev Severity) *Report {
	if rep == nil {
		return nil
	}
	out := &Report{Mode: rep.Mode, Target: rep.Target, Class: rep.Class}
	for _, f := range rep.Findings {
		if severityAtLeast(f.Severity, minSev) {
			out.AddFinding(f)
		}
	}
	return out
}

func severityAtLeast(got, min Severity) bool {
	ranks := map[Severity]int{
		SevInfo:   0,
		SevLow:    1,
		SevMedium: 2,
		SevHigh:   3,
	}
	return ranks[got] >= ranks[min]
}

// TotalSeverity retorna count por severity.
func TotalSeverity(rep *Report) map[Severity]int {
	if rep == nil {
		return nil
	}
	return rep.Counts.BySeverity
}
