package testexec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Aceitação: Execute com sucesso.
func TestExecuteOK(t *testing.T) {
	if _, err := os.Stat("/bin/echo"); err != nil {
		t.Skip("echo não encontrado")
	}
	e := NewExecutor()
	e.Timeout = 5 * time.Second
	out, err := e.Execute(context.Background(), Command{Bin: "/bin/echo", Args: []string{"hi"}}, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.Passed {
		t.Errorf("devia passar: %+v", out)
	}
	if out.ExitCode != 0 {
		t.Errorf("exit = %d", out.ExitCode)
	}
}

// Aceitação: Execute com exit != 0 → fail.
func TestExecuteFail(t *testing.T) {
	if _, err := os.Stat("/bin/false"); err != nil {
		t.Skip("false não encontrado")
	}
	e := NewExecutor()
	out, _ := e.Execute(context.Background(), Command{Bin: "/bin/false"}, "")
	if out.Passed {
		t.Errorf("devia falhar")
	}
	if out.ExitCode == 0 {
		t.Errorf("exit = 0")
	}
}

// Aceitação: Outcome.Summary.
func TestOutcomeSummary(t *testing.T) {
	o := Outcome{Total: 10, PassedN: 7, FailedN: 2, SkippedN: 1, Duration: 5 * time.Second}
	s := o.Summary()
	if !strings.Contains(s, "7/10") {
		t.Errorf("got %s", s)
	}
}

// Aceitação: Outcome.HasFailures.
func TestOutcomeHasFailures(t *testing.T) {
	if !(Outcome{FailedN: 1}).HasFailures() {
		t.Errorf("devia ser true")
	}
	if !(Outcome{ErroredN: 1}).HasFailures() {
		t.Errorf("devia ser true")
	}
	if (Outcome{PassedN: 5}).HasFailures() {
		t.Errorf("devia ser false")
	}
}

// Aceitação: Outcome.PassedRatio.
func TestOutcomePassedRatio(t *testing.T) {
	if (Outcome{}).PassedRatio() != 0 {
		t.Errorf("zero total devia dar 0")
	}
	r := (Outcome{Total: 10, PassedN: 8}).PassedRatio()
	if r != 0.8 {
		t.Errorf("got = %v", r)
	}
}

// Aceitação: applyJUnit extrai counts.
func TestApplyJUnit(t *testing.T) {
	xml := `<?xml version="1.0"?>
<testsuite name="t" tests="5" failures="1" errors="1" skipped="1" time="2.5">
  <testcase name="a"/>
  <testcase name="b"/>
</testsuite>`
	o := applyJUnit(Outcome{}, []byte(xml))
	if o.Total != 5 || o.FailedN != 1 || o.ErroredN != 1 || o.SkippedN != 1 {
		t.Errorf("counts = %+v", o)
	}
	if o.PassedN != 2 {
		t.Errorf("passed = %d, quero 2", o.PassedN)
	}
}

// Aceitação: applyJUnit multi-suite.
func TestApplyJUnitMultiSuite(t *testing.T) {
	xml := `<?xml version="1.0"?>
<testsuites>
  <testsuite name="a" tests="2" failures="0" errors="0" skipped="0"/>
  <testsuite name="b" tests="3" failures="1" errors="0" skipped="1"/>
</testsuites>`
	o := applyJUnit(Outcome{}, []byte(xml))
	if o.Total != 5 || o.FailedN != 1 || o.SkippedN != 1 {
		t.Errorf("got %+v", o)
	}
}

// Aceitação: applyJUnit XML inválido ignora.
func TestApplyJUnitInvalid(t *testing.T) {
	o := applyJUnit(Outcome{}, []byte("not xml"))
	if o.Total != 0 {
		t.Errorf("devia ignorar")
	}
}

// Aceitação: ParseJUnit extrai suites e cases.
func TestParseJUnit(t *testing.T) {
	xml := `<?xml version="1.0"?>
<testsuite name="s1" tests="2" failures="1" errors="0" skipped="0">
  <testcase name="a" time="0.1"/>
  <testcase name="b" time="0.2"><failure>boom</failure></testcase>
</testsuite>`
	suites, err := ParseJUnit([]byte(xml))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(suites) != 1 || suites[0].Name != "s1" {
		t.Errorf("got %+v", suites)
	}
	if len(suites[0].Cases) != 2 {
		t.Errorf("cases = %d", len(suites[0].Cases))
	}
	if suites[0].Cases[1].Failure == nil {
		t.Errorf("failure não extraída")
	}
}

// Aceitação: ParseJUnit root testsuites.
func TestParseJUnitRootSuites(t *testing.T) {
	xml := `<?xml version="1.0"?>
<testsuites>
  <testsuite name="s1" tests="1" failures="0" errors="0" skipped="0"/>
  <testsuite name="s2" tests="1" failures="0" errors="0" skipped="0"/>
</testsuites>`
	suites, _ := ParseJUnit([]byte(xml))
	if len(suites) != 2 {
		t.Errorf("len = %d", len(suites))
	}
}

// Aceitação: ParseJUnit inválido.
func TestParseJUnitInvalid(t *testing.T) {
	if _, err := ParseJUnit([]byte("garbage")); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Merge.
func TestMergeOutcomes(t *testing.T) {
	os := []Outcome{
		{Total: 5, PassedN: 5, Passed: true, Duration: time.Second},
		{Total: 3, PassedN: 2, FailedN: 1, Passed: false, Duration: 500 * time.Millisecond},
	}
	m := Merge(os)
	if m.Total != 8 || m.PassedN != 7 || m.FailedN != 1 {
		t.Errorf("merge = %+v", m)
	}
	if m.Passed {
		t.Errorf("merged devia ser fail")
	}
	if m.Duration != 1500*time.Millisecond {
		t.Errorf("duration = %v", m.Duration)
	}
}

// Aceitação: Merge empty.
func TestMergeEmpty(t *testing.T) {
	m := Merge(nil)
	if m.Total != 0 {
		t.Errorf("vazio = %+v", m)
	}
}

// Aceitação: Execute lê JUnit file.
func TestExecuteWithReport(t *testing.T) {
	if _, err := os.Stat("/bin/echo"); err != nil {
		t.Skip("echo não encontrado")
	}
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "junit.xml")
	xml := `<?xml version="1.0"?>
<testsuite name="t" tests="3" failures="0" errors="0" skipped="0"/>`
	os.WriteFile(reportPath, []byte(xml), 0644)
	e := NewExecutor()
	out, _ := e.Execute(context.Background(), Command{Bin: "/bin/echo"}, reportPath)
	if out.Total != 3 {
		t.Errorf("total = %d", out.Total)
	}
	if out.Report != reportPath {
		t.Errorf("report = %s", out.Report)
	}
}

// Aceitação: ReportDir encontra junit.xml.
func TestExecuteReportDir(t *testing.T) {
	if _, err := os.Stat("/bin/echo"); err != nil {
		t.Skip("echo não encontrado")
	}
	dir := t.TempDir()
	xml := `<?xml version="1.0"?>
<testsuite name="t" tests="2" failures="0" errors="0" skipped="0"/>`
	os.WriteFile(filepath.Join(dir, "junit.xml"), []byte(xml), 0644)
	e := NewExecutor()
	e.ReportDir = dir
	out, _ := e.Execute(context.Background(), Command{Bin: "/bin/echo"}, "")
	if out.Total != 2 {
		t.Errorf("total = %d", out.Total)
	}
}

// Aceitação: Outcome.Passed false quando exit != 0.
func TestOutcomePassedExitCode(t *testing.T) {
	o := Outcome{ExitCode: 1, PassedN: 5, Total: 5}
	if o.Passed {
		t.Errorf("exit != 0 devia ser fail")
	}
}

// Aceitação: Outcome.Passed false quando TimedOut.
func TestOutcomePassedTimedOut(t *testing.T) {
	o := Outcome{ExitCode: 0, PassedN: 5, Total: 5, TimedOut: true}
	if o.Passed {
		t.Errorf("timedout devia ser fail")
	}
}

// Aceitação: Outcome.Passed false quando FailedN > 0.
func TestOutcomePassedFailures(t *testing.T) {
	o := Outcome{ExitCode: 0, Total: 5, PassedN: 4, FailedN: 1}
	if o.Passed {
		t.Errorf("failed devia ser fail")
	}
}

// Aceitação: firstNonEmpty.
func TestFirstNonEmpty(t *testing.T) {
	if firstNonEmpty("", "x", "y") != "x" {
		t.Errorf("err")
	}
	if firstNonEmpty() != "" {
		t.Errorf("err")
	}
}

// Aceitação: containsAny.
func TestContainsAny(t *testing.T) {
	if !containsAny("hello world", "world", "nope") {
		t.Errorf("err")
	}
	if containsAny("hello", "nope") {
		t.Errorf("err")
	}
}

// Aceitação: ensureStdoutTrimmed.
func TestEnsureStdoutTrimmed(t *testing.T) {
	if got := ensureStdoutTrimmed("hi", 10); got != "hi" {
		t.Errorf("curto: %q", got)
	}
	if got := ensureStdoutTrimmed(strings.Repeat("a", 100), 10); !strings.Contains(got, "truncated") {
		t.Errorf("longo: %q", got)
	}
}

// Aceitação: truncateStrings.
func TestTruncateStrings(t *testing.T) {
	m := map[string]string{"a": strings.Repeat("x", 100), "b": "short"}
	truncateStrings(m, 10)
	if !strings.Contains(m["a"], "truncated") {
		t.Errorf("err: %q", m["a"])
	}
	if m["b"] != "short" {
		t.Errorf("err")
	}
}

// Aceitação: readAll.
func TestReadAll(t *testing.T) {
	data, err := readAll(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("got = %q", data)
	}
}

// Aceitação: Outcome.Error captura erro.
func TestOutcomeError(t *testing.T) {
	if _, err := os.Stat("/bin/cat"); err != nil {
		t.Skip("cat não encontrado")
	}
	e := NewExecutor()
	e.Timeout = 1 * time.Second
	out, _ := e.Execute(context.Background(), Command{Bin: "/bin/cat", Args: []string{"/nonexistent/abc"}}, "")
	if out.Error == "" {
		t.Errorf("devia capturar erro")
	}
	if out.Passed {
		t.Errorf("devia ser fail")
	}
}
