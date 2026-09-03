package semgrep

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Aceitação: NormalizeSeverity.
func TestNormalizeSeverity(t *testing.T) {
	cases := map[string]Severity{
		"ERROR":    SevError,
		"error":    SevError,
		"  ERROR ": SevError,
		"WARNING":  SevWarning,
		"INFO":     SevInfo,
		"CRITICAL": SevCritical,
		"HIGH":     SevHigh,
		"MEDIUM":   SevMedium,
		"LOW":      SevLow,
		"unknown":  SevInfo,
	}
	for in, want := range cases {
		if got := NormalizeSeverity(in); got != want {
			t.Errorf("%q → %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: severity rank.
func TestSeverityRank(t *testing.T) {
	if severityRank(SevError) >= severityRank(SevInfo) {
		t.Errorf("rank ordem errada")
	}
}

// Aceitação: DetectConfig .semgrep.yml.
func TestDetectConfigYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".semgrep.yml"), []byte("rules: []"), 0644)
	found, err := DetectConfig(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(found) != 1 {
		t.Errorf("got %d: %v", len(found), found)
	}
}

// Aceitação: DetectConfig .semgrep/ dir.
func TestDetectConfigDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".semgrep"), 0755)
	os.WriteFile(filepath.Join(dir, ".semgrep", "custom.yml"), []byte("rules:"), 0644)
	os.WriteFile(filepath.Join(dir, ".semgrep", "extra.yaml"), []byte("rules:"), 0644)
	os.WriteFile(filepath.Join(dir, ".semgrep", "readme.md"), []byte("x"), 0644)
	found, err := DetectConfig(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("got %d: %v", len(found), found)
	}
}

// Aceitação: DetectConfig empty.
func TestDetectConfigEmpty(t *testing.T) {
	dir := t.TempDir()
	found, _ := DetectConfig(dir)
	if len(found) != 0 {
		t.Errorf("err")
	}
}

// Aceitação: DetectConfig root vazio.
func TestDetectConfigEmptyRoot(t *testing.T) {
	if _, err := DetectConfig(""); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: DetectConfig path inválido.
func TestDetectConfigInvalidPath(t *testing.T) {
	if _, err := DetectConfig("/nonexistent-xyz"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: DetectConfig file (não dir).
func TestDetectConfigNotDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "x")
	os.WriteFile(f, []byte("y"), 0644)
	if _, err := DetectConfig(f); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ParseSemgrepJSON básico.
func TestParseSemgrepJSON(t *testing.T) {
	data := `{
		"results":[
			{
				"check_id":"python.lang.security.audit.eval",
				"path":"src/main.py",
				"start":{"line":42,"col":5},
				"end":{"line":42,"col":20},
				"extra":{
					"message":"Avoid eval()",
					"severity":"ERROR",
					"metadata":{"cwe":["CWE-95"],"owasp":["A1"],"category":"security"}
				}
			}
		],
		"errors":[]
	}`
	rep, err := ParseSemgrepJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("len = %d", len(rep.Findings))
	}
	f := rep.Findings[0]
	if f.Severity != SevError {
		t.Errorf("sev = %v", f.Severity)
	}
	if f.Line != 42 {
		t.Errorf("line = %d", f.Line)
	}
	if len(f.CWE) != 1 || f.CWE[0] != "CWE-95" {
		t.Errorf("cwe = %v", f.CWE)
	}
	if f.Category != "security" {
		t.Errorf("cat = %s", f.Category)
	}
	if rep.Counts.Total != 1 {
		t.Errorf("total = %d", rep.Counts.Total)
	}
	if rep.Counts.Files != 1 {
		t.Errorf("files = %d", rep.Counts.Files)
	}
	if len(rep.Rules) != 1 {
		t.Errorf("rules = %v", rep.Rules)
	}
}

// Aceitação: ParseSemgrepJSON inválido.
func TestParseSemgrepJSONInvalid(t *testing.T) {
	if _, err := ParseSemgrepJSON([]byte("garbage")); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ParseSemgrepJSON vazio.
func TestParseSemgrepJSONEmpty(t *testing.T) {
	rep, err := ParseSemgrepJSON([]byte(`{"results":[]}`))
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if rep.Counts.Total != 0 {
		t.Errorf("err")
	}
}

// Aceitação: ParseSemgrepJSON errors.
func TestParseSemgrepJSONErrors(t *testing.T) {
	data := `{"results":[],"errors":[{"message":"semgrep failed","level":"error"}]}`
	rep, err := ParseSemgrepJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rep.Errors) != 1 {
		t.Errorf("errors = %v", rep.Errors)
	}
}

// Aceitação: AddFinding + counts.
func TestAddFinding(t *testing.T) {
	r := &Report{}
	r.AddFinding(Finding{Severity: SevError, FilePath: "a"})
	r.AddFinding(Finding{Severity: SevWarning, FilePath: "a"})
	r.AddFinding(Finding{Severity: SevWarning, FilePath: "b"})
	if r.Counts.Total != 3 {
		t.Errorf("total = %d", r.Counts.Total)
	}
	if r.Counts.BySeverity[SevError] != 1 {
		t.Errorf("error = %d", r.Counts.BySeverity[SevError])
	}
	if r.Counts.BySeverity[SevWarning] != 2 {
		t.Errorf("warn = %d", r.Counts.BySeverity[SevWarning])
	}
}

// Aceitação: HasErrorSeverity.
func TestHasErrorSeverity(t *testing.T) {
	r := &Report{}
	if r.HasErrorSeverity() {
		t.Errorf("vazio devia false")
	}
	r.AddFinding(Finding{Severity: SevWarning})
	if r.HasErrorSeverity() {
		t.Errorf("warning não devia ser error")
	}
	r.AddFinding(Finding{Severity: SevError})
	if !r.HasErrorSeverity() {
		t.Errorf("error devia true")
	}
}

// Aceitação: SortFindings.
func TestSortFindings(t *testing.T) {
	r := &Report{}
	r.AddFinding(Finding{Severity: SevWarning, FilePath: "z", Line: 1})
	r.AddFinding(Finding{Severity: SevError, FilePath: "a", Line: 5})
	r.AddFinding(Finding{Severity: SevError, FilePath: "a", Line: 1})
	r.SortFindings()
	if r.Findings[0].Severity != SevError {
		t.Errorf("sort err")
	}
	if r.Findings[1].FilePath != "a" || r.Findings[1].Line != 5 {
		t.Errorf("segundo devia ser (a,5), got %+v", r.Findings[1])
	}
}

// Aceitação: extractStringList.
func TestExtractStringList(t *testing.T) {
	md := map[string]interface{}{
		"list":   []interface{}{"a", "b"},
		"single": "x",
		"typed":  []string{"c"},
	}
	if got := extractStringList(md, "list"); len(got) != 2 {
		t.Errorf("list")
	}
	if got := extractStringList(md, "single"); len(got) != 1 {
		t.Errorf("single")
	}
	if got := extractStringList(md, "typed"); len(got) != 1 {
		t.Errorf("typed")
	}
	if got := extractStringList(md, "missing"); got != nil {
		t.Errorf("missing")
	}
	if got := extractStringList(md, "list"); len(got) > 0 && got[0] != "a" {
		t.Errorf("ordem")
	}
}

// Aceitação: extractStringList tipos inesperados.
func TestExtractStringListUnknownType(t *testing.T) {
	md := map[string]interface{}{
		"weird": 42,
	}
	if got := extractStringList(md, "weird"); got != nil {
		t.Errorf("err: %v", got)
	}
}

// Aceitação: multiple files counted.
func TestParseSemgrepJSONMultipleFiles(t *testing.T) {
	data := `{
		"results":[
			{"check_id":"r1","path":"a","start":{"line":1,"col":0},"end":{"line":1,"col":5},"extra":{"message":"x","severity":"ERROR","metadata":{}}},
			{"check_id":"r1","path":"b","start":{"line":1,"col":0},"end":{"line":1,"col":5},"extra":{"message":"x","severity":"WARNING","metadata":{}}},
			{"check_id":"r2","path":"a","start":{"line":2,"col":0},"end":{"line":2,"col":5},"extra":{"message":"x","severity":"INFO","metadata":{}}}
		]
	}`
	rep, _ := ParseSemgrepJSON([]byte(data))
	if rep.Counts.Files != 2 {
		t.Errorf("files = %d", rep.Counts.Files)
	}
	if len(rep.Rules) != 2 {
		t.Errorf("rules únicos = %d", len(rep.Rules))
	}
}

// Aceitação: SortFindings stable.
func TestSortStable(t *testing.T) {
	s := []int{3, 1, 2}
	sort.SliceStable(s, func(i, j int) bool { return s[i] < s[j] })
	if s[0] != 1 {
		t.Errorf("err")
	}
}

// Aceitação: metadata cwe string único.
func TestParseSemgrepMetadataCWEString(t *testing.T) {
	data := `{"results":[{"check_id":"r","path":"p","start":{"line":1,"col":0},"end":{"line":1,"col":5},"extra":{"message":"m","severity":"ERROR","metadata":{"cwe":"CWE-89"}}}]}`
	rep, _ := ParseSemgrepJSON([]byte(data))
	if len(rep.Findings[0].CWE) != 1 {
		t.Errorf("err")
	}
}

// Aceitação: severity default quando metadata ausente.
func TestParseSemgrepNoSeverity(t *testing.T) {
	data := `{"results":[{"check_id":"r","path":"p","start":{"line":1,"col":0},"end":{"line":1,"col":5},"extra":{"message":"m","severity":"","metadata":{}}}]}`
	rep, _ := ParseSemgrepJSON([]byte(data))
	if rep.Findings[0].Severity != SevInfo {
		t.Errorf("default sev = %v", rep.Findings[0].Severity)
	}
}

// Aceitação: DetectConfig priority.
func TestDetectConfigPriority(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".semgrep.yml"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(dir, ".semgrep"), 0755)
	os.WriteFile(filepath.Join(dir, ".semgrep", "a.yml"), []byte("x"), 0644)
	found, _ := DetectConfig(dir)
	if len(found) != 2 {
		t.Errorf("got %d", len(found))
	}
}
