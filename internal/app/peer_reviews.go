// Solidify peer-reviews subcommand (SAI-121).
//
// Subcomandos:
//   - `solidify peer-reviews stats [--since RFC3339]` — lê JSONL de
//     telemetria (~/.solidify/metrics/peer_reviews.jsonl) e imprime
//     agregado: total, by_schema (v1 vs v2), by_actor, by_verdict,
//     oldest/newest, v1_count, v2_count, v1_fraction.
package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/diegoaraujo/solidify/internal/errs"
	"github.com/diegoaraujo/solidify/internal/mcpserver"
)

// runPeerReviews dispatcha `solidify peer-reviews <sub>`.
func runPeerReviews(args []string, env Env, logger *slog.Logger) error {
	if len(args) == 0 {
		_, _ = io.WriteString(env.Stdout, "uso: solidify peer-reviews <stats|migrate> [flags]\n")
		return nil
	}
	switch args[0] {
	case "stats":
		return runPeerReviewsStats(args[1:], env, logger)
	case "migrate":
		return runPeerReviewsMigrate(args[1:], env, logger)
	default:
		return errs.Newf(errs.CodeUsage, "subcomando peer-reviews desconhecido: %s", args[0])
	}
}

// runPeerReviewsStats lê JSONL e renderiza sumário.
func runPeerReviewsStats(args []string, env Env, logger *slog.Logger) error {
	fs := flag.NewFlagSet("peer-reviews stats", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	since := fs.String("since", "", "filtra eventos >= RFC3339 (default: todos)")
	asJSON := fs.Bool("json", false, "emite sumário em JSON")
	metricsPath := fs.String("metrics-path", "", "path do JSONL (default: ~/.solidify/metrics/peer_reviews.jsonl)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			_, _ = io.WriteString(env.Stdout, "uso: solidify peer-reviews stats [--since=RFC3339] [--metrics-path=PATH] [--json]\n")
			return nil
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas para peer-reviews stats", err)
	}
	// Override metrics path via flag ou env (sem config struct).
	if *metricsPath != "" {
		mcpserver.MetricsPath = *metricsPath
	}
	events, err := mcpserver.ReadMetrics()
	if err != nil {
		return errs.Wrap(errs.CodeIO, "ler metrics JSONL", err)
	}
	var from time.Time
	if *since != "" {
		t, perr := time.Parse(time.RFC3339, *since)
		if perr != nil {
			return errs.Newf(errs.CodeUsage, "since inválido (esperado RFC3339): %v", perr)
		}
		from = t
	}
	summary := mcpserver.SummarizeMetrics(events, from, time.Time{})
	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}
	if len(events) == 0 {
		fmt.Fprintln(env.Stdout, "nenhuma submission registrada (path vazio ou sem eventos)")
		logger.Info("peer-reviews stats vazio", "path", mcpserver.MetricsPath)
		return nil
	}
	fmt.Fprintf(env.Stdout, "Total:       %d\n", summary.Total)
	fmt.Fprintf(env.Stdout, "V1 count:    %d (%.1f%%)\n", summary.V1Count, summary.V1Fraction*100)
	fmt.Fprintf(env.Stdout, "V2 count:    %d\n", summary.V2Count)
	fmt.Fprintf(env.Stdout, "Oldest:      %s\n", summary.Oldest.Format(time.RFC3339))
	fmt.Fprintf(env.Stdout, "Newest:      %s\n", summary.Newest.Format(time.RFC3339))
	if cutoff := v1CutoffDisplay(); !cutoff.IsZero() {
		fmt.Fprintf(env.Stdout, "V1 cutoff:   %s\n", cutoff.Format(time.RFC3339))
		if summary.V1Count > 0 && time.Now().After(cutoff) {
			fmt.Fprintln(env.Stdout, "AVISO: cutoff venceu; novos submissions v1 = hard fail")
		}
	} else {
		fmt.Fprintln(env.Stdout, "V1 cutoff:   (não configurado; v1 sempre warning)")
	}
	if len(summary.BySchema) > 0 {
		fmt.Fprintln(env.Stdout, "\nBy schema:")
		for k, v := range summary.BySchema {
			fmt.Fprintf(env.Stdout, "  %-3s %d\n", k, v)
		}
	}
	if len(summary.ByActor) > 0 {
		fmt.Fprintln(env.Stdout, "\nBy actor:")
		for k, v := range summary.ByActor {
			fmt.Fprintf(env.Stdout, "  %-8s %d\n", k, v)
		}
	}
	logger.Info("peer-reviews stats", "total", summary.Total, "v1", summary.V1Count, "v2", summary.V2Count)
	return nil
}

// v1CutoffDisplay devolve cutoff global tipado (read-only).
func v1CutoffDisplay() time.Time {
	return mcpserver.V1Cutoff
}

// runPeerReviewsMigrate migra records v1 → v2 in-place (SAI-122).
//
// `solidify peer-reviews migrate [--store-dir PATH] [--actor NAME]
// [--dry-run]` — re-parseia Notes como JSON, popula
// CanonicalPayload, limpa Notes, bump schema, recomputa hash.
func runPeerReviewsMigrate(args []string, env Env, logger *slog.Logger) error {
	fs := flag.NewFlagSet("peer-reviews migrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	storeDir := fs.String("store-dir", "", "PeerReviewStore dir (default: ~/.solidify/peer_reviews)")
	actor := fs.String("actor", "", "filtra por actor (peer_a|peer_b, vazio = todos)")
	dryRun := fs.Bool("dry-run", false, "não escreve; só mostra o que seria migrado")
	asJSON := fs.Bool("json", false, "emite resultado em JSON")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			_, _ = io.WriteString(env.Stdout, "uso: solidify peer-reviews migrate [--store-dir=PATH] [--actor=NAME] [--dry-run] [--json]\n")
			return nil
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas para peer-reviews migrate", err)
	}
	dir, err := resolveStoreDir(*storeDir)
	if err != nil {
		return err
	}
	store, err := mcpserver.NewPeerReviewStore(dir)
	if err != nil {
		return errs.Wrap(errs.CodeConfig, "criar PeerReviewStore", err)
	}
	result, err := mcpserver.MigrateV1ToV2(store, mcpserver.MigrateOptions{
		DryRun: *dryRun,
		Actor:  *actor,
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "migrate", err)
	}
	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	mode := "applied"
	if *dryRun {
		mode = "(dry-run, nada foi escrito)"
	}
	fmt.Fprintf(env.Stdout, "Total in store: %d\n", result.TotalInStore)
	fmt.Fprintf(env.Stdout, "Scanned:        %d (após filtro)\n", result.Scanned)
	fmt.Fprintf(env.Stdout, "Migrated:       %d %s\n", result.Migrated, mode)
	fmt.Fprintf(env.Stdout, "Skipped v2:     %d (já migrados)\n", result.SkippedV2)
	fmt.Fprintf(env.Stdout, "Skipped bad:    %d (Notes não-JSON ou vazio)\n", result.SkippedBad)
	if len(result.Errors) > 0 {
		fmt.Fprintln(env.Stdout, "\nErrors:")
		for _, e := range result.Errors {
			fmt.Fprintf(env.Stdout, "  - %s\n", e)
		}
	}
	logger.Info("peer-reviews migrate", "scanned", result.Scanned, "migrated", result.Migrated, "dry_run", *dryRun)
	return nil
}

// silence unused os import; reservado para futuras flags que
// precisem resolver paths custom (--metrics-path).
var _ = os.Getenv
