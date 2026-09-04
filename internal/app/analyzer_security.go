package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/report"
	"github.com/diegoaraujo/solidify/internal/security"
	"github.com/diegoaraujo/solidify/internal/zap"
)

// runSecurityAnalyzer agrega gitleaks (secrets), osv-scanner (deps) e
// semgrep (SAST) via exec — cada um roda só se habilitado na config E o
// binário existe no PATH; ausência de UM não derruba os outros, só some
// dessa fonte na limitação. ZAP ativo é tratado à parte (analyzer "zap"
// não existe hoje no report — fica para quando o dashboard precisar dele;
// ver plano) e sempre atrás de zap.TargetGuard, nunca direto.
func runSecurityAnalyzer(ctx context.Context, cfg config.Config, dir string, logger *slog.Logger) report.Analyzer {
	sec := cfg.Analyzers.Security
	var findings []security.Finding
	var limitations []string

	if sec.Gitleaks {
		fs, err := runGitleaks(ctx, dir)
		if err != nil {
			limitations = append(limitations, "gitleaks: "+err.Error())
		} else {
			findings = append(findings, fs...)
		}
	}
	if sec.OSVScanner {
		fs, err := runOSVScanner(ctx, dir)
		if err != nil {
			limitations = append(limitations, "osv-scanner: "+err.Error())
		} else {
			findings = append(findings, fs...)
		}
	}
	if sec.Semgrep {
		fs, err := runSemgrep(ctx, dir)
		if err != nil {
			limitations = append(limitations, "semgrep: "+err.Error())
		} else {
			findings = append(findings, fs...)
		}
	}

	security.SortFindings(findings)
	gate := security.EvaluateGate(findings, security.DefaultThreshold())
	findingMaps := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		findingMaps = append(findingMaps, map[string]any{
			"source": string(f.Source), "rule": f.Rule, "severity": string(f.Severity),
			"file_path": f.FilePath, "line": f.Line, "message": f.Message,
		})
	}
	score := float64(gate.Score.Value)
	logger.Debug("security: agregado", "total", gate.Score.Total, "allow", gate.Allow)

	return report.Analyzer{
		ID: "security", Applicability: "APPLICABLE", ExecutionStatus: statusPtr("completed"),
		Score: &score, Findings: findingMaps,
		Metrics:     map[string]any{"gate_allow": gate.Allow, "gate_reason": gate.Reason, "by_severity": gate.Score.BySeverity, "by_source": gate.Score.BySource},
		Limitations: limitations,
	}
}

func toolAvailable(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func runGitleaks(ctx context.Context, dir string) ([]security.Finding, error) {
	if !toolAvailable("gitleaks") {
		return nil, errToolUnavailable("gitleaks")
	}
	out, err := os.CreateTemp("", "solidify-gitleaks-*.json")
	if err != nil {
		return nil, err
	}
	outPath := out.Name()
	_ = out.Close()
	defer os.Remove(outPath)

	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, "gitleaks", "detect", "--source", dir, "--no-git",
		"--report-format", "json", "--report-path", outPath, "--exit-code", "0")
	if runOut, runErr := cmd.CombinedOutput(); runErr != nil {
		return nil, errWithOutput("gitleaks", runErr, runOut)
	}

	data, err := os.ReadFile(outPath)
	if err != nil || len(data) == 0 {
		return nil, nil // sem findings = arquivo vazio/ausente, não é erro
	}
	var raw []struct {
		RuleID  string `json:"RuleID"`
		File    string `json:"File"`
		Line    int    `json:"StartLine"`
		Message string `json:"Description"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out2 := make([]security.Finding, 0, len(raw))
	for _, r := range raw {
		out2 = append(out2, security.Finding{
			Source: security.SourceSecrets, Rule: r.RuleID, Severity: security.SevCritical,
			FilePath: r.File, Line: r.Line, Message: r.Message,
		})
	}
	return out2, nil
}

func runOSVScanner(ctx context.Context, dir string) ([]security.Finding, error) {
	if !toolAvailable("osv-scanner") {
		return nil, errToolUnavailable("osv-scanner")
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, "osv-scanner", "--format", "json", "-r", dir)
	out, _ := cmd.Output() // exit code != 0 quando acha vuln; não é erro de execução

	var raw struct {
		Results []struct {
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
			Packages []struct {
				Package struct {
					Name string `json:"name"`
				} `json:"package"`
				Vulnerabilities []struct {
					ID       string `json:"id"`
					Summary  string `json:"summary"`
					Severity []struct {
						Score string `json:"score"`
					} `json:"severity"`
				} `json:"vulnerabilities"`
			} `json:"packages"`
		} `json:"results"`
	}
	if len(out) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	var findings []security.Finding
	for _, res := range raw.Results {
		for _, pkg := range res.Packages {
			for _, v := range pkg.Vulnerabilities {
				findings = append(findings, security.Finding{
					Source: security.SourceOSV, Rule: v.ID, Severity: osvSeverity(v.Severity),
					FilePath: res.Source.Path, Message: pkg.Package.Name + ": " + v.Summary,
				})
			}
		}
	}
	return findings, nil
}

func osvSeverity(sev []struct {
	Score string `json:"score"`
}) security.Severity {
	// ponytail: CVSS vector parsing real (score numérico) fica pra quando o
	// dashboard precisar filtrar por faixa; hoje só usamos "tem vuln ou não".
	if len(sev) == 0 {
		return security.SevMedium
	}
	return security.SevHigh
}

func runSemgrep(ctx context.Context, dir string) ([]security.Finding, error) {
	if !toolAvailable("semgrep") {
		return nil, errToolUnavailable("semgrep")
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, "semgrep", "scan", "--json", "--config=auto", dir)
	out, _ := cmd.Output() // findings ⇒ exit != 0; não é erro de execução

	var raw struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
			} `json:"extra"`
		} `json:"results"`
	}
	if len(out) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	findings := make([]security.Finding, 0, len(raw.Results))
	for _, r := range raw.Results {
		findings = append(findings, security.Finding{
			Source: security.SourceSemgrep, Rule: r.CheckID, Severity: semgrepSeverity(r.Extra.Severity),
			FilePath: r.Path, Line: r.Start.Line, Message: r.Extra.Message,
		})
	}
	return findings, nil
}

func semgrepSeverity(s string) security.Severity {
	switch s {
	case "ERROR":
		return security.SevHigh
	case "WARNING":
		return security.SevMedium
	default:
		return security.SevLow
	}
}

func errToolUnavailable(bin string) error { return &toolUnavailableErr{bin} }

type toolUnavailableErr struct{ bin string }

func (e *toolUnavailableErr) Error() string { return "binário " + e.bin + " não encontrado no PATH" }

func errWithOutput(bin string, err error, out []byte) error {
	return &execErr{bin: bin, err: err, out: string(out)}
}

type execErr struct {
	bin, out string
	err      error
}

func (e *execErr) Error() string { return e.bin + ": " + e.err.Error() + ": " + e.out }
func (e *execErr) Unwrap() error { return e.err }

// zapTargetGuard reusa o guard-rail de allowlist/prod do ZAP para
// qualquer chamada de rede ativa deste analyzer (nenhuma hoje, mas mantém
// o import intencional para o dia em que active-scan de deps for ligado).
var _ = zap.TargetGuard{}
