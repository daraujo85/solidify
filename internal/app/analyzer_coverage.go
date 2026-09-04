package app

import (
	"os"
	"path/filepath"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/coverage"
	"github.com/diegoaraujo/solidify/internal/report"
)

// runCoverageAnalyzer só lê um relatório de coverage pré-gerado
// (ReportPath) — nunca roda `go test -cover` (isso é o TestsAnalyzer).
// Sem config/arquivo → skip explícito, nunca fabrica score.
func runCoverageAnalyzer(cfg config.Config, dir string) report.Analyzer {
	cv := cfg.Analyzers.Coverage
	if !cv.Enabled || cv.ReportPath == "" {
		return skippedAnalyzer("coverage", "CONDITIONAL", "skipped:not_configured",
			"analyzers.coverage.report_path não configurado")
	}
	path := cv.ReportPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return skippedAnalyzer("coverage", "APPLICABLE", "skipped:report_missing", err.Error())
	}
	defer f.Close()

	rep, err := coverage.Parse(cv.Format, f)
	if err != nil {
		return failedAnalyzer("coverage", err)
	}

	score := rep.LinePct
	findings := make([]map[string]any, 0)
	for _, wf := range rep.WorstFiles(10) {
		if wf.LinePct < cv.Threshold {
			findings = append(findings, map[string]any{
				"file_path": wf.Path, "line_pct": wf.LinePct, "severity": "medium",
			})
		}
	}
	metrics := map[string]any{
		"line_pct": rep.LinePct, "branch_pct": rep.BranchPct,
		"lines_total": rep.LinesTotal, "lines_covered": rep.LinesCovered,
		"threshold": cv.Threshold, "is_good": rep.IsGood(cv.Threshold),
	}

	return report.Analyzer{
		ID: "coverage", Applicability: "APPLICABLE", ExecutionStatus: statusPtr("completed"),
		Score: &score, Findings: findings, Metrics: metrics,
	}
}
