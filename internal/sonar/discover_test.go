package sonar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: cria árvore.
func makeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Aceitação: parseProperties básico.
func TestParsePropertiesBasic(t *testing.T) {
	content := `
# comment
sonar.host.url=https://sonar.example.com
sonar.projectKey=my-project
sonar.sources=src,lib
`
	out, err := parseProperties(content)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out["sonar.host.url"] != "https://sonar.example.com" {
		t.Errorf("got %v", out)
	}
	if out["sonar.sources"] != "src,lib" {
		t.Errorf("sources = %s", out["sonar.sources"])
	}
}

// Aceitação: parseProperties com =.
func TestParsePropertiesEquals(t *testing.T) {
	out, _ := parseProperties("sonar.token=abc")
	if out["sonar.token"] != "abc" {
		t.Errorf("err: %v", out)
	}
}

// Aceitação: parseProperties com :.
func TestParsePropertiesColon(t *testing.T) {
	out, _ := parseProperties("sonar.token:abc")
	if out["sonar.token"] != "abc" {
		t.Errorf("err: %v", out)
	}
}

// Aceitação: parseProperties comments.
func TestParsePropertiesComments(t *testing.T) {
	content := `# comment
! also comment
sonar.host.url=https://x
`
	out, _ := parseProperties(content)
	if out["sonar.host.url"] != "https://x" {
		t.Errorf("err")
	}
	if len(out) != 1 {
		t.Errorf("len = %d", len(out))
	}
}

// Aceitação: parseProperties line sem =.
func TestParsePropertiesNoEq(t *testing.T) {
	out, _ := parseProperties("invalidline")
	if len(out) != 0 {
		t.Errorf("err: %v", out)
	}
}

// Aceitação: splitComma.
func TestSplitComma(t *testing.T) {
	cases := map[string][]string{
		"a,b,c":   {"a", "b", "c"},
		" a , b ": {"a", "b"},
		"":        {},
		"single":  {"single"},
	}
	for in, want := range cases {
		got := splitComma(in)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("splitComma(%q) = %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: Discover properties básico.
func TestDiscoverProperties(t *testing.T) {
	props := `sonar.host.url=https://sonar.example.com
sonar.projectKey=myproj
sonar.token=secrettoken
sonar.sources=src
sonar.coverageReportPaths=coverage.xml
`
	dir := makeTree(t, map[string]string{"sonar-project.properties": props})
	d := New(dir)
	cfg, found, err := d.Discover()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !found {
		t.Errorf("not found")
	}
	if cfg.HostURL != "https://sonar.example.com" {
		t.Errorf("host = %s", cfg.HostURL)
	}
	if cfg.ProjectKey != "myproj" {
		t.Errorf("project = %s", cfg.ProjectKey)
	}
	if cfg.Source != "properties" {
		t.Errorf("source = %s", cfg.Source)
	}
}

// Aceitação: Discover override tem precedência.
func TestDiscoverOverridePrecedence(t *testing.T) {
	props := `sonar.host.url=https://sonar.example.com
sonar.projectKey=old
`
	override := `{"project_key": "new", "organization": "myorg"}`
	dir := makeTree(t, map[string]string{
		"sonar-project.properties": props,
		".solidify/sonar.json":      override,
	})
	d := New(dir)
	cfg, _, _ := d.Discover()
	if cfg.ProjectKey != "new" {
		t.Errorf("override devia ganhar: %s", cfg.ProjectKey)
	}
	if cfg.Source != "override" {
		t.Errorf("source = %s", cfg.Source)
	}
}

// Aceitação: Discover env ref.
func TestDiscoverTokenEnvRef(t *testing.T) {
	props := `sonar.host.url=https://sonar.example.com
sonar.projectKey=p
sonar.token=env:SONAR_TOKEN
`
	dir := makeTree(t, map[string]string{"sonar-project.properties": props})
	d := New(dir)
	cfg, _, _ := d.Discover()
	if cfg.TokenRef != "env:SONAR_TOKEN" {
		t.Errorf("TokenRef = %s", cfg.TokenRef)
	}
	if cfg.Token != "" {
		t.Errorf("Token devia estar vazio (sem resolver)")
	}
}

// Aceitação: Discover env fall-back.
func TestDiscoverEnvFallback(t *testing.T) {
	os.Setenv("SONAR_HOST_URL", "https://sonarcloud.io")
	defer os.Unsetenv("SONAR_HOST_URL")
	dir := t.TempDir()
	d := New(dir)
	cfg, found, _ := d.Discover()
	if !found || cfg.HostURL != "https://sonarcloud.io" {
		t.Errorf("err: %+v", cfg)
	}
	if cfg.Source != "env" {
		t.Errorf("source = %s", cfg.Source)
	}
}

// Aceitação: Discover empty root.
func TestDiscoverEmptyRoot(t *testing.T) {
	d := New("")
	if _, _, err := d.Discover(); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Discover nothing.
func TestDiscoverNothing(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)
	_, found, _ := d.Discover()
	if found {
		t.Errorf("devia ser not found")
	}
}

// Aceitação: EnvRefs coleta refs.
func TestEnvRefs(t *testing.T) {
	cfg := &Config{
		TokenRef: "env:SONAR_TOKEN",
		Extra: map[string]string{
			"sonar.foo": "env:FOO_VAR",
			"sonar.bar": "literal",
		},
	}
	d := New("/tmp")
	refs := d.EnvRefs(cfg)
	if len(refs) != 2 {
		t.Errorf("len = %d", len(refs))
	}
}

// Aceitação: EnvRefs empty.
func TestEnvRefsEmpty(t *testing.T) {
	d := New("/tmp")
	if refs := d.EnvRefs(&Config{}); len(refs) != 0 {
		t.Errorf("err")
	}
}

// Aceitação: ResolveEnv.
func TestResolveEnv(t *testing.T) {
	os.Setenv("SOLIDIFY_TEST_SONAR", "secretvalue")
	defer os.Unsetenv("SOLIDIFY_TEST_SONAR")
	cfg := &Config{TokenRef: "env:SOLIDIFY_TEST_SONAR"}
	d := New("/tmp")
	d.ResolveEnv(cfg)
	if cfg.Token != "secretvalue" {
		t.Errorf("Token = %s", cfg.Token)
	}
}

// Aceitação: HasMinimumConfig.
func TestHasMinimumConfig(t *testing.T) {
	cases := []struct {
		cfg  Config
		want bool
	}{
		{Config{}, false},
		{Config{HostURL: "x"}, false},
		{Config{HostURL: "x", Token: "t"}, false},
		{Config{HostURL: "x", Token: "t", ProjectKey: "p"}, true},
		{Config{HostURL: "x", TokenRef: "env:T", ProjectKey: "p"}, true},
	}
	for _, c := range cases {
		if got := c.cfg.HasMinimumConfig(); got != c.want {
			t.Errorf("got %v, quero %v", got, c.want)
		}
	}
}

// Aceitação: IsCloud.
func TestIsCloud(t *testing.T) {
	if !(&Config{HostURL: "https://sonarcloud.io"}).IsCloud() {
		t.Errorf("devia ser cloud")
	}
	if (&Config{HostURL: "https://sonar.example.com"}).IsCloud() {
		t.Errorf("devia ser server")
	}
}

// Aceitação: Override JSON inválido.
func TestDiscoverInvalidOverride(t *testing.T) {
	dir := makeTree(t, map[string]string{
		".solidify/sonar.json": "{invalid",
	})
	d := New(dir)
	if _, _, err := d.Discover(); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: applyOverride extras.
func TestApplyOverrideExtras(t *testing.T) {
	ov := Override{Extra: map[string]string{"sonar.custom": "value"}}
	cfg := applyOverride(Config{}, ov)
	if cfg.Extra["sonar.custom"] != "value" {
		t.Errorf("err")
	}
}

// Aceitação: Discover arrays.
func TestDiscoverArrays(t *testing.T) {
	props := `sonar.host.url=https://x
sonar.projectKey=p
sonar.token=t
sonar.sources=src,lib,test
sonar.exclusions=**/*.test.ts
sonar.coverageReportPaths=coverage.xml,lcov.info
sonar.test.reportPaths=junit.xml
`
	dir := makeTree(t, map[string]string{"sonar-project.properties": props})
	d := New(dir)
	cfg, _, _ := d.Discover()
	if len(cfg.Sources) != 3 {
		t.Errorf("sources = %v", cfg.Sources)
	}
	if len(cfg.Exclusions) != 1 {
		t.Errorf("exclusions = %v", cfg.Exclusions)
	}
	if len(cfg.CovPaths) != 2 {
		t.Errorf("cov = %v", cfg.CovPaths)
	}
}

// Aceitação: Discover extras.
func TestDiscoverExtras(t *testing.T) {
	props := `sonar.host.url=https://x
sonar.projectKey=p
sonar.token=t
sonar.branch.name=feature/foo
sonar.qualitygate.wait=true
`
	dir := makeTree(t, map[string]string{"sonar-project.properties": props})
	d := New(dir)
	cfg, _, _ := d.Discover()
	if cfg.Extra["sonar.branch.name"] != "feature/foo" {
		t.Errorf("branch.name = %v", cfg.Extra)
	}
	if cfg.Extra["sonar.qualitygate.wait"] != "true" {
		t.Errorf("qg = %v", cfg.Extra)
	}
}

// Aceitação: Override JSON completo.
func TestOverrideJSONFull(t *testing.T) {
	ov := Override{
		HostURL:    "https://x",
		TokenRef:   "env:TOK",
		ProjectKey: "p",
		Sources:    []string{"src"},
		Extra:      map[string]string{"sonar.x": "y"},
	}
	data, _ := json.Marshal(ov)
	if !strings.Contains(string(data), `"host_url"`) {
		t.Errorf("err: %s", data)
	}
}
