// Solidify mcp subcommand (SAI-119).
//
// Subcomandos:
//   - `solidify mcp serve` — inicia MCP server stdio com tools
//     solidify_begin_review / solidify_submit_peer_review /
//     solidify_evidence_get. Claude Code / OpenCode registram
//     este server via snippet do ADR-0044. Agente envia peer_a
//     verdict chamando solidify_submit_peer_review, que persiste
//     em PeerReviewStore no diretório configurado.
//
//   - `solidify mcp setup [--target claude-code|codex|generic]
//     [--binary PATH]` — imprime setup doc não-destrutivo
//     (snippet + passos). Caller decide onde aplicar.
package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"

	"github.com/diegoaraujo/solidify/internal/errs"
	"github.com/diegoaraujo/solidify/internal/mcpserver"
)

// resolveStoreDir resolve o store dir com fallback para default
// (~/.solidify/peer_reviews/) quando config não passar.
func resolveStoreDir(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// fallback raro: HOME env
		home = os.Getenv("HOME")
	}
	if home == "" {
		return "", errs.New(errs.CodeIO, "home dir não resolvível para peer_a store")
	}
	return filepath.Join(home, ".solidify", "peer_reviews"), nil
}

// runMCP dispatcha `solidify mcp <sub>`.
func runMCP(args []string, env Env, logger *slog.Logger) error {
	if len(args) == 0 {
		_, _ = io.WriteString(env.Stdout, "uso: solidify mcp <serve|setup> [flags]\n")
		return nil
	}
	switch args[0] {
	case "serve":
		return runMCPServe(args[1:], env, logger)
	case "setup":
		return runMCPSetup(args[1:], env, logger)
	default:
		return errs.Newf(errs.CodeUsage, "subcomando mcp desconhecido: %s", args[0])
	}
}

// runMCPServe inicia o MCP server com tools canônicos.
// Bloqueia até EOF no stdin (Claude Code desconecta).
func runMCPServe(args []string, env Env, logger *slog.Logger) error {
	fs := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	storeDir := fs.String("store-dir", "", "PeerReviewStore dir (default: ~/.solidify/peer_reviews)")
	artifactDir := fs.String("artifact-dir", "", "base dir para evidence_get (default: cwd)")
	metricsPath := fs.String("metrics-path", "", "JSONL de telemetria (default: ~/.solidify/metrics/peer_reviews.jsonl)")
	v1Cutoff := fs.String("v1-cutoff", "", "data RFC3339 após a qual v1 vira hard error (vazio: sempre warning)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			_, _ = io.WriteString(env.Stdout, "uso: solidify mcp serve [--store-dir=PATH] [--artifact-dir=PATH]\n")
			return nil
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas para mcp serve", err)
	}
	dir, err := resolveStoreDir(*storeDir)
	if err != nil {
		return err
	}
	if *artifactDir == "" {
		cwd, _ := os.Getwd()
		*artifactDir = cwd
	}
	// SAI-121: telemetria + v1 cutoff globais (carregados aqui pra
	// validator usar em runtime). Override via flag CLI; default
	// respeita config se caller setou antes.
	if *metricsPath != "" {
		mcpserver.MetricsPath = *metricsPath
	}
	if err := mcpserver.SetV1Cutoff(*v1Cutoff); err != nil {
		return errs.Wrap(errs.CodeUsage, "v1-cutoff inválido", err)
	}
	store, err := mcpserver.NewPeerReviewStore(dir)
	if err != nil {
		return errs.Wrap(errs.CodeConfig, "criar PeerReviewStore", err)
	}
	logger.Info("mcp serve start", "store_dir", dir, "artifact_dir", *artifactDir)
	srv := mcpserver.New()
	// Tools canônicos: submit_peer_review usa store, evidence_get
	// usa artifact_dir, begin_review fica wired mínimo (não usado
	// diretamente pelo orchestrator — agente cria run local).
	srv.RegisterSubmitPeerReview(mcpserver.DefaultSubmitPeerReviewHandler(store, mcpserver.DefaultPeerReviewValidator()))
	srv.RegisterEvidenceGet(mcpserver.DefaultEvidenceHandler(*artifactDir))
	srv.RegisterBeginReview(func(ctx context.Context, in *mcpserver.BeginReviewInput) (*mcpserver.BeginReviewOutput, error) {
		// SAI-119 MVP: retorna contract refs fixo. SAI-120+ pluga
		// no orchestrator real (criar run via runRun).
		return &mcpserver.BeginReviewOutput{
			RunID:        "stub-begin-review",
			Schema:       mcpserver.PeerReviewSchemaVersion,
			Instructions: "Use solidify_evidence_get para inspecionar artefatos; chame solidify_submit_peer_review com verdict + canonical schema em notes.",
		}, nil
	})
	ctx := context.Background()
	if err := srv.Run(ctx); err != nil {
		return errs.Wrap(errs.CodeInternal, "mcp serve run", err)
	}
	logger.Info("mcp serve exit")
	return nil
}

// runMCPSetup imprime setup doc não-destrutivo.
func runMCPSetup(args []string, env Env, logger *slog.Logger) error {
	fs := flag.NewFlagSet("mcp setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	target := fs.String("target", "claude-code", "target: claude-code | codex | generic")
	binary := fs.String("binary", "solidify", "binário a referenciar no snippet")
	asMarkdown := fs.Bool("md", false, "renderiza como markdown (default plain)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			_, _ = io.WriteString(env.Stdout, "uso: solidify mcp setup [--target=T] [--binary=PATH] [--md]\n")
			return nil
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas para mcp setup", err)
	}
	doc := mcpserver.GenerateSetupDoc(mcpserver.SetupOptions{
		BinaryPath: *binary,
		Target:     mcpserver.SetupTarget(*target),
	})
	var out string
	if *asMarkdown {
		out = doc.Markdown()
	} else {
		out = doc.PlainText()
	}
	_, _ = io.WriteString(env.Stdout, out+"\n")
	// log no stderr pra audit (não polui o snippet)
	if u, _ := user.Current(); u != nil {
		logger.Info("mcp setup rendered", "target", *target, "binary", *binary, "user", u.Username)
	}
	return nil
}

// silence unused fmt import; usado se version flags evoluírem.
var _ = fmt.Sprintf