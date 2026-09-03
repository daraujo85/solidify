package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/diegoaraujo/solidify/internal/analyzer"
)

// stubAnalyzer — analyzer para scheduler.
type stubAnalyzer struct {
	name        string
	class       analyzer.Class
	delay       time.Duration
	detect      bool
	inFlight    *atomic.Int32
	maxInFlight *atomic.Int32
	fail        bool
}

func (s *stubAnalyzer) Name() string          { return s.name }
func (s *stubAnalyzer) Version() string       { return "1" }
func (s *stubAnalyzer) Class() analyzer.Class { return s.class }
func (s *stubAnalyzer) Detect(_ context.Context, _ analyzer.Context) (bool, error) {
	return s.detect, nil
}
func (s *stubAnalyzer) Run(ctx context.Context, _ analyzer.Context) (analyzer.Result, error) {
	if s.inFlight != nil {
		c := s.inFlight.Add(1)
		if s.maxInFlight != nil {
			for {
				m := s.maxInFlight.Load()
				if c <= m || s.maxInFlight.CompareAndSwap(m, c) {
					break
				}
			}
		}
		defer s.inFlight.Add(-1)
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return analyzer.Result{Status: analyzer.StatusError, Error: ctx.Err().Error()}, ctx.Err()
		}
	}
	if s.fail {
		return analyzer.Result{Status: analyzer.StatusError, Error: "fail"}, errors.New("fail")
	}
	return analyzer.Result{Status: analyzer.StatusOK}, nil
}

// Aceitação: browser jobs nunca simultâneos por default.
func TestBrowserConcurrencyLimit(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	s := MustNew(DefaultConfig())
	jobs := make([]Job, 4)
	for i := 0; i < 4; i++ {
		jobs[i] = Job{
			Analyzer: &stubAnalyzer{
				name:        "browser-" + string(rune('a'+i)),
				class:       analyzer.ClassBrowser,
				delay:       50 * time.Millisecond,
				inFlight:    &inFlight,
				maxInFlight: &maxInFlight,
			},
			Context: analyzer.Context{},
		}
	}
	s.RunAll(context.Background(), jobs)
	if maxInFlight.Load() > 1 {
		t.Errorf("browser max in flight = %d, quero <=1", maxInFlight.Load())
	}
}

// Aceitação: light jobs paralelos até NumCPU.
func TestLightConcurrencyLimit(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	s := MustNew(DefaultConfig())
	n := 8
	jobs := make([]Job, n)
	for i := 0; i < n; i++ {
		jobs[i] = Job{
			Analyzer: &stubAnalyzer{
				name:        "light-" + string(rune('a'+i)),
				class:       analyzer.ClassLight,
				delay:       50 * time.Millisecond,
				inFlight:    &inFlight,
				maxInFlight: &maxInFlight,
			},
		}
	}
	s.RunAll(context.Background(), jobs)
	limit := s.Limits()[analyzer.ClassLight]
	if int32(limit) > 0 && maxInFlight.Load() > int32(limit) {
		t.Errorf("light max in flight = %d, quero <=%d", maxInFlight.Load(), limit)
	}
}

// Aceitação: results na mesma ordem dos jobs.
func TestRunAllOrder(t *testing.T) {
	s := MustNew(DefaultConfig())
	jobs := []Job{
		{Analyzer: &stubAnalyzer{name: "a", class: analyzer.ClassLight}, Context: analyzer.Context{}},
		{Analyzer: &stubAnalyzer{name: "b", class: analyzer.ClassLight}, Context: analyzer.Context{}},
		{Analyzer: &stubAnalyzer{name: "c", class: analyzer.ClassLight}, Context: analyzer.Context{}},
	}
	res := s.RunAll(context.Background(), jobs)
	if res[0].Analyzer != "a" || res[1].Analyzer != "b" || res[2].Analyzer != "c" {
		t.Errorf("order: %v", res)
	}
}

// Aceitação: RunAll vazio.
func TestRunAllEmpty(t *testing.T) {
	s := MustNew(DefaultConfig())
	if got := s.RunAll(context.Background(), nil); got != nil {
		t.Errorf("got = %v", got)
	}
}

// Aceitação: contexto cancelado → error.
func TestContextCancel(t *testing.T) {
	cfg := Config{Limits: map[analyzer.Class]int{analyzer.ClassLight: 1}}
	s := MustNew(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	jobs := []Job{{
		Analyzer: &stubAnalyzer{name: "x", class: analyzer.ClassLight, detect: true, delay: 100 * time.Millisecond},
	}}
	res := s.RunAll(ctx, jobs)
	if res[0].Error == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Config.Validate.
func TestConfigValidate(t *testing.T) {
	if err := (Config{Limits: map[analyzer.Class]int{analyzer.ClassLight: -1}}).Validate(); err == nil {
		t.Errorf("devia falhar")
	}
	if err := (Config{Limits: map[analyzer.Class]int{analyzer.ClassLight: 1}}).Validate(); err != nil {
		t.Errorf("err: %v", err)
	}
}

// Aceitação: Config.Limit.
func TestConfigLimit(t *testing.T) {
	c := DefaultConfig()
	if c.Limit(analyzer.ClassBrowser) != 1 {
		t.Errorf("browser default = %d", c.Limit(analyzer.ClassBrowser))
	}
	if c.Limit(analyzer.ClassLight) == 0 {
		t.Errorf("light default zero")
	}
	if (Config{}).Limit(analyzer.ClassLight) != 0 {
		t.Errorf("nil limits")
	}
}

// Aceitação: New com config inválida.
func TestNewInvalid(t *testing.T) {
	if _, err := New(Config{Limits: map[analyzer.Class]int{analyzer.ClassLight: -1}}); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: class sem limite roda sem semáforo.
func TestNoLimitClass(t *testing.T) {
	cfg := Config{Limits: map[analyzer.Class]int{analyzer.ClassLight: 0}}
	s := MustNew(cfg)
	if !s.SlotAvailable(analyzer.ClassLight) {
		t.Errorf("sem limite devia estar disponível")
	}
}

// Aceitação: SlotAvailable.
func TestSlotAvailable(t *testing.T) {
	cfg := Config{Limits: map[analyzer.Class]int{analyzer.ClassBrowser: 1}}
	s := MustNew(cfg)
	if !s.SlotAvailable(analyzer.ClassBrowser) {
		t.Errorf("devia estar livre")
	}
	// Simula ocupação.
	s.sems[analyzer.ClassBrowser] <- struct{}{}
	defer func() { <-s.sems[analyzer.ClassBrowser] }()
	if s.SlotAvailable(analyzer.ClassBrowser) {
		t.Errorf("devia estar ocupado")
	}
}

// Aceitação: ConcurrencyInFlight.
func TestConcurrencyInFlight(t *testing.T) {
	s := MustNew(DefaultConfig())
	m := s.ConcurrencyInFlight()
	if _, ok := m[analyzer.ClassBrowser]; !ok {
		t.Errorf("browser sumiu")
	}
	if m[analyzer.ClassBrowser] != 0 {
		t.Errorf("inicial = %d", m[analyzer.ClassBrowser])
	}
}

// Aceitação: DetectAndRun skipped não-detectados.
func TestDetectAndRunSkipped(t *testing.T) {
	s := MustNew(DefaultConfig())
	jobs := []Job{
		{Analyzer: &stubAnalyzer{name: "no", class: analyzer.ClassLight, detect: false}, Context: analyzer.Context{}},
		{Analyzer: &stubAnalyzer{name: "yes", class: analyzer.ClassLight, detect: true}, Context: analyzer.Context{}},
	}
	res := s.DetectAndRun(context.Background(), jobs)
	if res[0].Result.Status != analyzer.StatusSkip {
		t.Errorf("no devia ser skip: %+v", res[0])
	}
	if res[1].Result.Status != analyzer.StatusOK {
		t.Errorf("yes devia ser ok: %+v", res[1])
	}
}

// Aceitação: DetectAndRun vazio.
func TestDetectAndRunEmpty(t *testing.T) {
	s := MustNew(DefaultConfig())
	if got := s.DetectAndRun(context.Background(), nil); got != nil {
		t.Errorf("got = %v", got)
	}
}

// Aceitação: Plan ordena.
func TestPlan(t *testing.T) {
	s := MustNew(DefaultConfig())
	jobs := []Job{
		{Analyzer: &stubAnalyzer{name: "z", class: analyzer.ClassLight}},
		{Analyzer: &stubAnalyzer{name: "a", class: analyzer.ClassBrowser}},
	}
	p := s.Plan(jobs)
	if p[0].Class != analyzer.ClassBrowser || p[0].Name != "a" {
		t.Errorf("order: %v", p)
	}
}

// Aceitação: AggregateByClass.
func TestAggregateByClass(t *testing.T) {
	results := []Result{
		{Result: analyzer.Result{Class: analyzer.ClassLight, Findings: []analyzer.Finding{{Code: "1"}}}},
		{Result: analyzer.Result{Class: analyzer.ClassBrowser, Findings: []analyzer.Finding{{Code: "2"}}}},
		{Result: analyzer.Result{Class: analyzer.ClassLight, Findings: []analyzer.Finding{{Code: "3"}}}},
	}
	m := AggregateByClass(results)
	if len(m[analyzer.ClassLight]) != 2 {
		t.Errorf("light = %d", len(m[analyzer.ClassLight]))
	}
	if len(m[analyzer.ClassBrowser]) != 1 {
		t.Errorf("browser = %d", len(m[analyzer.ClassBrowser]))
	}
}

// Aceitação: Limits devolve cópia.
func TestLimitsCopy(t *testing.T) {
	s := MustNew(DefaultConfig())
	m := s.Limits()
	m[analyzer.ClassBrowser] = 999
	if s.Limits()[analyzer.ClassBrowser] == 999 {
		t.Errorf("compartilhou map")
	}
}

// Aceitação: run error preservado.
func TestRunError(t *testing.T) {
	s := MustNew(DefaultConfig())
	jobs := []Job{{
		Analyzer: &stubAnalyzer{name: "x", class: analyzer.ClassLight, detect: true, fail: true},
	}}
	res := s.RunAll(context.Background(), jobs)
	if res[0].Error == nil {
		t.Errorf("devia falhar")
	}
	if res[0].Result.Status != analyzer.StatusError {
		t.Errorf("status = %s", res[0].Result.Status)
	}
}
