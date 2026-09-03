package secretcorpus

import (
	"strings"
	"testing"
)

// Aceitação: Verify roda em todas fixtures.
func TestVerifyCount(t *testing.T) {
	rs := Verify(func(s string) string {
		// mock redactor — substitui tudo por [REDACTED].
		return "[REDACTED]"
	})
	if len(rs) != len(CanonicalCorpus) {
		t.Errorf("count: %d", len(rs))
	}
	for _, r := range rs {
		if !r.Redacted {
			t.Errorf("not redacted: %s", r.FixtureName)
		}
	}
}

// Aceitação: Summarize total = len(results).
func TestSummarize(t *testing.T) {
	rs := Verify(func(s string) string { return "[R]" })
	s := Summarize(rs)
	if s.Total != len(CanonicalCorpus) {
		t.Errorf("total")
	}
	if s.Redacted != len(CanonicalCorpus) {
		t.Errorf("redacted: %d", s.Redacted)
	}
	if s.Leaked != 0 {
		t.Errorf("leaked: %d", s.Leaked)
	}
	if len(s.ByCategory) == 0 {
		t.Errorf("no cats")
	}
}

// Aceitação: Verify detecta leak quando redactor é no-op.
func TestVerifyLeak(t *testing.T) {
	rs := Verify(func(s string) string { return s })
	s := Summarize(rs)
	if s.Leaked != len(CanonicalCorpus) {
		t.Errorf("expected all leaked: %d", s.Leaked)
	}
}

// Aceitação: Categories cobertas.
func TestCategories(t *testing.T) {
	cats := map[string]bool{}
	for _, f := range CanonicalCorpus {
		cats[f.Category] = true
	}
	for _, want := range []string{"api_key", "bearer", "env", "conn", "password"} {
		if !cats[want] {
			t.Errorf("missing cat: %s", want)
		}
	}
}

// Aceitação: contains helper.
func TestContains(t *testing.T) {
	if !contains("hello world", "world") {
		t.Errorf("yes")
	}
	if contains("hello", "xyz") {
		t.Errorf("no")
	}
	if !contains("anything", "") {
		t.Errorf("empty needle")
	}
}

// Aceitação: integration com redact real (linha env).
func TestIntegrationRedactEnv(t *testing.T) {
	rs := Verify(func(s string) string {
		// Simula RedactEnvLine — substitui valor após `=`.
		i := strings.Index(s, "=")
		if i < 0 {
			return "[REDACTED]"
		}
		return s[:i+1] + "[REDACTED]"
	})
	s := Summarize(rs)
	if s.Redacted < len(CanonicalCorpus)/2 {
		t.Errorf("many leaks: %d", s.Leaked)
	}
}
