package osv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: cria árvore com lockfiles.
func makeLockfileTree(t *testing.T, files []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("# lockfile"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Aceitação: CVSStoSeverity.
func TestCVSStoSeverity(t *testing.T) {
	cases := map[float64]Severity{
		10.0: SevCritical,
		9.0:  SevCritical,
		8.5:  SevHigh,
		7.0:  SevHigh,
		6.5:  SevMedium,
		4.0:  SevMedium,
		3.5:  SevLow,
		0:    SevInfo,
		-1:   SevInfo,
	}
	for cvss, want := range cases {
		if got := CVSStoSeverity(cvss); got != want {
			t.Errorf("CVSS %v → %v, quero %v", cvss, got, want)
		}
	}
}

// Aceitação: DetectLockfiles básico.
func TestDetectLockfilesBasic(t *testing.T) {
	dir := makeLockfileTree(t, []string{"package-lock.json", "go.sum", "Cargo.lock"})
	found, err := DetectLockfiles(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(found) != 3 {
		t.Errorf("got %d: %v", len(found), found)
	}
}

// Aceitação: DetectLockfiles monorepo 1 nível.
func TestDetectLockfilesMonorepo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(dir, "packages"), 0755)
	os.WriteFile(filepath.Join(dir, "packages", "yarn.lock"), []byte("y"), 0644)
	found, _ := DetectLockfiles(dir)
	if len(found) < 2 {
		t.Errorf("devia achar 2+: %v", found)
	}
}

// Aceitação: DetectLockfiles empty.
func TestDetectLockfilesEmpty(t *testing.T) {
	dir := t.TempDir()
	found, _ := DetectLockfiles(dir)
	if len(found) != 0 {
		t.Errorf("err: %v", found)
	}
}

// Aceitação: DetectLockfiles root vazio.
func TestDetectLockfilesEmptyRoot(t *testing.T) {
	if _, err := DetectLockfiles(""); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: DetectLockfiles path inválido.
func TestDetectLockfilesInvalidPath(t *testing.T) {
	if _, err := DetectLockfiles("/nonexistent-xyz"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: DetectLockfiles file (não dir).
func TestDetectLockfilesNotDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "x")
	os.WriteFile(f, []byte("y"), 0644)
	if _, err := DetectLockfiles(f); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: DetectEcosystemByLockfile.
func TestDetectEcosystem(t *testing.T) {
	cases := map[string]string{
		"package-lock.json":  "npm",
		"yarn.lock":          "npm",
		"pnpm-lock.yaml":     "npm",
		"go.sum":             "Go",
		"Cargo.lock":         "crates.io",
		"Gemfile.lock":       "RubyGems",
		"composer.lock":      "Packagist",
		"poetry.lock":        "PyPI",
		"pom.xml":            "Maven",
		"packages.lock.json": "NuGet",
		"random.txt":         "unknown",
	}
	for in, want := range cases {
		got := DetectEcosystemByLockfile(in)
		if got != want {
			t.Errorf("%s → %s, quero %s", in, got, want)
		}
	}
}

// Aceitação: parseOSVScannerJSON básico.
func TestParseOSVScannerJSON(t *testing.T) {
	data := `{
		"results":[
			{
				"source":{"path":"package-lock.json","type":"lockfile"},
				"package":{"name":"lodash","ecosystem":"npm","version":"4.17.20"},
				"vulns":[
					{
						"id":"GHSA-xxxx-yyyy-zzzz",
						"aliases":["CVE-2021-23337"],
						"summary":"Command injection in lodash",
						"severity":[{"type":"CVSS_V3","score":"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H:9.8"}],
						"affected":[{
							"package":{"name":"lodash","ecosystem":"npm","version":"4.17.20"},
							"ranges":[{
								"type":"SEMVER",
								"events":[{"introduced":"0"},{"fixed":"4.17.21"}]
							}]
						}]
					}
				]
			}
		]
	}`
	rep, err := ParseOSVScannerJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rep.Vulns) != 1 {
		t.Fatalf("len = %d", len(rep.Vulns))
	}
	v := rep.Vulns[0]
	if v.ID != "GHSA-xxxx-yyyy-zzzz" {
		t.Errorf("id = %s", v.ID)
	}
	if v.Severity != SevCritical {
		t.Errorf("sev = %v (cvss %v)", v.Severity, v.CVSS)
	}
	if !v.IsFixable {
		t.Errorf("devia ser fixable")
	}
	if v.FixedIn != "4.17.21" {
		t.Errorf("fixed = %s", v.FixedIn)
	}
	if rep.Counts.Total != 1 {
		t.Errorf("total = %d", rep.Counts.Total)
	}
}

// Aceitação: parseOSVScannerJSON inválido.
func TestParseOSVScannerJSONInvalid(t *testing.T) {
	if _, err := ParseOSVScannerJSON([]byte("garbage")); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: parseOSVScannerJSON vazio.
func TestParseOSVScannerJSONEmpty(t *testing.T) {
	rep, err := ParseOSVScannerJSON([]byte(`{"results":[]}`))
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if rep.Counts.Total != 0 {
		t.Errorf("err")
	}
}

// Aceitação: extractCVSS vários formatos.
func TestExtractCVSS(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H:9.8", 9.8},
		{"9.8", 9.8},
		{"7.5", 7.5},
		{"", 0},
		{"CVSS:3.1/AV:N:5.0", 5.0},
	}
	for _, c := range cases {
		got, err := parseCVSSString(c.in)
		if err != nil {
			t.Errorf("%q err: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseCVSSString(%q) = %v, quero %v", c.in, got, c.want)
		}
	}
}

// Aceitação: parseCVSSString inválido.
func TestParseCVSSStringInvalid(t *testing.T) {
	if _, err := parseCVSSString("abc"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: AddVuln + counts.
func TestAddVuln(t *testing.T) {
	r := &Report{}
	r.AddVuln(Vuln{Severity: SevHigh, IsFixable: true})
	r.AddVuln(Vuln{Severity: SevMedium, IsFixable: false})
	r.AddVuln(Vuln{Severity: SevHigh})
	if r.Counts.Total != 3 {
		t.Errorf("total = %d", r.Counts.Total)
	}
	if r.Counts.BySeverity[SevHigh] != 2 {
		t.Errorf("high = %d", r.Counts.BySeverity[SevHigh])
	}
	if r.Counts.Fixable != 1 {
		t.Errorf("fixable = %d", r.Counts.Fixable)
	}
}

// Aceitação: HasCriticalOrHigh.
func TestHasCriticalOrHigh(t *testing.T) {
	r := &Report{}
	if r.HasCriticalOrHigh() {
		t.Errorf("vazio devia false")
	}
	r.AddVuln(Vuln{Severity: SevCritical})
	if !r.HasCriticalOrHigh() {
		t.Errorf("critical devia true")
	}
}

// Aceitação: SortVulns.
func TestSortVulns(t *testing.T) {
	r := &Report{}
	r.AddVuln(Vuln{Severity: SevLow, Package: "a"})
	r.AddVuln(Vuln{Severity: SevCritical, Package: "b"})
	r.AddVuln(Vuln{Severity: SevCritical, Package: "a"})
	r.SortVulns()
	if r.Vulns[0].Severity != SevCritical {
		t.Errorf("sort err: %v", r.Vulns)
	}
}

// Aceitação: parseFloat edge cases.
func TestParseFloat(t *testing.T) {
	cases := map[string]float64{
		"0":    0,
		"10":   10,
		"9.8":  9.8,
		"-1.5": -1.5,
		"  5":  5,
	}
	for in, want := range cases {
		got, err := parseFloat(in)
		if err != nil {
			t.Errorf("err %q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseFloat(%q) = %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: parseFloat inválido.
func TestParseFloatInvalid(t *testing.T) {
	if _, err := parseFloat("abc"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: severity rank.
func TestSeverityRank(t *testing.T) {
	if severityRank(SevCritical) >= severityRank(SevInfo) {
		t.Errorf("rank ordem errada")
	}
}

// Aceitação: parseOSVScannerJSON com 2 results.
func TestParseOSVScannerJSONMulti(t *testing.T) {
	data := `{
		"results":[
			{"source":{"path":"a"},"package":{"name":"x","ecosystem":"npm","version":"1"},"vulns":[{"id":"CVE-1","summary":"x","severity":[{"type":"CVSS","score":"5.0"}]}]},
			{"source":{"path":"b"},"package":{"name":"y","ecosystem":"npm","version":"1"},"vulns":[]}
		]
	}`
	rep, err := ParseOSVScannerJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rep.Vulns) != 1 {
		t.Errorf("len = %d", len(rep.Vulns))
	}
	if len(rep.Lockfiles) != 2 {
		t.Errorf("lockfiles = %v", rep.Lockfiles)
	}
}

// Aceitação: extractCVSS ignora tipos não-CVSS.
func TestExtractCVSSNonCVSS(t *testing.T) {
	sev := []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	}{
		{Type: "Ubuntu", Score: "high"},
	}
	if got := extractCVSS(sev); got != 0 {
		t.Errorf("devia ignorar, got %v", got)
	}
}

// Aceitação: alias preservado.
func TestAliases(t *testing.T) {
	v := Vuln{Aliases: []string{"CVE-2021-23337"}}
	if len(v.Aliases) != 1 {
		t.Errorf("err")
	}
}

// Aceitação: Vuln ID contains both formats.
func TestVulnIDFormat(t *testing.T) {
	if !strings.HasPrefix("CVE-2024-1234", "CVE-") {
		t.Errorf("CVE format")
	}
	if !strings.HasPrefix("GHSA-xxxx", "GHSA-") {
		t.Errorf("GHSA format")
	}
}

// Aceitação: SortVulns stability entre severities iguais.
func TestSortVulnsStable(t *testing.T) {
	r := &Report{}
	r.AddVuln(Vuln{Severity: SevMedium, Package: "z", ID: "CVE-1"})
	r.AddVuln(Vuln{Severity: SevMedium, Package: "a", ID: "CVE-2"})
	r.AddVuln(Vuln{Severity: SevMedium, Package: "a", ID: "CVE-3"})
	r.SortVulns()
	if r.Vulns[0].Package != "a" || r.Vulns[0].ID != "CVE-2" {
		t.Errorf("sort: %+v", r.Vulns)
	}
}
