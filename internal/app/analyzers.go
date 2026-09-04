package app

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/diegoaraujo/solidify/internal/applicability"
	"github.com/diegoaraujo/solidify/internal/component"
	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/gitx"
	"github.com/diegoaraujo/solidify/internal/report"
)

// diffPaths extrai os paths pós-mudança do diff (Path já é o destino em
// rename/copy — ver gitx.DiffFile).
func diffPaths(files []gitx.DiffFile) []string {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	return paths
}

// hasMigrationChanges: heurística simples de path — sem parser de linguagem,
// só procura o segmento "migrat" (cobre migrations/, migration/, *_migrate.go).
func hasMigrationChanges(paths []string) bool {
	for _, p := range paths {
		if strings.Contains(strings.ToLower(p), "migrat") {
			return true
		}
	}
	return false
}

// hasEnvChanges: heurística de path — .env* na raiz do path.
func hasEnvChanges(paths []string) bool {
	for _, p := range paths {
		base := p
		if idx := strings.LastIndexByte(p, '/'); idx >= 0 {
			base = p[idx+1:]
		}
		if strings.HasPrefix(base, ".env") {
			return true
		}
	}
	return false
}

// buildApplicabilityProfile monta applicability.Profile a partir do diff real
// e da config carregada — sem discovery de manifests/stack (fora de escopo
// desta wiring; ClassifyAll degrada bem com ms/ds nil, só perde o caso
// especial Flutter).
func buildApplicabilityProfile(cfg config.Config, changedPaths []string, hasMigrations, hasEnvChanges bool) applicability.Profile {
	dist := component.ClassifyAll(changedPaths, nil, nil)
	comps := make([]component.Component, 0, len(dist))
	for _, d := range dist {
		comps = append(comps, d.Component)
	}
	return applicability.Profile{
		Components:       comps,
		ChangedPaths:     changedPaths,
		HasMigrations:    hasMigrations,
		HasEnvChanges:    hasEnvChanges,
		LighthouseTarget: firstOrEmpty(cfg.Targets.Frontend),
		SonarConfigured:  cfg.Analyzers.Sonar.Enabled && cfg.Analyzers.Sonar.ProjectKey != "",
		ZAPConfigured:    cfg.Analyzers.Security.ZAPActive && len(cfg.Analyzers.Security.ActiveTargetAllowlist) > 0,
		K6Configured:     cfg.Analyzers.Load.Enabled && cfg.Analyzers.Load.Mode != "disabled" && len(cfg.Targets.API) > 0,
	}
}

// buildAnalyzers roda (ou pula, com motivo explícito) os 5 analyzers
// determinísticos. Cada um vira exatamente 1 report.Analyzer — skip
// nunca fabrica score/finding (contrato: estado vazio explícito).
func buildAnalyzers(ctx context.Context, cfg config.Config, dir string, changedPaths []string, hasMigrations, hasEnvChanges bool, logger *slog.Logger) []report.Analyzer {
	profile := buildApplicabilityProfile(cfg, changedPaths, hasMigrations, hasEnvChanges)
	decisions := applicability.Decide(profile)
	byGate := make(map[applicability.Gate]applicability.Decision, len(decisions))
	for _, d := range decisions {
		byGate[d.Gate] = d
	}

	out := make([]report.Analyzer, 0, 5)
	out = append(out, runOrSkip(ctx, byGate[applicability.GateSonar], "sonar", cfg, dir, logger, runSonarAnalyzer))
	out = append(out, runOrSkip(ctx, byGate[applicability.GateLighthouse], "lighthouse", cfg, dir, logger, runLighthouseAnalyzer))
	out = append(out, runOrSkip(ctx, byGate[applicability.GateSecurity], "security", cfg, dir, logger, runSecurityAnalyzer))
	out = append(out, runOrSkip(ctx, byGate[applicability.GateK6], "k6", cfg, dir, logger, runK6Analyzer))
	out = append(out, runCoverageAnalyzer(cfg, dir)) // coverage não depende de applicability — só de arquivo existir
	return out
}

// analyzerFunc é o formato comum de cada runner concreto.
type analyzerFunc func(ctx context.Context, cfg config.Config, dir string, logger *slog.Logger) report.Analyzer

// runOrSkip aplica o veredito de applicability ANTES de chamar o runner:
// NOT_APPLICABLE/CONDITIONAL sem config vira skip sem gastar exec/rede.
func runOrSkip(ctx context.Context, d applicability.Decision, id string, cfg config.Config, dir string, logger *slog.Logger, fn analyzerFunc) report.Analyzer {
	if d.Verdict == applicability.NotApplicable {
		return skippedAnalyzer(id, string(d.Verdict), "skipped:not_applicable", d.Reason)
	}
	if d.Verdict == applicability.Conditional {
		return skippedAnalyzer(id, string(d.Verdict), "skipped:not_configured", d.Reason)
	}
	return fn(ctx, cfg, dir, logger)
}

func skippedAnalyzer(id, applicabilityVerdict, status, reason string) report.Analyzer {
	return report.Analyzer{
		ID:              id,
		Applicability:   applicabilityVerdict,
		ExecutionStatus: statusPtr(status),
		Findings:        []map[string]any{},
		Metrics:         map[string]any{},
		Limitations:     []string{reason},
	}
}

func firstOrEmpty(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

func statusPtr(s string) *string { return &s }

// envOrEmpty lê o valor de uma variável de ambiente referenciada por nome
// (campo config *_env) — nunca loga o valor, só o usa em runtime.
func envOrEmpty(name string) string {
	if name == "" {
		return ""
	}
	return os.Getenv(name)
}
