package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/diegoaraujo/solidify/internal/dashboard"
	"github.com/diegoaraujo/solidify/internal/errs"
	"github.com/diegoaraujo/solidify/internal/print"
	"github.com/diegoaraujo/solidify/internal/report"
)

func runReport(args []string, env Env, logger *slog.Logger) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)

	format := fs.String("format", "json", "Formato de saída: json|pdf")
	store := fs.String("store", "./out/runs", "Diretório de reports finalizados")
	runID := fs.String("run", "", "ID do run (obrigatório para pdf)")
	out := fs.String("out", "", "Caminho do arquivo de saída (obrigatório para pdf)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas", err)
	}

	switch *format {
	case "json":
		return runReportJSON(*store, *runID, env.Stdout)
	case "pdf":
		if *runID == "" {
			return errs.New(errs.CodeUsage, "--run é obrigatório para --format=pdf")
		}
		if *out == "" {
			return errs.New(errs.CodeUsage, "--out é obrigatório para --format=pdf")
		}
		return runReportPDF(*store, *runID, *out, env, logger)
	default:
		return errs.Newf(errs.CodeUsage, "formato desconhecido: %s", *format)
	}
}

func runReportJSON(store, runID string, stdout io.Writer) error {
	if runID == "" {
		// list logic if needed, but instructions just say "load report from store and print it"
		// If runID is empty, you could fail or list. We'll fail to be safe.
		return errs.New(errs.CodeUsage, "--run é obrigatório")
	}
	path := filepath.Join(store, runID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return errs.Wrap(errs.CodeIncomplete, "falha ao ler report", err)
	}
	var r report.Report
	if err := json.Unmarshal(data, &r); err != nil {
		return errs.Wrap(errs.CodeIncomplete, "report inválido", err)
	}
	
	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "erro json", err)
	}
	_, err = stdout.Write(append(out, '\n'))
	return err
}

func runReportPDF(store, runID, outPath string, env Env, logger *slog.Logger) error {
	// Port é dummy: ServeEphemeral sempre faz bind em 127.0.0.1:0 (porta
	// aleatória do SO) e ignora cfg.Port pra isso — mas buildMux() valida
	// cfg.Port != "" incondicionalmente (checagem pensada só pro caminho
	// Serve()/Run(), que monta ":"+cfg.Port). Sem isso, --format=pdf falhava
	// sempre com "port vazia" (achado da validação e2e manual do usuário).
	addr, shutdown, err := dashboard.ServeEphemeral(dashboard.Config{
		StoreDir: store,
		Port:     "0",
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "erro subindo dashboard", err)
	}
	defer shutdown(context.Background())

	// SAI-129 (achado validação e2e): front-end (web/src/main.js) usa hash
	// routing (#/run/<id>), não path routing — o mux só serve arquivos
	// estáticos, sem fallback SPA. URL antiga (/run/<id>?print=1) sempre
	// 404ava (http.FileServer não acha arquivo literal "run/<id>"); query
	// string tem que vir ANTES do hash na URL, senão vira parte do hash.
	url := fmt.Sprintf("http://%s/?print=1#/run/%s", addr, runID)
	logger.Info("gerando pdf", "url", url, "out", outPath)

	opts := print.PDFOptions{
		URL:    url,
		Output: outPath,
		Wait:   2 * time.Second,
	}

	if err := print.GeneratePDF(opts); err != nil {
		return errs.Wrap(errs.CodeInternal, "erro gerando pdf", err)
	}

	// Validate PDF
	reqs := []print.RequiredSection{
		{ID: "run_id", Marker: runID, MinHits: 1},
	}
	res, err := print.ValidatePDF(outPath, reqs)
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "erro validando pdf", err)
	}
	if !res.OK {
		return errs.New(errs.CodeIncomplete, "pdf gerado com falhas (seções incompletas)")
	}

	logger.Info("pdf gerado com sucesso", "path", outPath)
	return nil
}
