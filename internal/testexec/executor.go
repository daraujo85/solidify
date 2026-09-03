// Package testexec — execução de comandos de teste com normalização.
//
// SAI-031: executa command via runner.Run, parseia JUnit XML se
// produzido, devolve Outcome normalizado com counts e duration.
package testexec

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/diegoaraujo/solidify/internal/runner"
)

// Outcome — resultado normalizado de execução de testes.
type Outcome struct {
	Command  string        `json:"command"`
	ExitCode int           `json:"exit_code"`
	Passed   bool          `json:"passed"`
	Duration time.Duration `json:"duration_ns"`
	TimedOut bool          `json:"timed_out"`
	Error    string        `json:"error,omitempty"`
	Stack    string        `json:"stack"`
	Total    int           `json:"total"`
	PassedN  int           `json:"passed_n"`
	FailedN  int           `json:"failed"`
	SkippedN int           `json:"skipped"`
	ErroredN int           `json:"errored"`
	Report   string        `json:"report,omitempty"`
	Started  time.Time     `json:"started"`
	Finished time.Time     `json:"finished"`
	Stdout   string        `json:"stdout,omitempty"`
	Stderr   string        `json:"stderr,omitempty"`
}

// TestCase de JUnit XML.
type TestCase struct {
	Name      string  `json:"name"`
	Classname string  `json:"classname,omitempty"`
	Time      float64 `json:"time,omitempty"`
	Failure   *string `json:"failure,omitempty"`
	Error     *string `json:"error,omitempty"`
	Skipped   *string `json:"skipped,omitempty"`
}

// TestSuite parseado de JUnit.
type TestSuite struct {
	Name     string     `json:"name"`
	Tests    int        `json:"tests"`
	Failures int        `json:"failures"`
	Errors   int        `json:"errors"`
	Skipped  int        `json:"skipped"`
	Time     float64    `json:"time"`
	Cases    []TestCase `json:"cases"`
}

// JUnit XML root.
type junitXML struct {
	XMLName xml.Name `xml:"testsuites"`
	Suites  []junitS `xml:"testsuite"`
}

// Suporta tanto <testsuites><testsuite> quanto <testsuite> root.
type junitS struct {
	XMLName  xml.Name `xml:"testsuite"`
	Name     string   `xml:"name,attr"`
	Tests    int      `xml:"tests,attr"`
	Failures int      `xml:"failures,attr"`
	Errors   int      `xml:"errors,attr"`
	Skipped  int      `xml:"skipped,attr"`
	Time     float64  `xml:"time,attr"`
	Cases    []junitC `xml:"testcase"`
}

type junitC struct {
	Name      string  `xml:"name,attr"`
	Classname string  `xml:"classname,attr"`
	Time      float64 `xml:"time,attr"`
	Failure   *string `xml:"failure"`
	Error     *string `xml:"error"`
	Skipped   *string `xml:"skipped"`
}

// Executor roda commands e normaliza.
type Executor struct {
	Timeout   time.Duration
	EnvAllow  []string
	MaxOutput int
	ReportDir string // se setado, JUnit XML é buscado aqui depois do run
}

// New cria executor com defaults sensatos.
func NewExecutor() *Executor {
	return &Executor{
		Timeout:   30 * time.Minute,
		MaxOutput: 5 << 20, // 5MB
	}
}

// Execute roda o comando e devolve Outcome. ReportPath opcional
// aponta pra um arquivo JUnit XML pré-existente ou produzido.
func (e *Executor) Execute(ctx context.Context, cmd Command, reportPath string) (Outcome, error) {
	cfg := runner.Config{
		Timeout:   e.Timeout,
		MaxOutput: e.MaxOutput,
		EnvAllow:  e.EnvAllow,
		Cwd:       cmd.Cwd,
	}
	res, err := runner.Run(ctx, cmd.Bin, cmd.Args, cfg)
	out := Outcome{
		Command:  res.Command,
		ExitCode: res.ExitCode,
		TimedOut: res.TimedOut,
		Stack:    cmd.Stack,
		Started:  time.Now().Add(-res.Duration),
		Finished: time.Now(),
		Duration: res.Duration,
		Stdout:   string(res.Stdout),
		Stderr:   string(res.Stderr),
	}
	if err != nil {
		out.Error = err.Error()
	}
	// Parses report.
	if reportPath != "" {
		if data, rerr := os.ReadFile(reportPath); rerr == nil {
			out.Report = reportPath
			out = applyJUnit(out, data)
		}
	}
	if e.ReportDir != "" && out.Total == 0 {
		// Procura reports conhecidos.
		for _, name := range []string{"junit.xml", "report.xml", "TEST-*.xml"} {
			matches, _ := filepath.Glob(filepath.Join(e.ReportDir, name))
			for _, m := range matches {
				data, rerr := os.ReadFile(m)
				if rerr != nil {
					continue
				}
				out.Report = m
				out = applyJUnit(out, data)
				break
			}
			if out.Total > 0 {
				break
			}
		}
	}
	// Pass/fail.
	out.Passed = out.ExitCode == 0 && out.FailedN == 0 && out.ErroredN == 0 && !out.TimedOut && out.Error == ""
	return out, nil
}

// applyJUnit extrai counts do XML.
func applyJUnit(out Outcome, data []byte) Outcome {
	suites, _ := parseJUnitSuites(data)
	for _, s := range suites {
		out.Total += s.Tests
		out.FailedN += s.Failures
		out.ErroredN += s.Errors
		out.SkippedN += s.Skipped
	}
	out.PassedN = out.Total - out.FailedN - out.ErroredN - out.SkippedN
	if out.PassedN < 0 {
		out.PassedN = 0
	}
	return out
}

// ParseJUnit devolve suites parseados (útil pra mais detalhe).
func ParseJUnit(data []byte) ([]TestSuite, error) {
	jss, err := parseJUnitSuites(data)
	if err != nil {
		return nil, err
	}
	out := make([]TestSuite, 0, len(jss))
	for _, s := range jss {
		ts := TestSuite{
			Name:     s.Name,
			Tests:    s.Tests,
			Failures: s.Failures,
			Errors:   s.Errors,
			Skipped:  s.Skipped,
			Time:     s.Time,
		}
		for _, c := range s.Cases {
			tc := TestCase{
				Name:      c.Name,
				Classname: c.Classname,
				Time:      c.Time,
				Failure:   c.Failure,
				Error:     c.Error,
				Skipped:   c.Skipped,
			}
			ts.Cases = append(ts.Cases, tc)
		}
		out = append(out, ts)
	}
	return out, nil
}

// parseJUnitSuites tenta os 2 formatos: <testsuites><testsuite>
// (múltiplos) ou <testsuite> root único.
func parseJUnitSuites(data []byte) ([]junitS, error) {
	var root junitXML
	if err := xml.Unmarshal(data, &root); err == nil && len(root.Suites) > 0 {
		return root.Suites, nil
	}
	var single junitS
	if err := xml.Unmarshal(data, &single); err == nil && single.Name != "" {
		return []junitS{single}, nil
	}
	return nil, fmt.Errorf("junit: parse falhou")
}

// Merge combina múltiplos outcomes (útil pra monorepos).
func Merge(outcomes []Outcome) Outcome {
	if len(outcomes) == 0 {
		return Outcome{}
	}
	out := outcomes[0]
	for i := 1; i < len(outcomes); i++ {
		o := outcomes[i]
		out.Total += o.Total
		out.PassedN += o.PassedN
		out.FailedN += o.FailedN
		out.SkippedN += o.SkippedN
		out.ErroredN += o.ErroredN
		out.Duration += o.Duration
		if !o.Passed {
			out.Passed = false
		}
	}
	return out
}

// PassedRatio devolve PassedN/Total (0 se Total=0).
func (o Outcome) PassedRatio() float64 {
	if o.Total == 0 {
		return 0
	}
	return float64(o.PassedN) / float64(o.Total)
}

// HasFailures devolve true se FailedN+ErroredN > 0.
func (o Outcome) HasFailures() bool {
	return o.FailedN+o.ErroredN > 0
}

// Summary devolve string curta.
func (o Outcome) Summary() string {
	return fmt.Sprintf("%d/%d passed, %d failed, %d skipped, %d errored in %s",
		o.PassedN, o.Total, o.FailedN, o.SkippedN, o.ErroredN, o.Duration)
}

// ReadAllFromReader é helper pra testes.
func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// ensureStdoutTrimmed limita tamanho em memória.
func ensureStdoutTrimmed(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n[... truncated ...]"
}

// truncateStrings em map.
func truncateStrings(m map[string]string, max int) {
	for k, v := range m {
		if len(v) > max {
			m[k] = v[:max] + "...[truncated]"
		}
	}
}

// Command helpers.

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
