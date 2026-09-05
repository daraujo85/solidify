package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

	// ModeContainer precisa de `docker` no PATH; sem isso o erro viria tarde
	// como "summary não gerado" — melhor pular cedo com motivo claro.
	if k6cfg.Mode == k6.ModeContainer && !toolAvailable("docker") {
		return skippedAnalyzer("k6", "APPLICABLE", "skipped:tool_unavailable", "docker não encontrado no PATH (mode=container)")
	}

	// Quando rodando em container, target 127.0.0.1/localhost aponta pro
	// próprio container; reescreve pra host.docker.internal (mapeado via
	// --add-host em RunK6). Só aplica a hosts loopback — host externo fica
	// intacto. Mantém porta.
	runTarget := target
	if k6cfg.Mode == k6.ModeContainer {
		if t, ok := rewriteLoopbackHost(target); ok {
			runTarget = t
		}
	}

	eps := k6.PrioritizeEndpoints(cfg.Targets.API, nil, 1.0, 1.0)
	script, _, err := k6.ResolveScript(runTarget, ld.ScriptPath, eps, k6cfg)
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
	result, err := k6.RunK6(cctx, k6.RunConfig{Mode: k6cfg.Mode, Bin: bin, Target: runTarget, Script: script})
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
	// Findings agora carregam valor medido vs esperado — antes o dash só
	// mostrava "p(95)<500 failed" sem dizer qual p95 rolou.
	findings := make([]map[string]any, 0, len(result.Summary.ThresholdFailed))
	if result.Summary.P95 > k6cfg.ThresholdP95 {
		findings = append(findings, map[string]any{
			"threshold":      "p(95)<" + k6cfg.ThresholdP95.String(),
			"severity":       "medium",
			"actual_value":   result.Summary.P95.Milliseconds(),
			"expected_value": k6cfg.ThresholdP95.Milliseconds(),
			"unit":           "ms",
		})
	}
	if result.Summary.P99 > k6cfg.ThresholdP99 {
		findings = append(findings, map[string]any{
			"threshold":      "p(99)<" + k6cfg.ThresholdP99.String(),
			"severity":       "medium",
			"actual_value":   result.Summary.P99.Milliseconds(),
			"expected_value": k6cfg.ThresholdP99.Milliseconds(),
			"unit":           "ms",
		})
	}
	if result.Summary.ErrorRate > k6cfg.MaxErrorRate {
		findings = append(findings, map[string]any{
			"threshold":      fmt.Sprintf("http_req_failed:rate<%f", k6cfg.MaxErrorRate),
			"severity":       "medium",
			"actual_value":   result.Summary.ErrorRate,
			"expected_value": k6cfg.MaxErrorRate,
			"unit":           "rate",
		})
	}
	for _, t := range result.Summary.ThresholdFailed {
		// não duplicar thresholds já detalhados acima (k6 reporta em duas formas)
		alreadyCovered := false
		for _, f := range findings {
			if f["threshold"] == t {
				alreadyCovered = true
				break
			}
		}
		if alreadyCovered {
			continue
		}
		findings = append(findings, map[string]any{"threshold": t, "severity": "medium"})
	}

	return report.Analyzer{
		ID: "k6", Applicability: "APPLICABLE", ExecutionStatus: statusPtr("completed"),
		Score: &score, Findings: findings, Metrics: metrics,
	}
}

// rewriteLoopbackHost troca 127.0.0.1 / localhost por host.docker.internal
// (resolve pro gateway do docker host com --add-host). Devolve ok=false se
// o host não é loopback — não toca em host externo.
func rewriteLoopbackHost(target string) (string, bool) {
	t := strings.TrimSpace(target)
	if t == "" {
		return target, false
	}
	// encontra scheme://host:port ou scheme://host
	scheme := ""
	rest := t
	if i := strings.Index(t, "://"); i >= 0 {
		scheme = t[:i+3]
		rest = t[i+3:]
	}
	host := rest
	port := ""
	if i := strings.Index(rest, "/"); i >= 0 {
		host = rest[:i]
		if scheme == "" {
			rest = rest[i:] // preserva path/query pra host sem scheme
		}
	}
	if i := strings.LastIndex(host, ":"); i >= 0 {
		// evita cortar IPv6; aqui host é simples (loopback só).
		port = host[i:]
		host = host[:i]
	}
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "0.0.0.0":
		host = "host.docker.internal"
	default:
		return target, false
	}
	out := scheme + host + port
	if scheme == "" && strings.Contains(t, "/") {
		// preserva path/query quando target sem scheme veio com path
		slash := strings.Index(t, "/")
		out += t[slash:]
	}
	return out, true
}
