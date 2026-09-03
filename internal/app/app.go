// Package app contém o dispatch de comandos da CLI do Solidify.
//
// Sem framework de CLI: o roteamento é uma tabela de subcomandos sobre
// flag.FlagSet da stdlib (ver ARCHITECTURE.md, regra de dependências).
//
// Contrato de saída: stdout carrega dados, stderr carrega logs e erros.
// O exit code vem de internal/errs e é estável.
package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"

	"github.com/diegoaraujo/solidify/internal/errs"
	"github.com/diegoaraujo/solidify/internal/logging"
	"github.com/diegoaraujo/solidify/internal/version"

	// SAI-113: doctor diagnostics
	"github.com/diegoaraujo/solidify/internal/doctor"
	// SAI-087: dashboard
	"github.com/diegoaraujo/solidify/internal/dashboard"
)

const usage = `solidify — release quality gate assistido por peer review de IA

Uso:
  solidify <comando> [flags]

Comandos:
  version         mostra a versão da build
  init            prepara .solidify/ e solidify.json num repositório Git
  doctor          checa pré-requisitos do ambiente (docker, git, 9router, …)
  dashboard       serve UI estática + API de reports (SAI-087)
  mcp             serve MCP server (stdio) ou setup doc não-destrutivo (SAI-119)
  peer-reviews    stats — telemetria v1 vs v2 submissions (SAI-121)
                 migrate — converte records v1 → v2 in-place (SAI-122)
  report          exporta resultados em formatos específicos (json, pdf)
  run             (em construção) orquestra análise completa — ver docs/SETUP-LOCAL.md
  help            mostra esta ajuda

Flags globais:
  --log-level   debug|info|warn|error (default info)
  --log-format  text|json (default text); json também formata erros em JSON
`

// Env agrupa as dependências externas de uma execução da CLI, para que Run
// seja testável sem tocar em os.Stdout/os.Exit.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	// OmitLogTime desliga o timestamp dos logs (golden tests).
	OmitLogTime bool
}

// globals são as flags válidas antes do nome do subcomando.
type globals struct {
	level  slog.Level
	format logging.Format
}

// Run executa a CLI e devolve o exit code do processo.
func Run(args []string, env Env) int {
	rest, opts, err := parseGlobals(args)
	if err == nil {
		err = dispatch(rest, opts, env)
	}
	if err == nil {
		return 0
	}

	if opts.format == logging.FormatJSON {
		// Em modo JSON o stderr precisa ser parseável: nada de usage em texto.
		if renderErr := errs.RenderJSON(env.Stderr, err); renderErr != nil {
			fmt.Fprintf(env.Stderr, "erro ao serializar erro: %v\n", renderErr)
			return errs.CodeInternal.ExitCode()
		}
		return errs.PublicExitCode(err)
	}

	if renderErr := errs.RenderHuman(env.Stderr, err); renderErr != nil {
		fmt.Fprintf(env.Stderr, "erro ao renderizar erro: %v\n", renderErr)
		return errs.CodeInternal.ExitCode()
	}
	if errs.CodeOf(err) == errs.CodeUsage {
		fmt.Fprint(env.Stderr, "\n", usage)
	}
	return errs.ExitCodeOf(err)
}

// parseGlobals consome as flags globais que precedem o subcomando.
func parseGlobals(args []string) ([]string, globals, error) {
	opts := globals{level: slog.LevelInfo, format: logging.FormatText}

	for len(args) > 0 {
		arg := args[0]
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		name, value, hasValue := splitFlag(arg)
		if !hasValue {
			if len(args) < 2 {
				return nil, opts, errs.Newf(errs.CodeUsage, "flag %s exige valor", name)
			}
			value = args[1]
			args = args[1:]
		}

		switch name {
		case "log-level":
			level, ok := logging.ParseLevel(value)
			if !ok {
				return nil, opts, errs.Newf(errs.CodeUsage, "nível de log inválido: %q", value).
					WithHint("use debug, info, warn ou error")
			}
			opts.level = level
		case "log-format":
			format, ok := logging.ParseFormat(value)
			if !ok {
				return nil, opts, errs.Newf(errs.CodeUsage, "formato de log inválido: %q", value).
					WithHint("use text ou json")
			}
			opts.format = format
		default:
			return nil, opts, errs.Newf(errs.CodeUsage, "flag global desconhecida: %s", arg)
		}
		args = args[1:]
	}
	return args, opts, nil
}

// splitFlag aceita --nome=valor, --nome, -nome=valor e -nome.
func splitFlag(arg string) (name, value string, hasValue bool) {
	trimmed := arg
	for len(trimmed) > 0 && trimmed[0] == '-' {
		trimmed = trimmed[1:]
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == '=' {
			return trimmed[:i], trimmed[i+1:], true
		}
	}
	return trimmed, "", false
}

func dispatch(args []string, opts globals, env Env) error {
	if len(args) == 0 {
		return errs.New(errs.CodeUsage, "nenhum comando informado")
	}

	logger := logging.New(logging.Options{
		Out:      env.Stderr,
		Level:    opts.level,
		Format:   opts.format,
		OmitTime: env.OmitLogTime,
	})

	switch cmd := args[0]; cmd {
	case "version":
		return runVersion(args[1:], env, logger)
	case "init":
		return runInit(args[1:], env, logger)
	case "doctor":
		return runDoctor(args[1:], env, logger)
	case "dashboard":
		return runDashboard(args[1:], env, logger)
	case "report":
		return runReport(args[1:], env, logger)
	case "run":
		return runRun(args[1:], env, logger)
	case "mcp":
		return runMCP(args[1:], env, logger)
	case "peer-reviews":
		return runPeerReviews(args[1:], env, logger)
	case "help", "-h", "--help":
		_, err := io.WriteString(env.Stdout, usage)
		return err
	default:
		return errs.Newf(errs.CodeUsage, "comando desconhecido: %s", cmd)
	}
}

func runVersion(args []string, env Env, logger *slog.Logger) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "emite a versão em JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, werr := io.WriteString(env.Stdout, "uso: solidify version [--json]\n")
			return werr
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas para version", err)
	}

	info := version.Get()
	logger.Debug("versão resolvida", "version", info.Version, "commit", info.Commit)

	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			return errs.Wrap(errs.CodeIO, "falha ao escrever versão em JSON", err)
		}
		return nil
	}

	if _, err := fmt.Fprintln(env.Stdout, info.String()); err != nil {
		return errs.Wrap(errs.CodeIO, "falha ao escrever versão", err)
	}
	return nil
}

// runDoctor implementa `solidify doctor` (SAI-113).
// Roda AllChecks, imprime tabela, devolve err se algum falha.
// SAI-124: --canary restringe a checks de peer review v1.
func runDoctor(args []string, env Env, logger *slog.Logger) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	only := fs.String("only", "", "roda só o check informado (vazio = todos)")
	asJSON := fs.Bool("json", false, "emite resultado em JSON")
	canary := fs.Bool("canary", false, "roda só checks canário v1 (peer_review_canary + peer_review_store_v1) — SAI-124")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, werr := io.WriteString(env.Stdout, "uso: solidify doctor [--only=NAME] [--canary] [--json]\n")
			return werr
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas para doctor", err)
	}

	var results []doctor.CheckResult
	switch {
	case *canary:
		// SAI-124: atalho pra rodar só os 2 checks de rollout v1.
		// --only e --canary são mutuamente exclusivos.
		if *only != "" {
			return errs.New(errs.CodeUsage, "--canary e --only são mutuamente exclusivos")
		}
		results = []doctor.CheckResult{
			doctor.RunCheck("peer_review_canary"),
			doctor.RunCheck("peer_review_store_v1"),
		}
	case *only != "":
		results = []doctor.CheckResult{doctor.RunCheck(*only)}
	default:
		results = doctor.RunAll()
	}
	summary := doctor.Summarize(results)

	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Summary doctor.Summary      `json:"summary"`
			Checks  []doctor.CheckResult `json:"checks"`
		}{summary, results})
	}

	for _, r := range results {
		mark := "OK  "
		if !r.OK {
			mark = "FAIL"
		}
		fmt.Fprintf(env.Stdout, "%s  %-12s  %s\n", mark, r.Name, r.Message)
	}
	fmt.Fprintf(env.Stdout, "\n%d checks: %d OK, %d FAIL\n", summary.Total, summary.OK, summary.Fail)

	if summary.Fail > 0 {
		logger.Warn("alguns checks falharam", "fail", summary.Fail)
		return doctor.ErrCheckFailed
	}
	return nil
}

// runDashboard implementa `solidify dashboard` (SAI-087).
// Delega pra dashboard.Run — flag parsing é responsabilidade dele.
func runDashboard(args []string, env Env, logger *slog.Logger) error {
	logger.Info("dashboard iniciando", "args", args)
	if code := dashboard.Run(args, env.Stdout, env.Stderr); code != 0 {
		return errs.Newf(errs.CodeInternal, "dashboard saiu com código %d", code)
	}
	return nil
}

// runRun moved to run.go (Fase 0 — quick profile orchestrator).
