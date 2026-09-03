// Package analyzer — interface Analyzer, result normalizado, registry.
//
// SAI-026: cada analyzer implementa Detect (true/false) + Run (Result).
// Class define o custo (light/cpu/browser/memory-heavy/active-network)
// para o scheduler (SAI-027).
//
// Normalização: Result é shape único (findings, metrics, status).
// Version metadata permite auditoria e freeze de versões.
package analyzer

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Class — perfil de recurso. Scheduler usa pra definir concurrency.
type Class string

const (
	ClassLight       Class = "light"          // pure Go, ms-scale
	ClassCPU         Class = "cpu"            // CPU bound, seconds
	ClassBrowser     Class = "browser"        // headless Chrome/Playwright
	ClassMemoryHeavy Class = "memory-heavy"   // >2GB RAM
	ClassActiveNet   Class = "active-network" // faz requests externos
)

// Status do analyzer run.
type Status string

const (
	StatusOK    Status = "ok"
	StatusWarn  Status = "warn"
	StatusError Status = "error"
	StatusSkip  Status = "skip"
)

// Severity do finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Finding é uma observação do analyzer.
type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Path     string   `json:"path,omitempty"`
	Line     int      `json:"line,omitempty"`
	Rule     string   `json:"rule,omitempty"`
}

// ComponentInfo — info mínima sobre component pro analyzer.
type ComponentInfo struct {
	Name     string   `json:"name"`
	Language string   `json:"language,omitempty"`
	Paths    []string `json:"paths,omitempty"`
}

// Context — input pro Detect/Run. RepoPath = repo raiz.
type Context struct {
	RunID      string                   `json:"run_id"`
	RepoPath   string                   `json:"repo_path"`
	Workdir    string                   `json:"workdir,omitempty"`
	Files      []string                 `json:"files,omitempty"`
	Components map[string]ComponentInfo `json:"components,omitempty"`
	Metadata   map[string]string        `json:"metadata,omitempty"`
}

// Metadata devolve valor de ctx.Metadata[key], "" se ausente.
func (c Context) MetadataGet(key string) string {
	if c.Metadata == nil {
		return ""
	}
	return c.Metadata[key]
}

// Result — output normalizado de um analyzer.
type Result struct {
	Analyzer   string         `json:"analyzer"`
	Version    string         `json:"version"`
	Class      Class          `json:"class"`
	Status     Status         `json:"status"`
	Findings   []Finding      `json:"findings"`
	Metrics    map[string]any `json:"metrics,omitempty"`
	Started    time.Time      `json:"started"`
	Finished   time.Time      `json:"finished"`
	Duration   time.Duration  `json:"duration_ns"`
	Error      string         `json:"error,omitempty"`
	Skipped    bool           `json:"skipped,omitempty"`
	SkipReason string         `json:"skip_reason,omitempty"`
	Detected   bool           `json:"detected"`
}

// Analyzer interface — todo analyzer implementa.
type Analyzer interface {
	Name() string
	Version() string
	Class() Class
	Detect(ctx context.Context, c Context) (bool, error)
	Run(ctx context.Context, c Context) (Result, error)
}

// Registry — coleção thread-safe de analyzers.
type Registry struct {
	mu        sync.RWMutex
	analyzers map[string]Analyzer
}

// NewRegistry cria registry vazio.
func NewRegistry() *Registry {
	return &Registry{analyzers: map[string]Analyzer{}}
}

// Register adiciona analyzer. Substitui se nome já existe.
func (r *Registry) Register(a Analyzer) error {
	if a == nil {
		return fmt.Errorf("analyzer: nil")
	}
	name := a.Name()
	if name == "" {
		return fmt.Errorf("analyzer: empty name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.analyzers[name]; exists {
		return fmt.Errorf("analyzer: name already registered: %s", name)
	}
	r.analyzers[name] = a
	return nil
}

// MustRegister registra e panic em erro.
func (r *Registry) MustRegister(a Analyzer) {
	if err := r.Register(a); err != nil {
		panic(err)
	}
}

// Get devolve analyzer por nome.
func (r *Registry) Get(name string) (Analyzer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.analyzers[name]
	return a, ok
}

// All devolve todos os analyzers (nomes ordenados).
func (r *Registry) All() []Analyzer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.analyzers))
	for n := range r.analyzers {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Analyzer, 0, len(names))
	for _, n := range names {
		out = append(out, r.analyzers[n])
	}
	return out
}

// Names devolve nomes registrados (ordenados).
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.analyzers))
	for n := range r.analyzers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Len devolve nº de analyzers.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.analyzers)
}

// Run executa analyzer único e preenche metadata padrão.
func Run(ctx context.Context, a Analyzer, c Context) (Result, error) {
	res := Result{
		Analyzer: a.Name(),
		Version:  a.Version(),
		Class:    a.Class(),
		Started:  time.Now(),
	}
	detected, err := a.Detect(ctx, c)
	res.Detected = detected
	if err != nil {
		res.Status = StatusError
		res.Error = err.Error()
		res.Finished = time.Now()
		res.Duration = res.Finished.Sub(res.Started)
		return res, err
	}
	if !detected {
		res.Status = StatusSkip
		res.Skipped = true
		res.SkipReason = "not detected"
		res.Finished = time.Now()
		res.Duration = res.Finished.Sub(res.Started)
		return res, nil
	}
	out, err := a.Run(ctx, c)
	if err != nil {
		res.Status = StatusError
		res.Error = err.Error()
		res.Finished = time.Now()
		res.Duration = res.Finished.Sub(res.Started)
		return res, err
	}
	out.Detected = detected
	if out.Status == "" {
		out.Status = StatusOK
	}
	if out.Analyzer == "" {
		out.Analyzer = a.Name()
	}
	if out.Version == "" {
		out.Version = a.Version()
	}
	if out.Class == "" {
		out.Class = a.Class()
	}
	if out.Started.IsZero() {
		out.Started = res.Started
	}
	if out.Finished.IsZero() {
		out.Finished = time.Now()
	}
	if out.Duration == 0 {
		out.Duration = out.Finished.Sub(out.Started)
	}
	return out, nil
}

// DetectAll executa Detect em todos os analyzers. Devolve os que
// retornaram true.
func DetectAll(ctx context.Context, r *Registry, c Context) []Analyzer {
	all := r.All()
	var detected []Analyzer
	for _, a := range all {
		ok, err := a.Detect(ctx, c)
		if err == nil && ok {
			detected = append(detected, a)
		}
	}
	return detected
}

// ByClass filtra analyzers por class.
func ByClass(r *Registry, class Class) []Analyzer {
	var out []Analyzer
	for _, a := range r.All() {
		if a.Class() == class {
			out = append(out, a)
		}
	}
	return out
}

// AllClasses devolve as classes conhecidas.
func AllClasses() []Class {
	return []Class{ClassLight, ClassCPU, ClassBrowser, ClassMemoryHeavy, ClassActiveNet}
}

// IsValidClass devolve true se c é uma class conhecida.
func IsValidClass(c Class) bool {
	for _, k := range AllClasses() {
		if k == c {
			return true
		}
	}
	return false
}

// Summary agrega findings por severity.
func (r Result) Summary() map[Severity]int {
	out := make(map[Severity]int)
	for _, f := range r.Findings {
		out[f.Severity]++
	}
	return out
}

// HasCritical devolve true se algum finding é critical.
func (r Result) HasCritical() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// HighestSeverity devolve a maior severity entre os findings.
func (r Result) HighestSeverity() Severity {
	if len(r.Findings) == 0 {
		return ""
	}
	order := map[Severity]int{
		SeverityInfo: 0, SeverityLow: 1, SeverityMedium: 2,
		SeverityHigh: 3, SeverityCritical: 4,
	}
	var top Severity
	topOrder := -1
	for _, f := range r.Findings {
		o, ok := order[f.Severity]
		if !ok {
			continue
		}
		if o > topOrder {
			topOrder = o
			top = f.Severity
		}
	}
	return top
}
