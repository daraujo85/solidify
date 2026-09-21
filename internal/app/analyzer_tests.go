package app

import (
	"context"
	"strings"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/report"
	"github.com/diegoaraujo/solidify/internal/testexec"
)

// runTestsAnalyzer executa comandos argv configurados. Ele serve também para
// builds: o gate só precisa saber se o comando obrigatório terminou com sucesso.
func runTestsAnalyzer(ctx context.Context, cfg config.Config, dir string) report.Analyzer {
	if !cfg.Analyzers.Tests.Enabled || len(cfg.Analyzers.Tests.Commands) == 0 {
		return skippedAnalyzer("tests", "CONDITIONAL", "skipped:not_configured", "analyzers.tests.commands não configurado", "")
	}

	exec := testexec.NewExecutor()
	commands := make([]string, 0, len(cfg.Analyzers.Tests.Commands))
	var limitations []string
	durationMS := int64(0)
	failed := false
	lastExitCode := 0
	for _, argv := range cfg.Analyzers.Tests.Commands {
		cmd := testexec.Command{Bin: argv[0], Args: argv[1:], Cwd: dir, Source: "config"}
		out, _ := exec.Execute(ctx, cmd, "")
		commands = append(commands, out.Command)
		durationMS += out.Duration.Milliseconds()
		lastExitCode = out.ExitCode
		if !out.Passed {
			failed = true
			limitations = append(limitations, strings.TrimSpace(out.Error+"\n"+out.Stderr))
		}
	}
	status := "completed"
	if failed {
		status = "failed"
	}
	return report.Analyzer{
		ID: "tests", Applicability: "APPLICABLE", ExecutionStatus: statusPtr(status), DurationMS: int(durationMS),
		Findings: []map[string]any{}, Metrics: map[string]any{"commands": commands, "exit_code": lastExitCode, "passed": !failed},
		Limitations: limitations,
	}
}
