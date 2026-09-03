// Package scheduler — execução concorrente de analyzers com limites
// por classe de recurso.
//
// SAI-027: cada analyzer tem custo diferente. Semáforo por class
// garante que dois browser jobs nunca rodam juntos (default=1). O
// scheduler executa Detect em paralelo leve e Run respeitando limites.
package scheduler

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/diegoaraujo/solidify/internal/analyzer"
)

// Config — limite de concorrência por class.
type Config struct {
	Limits map[analyzer.Class]int
}

// DefaultConfig limites por class. Browser=1 garante aceitação.
func DefaultConfig() Config {
	return Config{
		Limits: map[analyzer.Class]int{
			analyzer.ClassLight:       runtime.NumCPU(),
			analyzer.ClassCPU:         2,
			analyzer.ClassBrowser:     1,
			analyzer.ClassMemoryHeavy: 1,
			analyzer.ClassActiveNet:   2,
		},
	}
}

// Validate checa limites (>=0).
func (c Config) Validate() error {
	for class, n := range c.Limits {
		if n < 0 {
			return fmt.Errorf("scheduler: limit negativo para %s: %d", class, n)
		}
	}
	return nil
}

// Limit devolve limite de uma class (0 = sem limite).
func (c Config) Limit(class analyzer.Class) int {
	if c.Limits == nil {
		return 0
	}
	return c.Limits[class]
}

// Job representa um analyzer a executar.
type Job struct {
	Analyzer analyzer.Analyzer
	Context  analyzer.Context
}

// Result é o outcome de um job.
type Result struct {
	Analyzer string
	Result   analyzer.Result
	Error    error
}

// Scheduler — executor de jobs com semáforo por class.
type Scheduler struct {
	cfg  Config
	sems map[analyzer.Class]chan struct{}
}

// New cria scheduler com config.
func New(cfg Config) (*Scheduler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	s := &Scheduler{
		cfg:  cfg,
		sems: make(map[analyzer.Class]chan struct{}),
	}
	for class, n := range cfg.Limits {
		if n > 0 {
			s.sems[class] = make(chan struct{}, n)
		}
	}
	return s, nil
}

// MustNew panic em erro.
func MustNew(cfg Config) *Scheduler {
	s, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return s
}

// Limits devolve config de limites (cópia).
func (s *Scheduler) Limits() map[analyzer.Class]int {
	out := make(map[analyzer.Class]int, len(s.cfg.Limits))
	for k, v := range s.cfg.Limits {
		out[k] = v
	}
	return out
}

// RunAll executa todos os jobs em paralelo respeitando limites por
// class. Devolve slice de Result (mesma ordem dos jobs).
func (s *Scheduler) RunAll(ctx context.Context, jobs []Job) []Result {
	if len(jobs) == 0 {
		return nil
	}
	results := make([]Result, len(jobs))
	var wg sync.WaitGroup
	wg.Add(len(jobs))
	for i, job := range jobs {
		i, job := i, job
		go func() {
			defer wg.Done()
			results[i] = s.runOne(ctx, job)
		}()
	}
	wg.Wait()
	return results
}

// runOne executa um job adquirindo semáforo da class.
func (s *Scheduler) runOne(ctx context.Context, job Job) Result {
	class := job.Analyzer.Class()
	if sem, ok := s.sems[class]; ok {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			return Result{
				Analyzer: job.Analyzer.Name(),
				Result: analyzer.Result{
					Analyzer: job.Analyzer.Name(),
					Class:    class,
					Status:   analyzer.StatusError,
					Error:    ctx.Err().Error(),
				},
				Error: ctx.Err(),
			}
		}
	}
	res, err := analyzer.Run(ctx, job.Analyzer, job.Context)
	return Result{
		Analyzer: job.Analyzer.Name(),
		Result:   res,
		Error:    err,
	}
}

// DetectAndRun executa Detect em todos (paralelo leve) e depois Run
// dos detectados (com semáforo). Devolve todos os results, incluindo
// skipped.
func (s *Scheduler) DetectAndRun(ctx context.Context, jobs []Job) []Result {
	if len(jobs) == 0 {
		return nil
	}
	// Phase 1: detect em paralelo leve.
	type detectOut struct {
		idx      int
		detected bool
		err      error
	}
	detCh := make(chan detectOut, len(jobs))
	var wg sync.WaitGroup
	for i, job := range jobs {
		i, job := i, job
		wg.Add(1)
		go func() {
			defer wg.Done()
			det, err := job.Analyzer.Detect(ctx, job.Context)
			detCh <- detectOut{idx: i, detected: det, err: err}
		}()
	}
	wg.Wait()
	close(detCh)
	detected := make(map[int]bool, len(jobs))
	for d := range detCh {
		if d.err == nil && d.detected {
			detected[d.idx] = true
		}
	}
	// Phase 2: run só dos detectados.
	runJobs := make([]Job, 0, len(jobs))
	runIdx := make([]int, 0, len(jobs))
	for i, job := range jobs {
		if detected[i] {
			runJobs = append(runJobs, job)
			runIdx = append(runIdx, i)
		}
	}
	results := make([]Result, len(jobs))
	// Preenche skipped.
	for i := range jobs {
		if !detected[i] {
			results[i] = Result{
				Analyzer: jobs[i].Analyzer.Name(),
				Result: analyzer.Result{
					Analyzer: jobs[i].Analyzer.Name(),
					Version:  jobs[i].Analyzer.Version(),
					Class:    jobs[i].Analyzer.Class(),
					Status:   analyzer.StatusSkip,
					Skipped:  true,
					Started:  time.Now(),
					Finished: time.Now(),
				},
			}
		}
	}
	runResults := s.RunAll(ctx, runJobs)
	for k, idx := range runIdx {
		results[idx] = runResults[k]
	}
	return results
}

// ConcurrencyInFlight devolve nº de jobs rodando agora (debug).
func (s *Scheduler) ConcurrencyInFlight() map[analyzer.Class]int {
	out := make(map[analyzer.Class]int, len(s.sems))
	for class, sem := range s.sems {
		out[class] = len(sem)
	}
	return out
}

// SlotAvailable devolve true se há slot livre na class.
func (s *Scheduler) SlotAvailable(class analyzer.Class) bool {
	sem, ok := s.sems[class]
	if !ok {
		return true
	}
	return len(sem) < cap(sem)
}

// Plan é um plano de execução ordenada (pra debug/log).
type Plan struct {
	Class analyzer.Class
	Name  string
}

// Plan devolve plano de execução (class + name, ordenado por class).
func (s *Scheduler) Plan(jobs []Job) []Plan {
	plans := make([]Plan, 0, len(jobs))
	for _, j := range jobs {
		plans = append(plans, Plan{Class: j.Analyzer.Class(), Name: j.Analyzer.Name()})
	}
	sort.SliceStable(plans, func(i, j int) bool {
		if plans[i].Class != plans[j].Class {
			return plans[i].Class < plans[j].Class
		}
		return plans[i].Name < plans[j].Name
	})
	return plans
}

// AggregateByClass soma findings por class.
func AggregateByClass(results []Result) map[analyzer.Class][]analyzer.Finding {
	out := make(map[analyzer.Class][]analyzer.Finding)
	for _, r := range results {
		out[r.Result.Class] = append(out[r.Result.Class], r.Result.Findings...)
	}
	return out
}
