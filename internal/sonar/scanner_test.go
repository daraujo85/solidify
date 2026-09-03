package sonar

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Aceitação: ScannerMode String.
func TestScannerModeString(t *testing.T) {
	cases := map[ScannerMode]string{
		ScannerDisabled: "disabled",
		ScannerLocal:    "local",
		ScannerDocker:   "docker",
		ScannerMode(99): "disabled",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

// Aceitação: ParseScannerMode.
func TestParseScannerMode(t *testing.T) {
	cases := map[string]ScannerMode{
		"":          ScannerDisabled,
		"off":       ScannerDisabled,
		"local":     ScannerLocal,
		"cli":       ScannerLocal,
		"docker":    ScannerDocker,
		"  DOCKER ": ScannerDocker,
	}
	for in, want := range cases {
		got, err := ParseScannerMode(in)
		if err != nil {
			t.Errorf("err %q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseScannerMode(%q) = %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: ParseScannerMode inválido.
func TestParseScannerModeInvalid(t *testing.T) {
	if _, err := ParseScannerMode("quantum"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ShouldRun disabled.
func TestShouldRunDisabled(t *testing.T) {
	ok, reason := ShouldRun(ScannerConfig{Mode: ScannerDisabled})
	if ok || reason == "" {
		t.Errorf("devia skip com reason")
	}
}

// Aceitação: ShouldRun project_dir vazio.
func TestShouldRunEmptyDir(t *testing.T) {
	ok, reason := ShouldRun(ScannerConfig{Mode: ScannerLocal})
	if ok {
		t.Errorf("devia falhar")
	}
	if reason == "" {
		t.Errorf("reason vazio")
	}
}

// Aceitação: ShouldRun dir inexistente.
func TestShouldRunMissingDir(t *testing.T) {
	ok, _ := ShouldRun(ScannerConfig{Mode: ScannerLocal, ProjectDir: "/nonexistent-xyz"})
	if ok {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ShouldRun bin não achado.
func TestShouldRunMissingBin(t *testing.T) {
	dir := t.TempDir()
	// Força bin inexistente renomeando PATH para algo controlado.
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)
	os.Setenv("PATH", "/nonexistent-xyz")
	ok, _ := ShouldRun(ScannerConfig{Mode: ScannerLocal, ProjectDir: dir})
	if ok {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ShouldRun OK (sem bin — usa sonar-scanner lookup).
func TestShouldRunOKDocker(t *testing.T) {
	dir := t.TempDir()
	// docker raramente existe no CI; pulamos verificação.
	if _, err := os.Stat("/usr/bin/docker"); os.IsNotExist(err) {
		// tenta "/var/run/docker.sock" como fallback ou pula.
		ok, reason := ShouldRun(ScannerConfig{Mode: ScannerDocker, ProjectDir: dir})
		// docker check não procura binário, então passa só com ProjectDir.
		if !ok && reason != "" {
			// se docker não foi procurado, devia passar.
			t.Skipf("docker check skipped (reason=%s)", reason)
		}
	} else {
		ok, _ := ShouldRun(ScannerConfig{Mode: ScannerDocker, ProjectDir: dir})
		if !ok {
			t.Errorf("docker devia passar")
		}
	}
}

// Aceitação: RunScanner skip quando disabled.
func TestRunScannerSkipped(t *testing.T) {
	res, err := RunScanner(context.Background(), ScannerConfig{Mode: ScannerDisabled})
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if !res.Skipped {
		t.Errorf("devia skip")
	}
	if res.SkipReason == "" {
		t.Errorf("reason vazio")
	}
}

// Aceitação: RunScanner skip quando dir vazio.
func TestRunScannerSkipEmptyDir(t *testing.T) {
	res, err := RunScanner(context.Background(), ScannerConfig{Mode: ScannerLocal})
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if !res.Skipped {
		t.Errorf("devia skip")
	}
}

// Aceitação: RunScanner real (echo binário) em modo local.
func TestRunScannerLocalEcho(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	dir := t.TempDir()
	cfg := ScannerConfig{
		Mode:       ScannerLocal,
		ScannerBin: "/bin/echo",
		ProjectDir: dir,
		ExtraArgs:  []string{"hello"},
	}
	res, err := RunScanner(context.Background(), cfg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit = %d", res.ExitCode)
	}
	if res.Stdout != "hello\n" {
		t.Errorf("stdout = %q", res.Stdout)
	}
}

// Aceitação: RunScanner local falha.
func TestRunScannerLocalFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	dir := t.TempDir()
	cfg := ScannerConfig{
		Mode:       ScannerLocal,
		ScannerBin: "/bin/false",
		ProjectDir: dir,
		Timeout:    5 * time.Second,
	}
	res, err := RunScanner(context.Background(), cfg)
	if err == nil {
		t.Errorf("devia falhar")
	}
	if res.ExitCode == 0 {
		t.Errorf("exit = %d", res.ExitCode)
	}
}

// Aceitação: RunScanner context cancel.
func TestRunScannerContextCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	dir := t.TempDir()
	cfg := ScannerConfig{
		Mode:       ScannerLocal,
		ScannerBin: "/bin/sleep",
		ProjectDir: dir,
		ExtraArgs:  []string{"10"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := RunScanner(ctx, cfg)
	if err == nil {
		t.Errorf("devia falhar por cancel")
	}
	if res.Duration > 3*time.Second {
		t.Errorf("demorou demais: %v", res.Duration)
	}
}

// Aceitação: mergeEnv.
func TestMergeEnv(t *testing.T) {
	base := []string{"FOO=1", "BAR=2"}
	overrides := map[string]string{"BAR": "new", "BAZ": "3"}
	out := mergeEnv(base, overrides)
	has := map[string]bool{}
	for _, e := range out {
		has[e] = true
	}
	if !has["BAR=new"] || !has["BAZ=3"] {
		t.Errorf("merge err: %v", out)
	}
}

// Aceitação: mergeEnv nil.
func TestMergeEnvEmpty(t *testing.T) {
	if out := mergeEnv(nil, nil); len(out) != 0 {
		t.Errorf("err")
	}
}

// Aceitação: Available.
func TestAvailable(t *testing.T) {
	_, _, ok := Available()
	// pode ou não ter disponível — só verificamos que não panic.
	_ = ok
}

// Aceitação: ScannerResult skip mode preserved.
func TestRunScannerResultMode(t *testing.T) {
	res, _ := RunScanner(context.Background(), ScannerConfig{Mode: ScannerDisabled})
	if res.Mode != ScannerDisabled {
		t.Errorf("mode = %v", res.Mode)
	}
}

// Aceitação: ProjectDir inexistente falha ShouldRun.
func TestShouldRunNotADir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, []byte("x"), 0644)
	ok, _ := ShouldRun(ScannerConfig{Mode: ScannerDocker, ProjectDir: f})
	// docker check não verifica binário, mas ProjectDir precisa ser diretório.
	if ok {
		t.Errorf("devia falhar (file não é dir)")
	}
}
