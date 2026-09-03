package doctor

import (
	"strings"
	"testing"
)

// Aceitação: RunCheck desconhecido.
func TestRunCheckUnknown(t *testing.T) {
	r := RunCheck("nope")
	if r.OK {
		t.Errorf("esperado fail")
	}
}

// Aceitação: docker check (PATH dependent).
func TestRunCheckDocker(t *testing.T) {
	r := RunCheck("docker")
	if r.Name != "docker" {
		t.Errorf("name")
	}
	// OK ou fail — depende do PATH.
}

// Aceitação: git check.
func TestRunCheckGit(t *testing.T) {
	r := RunCheck("git")
	if r.Name != "git" {
		t.Errorf("name")
	}
}

// Aceitação: go check.
func TestRunCheckGo(t *testing.T) {
	r := RunCheck("go")
	if r.Name != "go" {
		t.Errorf("name")
	}
}

// Aceitação: disk check.
func TestRunCheckDisk(t *testing.T) {
	r := RunCheck("disk_space")
	if !r.OK {
		t.Errorf("disk: %s", r.Message)
	}
}

// Aceitação: 9router sem env.
func TestRunCheck9RouterMissing(t *testing.T) {
	t.Setenv("NINEROUTER_URL", "")
	r := RunCheck("9router")
	if r.OK {
		t.Errorf("expected fail")
	}
}

// Aceitação: 9router URL inválida.
func TestRunCheck9RouterBadURL(t *testing.T) {
	t.Setenv("NINEROUTER_URL", "nothttp")
	r := RunCheck("9router")
	if r.OK {
		t.Errorf("expected fail")
	}
}

// Aceitação: 9router URL OK (sem probe real; token setado para passar a fase 1).
func TestRunCheck9RouterOK(t *testing.T) {
	t.Setenv("NINEROUTER_URL", "http://localhost:1") // porta morta — vai falhar no probe
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")
	r := RunCheck("9router")
	// Como localhost:1 está morto, esperamos FAIL com mensagem sobre connect.
	// O importante é que a checagem NÃO falhe por falta de token.
	if r.OK {
		t.Errorf("expected fail (port morta): %s", r.Message)
	}
	if !strings.Contains(r.Message, "probe") {
		t.Errorf("expected probe error, got: %s", r.Message)
	}
}

// Aceitação: RunAll retorna todos os checks.
func TestRunAll(t *testing.T) {
	rs := RunAll()
	if len(rs) != len(AllChecks) {
		t.Errorf("count: %d", len(rs))
	}
}

// Aceitação: Summarize total.
func TestSummarize(t *testing.T) {
	rs := []CheckResult{
		{OK: true}, {OK: true}, {OK: false},
	}
	s := Summarize(rs)
	if s.Total != 3 || s.OK != 2 || s.Fail != 1 {
		t.Errorf("agg: %+v", s)
	}
}
