package applicability

import (
	"testing"

	"github.com/diegoaraujo/solidify/internal/component"
)

// findDecision devolve o Decision do gate, ou falha.
func findDecision(t *testing.T, d []Decision, g Gate) Decision {
	t.Helper()
	for _, x := range d {
		if x.Gate == g {
			return x
		}
	}
	t.Fatalf("gate %s ausente das decisões", g)
	return Decision{}
}

// Aceitação crítica: backend-only ⇒ Lighthouse NOT_APPLICABLE.
func TestBackendOnlyLighthouseNotApplicable(t *testing.T) {
	p := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go", "apps/api/handlers/user.go"},
	}
	d := Decide(p)
	lh := findDecision(t, d, GateLighthouse)
	if lh.Verdict != NotApplicable {
		t.Errorf("backend-only: Lighthouse = %s, quero NOT_APPLICABLE", lh.Verdict)
	}
}

// Frontend-web sem target ⇒ CONDITIONAL.
func TestFrontendNoTargetLighthouseConditional(t *testing.T) {
	p := Profile{
		Components:   []component.Component{component.FrontendWeb},
		ChangedPaths: []string{"apps/web/src/App.tsx"},
	}
	d := Decide(p)
	lh := findDecision(t, d, GateLighthouse)
	if lh.Verdict != Conditional {
		t.Errorf("frontend sem target: Lighthouse = %s, quero CONDITIONAL", lh.Verdict)
	}
}

// Frontend-web + target ⇒ APPLICABLE.
func TestFrontendWithTargetLighthouseApplicable(t *testing.T) {
	p := Profile{
		Components:       []component.Component{component.FrontendWeb},
		ChangedPaths:     []string{"apps/web/src/App.tsx"},
		LighthouseTarget: "https://app.example.com",
	}
	d := Decide(p)
	lh := findDecision(t, d, GateLighthouse)
	if lh.Verdict != Applicable {
		t.Errorf("frontend + target: Lighthouse = %s, quero APPLICABLE", lh.Verdict)
	}
}

// Mobile-only ⇒ Lighthouse NOT_APPLICABLE (não é web).
func TestMobileOnlyLighthouseNotApplicable(t *testing.T) {
	p := Profile{
		Components:   []component.Component{component.Mobile},
		ChangedPaths: []string{"apps/mobile/lib/main.dart"},
	}
	d := Decide(p)
	lh := findDecision(t, d, GateLighthouse)
	if lh.Verdict != NotApplicable {
		t.Errorf("mobile-only: Lighthouse = %s, quero NOT_APPLICABLE", lh.Verdict)
	}
}

// Monorepo frontend + backend: Lighthouse roda (frontend presente).
func TestMonorepoFrontendAndBackendLighthouseApplicable(t *testing.T) {
	p := Profile{
		Components: []component.Component{
			component.FrontendWeb, component.BackendAPI,
		},
		ChangedPaths: []string{
			"apps/web/src/App.tsx",
			"apps/api/handlers/user.go",
		},
		LighthouseTarget: "https://app.example.com",
	}
	d := Decide(p)
	lh := findDecision(t, d, GateLighthouse)
	if lh.Verdict != Applicable {
		t.Errorf("monorepo: Lighthouse = %s, quero APPLICABLE", lh.Verdict)
	}
}

// Backend-only com código Go: Sonar APPLICABLE se configurado.
func TestBackendCodeWithSonarConfigured(t *testing.T) {
	p := Profile{
		Components:      []component.Component{component.BackendAPI},
		ChangedPaths:    []string{"apps/api/server.go"},
		SonarConfigured: true,
	}
	d := Decide(p)
	s := findDecision(t, d, GateSonar)
	if s.Verdict != Applicable {
		t.Errorf("backend + sonar: Sonar = %s, quero APPLICABLE", s.Verdict)
	}
}

// Backend-only com código, sem Sonar config: CONDITIONAL.
func TestBackendCodeWithoutSonarConditional(t *testing.T) {
	p := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	d := Decide(p)
	s := findDecision(t, d, GateSonar)
	if s.Verdict != Conditional {
		t.Errorf("backend sem sonar: Sonar = %s, quero CONDITIONAL", s.Verdict)
	}
}

// Range só de docs: Sonar NOT_APPLICABLE.
func TestDocsOnlySonarNotApplicable(t *testing.T) {
	p := Profile{
		Components:   []component.Component{component.Unknown},
		ChangedPaths: []string{"README.md", "docs/intro.md", "CHANGELOG.md"},
	}
	d := Decide(p)
	s := findDecision(t, d, GateSonar)
	if s.Verdict != NotApplicable {
		t.Errorf("docs-only: Sonar = %s, quero NOT_APPLICABLE", s.Verdict)
	}
}

// Range de migrations: Sonar NOT_APPLICABLE (SQL/migrations não contam).
func TestMigrationsOnlySonarNotApplicable(t *testing.T) {
	p := Profile{
		Components:    []component.Component{component.Database},
		ChangedPaths:  []string{"migrations/001_init.sql", "db/schema/users.sql"},
		HasMigrations: true,
	}
	d := Decide(p)
	s := findDecision(t, d, GateSonar)
	if s.Verdict != NotApplicable {
		t.Errorf("migrations-only: Sonar = %s, quero NOT_APPLICABLE", s.Verdict)
	}
}

// Migration detector: APPLICABLE quando HasMigrations.
func TestMigrationApplicableWhenFlagSet(t *testing.T) {
	p := Profile{
		Components:    []component.Component{component.Database},
		HasMigrations: true,
	}
	d := Decide(p)
	m := findDecision(t, d, GateMigration)
	if m.Verdict != Applicable {
		t.Errorf("migrations flag: Migration = %s, quero APPLICABLE", m.Verdict)
	}
}

// Migration detector: NOT_APPLICABLE sem flag.
func TestMigrationNotApplicableByDefault(t *testing.T) {
	p := Profile{Components: []component.Component{component.BackendAPI}}
	d := Decide(p)
	m := findDecision(t, d, GateMigration)
	if m.Verdict != NotApplicable {
		t.Errorf("sem migrations: Migration = %s", m.Verdict)
	}
}

// Env: APPLICABLE quando flag setado.
func TestEnvApplicableWhenFlagSet(t *testing.T) {
	p := Profile{
		Components:    []component.Component{component.BackendAPI},
		HasEnvChanges: true,
	}
	d := Decide(p)
	e := findDecision(t, d, GateEnv)
	if e.Verdict != Applicable {
		t.Errorf("env flag: Env = %s, quero APPLICABLE", e.Verdict)
	}
}

// Security: backend-api presente ⇒ APPLICABLE.
func TestSecurityApplicableForBackend(t *testing.T) {
	p := Profile{Components: []component.Component{component.BackendAPI}}
	d := Decide(p)
	s := findDecision(t, d, GateSecurity)
	if s.Verdict != Applicable {
		t.Errorf("backend: Security = %s, quero APPLICABLE", s.Verdict)
	}
}

// Security: só docs ⇒ CONDITIONAL (reduzida).
func TestSecurityConditionalForDocsOnly(t *testing.T) {
	p := Profile{Components: []component.Component{component.Unknown}}
	d := Decide(p)
	s := findDecision(t, d, GateSecurity)
	if s.Verdict != Conditional {
		t.Errorf("docs-only: Security = %s, quero CONDITIONAL", s.Verdict)
	}
}

// ZAP: backend-api presente sem config ⇒ CONDITIONAL.
func TestZAPConditionalForBackendNoConfig(t *testing.T) {
	p := Profile{Components: []component.Component{component.BackendAPI}}
	d := Decide(p)
	z := findDecision(t, d, GateZAP)
	if z.Verdict != Conditional {
		t.Errorf("backend + ZAP no config: ZAP = %s, quero CONDITIONAL", z.Verdict)
	}
}

// ZAP: backend-api + config ⇒ APPLICABLE.
func TestZAPApplicableForBackendConfigured(t *testing.T) {
	p := Profile{
		Components:    []component.Component{component.BackendAPI},
		ZAPConfigured: true,
	}
	d := Decide(p)
	z := findDecision(t, d, GateZAP)
	if z.Verdict != Applicable {
		t.Errorf("backend + ZAP config: ZAP = %s, quero APPLICABLE", z.Verdict)
	}
}

// ZAP: mobile-only ⇒ NOT_APPLICABLE (sem web/api).
func TestZAPNotApplicableForMobileOnly(t *testing.T) {
	p := Profile{Components: []component.Component{component.Mobile}}
	d := Decide(p)
	z := findDecision(t, d, GateZAP)
	if z.Verdict != NotApplicable {
		t.Errorf("mobile-only: ZAP = %s, quero NOT_APPLICABLE", z.Verdict)
	}
}

// k6: worker presente ⇒ APPLICABLE (se configurado).
func TestK6ApplicableForWorker(t *testing.T) {
	p := Profile{
		Components:   []component.Component{component.Worker},
		K6Configured: true,
	}
	d := Decide(p)
	k := findDecision(t, d, GateK6)
	if k.Verdict != Applicable {
		t.Errorf("worker + k6 config: k6 = %s, quero APPLICABLE", k.Verdict)
	}
}

// k6: mobile-only ⇒ NOT_APPLICABLE.
func TestK6NotApplicableForMobileOnly(t *testing.T) {
	p := Profile{Components: []component.Component{component.Mobile}}
	d := Decide(p)
	k := findDecision(t, d, GateK6)
	if k.Verdict != NotApplicable {
		t.Errorf("mobile-only: k6 = %s, quero NOT_APPLICABLE", k.Verdict)
	}
}

// Tests: código presente ⇒ APPLICABLE.
func TestTestsApplicableWhenCodePresent(t *testing.T) {
	p := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	d := Decide(p)
	ts := findDecision(t, d, GateTests)
	if ts.Verdict != Applicable {
		t.Errorf("código presente: Tests = %s", ts.Verdict)
	}
}

// Tests: só docs ⇒ NOT_APPLICABLE.
func TestTestsNotApplicableForDocsOnly(t *testing.T) {
	p := Profile{
		Components:   []component.Component{component.Unknown},
		ChangedPaths: []string{"README.md"},
	}
	d := Decide(p)
	ts := findDecision(t, d, GateTests)
	if ts.Verdict != NotApplicable {
		t.Errorf("docs-only: Tests = %s", ts.Verdict)
	}
}

// Decide devolve exatamente 8 decisões (uma por gate).
func TestDecideReturnsAllGates(t *testing.T) {
	d := Decide(Profile{})
	if len(d) != 8 {
		t.Errorf("decisões = %d, quero 8", len(d))
	}
	wantGates := []string{
		"sonar", "tests", "security", "lighthouse",
		"zap", "k6", "migration", "env",
	}
	seen := map[Gate]bool{}
	for _, x := range d {
		seen[x.Gate] = true
	}
	for _, g := range wantGates {
		if !seen[Gate(g)] {
			t.Errorf("gate %s ausente", g)
		}
	}
}

// Range vazio ⇒ tudo NOT_APPLICABLE ou CONDITIONAL coerente.
func TestEmptyRangeAllNotApplicable(t *testing.T) {
	p := Profile{Components: []component.Component{component.Unknown}}
	d := Decide(p)
	for _, x := range d {
		if x.Verdict == Applicable {
			t.Errorf("range vazio: %s = %s (deveria não ser Applicable)", x.Gate, x.Verdict)
		}
	}
}

// hasCodeChanges direto: paths variados.
func TestHasCodeChanges(t *testing.T) {
	cases := []struct {
		in   []string
		want bool
	}{
		{nil, false},
		{[]string{}, false},
		{[]string{"README.md", "CHANGELOG.md"}, false},
		{[]string{"migrations/001.sql"}, false},
		{[]string{".env.production"}, false},
		{[]string{"server.go"}, true},
		{[]string{"apps/api/handler.go"}, true},
		{[]string{"main.py", "tests/test_foo.py"}, true},
		{[]string{"src/app.tsx"}, true},
		{[]string{"go.mod"}, true},
	}
	for _, tc := range cases {
		if got := hasCodeChanges(tc.in); got != tc.want {
			t.Errorf("hasCodeChanges(%v) = %v, quero %v", tc.in, got, tc.want)
		}
	}
}
