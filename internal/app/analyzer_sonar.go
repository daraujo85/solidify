package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/report"
	"github.com/diegoaraujo/solidify/internal/sonar"
)

// runSonarAnalyzer consulta o Web API do Sonar (BaseURL+ProjectKey já
// validados como "configured" pela applicability antes de chegar aqui).
// Não dispara sonar-scanner (isso exigiria um scan CI completo fora do
// escopo de `solidify run`) — só lê o estado do servidor já analisado.
func runSonarAnalyzer(ctx context.Context, cfg config.Config, dir string, logger *slog.Logger) report.Analyzer {
	sc := cfg.Analyzers.Sonar
	token := ""
	if sc.TokenEnv != "" {
		token = envOrEmpty(sc.TokenEnv)
	}
	client := sonar.NewClient(sonar.ClientConfig{HostURL: sc.BaseURL, Token: token, Timeout: 30 * time.Second})

	measuresRaw, err := client.GetMeasures(ctx, sc.ProjectKey, []string{
		"coverage", "bugs", "vulnerabilities", "code_smells", "duplicated_lines_density",
	})
	if err != nil {
		logger.Warn("sonar: GetMeasures falhou", "err", err)
		return failedAnalyzer("sonar", err)
	}
	issuesRaw, err := client.GetIssues(ctx, sc.ProjectKey, []string{"BLOCKER", "CRITICAL", "MAJOR"}, 200)
	if err != nil {
		logger.Warn("sonar: GetIssues falhou", "err", err)
		return failedAnalyzer("sonar", err)
	}
	qgRaw, err := client.GetQualityGate(ctx, sc.ProjectKey)
	if err != nil {
		logger.Warn("sonar: GetQualityGate falhou", "err", err)
		return failedAnalyzer("sonar", err)
	}

	measures := sonar.NormalizeMeasures(measuresRaw.Component.Measures)
	issues := sonar.NormalizeIssues(issuesRaw.Issues, sc.ProjectKey, nil)
	qg := sonar.NormalizeQualityGate(qgRaw)
	rep := sonar.BuildReport(sc.BaseURL, sc.ProjectKey, measures, issues, &qg)

	findings := make([]map[string]any, 0, len(rep.Issues))
	for _, iss := range rep.Issues {
		findings = append(findings, map[string]any{
			"rule": iss.Rule, "severity": string(iss.Severity), "file_path": iss.FilePath,
			"line": iss.Line, "message": iss.Message, "scope": string(iss.Scope),
		})
	}
	metrics := map[string]any{"quality_gate_status": string(qg.Status), "counts": rep.Counts}
	if m, ok := sonar.FindMeasure(measures, "coverage"); ok {
		metrics["coverage"] = m.Value
	}
	score := qgScoreOf(qg.Status)

	return report.Analyzer{
		ID: "sonar", Applicability: "APPLICABLE", ExecutionStatus: statusPtr("completed"),
		Score: &score, Findings: findings, Metrics: metrics,
	}
}

func qgScoreOf(status sonar.QGStatus) float64 {
	switch status {
	case sonar.QGOK:
		return 100
	case sonar.QGWarn:
		return 60
	default:
		return 0
	}
}

func failedAnalyzer(id string, err error) report.Analyzer {
	return report.Analyzer{
		ID: id, Applicability: "APPLICABLE", ExecutionStatus: statusPtr("failed"),
		Findings: []map[string]any{}, Metrics: map[string]any{},
		Limitations: []string{err.Error()},
	}
}
