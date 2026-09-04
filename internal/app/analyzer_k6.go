package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/k6"
	"github.com/diegoaraujo/solidify/internal/report"
	"github.com/diegoaraujo/solidify/internal/zap"
)

// runK6Analyzer roda um smoke test k6 contra cfg.Targets.API[0] (applicability
// já garantiu K6Configured). Endpoints vêm da lista fixa declarada pelo
// usuário — não construímos parser de rotas por linguagem, fora de escopo.
// Guard-rail obrigatório: zap.TargetGuard reusa a mesma allowlist/prod do
// ZAP, sem duplicar lógica; só prossegue se validar.
func runK6Analyzer(ctx context.Context, cfg config.Config, dir string, logger *slog.Logger) report.Analyzer {
	ld := cfg.Analyzers.Load
	if len(cfg.Targets.API) == 0 {
		return skippedAnalyzer("k6", "CONDITIONAL", "skipped:not_configured", "targets.api vazio")
	}
	target := cfg.Targets.API[0]

	guard := &zap.TargetGuard{Allowlist: zap.DefaultAllowlist(), AllowProd: ld.AllowProd}
	if err := guard.Validate(target); err != nil {
		return skippedAnalyzer("k6", "APPLICABLE", "skipped:target_not_allowed", err.Error())
	}

	k6cfg := k6.Config{
		Mode:         k6.Mode(ld.Mode),
		ScriptPath:   ld.ScriptPath,
		VUs:          ld.VUs,
		Duration:     time.Duration(ld.DurationSeconds) * time.Second,
		ThresholdP95: time.Duration(ld.ThresholdP95MS) * time.Millisecond,
		ThresholdP99: time.Duration(ld.ThresholdP99MS) * time.Millisecond,
		MaxErrorRate: ld.MaxErrorRate,
		AllowProd:    ld.AllowProd,
	}
	if k6cfg.Mode == "" || k6cfg.Mode == k6.ModeDisabled {
		return skippedAnalyzer("k6", "CONDITIONAL", "skipped:not_configured", "analyzers.load.mode = disabled")
	}

	eps := k6.PrioritizeEndpoints(cfg.Targets.API, nil, 1.0, 1.0)
	script, _, err := k6.ResolveScript(target, ld.ScriptPath, eps, k6cfg)
	if err != nil {
		return failedAnalyzer("k6", err)
	}
	if script == nil {
		return skippedAnalyzer("k6", "APPLICABLE", "skipped:not_configured", "nenhum script/endpoint disponível")
	}

	bin := "k6"
	if !toolAvailable(bin) && k6cfg.Mode == k6.ModeBinary {
		return skippedAnalyzer("k6", "APPLICABLE", "skipped:tool_unavailable", "binário k6 não encontrado no PATH")
	}

	cctx, cancel := context.WithTimeout(ctx, k6cfg.Duration+2*time.Minute)
	defer cancel()
	result, err := k6.RunK6(cctx, k6.RunConfig{Mode: k6cfg.Mode, Bin: bin, Target: target, Script: script})
	if err != nil {
		logger.Warn("k6: execução falhou", "err", err)
		return failedAnalyzer("k6", err)
	}

	pass := result.Summary.PassThresholds(k6cfg)
	score := 100.0
	if !pass {
		score = 100.0 * (1.0 - result.Summary.ErrorRate)
		if score < 0 {
			score = 0
		}
	}
	metrics := map[string]any{
		"p95_ms": result.Summary.P95.Milliseconds(), "p99_ms": result.Summary.P99.Milliseconds(),
		"error_rate": result.Summary.ErrorRate, "requests": result.Summary.Requests,
		"throughput_rps": result.Summary.Throughput, "pass_thresholds": pass,
	}
	findings := make([]map[string]any, 0, len(result.Summary.ThresholdFailed))
	for _, t := range result.Summary.ThresholdFailed {
		findings = append(findings, map[string]any{"threshold": t, "severity": "medium"})
	}

	return report.Analyzer{
		ID: "k6", Applicability: "APPLICABLE", ExecutionStatus: statusPtr("completed"),
		Score: &score, Findings: findings, Metrics: metrics,
	}
}
