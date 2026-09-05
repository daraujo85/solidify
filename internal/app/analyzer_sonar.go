package app

import (
	"context"
	"log/slog"
	"strconv"
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
		"security_hotspots", "sqale_index",
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
	// Flat keys 1:1 das measures do Sonar (totais reais do projeto — mais
	// completo que contar issues filtradas por severidade); hydrate.js lê
	// direto por essas chaves, nunca fabrica "—" quando a measure existe.
	for metricKey, sonarKey := range map[string]string{
		"coverage": "coverage", "bugs": "bugs", "vulnerabilities": "vulnerabilities",
		"code_smells": "code_smells", "duplication_pct": "duplicated_lines_density",
		"security_hotspots": "security_hotspots",
	} {
		if m, ok := sonar.FindMeasure(measures, sonarKey); ok {
			metrics[metricKey] = m.Value
		}
	}
	if m, ok := sonar.FindMeasure(measures, "sqale_index"); ok {
		metrics["technical_debt"] = formatDebtMinutes(m.Value)
	}
	score := qgScoreOf(qg.Status)

	return report.Analyzer{
		ID: "sonar", Applicability: "APPLICABLE", ExecutionStatus: statusPtr("completed"),
		Score: &score, Findings: findings, Metrics: metrics,
	}
}

// formatDebtMinutes converte sqale_index (minutos) pra "XdYhZm" (Sonar-style).
func formatDebtMinutes(totalMin float64) string {
	m := int(totalMin)
	d, m := m/(8*60), m%(8*60)
	h, m := m/60, m%60
	switch {
	case d > 0:
		return sprintfDebt(d, "d", h, "h")
	case h > 0:
		return sprintfDebt(h, "h", m, "min")
	default:
		return sprintfDebt(m, "min", 0, "")
	}
}

func sprintfDebt(a int, aUnit string, b int, bUnit string) string {
	if b > 0 {
		return strconv.Itoa(a) + aUnit + strconv.Itoa(b) + bUnit
	}
	return strconv.Itoa(a) + aUnit
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
