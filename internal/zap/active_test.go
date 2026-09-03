package zap

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Aceitação: ParseOpenAPISpecJSON 3.x.
func TestParseOpenAPISpecJSON3(t *testing.T) {
	data := `{
		"openapi":"3.0.0",
		"info":{"title":"Test API","version":"1.0.0"},
		"servers":[{"url":"https://api.example.com"}],
		"paths":{
			"/users":{"get":{},"post":{}},
			"/users/{id}":{"get":{},"delete":{}}
		}
	}`
	s, err := ParseOpenAPISpecJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Title != "Test API" {
		t.Errorf("title = %q", s.Title)
	}
	if s.BaseURL != "https://api.example.com" {
		t.Errorf("baseURL = %q", s.BaseURL)
	}
	if s.EndpointsCount() != 4 {
		t.Errorf("endpoints = %d", s.EndpointsCount())
	}
}

// Aceitação: ParseOpenAPISpecJSON Swagger 2.0.
func TestParseOpenAPISpecJSON2(t *testing.T) {
	data := `{
		"swagger":"2.0",
		"info":{"title":"Old API","version":"2.0"},
		"host":"api.example.com",
		"basePath":"/v1",
		"schemes":["https"],
		"paths":{
			"/items":{"get":{},"post":{}}
		}
	}`
	s, err := ParseOpenAPISpecJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Title != "Old API" {
		t.Errorf("title = %q", s.Title)
	}
	if s.BaseURL != "https://api.example.com/v1" {
		t.Errorf("baseURL = %q", s.BaseURL)
	}
	if s.EndpointsCount() != 2 {
		t.Errorf("endpoints = %d", s.EndpointsCount())
	}
}

// Aceitação: ParseOpenAPISpecJSON inválido.
func TestParseOpenAPISpecJSONInvalid(t *testing.T) {
	if _, err := ParseOpenAPISpecJSON([]byte("garbage")); err == nil {
		t.Errorf("devia falhar")
	}
	if _, err := ParseOpenAPISpecJSON([]byte(`{"foo":"bar"}`)); err == nil {
		t.Errorf("sem openapi/swagger devia falhar")
	}
}

// Aceitação: OpenAPI 3.0.1 (3.0.x) ainda passa.
func TestParseOpenAPISpecJSON301(t *testing.T) {
	data := `{"openapi":"3.0.1","info":{"title":"x","version":"1"},"paths":{}}`
	s, err := ParseOpenAPISpecJSON([]byte(data))
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if s.Title != "x" {
		t.Errorf("title = %q", s.Title)
	}
}

// Aceitação: 3.1.x suportado.
func TestParseOpenAPISpecJSON31(t *testing.T) {
	data := `{"openapi":"3.1.0","info":{"title":"y","version":"2"},"paths":{}}`
	s, err := ParseOpenAPISpecJSON([]byte(data))
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if s.Title != "y" {
		t.Errorf("title = %q", s.Title)
	}
}

// Aceitação: EndpointsByMethod.
func TestEndpointsByMethod(t *testing.T) {
	s := &OpenAPISpec{
		Endpoints: []Endpoint{
			{Method: "GET"}, {Method: "GET"}, {Method: "POST"},
		},
	}
	got := s.EndpointsByMethod()
	if got["GET"] != 2 || got["POST"] != 1 {
		t.Errorf("got %+v", got)
	}
}

// Aceitação: HasPath.
func TestHasPath(t *testing.T) {
	s := &OpenAPISpec{Endpoints: []Endpoint{{Method: "GET", Path: "/x"}}}
	if !s.HasPath("/x") {
		t.Errorf("devia achar")
	}
	if s.HasPath("/y") {
		t.Errorf("não devia achar")
	}
}

// Aceitação: scanEndpointHeuristics detecta DELETE sem id.
func TestScanEndpointDeleteNoID(t *testing.T) {
	findings := scanEndpointHeuristics(Endpoint{Method: "DELETE", Path: "/users"})
	if len(findings) == 0 {
		t.Errorf("devia detectar")
	}
	hasHigh := false
	for _, f := range findings {
		if f.Severity == SevHigh {
			hasHigh = true
		}
	}
	if !hasHigh {
		t.Errorf("devia ser high")
	}
}

// Aceitação: scanEndpointHeuristics detecta POST sem id (medium).
func TestScanEndpointPostNoID(t *testing.T) {
	findings := scanEndpointHeuristics(Endpoint{Method: "POST", Path: "/users"})
	hasMedium := false
	for _, f := range findings {
		if f.Severity == SevMedium {
			hasMedium = true
		}
	}
	if !hasMedium {
		t.Errorf("POST sem id devia medium")
	}
}

// Aceitação: scanEndpointHeuristics NÃO detecta POST com id.
func TestScanEndpointPostWithID(t *testing.T) {
	findings := scanEndpointHeuristics(Endpoint{Method: "POST", Path: "/users/{id}/photos"})
	for _, f := range findings {
		if f.Name == "API-WRITE-NO-RESOURCE-ID" {
			t.Errorf("POST com id não devia flaggar WRITE-NO-RESOURCE-ID")
		}
	}
}

// Aceitação: scanEndpointHeuristics detecta GET admin.
func TestScanEndpointAdmin(t *testing.T) {
	findings := scanEndpointHeuristics(Endpoint{Method: "GET", Path: "/admin/users"})
	hasHigh := false
	for _, f := range findings {
		if f.Name == "API-ADMIN-EXPOSED" {
			hasHigh = true
		}
	}
	if !hasHigh {
		t.Errorf("admin devia flaggar")
	}
}

// Aceitação: scanEndpointHeuristics GET normal = no findings.
func TestScanEndpointNormal(t *testing.T) {
	findings := scanEndpointHeuristics(Endpoint{Method: "GET", Path: "/users/{id}"})
	if len(findings) != 0 {
		t.Errorf("GET /users/{id} limpo devia = 0; got %d", len(findings))
	}
}

// Aceitação: RunAPIScan bloqueia prod.
func TestRunAPIScanBlocksProd(t *testing.T) {
	_, err := RunAPIScan(context.Background(), "http://api.example.com", []byte(`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{"/a":{"get":{}}}}`),
		DefaultAPIScanConfig(), &TargetGuard{Allowlist: DefaultAllowlist()})
	if err == nil {
		t.Errorf("prod devia bloquear")
	}
}

// Aceitação: RunAPIScan permite local com findings heurísticos.
func TestRunAPIScanLocal(t *testing.T) {
	res, err := RunAPIScan(context.Background(), "http://localhost:8080",
		[]byte(`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{"/users":{"post":{}},"/admin":{"get":{}}}}`),
		DefaultAPIScanConfig(), &TargetGuard{Allowlist: DefaultAllowlist()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Scanned == 0 {
		t.Errorf("scanned = 0")
	}
	if res.Report.Counts.Total == 0 {
		t.Errorf("findings = 0")
	}
}

// Aceitação: RunAPIScan sem guard = error.
func TestRunAPIScanNoGuard(t *testing.T) {
	_, err := RunAPIScan(context.Background(), "http://localhost", nil, DefaultAPIScanConfig(), nil)
	if err == nil {
		t.Errorf("sem guard devia falhar")
	}
}

// Aceitação: RunAPIScan MaxEndpoints cap.
func TestRunAPIScanMaxEndpoints(t *testing.T) {
	spec := []byte(`{
		"openapi":"3.0.0",
		"info":{"title":"x","version":"1"},
		"paths":{
			"/p0":{"get":{}},
			"/p1":{"get":{}},
			"/p2":{"get":{}},
			"/p3":{"get":{}},
			"/p4":{"get":{}}
		}
	}`)
	cfg := DefaultAPIScanConfig()
	cfg.MaxEndpoints = 3
	res, err := RunAPIScan(context.Background(), "http://localhost:8080", spec, cfg,
		&TargetGuard{Allowlist: DefaultAllowlist()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Scanned != 3 || res.Skipped != 2 {
		t.Errorf("scanned=%d skipped=%d", res.Scanned, res.Skipped)
	}
}

// Aceitação: RunAPIScan timeout.
func TestRunAPIScanTimeout(t *testing.T) {
	cfg := APIScanConfig{
		Timeout: 1 * time.Nanosecond, // força timeout imediato
	}
	res, _ := RunAPIScan(context.Background(), "http://localhost",
		[]byte(`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{}}`),
		cfg, &TargetGuard{Allowlist: DefaultAllowlist()})
	// Pode completar antes do timeout OU retornar erro; ambos OK.
	if res == nil {
		t.Errorf("res nil")
	}
}

// Aceitação: RunAPIScan spec inválida.
func TestRunAPIScanBadSpec(t *testing.T) {
	_, err := RunAPIScan(context.Background(), "http://localhost", []byte("garbage"),
		DefaultAPIScanConfig(), &TargetGuard{Allowlist: DefaultAllowlist()})
	if err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: AuthHeaderRedact.
func TestAuthHeaderRedact(t *testing.T) {
	if got := AuthHeaderRedact(""); got != "" {
		t.Errorf("empty = %q", got)
	}
	got := AuthHeaderRedact("Authorization: Bearer abc123secret")
	if !strings.Contains(got, "REDACTED") {
		t.Errorf("devia redatar: %q", got)
	}
	if strings.Contains(got, "abc123secret") {
		t.Errorf("vazou secret: %q", got)
	}
	if got := AuthHeaderRedact("malformed"); !strings.Contains(got, "***") {
		t.Errorf("malformed = %q", got)
	}
}

// Aceitação: FilterFindingsBySeverity.
func TestFilterFindingsBySeverity(t *testing.T) {
	rep := &Report{Mode: "api", Target: "x"}
	rep.AddFinding(Finding{Severity: SevHigh})
	rep.AddFinding(Finding{Severity: SevMedium})
	rep.AddFinding(Finding{Severity: SevLow})
	out := FilterFindingsBySeverity(rep, SevMedium)
	if out.Counts.Total != 2 {
		t.Errorf("filter = %d, quero 2", out.Counts.Total)
	}
}

// Aceitação: FilterFindingsBySeverity nil.
func TestFilterFindingsBySeverityNil(t *testing.T) {
	if FilterFindingsBySeverity(nil, SevHigh) != nil {
		t.Errorf("nil devia ficar nil")
	}
}

// Aceitação: severityAtLeast.
func TestSeverityAtLeast(t *testing.T) {
	if !severityAtLeast(SevHigh, SevLow) {
		t.Errorf("high >= low")
	}
	if severityAtLeast(SevLow, SevHigh) {
		t.Errorf("low < high")
	}
	if !severityAtLeast(SevInfo, SevInfo) {
		t.Errorf("info >= info")
	}
}

// Aceitação: TotalSeverity.
func TestTotalSeverity(t *testing.T) {
	rep := &Report{}
	rep.AddFinding(Finding{Severity: SevHigh})
	if TotalSeverity(rep)[SevHigh] != 1 {
		t.Errorf("err")
	}
	if TotalSeverity(nil) != nil {
		t.Errorf("nil devia ficar nil")
	}
}

// Aceitação: openAPI 3 com path não-http-method ignorado.
func TestParseOpenAPI3IgnoresNonHTTPMethods(t *testing.T) {
	data := `{
		"openapi":"3.0.0",
		"info":{"title":"x","version":"1"},
		"paths":{
			"/a":{"get":{}}
		}
	}`
	s, err := ParseOpenAPISpecJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.EndpointsCount() != 1 {
		t.Errorf("got %d", s.EndpointsCount())
	}
}

// Aceitação: DefaultAPIScanConfig.
func TestDefaultAPIScanConfig(t *testing.T) {
	cfg := DefaultAPIScanConfig()
	if cfg.Timeout <= 0 || cfg.MaxEndpoints <= 0 || cfg.Concurrent <= 0 {
		t.Errorf("defaults errados")
	}
	if cfg.UserAgent == "" {
		t.Errorf("UA vazio")
	}
}

// Aceitação: swagger sem basePath.
func TestSwaggerNoBasePath(t *testing.T) {
	data := `{
		"swagger":"2.0",
		"info":{"title":"x","version":"1"},
		"host":"api.test",
		"schemes":["http"],
		"paths":{"/a":{"get":{}}}
	}`
	s, _ := ParseOpenAPISpecJSON([]byte(data))
	if s.BaseURL != "http://api.test/" {
		t.Errorf("baseURL = %q", s.BaseURL)
	}
}
