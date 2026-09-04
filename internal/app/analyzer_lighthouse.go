package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/lighthouse"
	"github.com/diegoaraujo/solidify/internal/report"
)

// runLighthouseAnalyzer roda o CLI `lighthouse` headless contra o target
// configurado (applicability já garantiu frontend-web + target presentes).
// Sem o binário no PATH → skip explícito (nunca inventa score).
func runLighthouseAnalyzer(ctx context.Context, cfg config.Config, dir string, logger *slog.Logger) report.Analyzer {
	target := cfg.Targets.Frontend[0] // não-vazio: applicability.LighthouseTarget veio daqui

	bin := "lighthouse"
	if _, err := exec.LookPath(bin); err != nil {
		return skippedAnalyzer("lighthouse", "APPLICABLE", "skipped:tool_unavailable",
			"binário lighthouse não encontrado no PATH")
	}

	outFile, err := os.CreateTemp("", "solidify-lighthouse-*.json")
	if err != nil {
		return failedAnalyzer("lighthouse", err)
	}
	outPath := outFile.Name()
	_ = outFile.Close()
	defer os.Remove(outPath)

	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cctx, bin, target,
		"--output=json", "--output-path="+outPath,
		"--chrome-flags=--headless --no-sandbox", "--quiet")
	cmd.Dir = dir
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		logger.Warn("lighthouse: execução falhou", "err", runErr, "output", string(out))
		return failedAnalyzer("lighthouse", errors.New("lighthouse CLI falhou: "+string(out)))
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return failedAnalyzer("lighthouse", err)
	}
	rep, err := lighthouse.ParseLighthouseJSON(data, target, cfg.Analyzers.Lighthouse.Mode)
	if err != nil {
		return failedAnalyzer("lighthouse", err)
	}

	findings := make([]map[string]any, 0, len(rep.FailedAudits(0.5)))
	for _, id := range rep.FailedAudits(0.5) {
		findings = append(findings, map[string]any{"audit_id": id, "severity": "medium"})
	}
	metrics := map[string]any{}
	metricsB, _ := json.Marshal(rep.Pillar)
	_ = json.Unmarshal(metricsB, &metrics)
	metrics["aggregate_score"] = float64(rep.AggregateScore)

	score := float64(rep.AggregateScore.ToPercent())
	return report.Analyzer{
		ID: "lighthouse", Applicability: "APPLICABLE", ExecutionStatus: statusPtr("completed"),
		Score: &score, Findings: findings, Metrics: metrics,
	}
}
