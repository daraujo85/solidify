package app

import (
	"context"
	"testing"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/report"
)

func TestRunTestsAnalyzerReportsCommandFailure(t *testing.T) {
	cfg := config.Default()
	cfg.Analyzers.Tests.Commands = [][]string{{"sh", "-c", "exit 7"}}

	got := runTestsAnalyzer(context.Background(), cfg, t.TempDir())
	if got.ExecutionStatus == nil || *got.ExecutionStatus != "failed" {
		t.Fatalf("execution_status = %v, want failed", got.ExecutionStatus)
	}
	if got.Metrics["exit_code"] != 7 {
		t.Fatalf("exit_code = %v, want 7", got.Metrics["exit_code"])
	}
	if !analyzerFailed([]report.Analyzer{got}, "tests") {
		t.Fatal("failed tests analyzer must be visible to gate")
	}
}
