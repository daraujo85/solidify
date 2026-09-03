// Package runner — execução segura de processos externos.
//
// SAI-028: nunca construir shell string. Args via slice, binário via
// exec.LookPath, env filtrado por allowlist, cwd restrito, timeout
// via context, output bounded.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Config — limites de execução.
type Config struct {
	Timeout     time.Duration // 0 = sem timeout
	MaxOutput   int           // bytes (stdout+stderr cada); 0 = 1MB
	EnvAllow    []string      // nomes permitidos; vazio = sem env
	EnvBase     []string      // env base (pares KEY=value); merged com allowlist
	Cwd         string        // diretório de trabalho; vazio = current
	Stdin       io.Reader     // stdin opcional
	AllowedDirs []string      // diretórios onde Cwd é aceito (security)
}

// Defaults preenche zero-values com padrões seguros.
func (c *Config) Defaults() {
	if c.MaxOutput == 0 {
		c.MaxOutput = 1 << 20 // 1MB
	}
	if c.Cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			c.Cwd = wd
		}
	}
}

// Result — outcome do run.
type Result struct {
	ExitCode int
	Duration time.Duration
	Stdout   []byte
	Stderr   []byte
	Error    error
	TimedOut bool
	Killed   bool
	Command  string // linha de comando reproduzível (audit)
}

// runError agrega info de falha.
type runError struct {
	phase string
	err   error
}

func (e *runError) Error() string {
	return fmt.Sprintf("runner: %s: %v", e.phase, e.err)
}

// Run executa binário com args. name é o binário (procura em PATH se
// não contém "/"). Args é passado verbatim — sem shell.
func Run(ctx context.Context, name string, args []string, cfg Config) (Result, error) {
	cfg.Defaults()
	res := Result{Command: renderCommand(name, args)}
	start := time.Now()

	if err := cfg.validate(); err != nil {
		res.Duration = time.Since(start)
		res.Error = err
		return res, err
	}

	// Resolve binário (bloqueia path traversal relativo).
	binPath, err := resolveBin(name)
	if err != nil {
		res.Duration = time.Since(start)
		res.Error = &runError{phase: "resolve", err: err}
		return res, res.Error
	}

	// Resolve cwd (deve estar em AllowedDirs se setado).
	cwd, err := resolveCwd(cfg.Cwd, cfg.AllowedDirs)
	if err != nil {
		res.Duration = time.Since(start)
		res.Error = &runError{phase: "cwd", err: err}
		return res, res.Error
	}

	// Filtra env.
	env, err := filterEnv(cfg.EnvAllow, cfg.EnvBase)
	if err != nil {
		res.Duration = time.Since(start)
		res.Error = &runError{phase: "env", err: err}
		return res, res.Error
	}

	// Timeout context.
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = cwd
	cmd.Env = env
	cmd.Stdin = cfg.Stdin

	stdout := &boundedBuffer{limit: cfg.MaxOutput}
	stderr := &boundedBuffer{limit: cfg.MaxOutput}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	res.Duration = time.Since(start)
	res.Stdout = stdout.Bytes()
	res.Stderr = stderr.Bytes()

	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.Killed = true
		res.ExitCode = -1
		res.Error = &runError{phase: "timeout", err: ctx.Err()}
		return res, res.Error
	}
	if ctx.Err() == context.Canceled {
		res.Killed = true
		res.ExitCode = -1
		res.Error = &runError{phase: "canceled", err: ctx.Err()}
		return res, res.Error
	}

	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
			res.Error = &runError{phase: "exit", err: err}
			return res, res.Error
		}
		res.ExitCode = -1
		res.Error = &runError{phase: "run", err: err}
		return res, res.Error
	}

	res.ExitCode = cmd.ProcessState.ExitCode()
	return res, nil
}

// validate checa config.
func (c Config) validate() error {
	if c.MaxOutput < 0 {
		return fmt.Errorf("runner: MaxOutput negativo")
	}
	if c.Timeout < 0 {
		return fmt.Errorf("runner: Timeout negativo")
	}
	return nil
}

// resolveBin procura binário. Se contém "/", usa literal e checa que
// não é path traversal (relative sem ser local).
func resolveBin(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("binário vazio")
	}
	if strings.Contains(name, "\x00") {
		return "", fmt.Errorf("binário contém NUL")
	}
	if strings.ContainsAny(name, " \t\n\r") {
		return "", fmt.Errorf("binário contém whitespace")
	}
	if strings.Contains(name, "/") {
		// Path literal. Garante que é absoluto OU local.
		if !filepath.IsAbs(name) {
			if !isLocal(name) {
				return "", fmt.Errorf("binário relativo com path traversal: %s", name)
			}
		}
		if _, err := os.Stat(name); err != nil {
			return "", err
		}
		return name, nil
	}
	// Procura em PATH.
	full, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return full, nil
}

// isLocal — filepath.IsLocal wrapper.
func isLocal(p string) bool {
	return filepath.IsLocal(p)
}

// resolveCwd checa cwd está dentro de AllowedDirs.
func resolveCwd(cwd string, allowed []string) (string, error) {
	if cwd == "" {
		return os.Getwd()
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	// Verifica que abs é diretório existente.
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("não é diretório: %s", abs)
	}
	// Se allowed setado, valida.
	if len(allowed) > 0 {
		ok := false
		for _, dir := range allowed {
			d, err := filepath.Abs(dir)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(d, abs)
			if err != nil {
				continue
			}
			if !strings.HasPrefix(rel, "..") && rel != ".." {
				ok = true
				break
			}
		}
		if !ok {
			return "", fmt.Errorf("cwd %s fora dos permitidos", abs)
		}
	}
	return abs, nil
}

// filterEnv combina base + allowlist. Se allowlist vazia, retorna
// só base. Se base vazia, retorna só allowlist (KEY=value da env).
func filterEnv(allow []string, base []string) ([]string, error) {
	if len(allow) == 0 {
		return append([]string{}, base...), nil
	}
	allowed := make(map[string]bool, len(allow))
	for _, k := range allow {
		if k == "" || strings.Contains(k, "=") {
			return nil, fmt.Errorf("env key inválida: %q", k)
		}
		allowed[k] = true
	}
	out := append([]string{}, base...)
	// Adiciona da os.Environ() filtrada.
	for _, e := range os.Environ() {
		eq := strings.IndexByte(e, '=')
		if eq <= 0 {
			continue
		}
		k := e[:eq]
		if allowed[k] && !contains(out, k) {
			out = append(out, e)
		}
	}
	return out, nil
}

func contains(env []string, key string) bool {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// renderCommand monta linha para audit.
func renderCommand(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(name))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

// shellQuote — quote seguro pra audit (não pra execução!).
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\r'\"\\$`;&|<>*?()[]{}#~!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// boundedBuffer limita nº de bytes.
type boundedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	rem := b.limit - b.buf.Len()
	if rem <= 0 {
		// Drop — não cresce.
		return len(p), nil
	}
	if len(p) > rem {
		b.buf.Write(p[:rem])
		return len(p), nil
	}
	b.buf.Write(p)
	return len(p), nil
}

func (b *boundedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}
