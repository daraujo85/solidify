package sonar

import (
	"sort"
	"testing"
)

// Aceitação: severity Sonar → interna.
func TestNormalizeSeverity(t *testing.T) {
	cases := map[string]Severity{
		"BLOCKER":  SevCritical,
		"CRITICAL": SevHigh,
		"MAJOR":    SevMedium,
		"MINOR":    SevLow,
		"INFO":     SevInfo,
		"unknown":  SevInfo,
		"":         SevInfo,
		"  MAJOR ": SevMedium,
	}
	for in, want := range cases {
		if got := NormalizeSeverity(in); got != want {
			t.Errorf("NormalizeSeverity(%q) = %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: severity rank.
func TestSeverityRank(t *testing.T) {
	if severityRank(SevCritical) >= severityRank(SevInfo) {
		t.Errorf("rank ordem errada")
	}
}

// Aceitação: stripProjectPrefix com prefixo.
func TestStripProjectPrefix(t *testing.T) {
	if got := stripProjectPrefix("myproj:src/foo.go", "myproj"); got != "src/foo.go" {
		t.Errorf("got %q", got)
	}
}

// Aceitação: stripProjectPrefix sem prefixo.
func TestStripProjectPrefixNone(t *testing.T) {
	if got := stripProjectPrefix("src/foo.go", "myproj"); got != "src/foo.go" {
		t.Errorf("got %q", got)
	}
}

// Aceitação: stripProjectPrefix vazio.
func TestStripProjectPrefixEmpty(t *testing.T) {
	if got := stripProjectPrefix("", "myproj"); got != "" {
		t.Errorf("got %q", got)
	}
}

// Aceitação: stripProjectPrefix fallback.
func TestStripProjectPrefixFallback(t *testing.T) {
	// tem ":" mas prefixo não bate — pega último segmento.
	if got := stripProjectPrefix("other:src/foo.go", "myproj"); got != "src/foo.go" {
		t.Errorf("got %q", got)
	}
}

// Aceitação: normalizePath.
func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"./src/foo.go": "src/foo.go",
		"src/foo.go":   "src/foo.go",
		"  src/x.go ":  "src/x.go",
	}
	for in, want := range cases {
		if got := normalizePath(in); got != want {
			t.Errorf("normalizePath(%q) = %q, quero %q", in, got, want)
		}
	}
}

// Aceitação: parseFloat.
func TestParseFloat(t *testing.T) {
	cases := map[string]float64{
		"42":     42,
		"42.5":   42.5,
		"  10  ": 10,
		"0":      0,
		"-3.14":  -3.14,
		"":       0,
	}
	for in, want := range cases {
		got, err := parseFloat(in)
		if err != nil {
			t.Errorf("parseFloat(%q) err: %v", in, err)
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

// Aceitação: NormalizeIssues básico + sort.
func TestNormalizeIssuesBasic(t *testing.T) {
	refs := []IssueRef{
		{Key: "1", Severity: "MINOR", FilePath: "p:src/foo.go", Status: "OPEN"},
		{Key: "2", Severity: "BLOCKER", FilePath: "p:src/bar.go", Status: "OPEN"},
		{Key: "3", Severity: "MAJOR", FilePath: "p:src/foo.go", Status: "OPEN"},
	}
	issues := NormalizeIssues(refs, "p", nil)
	if len(issues) != 3 {
		t.Fatalf("len = %d", len(issues))
	}
	if issues[0].Severity != SevCritical {
		t.Errorf("primeiro devia ser critical")
	}
}

// Aceitação: scope diff quando path bate.
func TestNormalizeIssuesScopeDiff(t *testing.T) {
	refs := []IssueRef{
		{Key: "1", Severity: "MAJOR", FilePath: "p:src/foo.go", Status: "OPEN"},
		{Key: "2", Severity: "MAJOR", FilePath: "p:src/bar.go", Status: "OPEN"},
	}
	diffPaths := []string{"src/foo.go"}
	issues := NormalizeIssues(refs, "p", diffPaths)
	byKey := map[string]Scope{}
	for _, i := range issues {
		byKey[i.Key] = i.Scope
	}
	if byKey["1"] != ScopeDiff {
		t.Errorf("key 1 scope = %v", byKey["1"])
	}
	if byKey["2"] != ScopeUnknown {
		t.Errorf("key 2 scope = %v", byKey["2"])
	}
}

// Aceitação: scope project-wide quando status resolved/closed.
func TestNormalizeIssuesScopeResolved(t *testing.T) {
	refs := []IssueRef{
		{Key: "1", Severity: "MAJOR", FilePath: "p:src/foo.go", Status: "RESOLVED"},
		{Key: "2", Severity: "MAJOR", FilePath: "p:src/foo.go", Status: "CLOSED"},
	}
	issues := NormalizeIssues(refs, "p", []string{"src/foo.go"})
	for _, i := range issues {
		if i.Scope == ScopeDiff {
			t.Errorf("resolved/closed devia ser project-wide")
		}
	}
}

// Aceitação: scope project-wide quando filePath vazio.
func TestNormalizeIssuesEmptyFile(t *testing.T) {
	refs := []IssueRef{
		{Key: "1", Severity: "MAJOR", FilePath: "", Status: "OPEN"},
	}
	issues := NormalizeIssues(refs, "p", []string{"src/foo.go"})
	if issues[0].Scope != ScopeProjectWide {
		t.Errorf("scope = %v", issues[0].Scope)
	}
}

// Aceitação: NormalizeMeasures parse value.
func TestNormalizeMeasures(t *testing.T) {
	in := []Measure{
		{Metric: "coverage", Value: "85.5"},
		{Metric: "bugs", Value: "12"},
	}
	out := NormalizeMeasures(in)
	if len(out) != 2 {
		t.Fatalf("len = %d", len(out))
	}
	// sorted alphabetically: bugs, coverage.
	var cov, bugs MeasureN
	for _, m := range out {
		switch m.Key {
		case "coverage":
			cov = m
		case "bugs":
			bugs = m
		}
	}
	if cov.Value != 85.5 {
		t.Errorf("coverage = %v", cov.Value)
	}
	if bugs.Value != 12 {
		t.Errorf("bugs = %v", bugs.Value)
	}
}

// Aceitação: FindMeasure.
func TestFindMeasure(t *testing.T) {
	ms := []MeasureN{{Key: "coverage", Value: 80}, {Key: "bugs", Value: 5}}
	if _, ok := FindMeasure(ms, "coverage"); !ok {
		t.Errorf("devia achar")
	}
	if _, ok := FindMeasure(ms, "missing"); ok {
		t.Errorf("não devia achar")
	}
}

// Aceitação: NormalizeQualityGate.
func TestNormalizeQualityGate(t *testing.T) {
	qg := QualityGateStatus{}
	qg.ProjectStatus.Status = "ERROR"
	qg.ProjectStatus.Conditions = []QGCondition{
		{Metric: "coverage", Comparator: "LT", Error: "80", Actual: "50", Status: "ERROR"},
	}
	out := NormalizeQualityGate(qg)
	if out.Status != QGError {
		t.Errorf("status = %v", out.Status)
	}
	if out.Conditions[0].Threshold != 80 {
		t.Errorf("thr = %v", out.Conditions[0].Threshold)
	}
	if out.Conditions[0].Actual != 50 {
		t.Errorf("act = %v", out.Conditions[0].Actual)
	}
}

// Aceitação: NormalizeQualityGate lowercase → upper.
func TestNormalizeQualityGateCase(t *testing.T) {
	qg := QualityGateStatus{}
	qg.ProjectStatus.Status = "ok"
	if NormalizeQualityGate(qg).Status != QGOK {
		t.Errorf("case")
	}
}

// Aceitação: BuildReport counts.
func TestBuildReportCounts(t *testing.T) {
	issues := []Issue{
		{Severity: SevCritical, Scope: ScopeDiff, Type: "BUG"},
		{Severity: SevHigh, Scope: ScopeProjectWide, Type: "VULNERABILITY"},
		{Severity: SevHigh, Scope: ScopeDiff, Type: "BUG"},
		{Severity: SevLow, Scope: ScopeUnknown, Type: "CODE_SMELL"},
	}
	r := BuildReport("h", "p", nil, issues, nil)
	if r.Counts.Total != 4 {
		t.Errorf("total = %d", r.Counts.Total)
	}
	if r.Counts.BySeverity[SevCritical] != 1 {
		t.Errorf("critical = %d", r.Counts.BySeverity[SevCritical])
	}
	if r.Counts.BySeverity[SevHigh] != 2 {
		t.Errorf("high = %d", r.Counts.BySeverity[SevHigh])
	}
	if r.Counts.ByScope[ScopeDiff] != 2 {
		t.Errorf("diff = %d", r.Counts.ByScope[ScopeDiff])
	}
	if r.Counts.ByType["BUG"] != 2 {
		t.Errorf("bug = %d", r.Counts.ByType["BUG"])
	}
}

// Aceitação: DiffIssues.
func TestDiffIssues(t *testing.T) {
	issues := []Issue{
		{Key: "1", Scope: ScopeDiff},
		{Key: "2", Scope: ScopeProjectWide},
		{Key: "3", Scope: ScopeDiff},
	}
	r := BuildReport("", "", nil, issues, nil)
	diff := r.DiffIssues()
	if len(diff) != 2 {
		t.Errorf("len = %d", len(diff))
	}
}

// Aceitação: BlockingIssues.
func TestBlockingIssues(t *testing.T) {
	issues := []Issue{
		{Key: "1", Severity: SevCritical, Status: "OPEN"},
		{Key: "2", Severity: SevHigh, Status: "OPEN"},
		{Key: "3", Severity: SevMedium, Status: "OPEN"},
		{Key: "4", Severity: SevCritical, Status: "RESOLVED"},
		{Key: "5", Severity: SevCritical, Status: "REOPENED"},
	}
	r := BuildReport("", "", nil, issues, nil)
	b := r.BlockingIssues()
	if len(b) != 3 {
		t.Errorf("len = %d (esperava 1,2,5)", len(b))
	}
}

// Aceitação: empty report.
func TestBuildReportEmpty(t *testing.T) {
	r := BuildReport("", "", nil, nil, nil)
	if r.Counts.Total != 0 {
		t.Errorf("err")
	}
	if len(r.DiffIssues()) != 0 {
		t.Errorf("err")
	}
}

// Aceitação: Sort estável por key quando severity igual.
func TestNormalizeIssuesStableSort(t *testing.T) {
	refs := []IssueRef{
		{Key: "b", Severity: "MAJOR"},
		{Key: "a", Severity: "MAJOR"},
		{Key: "c", Severity: "MAJOR"},
	}
	issues := NormalizeIssues(refs, "p", nil)
	keys := []string{issues[0].Key, issues[1].Key, issues[2].Key}
	want := []string{"a", "b", "c"}
	if keys[0] != want[0] || keys[1] != want[1] || keys[2] != want[2] {
		t.Errorf("got %v want %v", keys, want)
	}
}

// Aceitação: blocking issues empty status considered open.
func TestBlockingIssuesEmptyStatus(t *testing.T) {
	r := BuildReport("", "", nil, []Issue{{Severity: SevCritical}}, nil)
	if len(r.BlockingIssues()) != 1 {
		t.Errorf("status vazio devia contar como open")
	}
}

// Aceitação: NormalizeMeasures sort por key.
func TestNormalizeMeasuresSort(t *testing.T) {
	in := []Measure{
		{Metric: "bugs"},
		{Metric: "coverage"},
		{Metric: "alert_status"},
	}
	out := NormalizeMeasures(in)
	want := []string{"alert_status", "bugs", "coverage"}
	got := []string{out[0].Key, out[1].Key, out[2].Key}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sort err: %v", got)
		}
	}
}

// Aceitação: Counts zero initialized.
func TestCountsZeroInit(t *testing.T) {
	c := Counts{
		BySeverity: make(map[Severity]int),
		ByScope:    make(map[Scope]int),
		ByType:     make(map[string]int),
	}
	if c.BySeverity[SevCritical] != 0 {
		t.Errorf("err")
	}
}

// Aceitação: Sort stability helper.
func TestSortStable(t *testing.T) {
	s := []int{3, 1, 2}
	sort.SliceStable(s, func(i, j int) bool { return s[i] < s[j] })
	if s[0] != 1 || s[1] != 2 || s[2] != 3 {
		t.Errorf("err")
	}
}
