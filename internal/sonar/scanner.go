// Package sonar — scanner runner opcional.
//
// SAI-036: roda sonar-scanner em container/perfil se usuário
// configurou. Não sobe Sonar Server automaticamente. Suporta
// CLI mode (com sonar-project.properties local) e skip mode.
package sonar

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ScannerMode controla como rodar scanner.
type ScannerMode int

const (
	// ScannerDisabled — não roda scanner; só consulta Web API.
	ScannerDisabled ScannerMode = iota
	// ScannerLocal — roda binário local (ex: sonar-scanner no PATH).
	ScannerLocal
	// ScannerDocker — roda imagem sonarsource/sonar-scanner-cli.
	ScannerDocker
)

// String devolve nome do mode.
func (m ScannerMode) String() string {
	switch m {
	case ScannerLocal:
		return "local"
	case ScannerDocker:
		return "docker"
	default:
		return "disabled"
	}
}

// ParseScannerMode converte string → mode.
func ParseScannerMode(s string) (ScannerMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off", "disabled", "false":
		return ScannerDisabled, nil
	case "local", "cli":
		return ScannerLocal, nil
	case "docker", "container":
		return ScannerDocker, nil
	}
	return ScannerDisabled, fmt.Errorf("sonar: scanner mode inválido: %q", s)
}

// ScannerConfig configura runner.
type ScannerConfig struct {
	Mode        ScannerMode
	DockerImage string
	ScannerBin  string // path/to/sonar-scanner
	ProjectDir  string
	ExtraArgs   []string
	Env         map[string]string
	Timeout     time.Duration
}

// ScannerResult do run.
type ScannerResult struct {
	Mode       ScannerMode   `json:"mode"`
	StartedAt  time.Time     `json:"started_at"`
	Duration   time.Duration `json:"duration"`
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	ExitCode   int           `json:"exit_code"`
	Skipped    bool          `json:"skipped"`
	SkipReason string        `json:"skip_reason,omitempty"`
}

// ShouldRun devolve true se mode != Disabled e config mínima OK.
func ShouldRun(cfg ScannerConfig) (bool, string) {
	if cfg.Mode == ScannerDisabled {
		return false, "scanner disabled"
	}
	if cfg.ProjectDir == "" {
		return false, "project_dir vazio"
	}
	fi, err := os.Stat(cfg.ProjectDir)
	if err != nil {
		return false, fmt.Sprintf("project_dir inacessível: %v", err)
	}
	if !fi.IsDir() {
		return false, "project_dir não é diretório"
	}
	if cfg.Mode == ScannerLocal {
		bin := cfg.ScannerBin
		if bin == "" {
			bin = "sonar-scanner"
		}
		if _, err := exec.LookPath(bin); err != nil {
			return false, fmt.Sprintf("binário %q não encontrado", bin)
		}
	}
	return true, ""
}

// RunScanner executa o scanner conforme config.
func RunScanner(ctx context.Context, cfg ScannerConfig) (*ScannerResult, error) {
	ok, reason := ShouldRun(cfg)
	if !ok {
		return &ScannerResult{
			Mode:       cfg.Mode,
			Skipped:    true,
			SkipReason: reason,
		}, nil
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	res := &ScannerResult{Mode: cfg.Mode, StartedAt: started}

	switch cfg.Mode {
	case ScannerLocal:
		return runLocal(cctx, cfg, res, started)
	case ScannerDocker:
		return runDocker(cctx, cfg, res, started)
	}
	return res, nil
}

func runLocal(ctx context.Context, cfg ScannerConfig, res *ScannerResult, started time.Time) (*ScannerResult, error) {
	bin := cfg.ScannerBin
	if bin == "" {
		bin = "sonar-scanner"
	}
	args := []string{}
	args = append(args, cfg.ExtraArgs...)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = cfg.ProjectDir
	cmd.Env = mergeEnv(os.Environ(), cfg.Env)
	out, err := cmd.CombinedOutput()
	res.Duration = time.Since(started)
	res.Stdout = string(out)
	res.Stderr = ""
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return res, fmt.Errorf("sonar: scanner local: %w", err)
	}
	return res, nil
}

func runDocker(ctx context.Context, cfg ScannerConfig, res *ScannerResult, started time.Time) (*ScannerResult, error) {
	image := cfg.DockerImage
	if image == "" {
		image = "sonarsource/sonar-scanner-cli:latest"
	}
	args := []string{"run", "--rm"}
	// mount project dir em /src.
	srcDir, err := filepath.Abs(cfg.ProjectDir)
	if err != nil {
		return res, fmt.Errorf("sonar: abs: %w", err)
	}
	args = append(args, "-v", srcDir+":/src", "-w", "/src")
	for k, v := range cfg.ExtraArgs {
		_ = k
		_ = v
	}
	// env vars do cfg.Env (sem expor secrets em logs).
	for k, v := range cfg.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, image)
	args = append(args, cfg.ExtraArgs...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	res.Duration = time.Since(started)
	res.Stdout = string(out)
	res.Stderr = ""
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return res, fmt.Errorf("sonar: scanner docker: %w", err)
	}
	return res, nil
}

// mergeEnv merge env base + overrides.
func mergeEnv(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	out = append(out, base...)
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

// Available devolve true se alguma opção viável existe (binário ou docker).
func Available() (mode ScannerMode, bin string, ok bool) {
	if p, err := exec.LookPath("sonar-scanner"); err == nil {
		return ScannerLocal, p, true
	}
	if p, err := exec.LookPath("docker"); err == nil {
		return ScannerDocker, p, true
	}
	return ScannerDisabled, "", false
}
