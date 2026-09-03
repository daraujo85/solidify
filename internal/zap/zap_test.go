package zap

import (
	"strings"
	"testing"
)

// Aceitação: ClassifyTarget.
func TestClassifyTarget(t *testing.T) {
	cases := map[string]TargetClass{
		"http://localhost:8080":          ClassLocal,
		"http://127.0.0.1:3000":          ClassLocal,
		"http://192.168.1.5":             ClassLocal,
		"http://10.0.0.1":                ClassLocal,
		"http://api.test.local":          ClassLocal,
		"http://app.test.example.com":    ClassTest,
		"http://test.example.com":        ClassTest,
		"http://api.staging.example.com": ClassStaging,
		"http://api.example.com":         ClassProd,
		"http://example.com":             ClassProd,
	}
	for in, want := range cases {
		got, err := ClassifyTarget(in)
		if err != nil {
			t.Errorf("%s err: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ClassifyTarget(%s) = %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: ClassifyTarget URL inválida.
func TestClassifyTargetInvalid(t *testing.T) {
	if _, err := ClassifyTarget(""); err == nil {
		t.Errorf("devia falhar")
	}
	if _, err := ClassifyTarget("not a url"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Allowlist Default não permite prod.
func TestAllowlistProdBlocked(t *testing.T) {
	al := DefaultAllowlist()
	if al.Allows("api.example.com", ClassProd) {
		t.Errorf("PROD nunca devia ser allowed")
	}
}

// Aceitação: Allowlist permite localhost.
func TestAllowlistLocal(t *testing.T) {
	al := DefaultAllowlist()
	if !al.Allows("localhost", ClassLocal) {
		t.Errorf("localhost devia ser allowed")
	}
}

// Aceitação: Allowlist wildcard test.
func TestAllowlistTestWildcard(t *testing.T) {
	al := DefaultAllowlist()
	if !al.Allows("app.test", ClassTest) {
		t.Errorf("app.test devia casar *.test")
	}
	if !al.Allows("api.test", ClassTest) {
		t.Errorf("api.test devia casar *.test")
	}
}

// Aceitação: Allowlist não casa prod.
func TestAllowlistTestDoesntMatchProd(t *testing.T) {
	al := DefaultAllowlist()
	if al.Allows("api.example.com", ClassTest) {
		t.Errorf("example.com não devia casar *.test")
	}
}

// Aceitação: Allowlist class upgrade.
func TestAllowlistClassUpgrade(t *testing.T) {
	al := NewAllowlist([]AllowlistEntry{
		{Host: "localhost", Class: ClassLocal},
	})
	// localhost classificado como Test devia ainda ser allowed.
	if !al.Allows("localhost", ClassTest) {
		t.Errorf("upgrade devia permitir")
	}
}

// Aceitação: Allowlist custom.
func TestAllowlistCustom(t *testing.T) {
	al := NewAllowlist([]AllowlistEntry{
		{Host: "qa.example.com", Class: ClassStaging},
	})
	if !al.Allows("qa.example.com", ClassStaging) {
		t.Errorf("devia permitir")
	}
	if al.Allows("localhost", ClassLocal) {
		t.Errorf("default bloqueado")
	}
}

// Aceitação: riskToSeverity.
func TestRiskToSeverity(t *testing.T) {
	cases := map[string]Severity{
		"HIGH":          SevHigh,
		"high":          SevHigh,
		"  High ":       SevHigh,
		"MEDIUM":        SevMedium,
		"LOW":           SevLow,
		"INFORMATIONAL": SevInfo,
		"unknown":       SevInfo,
	}
	for in, want := range cases {
		if got := riskToSeverity(in); got != want {
			t.Errorf("%s → %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: NormalizeAlert.
func TestNormalizeAlert(t *testing.T) {
	a := Alert{
		Name: "XSS", Risk: "HIGH", Confidence: "medium",
		URL: "http://x/y", Method: "GET", CWE: "79",
	}
	f := NormalizeAlert(a, "zap-baseline")
	if f.Severity != SevHigh {
		t.Errorf("sev = %v", f.Severity)
	}
	if f.CWE != "79" {
		t.Errorf("cwe = %s", f.CWE)
	}
	if f.Source != "zap-baseline" {
		t.Errorf("source = %s", f.Source)
	}
}

// Aceitação: AddFinding + counts.
func TestAddFinding(t *testing.T) {
	r := &Report{}
	r.AddFinding(Finding{Severity: SevHigh})
	r.AddFinding(Finding{Severity: SevMedium})
	r.AddFinding(Finding{Severity: SevHigh})
	if r.Counts.Total != 3 {
		t.Errorf("total = %d", r.Counts.Total)
	}
	if r.Counts.BySeverity[SevHigh] != 2 {
		t.Errorf("high = %d", r.Counts.BySeverity[SevHigh])
	}
}

// Aceitação: HasHigh.
func TestHasHigh(t *testing.T) {
	r := &Report{}
	if r.HasHigh() {
		t.Errorf("vazio devia false")
	}
	r.AddFinding(Finding{Severity: SevLow})
	if r.HasHigh() {
		t.Errorf("low não devia ser high")
	}
	r.AddFinding(Finding{Severity: SevHigh})
	if !r.HasHigh() {
		t.Errorf("com high devia true")
	}
}

// Aceitação: ParseZAPReportJSON.
func TestParseZAPReportJSON(t *testing.T) {
	data := `{
		"site":[
			{
				"alerts":[
					{
						"name":"Cross Site Scripting (Reflected)",
						"risk":"HIGH",
						"confidence":"medium",
						"url":"http://x/y?q=<script>",
						"method":"GET",
						"cwe":"79"
					},
					{
						"name":"Missing Anti-clickjacking Header",
						"risk":"LOW",
						"confidence":"high",
						"url":"http://x/",
						"method":"GET",
						"cwe":"1021"
					}
				]
			}
		]
	}`
	rep, err := ParseZAPReportJSON([]byte(data), "baseline", "http://x/", ClassLocal)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rep.Findings) != 2 {
		t.Errorf("len = %d", len(rep.Findings))
	}
	if rep.Mode != "baseline" {
		t.Errorf("mode = %s", rep.Mode)
	}
	if rep.Class != ClassLocal {
		t.Errorf("class = %s", rep.Class)
	}
	if rep.Counts.BySeverity[SevHigh] != 1 {
		t.Errorf("high = %d", rep.Counts.BySeverity[SevHigh])
	}
}

// Aceitação: ParseZAPReportJSON inválido.
func TestParseZAPReportJSONInvalid(t *testing.T) {
	if _, err := ParseZAPReportJSON([]byte("garbage"), "baseline", "", ClassLocal); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ParseZAPReportJSON vazio.
func TestParseZAPReportJSONEmpty(t *testing.T) {
	rep, _ := ParseZAPReportJSON([]byte(`{"site":[]}`), "baseline", "", ClassLocal)
	if rep.Counts.Total != 0 {
		t.Errorf("err")
	}
}

// Aceitação: TargetGuard bloqueia prod.
func TestTargetGuardBlocksProd(t *testing.T) {
	g := &TargetGuard{Allowlist: DefaultAllowlist()}
	if err := g.Validate("http://api.example.com"); err == nil {
		t.Errorf("prod devia bloquear")
	} else if !strings.Contains(err.Error(), "PROD") {
		t.Errorf("err = %v", err)
	}
}

// Aceitação: TargetGuard permite local.
func TestTargetGuardAllowsLocal(t *testing.T) {
	g := &TargetGuard{Allowlist: DefaultAllowlist()}
	if err := g.Validate("http://localhost:8080"); err != nil {
		t.Errorf("local devia passar: %v", err)
	}
}

// Aceitação: TargetGuard bloqueia não allowlist.
func TestTargetGuardBlocksUnknown(t *testing.T) {
	g := &TargetGuard{Allowlist: DefaultAllowlist()}
	if err := g.Validate("http://randomhost.local"); err == nil {
		t.Errorf("não allowlist devia bloquear")
	}
}

// Aceitação: TargetGuard AllowProd override.
func TestTargetGuardAllowProdOverride(t *testing.T) {
	g := &TargetGuard{Allowlist: DefaultAllowlist(), AllowProd: true}
	al := NewAllowlist([]AllowlistEntry{{Host: "api.example.com", Class: ClassProd}})
	g.Allowlist = al
	if err := g.Validate("http://api.example.com"); err != nil {
		t.Errorf("override devia permitir: %v", err)
	}
}

// Aceitação: TargetGuard URL inválida.
func TestTargetGuardInvalid(t *testing.T) {
	g := &TargetGuard{Allowlist: DefaultAllowlist()}
	if err := g.Validate(""); err == nil {
		t.Errorf("err")
	}
}

// Aceitação: urlHost.
func TestUrlHost(t *testing.T) {
	if got := urlHost("http://api.example.com:8080/x"); got != "api.example.com" {
		t.Errorf("got %q", got)
	}
	if got := urlHost("not a url"); got != "" {
		t.Errorf("got %q", got)
	}
}

// Aceitação: matchHost edge cases.
func TestMatchHost(t *testing.T) {
	if !matchHost("api.test", "*.test") {
		t.Errorf("wildcard")
	}
	if !matchHost("api.test", "api.test") {
		t.Errorf("exact")
	}
	if matchHost("api.test", "*.example") {
		t.Errorf("não devia casar")
	}
}

// Aceitação: NewAllowlist empty.
func TestNewAllowlistEmpty(t *testing.T) {
	al := NewAllowlist(nil)
	if al.Allows("localhost", ClassLocal) {
		t.Errorf("vazio devia bloquear")
	}
}

// Aceitação: ClassifyTarget IPv6.
func TestClassifyTargetIPv6(t *testing.T) {
	if got, _ := ClassifyTarget("http://[::1]:8080"); got != ClassLocal {
		t.Errorf("IPv6 = %v", got)
	}
}
