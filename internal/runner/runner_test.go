package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helper: retorna caminho absoluto do /bin/echo (cross-platform não,
// mas testes rodam em container linux).
func echoPath(t *testing.T) string {
	t.Helper()
	for _, p := range []string{"/bin/echo", "/usr/bin/echo"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("echo não encontrado")
	return ""
}

// Aceitação: Run básico com sucesso.
func TestRunOK(t *testing.T) {
	bin := echoPath(t)
	res, err := Run(context.Background(), bin, []string{"hello"}, Config{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit = %d", res.ExitCode)
	}
	if !strings.Contains(string(res.Stdout), "hello") {
		t.Errorf("stdout = %q", string(res.Stdout))
	}
}

// Aceitação: timeout.
func TestRunTimeout(t *testing.T) {
	bin, err := binSleep()
	if err != nil {
		t.Skip("sleep não encontrado")
	}
	res, err := Run(context.Background(), bin, []string{"2"}, Config{Timeout: 100 * time.Millisecond})
	if err == nil {
		t.Errorf("devia dar timeout")
	}
	if !res.TimedOut {
		t.Errorf("não marcou TimedOut")
	}
	if res.ExitCode != -1 {
		t.Errorf("exit code = %d", res.ExitCode)
	}
}

// Aceitação: exit code != 0.
func TestRunExitCode(t *testing.T) {
	bin, err := binFalse()
	if err != nil {
		t.Skip("false não encontrado")
	}
	res, _ := Run(context.Background(), bin, nil, Config{})
	if res.ExitCode != 1 {
		t.Errorf("exit = %d, quero 1", res.ExitCode)
	}
}

// Aceitação: bounded stdout.
func TestRunBoundedOutput(t *testing.T) {
	bin, err := binYes()
	if err != nil {
		t.Skip("yes não encontrado")
	}
	res, _ := Run(context.Background(), bin, []string{"x"}, Config{MaxOutput: 100, Timeout: 200 * time.Millisecond})
	if len(res.Stdout) > 100 {
		t.Errorf("stdout len = %d, quero <=100", len(res.Stdout))
	}
}

// Aceitação: env allowlist filtra.
func TestRunEnvAllowlist(t *testing.T) {
	bin := echoPath(t)
	os.Setenv("SOLIDIFY_TEST_ALLOW", "allowed-value")
	defer os.Unsetenv("SOLIDIFY_TEST_ALLOW")
	os.Setenv("SOLIDIFY_TEST_DENY", "denied-value")
	defer os.Unsetenv("SOLIDIFY_TEST_DENY")
	res, _ := Run(context.Background(), bin, []string{"$SOLIDIFY_TEST_ALLOW", "$SOLIDIFY_TEST_DENY"}, Config{EnvAllow: []string{"SOLIDIFY_TEST_ALLOW"}})
	// /bin/echo não expande vars — checa via env command.
	bin2, err := binPrintenv()
	if err == nil {
		res, _ = Run(context.Background(), bin2, []string{"SOLIDIFY_TEST_ALLOW"}, Config{EnvAllow: []string{"SOLIDIFY_TEST_ALLOW"}})
		if !strings.Contains(string(res.Stdout), "allowed-value") {
			t.Errorf("allow sumiu: %q", string(res.Stdout))
		}
	}
	res, _ = Run(context.Background(), bin2, []string{"SOLIDIFY_TEST_DENY"}, Config{EnvAllow: []string{"SOLIDIFY_TEST_ALLOW"}})
	if strings.Contains(string(res.Stdout), "denied-value") {
		t.Errorf("deny vazou: %q", string(res.Stdout))
	}
}

// Aceitação: cwd restrito.
func TestRunCwdRestricted(t *testing.T) {
	bin := echoPath(t)
	dir := t.TempDir()
	res, err := Run(context.Background(), bin, []string{"ok"}, Config{Cwd: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit = %d", res.ExitCode)
	}
}

// Aceitação: cwd fora de AllowedDirs falha.
func TestRunCwdNotAllowed(t *testing.T) {
	bin := echoPath(t)
	dir := t.TempDir()
	other := t.TempDir()
	_, err := Run(context.Background(), bin, []string{"x"}, Config{Cwd: other, AllowedDirs: []string{dir}})
	if err == nil {
		t.Errorf("devia falhar (cwd fora)")
	}
}

// Aceitação: binário com whitespace falha.
func TestRunBinaryWhitespace(t *testing.T) {
	_, err := Run(context.Background(), "/bin/echo with space", nil, Config{})
	if err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: binário com path traversal.
func TestRunBinaryTraversal(t *testing.T) {
	_, err := Run(context.Background(), "../../../etc/passwd", nil, Config{})
	if err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: binário vazio.
func TestRunBinaryEmpty(t *testing.T) {
	_, err := Run(context.Background(), "", nil, Config{})
	if err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: binário não existe.
func TestRunBinaryNotFound(t *testing.T) {
	_, err := Run(context.Background(), "/nope/no/such/binary", nil, Config{})
	if err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: contexto cancelado.
func TestRunContextCancel(t *testing.T) {
	bin, err := binSleep()
	if err != nil {
		t.Skip("sleep não encontrado")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res, _ := Run(ctx, bin, []string{"2"}, Config{})
	if !res.Killed && res.Error == nil {
		t.Errorf("devia falhar/cancelar")
	}
}

// Aceitação: shell quote seguro.
func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"hello":      "hello",
		"":           "''",
		"a b":        "'a b'",
		"a'b":        "'a'\\''b'",
		"with$dol":   "'with$dol'",
		"normal.txt": "normal.txt",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, quero %q", in, got, want)
		}
	}
}

// Aceitação: renderCommand audit.
func TestRenderCommand(t *testing.T) {
	got := renderCommand("echo", []string{"hello", "world"})
	if got != "echo hello world" {
		t.Errorf("got = %q", got)
	}
}

// Aceitação: filterEnv sem allowlist retorna base.
func TestFilterEnvNoAllow(t *testing.T) {
	got, err := filterEnv(nil, []string{"A=1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0] != "A=1" {
		t.Errorf("got = %v", got)
	}
}

// Aceitação: filterEnv com allowlist filtra os.Environ.
func TestFilterEnvAllow(t *testing.T) {
	os.Setenv("SOLIDIFY_FENV_TEST", "yes")
	defer os.Unsetenv("SOLIDIFY_FENV_TEST")
	got, err := filterEnv([]string{"SOLIDIFY_FENV_TEST"}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for _, e := range got {
		if e == "SOLIDIFY_FENV_TEST=yes" {
			found = true
		}
	}
	if !found {
		t.Errorf("env não apareceu: %v", got)
	}
}

// Aceitação: filterEnv rejeita key com =.
func TestFilterEnvInvalidKey(t *testing.T) {
	_, err := filterEnv([]string{"A=B"}, nil)
	if err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: filterEnv rejeita key vazia.
func TestFilterEnvEmptyKey(t *testing.T) {
	_, err := filterEnv([]string{""}, nil)
	if err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: validate rejeita MaxOutput negativo.
func TestValidateNegativeMaxOutput(t *testing.T) {
	if err := (Config{MaxOutput: -1}).validate(); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: validate rejeita Timeout negativo.
func TestValidateNegativeTimeout(t *testing.T) {
	if err := (Config{Timeout: -1 * time.Second}).validate(); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: validate aceita defaults.
func TestValidateOK(t *testing.T) {
	if err := (Config{}).validate(); err != nil {
		t.Errorf("err: %v", err)
	}
}

// Aceitação: boundedBuffer respeita limit.
func TestBoundedBuffer(t *testing.T) {
	b := &boundedBuffer{limit: 5}
	b.Write([]byte("hello"))
	b.Write([]byte("world"))
	if b.buf.Len() != 5 {
		t.Errorf("len = %d, quero 5", b.buf.Len())
	}
}

// Aceitação: boundedBuffer com 0 não cresce.
func TestBoundedBufferZero(t *testing.T) {
	b := &boundedBuffer{limit: 0}
	b.Write([]byte("hi"))
	if b.buf.Len() != 0 {
		t.Errorf("len = %d", b.buf.Len())
	}
}

// Aceitação: resolveCwd path inválido.
func TestResolveCwdInvalid(t *testing.T) {
	if _, err := resolveCwd("/nonexistent/path/here/12345", nil); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: resolveCwd arquivo (não diretório).
func TestResolveCwdNotDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, []byte("x"), 0644)
	if _, err := resolveCwd(f, nil); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: resolveBin rejeita NUL.
func TestResolveBinNUL(t *testing.T) {
	if _, err := resolveBin("/bin/echo\x00bad"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: result command reproduzível.
func TestRunCommandAudit(t *testing.T) {
	bin := echoPath(t)
	res, _ := Run(context.Background(), bin, []string{"a", "b c"}, Config{})
	if !strings.Contains(res.Command, "echo") || !strings.Contains(res.Command, "b c") {
		t.Errorf("command = %q", res.Command)
	}
}

// Aceitação: Stdin via Reader.
func TestRunStdin(t *testing.T) {
	bin, err := binCat()
	if err != nil {
		t.Skip("cat não encontrado")
	}
	res, _ := Run(context.Background(), bin, nil, Config{Stdin: strings.NewReader("from stdin")})
	if !strings.Contains(string(res.Stdout), "from stdin") {
		t.Errorf("stdout = %q", string(res.Stdout))
	}
}

// Aceitação: Defaults preenche.
func TestDefaults(t *testing.T) {
	c := Config{}
	c.Defaults()
	if c.MaxOutput != 1<<20 {
		t.Errorf("MaxOutput = %d", c.MaxOutput)
	}
	if c.Cwd == "" {
		t.Errorf("Cwd vazio")
	}
}

// Aceitação: EnvAllow + EnvBase combina.
func TestEnvAllowAndBase(t *testing.T) {
	os.Setenv("SOLIDIFY_TEST_COMBO", "combo")
	defer os.Unsetenv("SOLIDIFY_TEST_COMBO")
	got, err := filterEnv([]string{"SOLIDIFY_TEST_COMBO"}, []string{"BASE=1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	foundCombo := false
	foundBase := false
	for _, e := range got {
		if e == "SOLIDIFY_TEST_COMBO=combo" {
			foundCombo = true
		}
		if e == "BASE=1" {
			foundBase = true
		}
	}
	if !foundCombo || !foundBase {
		t.Errorf("got = %v", got)
	}
}

// --- helpers de binários ---

func binSleep() (string, error) {
	for _, p := range []string{"/bin/sleep", "/usr/bin/sleep"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

func binFalse() (string, error) {
	for _, p := range []string{"/bin/false", "/usr/bin/false"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

func binYes() (string, error) {
	for _, p := range []string{"/bin/yes", "/usr/bin/yes"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

func binCat() (string, error) {
	for _, p := range []string{"/bin/cat", "/usr/bin/cat"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

func binPrintenv() (string, error) {
	for _, p := range []string{"/usr/bin/printenv", "/bin/printenv"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}
