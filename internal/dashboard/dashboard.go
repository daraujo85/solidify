// SAI-087 — `solidify dashboard` subcommand.
//
// Serve dashboard estático + API de runs. Binário único,
// zero Node no runtime. Usa http.FileServer + handlers
// JSON finos.
package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/diegoaraujo/solidify/internal/doctor"
	"github.com/diegoaraujo/solidify/internal/mcpserver"
	"github.com/diegoaraujo/solidify/internal/report"
	"github.com/diegoaraujo/solidify/web"
)

// Config entrada.
type Config struct {
	Port      string
	StoreDir  string // dir com reports JSON finalizados
	StaticDir string // opcional; default web/public
	Open      bool
}

// Run entrypoint CLI. Devolve exit code (0 ok, 2 usage, 1 internal).
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		port      = fs.String("port", "8765", "porta HTTP")
		storeDir  = fs.String("store", "./out/runs", "diretório de reports finalizados")
		staticDir = fs.String("static", "./web/public", "diretório estático")
		open      = fs.Bool("open", false, "abre browser após iniciar")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg := Config{
		Port:      *port,
		StoreDir:  *storeDir,
		StaticDir: *staticDir,
		Open:      *open,
	}
	if err := serve(cfg, stdout); err != nil {
		fmt.Fprintln(stderr, "dashboard:", err)
		return 1
	}
	return 0
}

func serve(cfg Config, stdout io.Writer) error {
	return Serve(cfg, stdout)
}

func buildMux(cfg Config) (*http.ServeMux, *runIndex, error) {
	if cfg.Port == "" {
		return nil, nil, errors.New("port vazia")
	}
	idx, err := loadIndex(cfg.StoreDir)
	if err != nil {
		return nil, nil, fmt.Errorf("store: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(idx.list())
	})
	mux.HandleFunc("/api/run/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/run/")
		if id == "" {
			http.Error(w, "run id vazio", 400)
			return
		}
		rep, err := idx.get(id)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rep)
	})
	// SAI-125: trend widget — v1_fraction ao longo do tempo.
	mux.HandleFunc("/api/peer-reviews/trend", func(w http.ResponseWriter, r *http.Request) {
		events, err := mcpserver.ReadMetrics()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		trend := mcpserver.ComputeTrend(events, 30, time.Time{}, doctor.CanaryFractionThreshold)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(trend)
	})

	// SAI-129 (achado validação e2e): index.html referencia
	// /src/main.js (web/src, sem bundler), mas só web/public era
	// servido — JS do dashboard sempre 404ava, dashboard e PDF
	// renderizavam sem dado nenhum (só header estático). Fix: expõe
	// /src/ também, tanto via disco (dir irmã de StaticDir) quanto
	// via embed (web.Public raiz, que contém "public/" e "src/").
	if cfg.StaticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(cfg.StaticDir)))
		mux.Handle("/src/", http.FileServer(http.Dir(filepath.Dir(cfg.StaticDir))))
	} else {
		sub, err := fs.Sub(web.Public, "public")
		if err != nil {
			return nil, nil, fmt.Errorf("embed sub: %w", err)
		}
		mux.Handle("/", http.FileServer(http.FS(sub)))
		mux.Handle("/src/", http.FileServer(http.FS(web.Public)))
	}
	return mux, idx, nil
}

func Serve(cfg Config, stdout io.Writer) error {
	mux, _, err := buildMux(cfg)
	if err != nil {
		return err
	}
	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		fmt.Fprintln(stdout, "dashboard shuting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			fmt.Fprintf(stdout, "dashboard shutdown error: %v\n", err)
		}
		close(idleConnsClosed)
	}()

	fmt.Fprintln(stdout, "dashboard servindo em http://localhost"+addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	<-idleConnsClosed
	return nil
}

func ServeEphemeral(cfg Config) (addr string, shutdown func(context.Context) error, err error) {
	mux, _, err := buildMux(cfg)
	if err != nil {
		return "", nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	srv := &http.Server{
		Handler: mux,
	}
	go srv.Serve(ln)
	return ln.Addr().String(), srv.Shutdown, nil
}

// runIndex releia o storeDir a cada list()/get() (protegido por
// RWMutex) em vez de indexar só no boot — sem isso, um run novo
// gerado após o dashboard subir nunca aparecia sem restart manual
// (achado no debug de "só mostra Avaliador A": era run antigo, o
// run5 nem tinha sido indexado ainda). Sem watcher/fsnotify: store
// tem poucos runs, WalkDir+parse por request é barato e sempre
// consistente com o disco.
type runIndex struct {
	dir  string
	mu   sync.RWMutex
	runs map[string]*report.Report
}

func loadIndex(dir string) (*runIndex, error) {
	idx := &runIndex{dir: dir}
	if err := idx.refresh(); err != nil {
		return nil, err
	}
	return idx, nil
}

func (i *runIndex) refresh() error {
	runs := map[string]*report.Report{}
	if i.dir != "" {
		if _, err := os.Stat(i.dir); err == nil {
			err := filepath.WalkDir(i.dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				if !strings.HasSuffix(path, ".json") {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				var r report.Report
				if err := json.Unmarshal(data, &r); err != nil {
					return nil
				}
				if r.Run.ID != "" {
					runs[r.Run.ID] = &r
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	i.mu.Lock()
	i.runs = runs
	i.mu.Unlock()
	return nil
}

func (i *runIndex) list() []runSummary {
	i.refresh() // ignora erro de reload pontual; mantém último índice válido
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := make([]runSummary, 0, len(i.runs))
	for _, r := range i.runs {
		out = append(out, summarize(r))
	}
	sort.Slice(out, func(a, b int) bool { return out[a].RunID < out[b].RunID })
	return out
}

func (i *runIndex) get(id string) (*report.Report, error) {
	i.refresh()
	i.mu.RLock()
	defer i.mu.RUnlock()
	r, ok := i.runs[id]
	if !ok {
		return nil, errors.New("run não encontrado")
	}
	return r, nil
}

type runSummary struct {
	RunID       string   `json:"run_id"`
	Profile     string   `json:"profile"`
	Quality     *float64 `json:"quality"`
	Grade       string   `json:"grade"`
	RiskLevel   string   `json:"risk_level"`
	GateStatus  string   `json:"gate_status"`
	FinalizedAt string   `json:"finalized_at"`
	// ProjectName/ProjectPath — basename/caminho absoluto do --dir
	// analisado (RunInfo, capturado em runRun); BaseRef/HeadRef — branches
	// do diff. Únicos dados reais de "qual projeto"/escopo disponíveis
	// pra distinguir runs na lista.
	ProjectName string `json:"project_name"`
	ProjectPath string `json:"project_path"`
	BaseRef     string `json:"base_ref"`
	HeadRef     string `json:"head_ref"`
}

func summarize(r *report.Report) runSummary {
	return runSummary{
		RunID:       r.Run.ID,
		Profile:     r.Run.Profile,
		Quality:     r.Scores.Quality,
		Grade:       r.Scores.Grade,
		RiskLevel:   r.Risk.Level,
		GateStatus:  r.QualityGate.Status,
		FinalizedAt: r.Run.FinishedAt,
		ProjectName: r.Run.ProjectName,
		ProjectPath: r.Run.ProjectPath,
		BaseRef:     r.Git.BaseRef,
		HeadRef:     r.Git.HeadRef,
	}
}
