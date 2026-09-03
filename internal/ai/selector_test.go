package ai

import (
	"context"
	"strings"
	"testing"
)

// Aceitação: DefaultSelectorOptions.
func TestDefaultOptions(t *testing.T) {
	opts := DefaultSelectorOptions()
	if opts.Mode != SelectorModeDiscover {
		t.Errorf("mode default")
	}
	if opts.Distinctness != DistinctnessProvider {
		t.Errorf("distinctness default")
	}
	if opts.MinModels != 2 {
		t.Errorf("min models")
	}
}

// Aceitação: NewSelector nil cache.
func TestNewSelectorNilCache(t *testing.T) {
	s := NewSelector(DefaultSelectorOptions(), nil)
	if s.probes == nil {
		t.Errorf("cache nil")
	}
}

// Aceitação: Select no providers.
func TestSelectNoProviders(t *testing.T) {
	s := NewSelector(DefaultSelectorOptions(), nil)
	if _, err := s.Select(context.Background(), nil); err == nil {
		t.Errorf("sem providers")
	}
}

// Aceitação: Pinned mode.
func TestSelectPinned(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:   SelectorModePinned,
		Pinned: []string{"mock-1"},
	}, nil)
	r, err := s.Select(context.Background(), []Provider{p})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(r.Decisions) != 1 {
		t.Errorf("decisions: %d", len(r.Decisions))
	}
	if r.Decisions[0].Source != "pinned" {
		t.Errorf("source")
	}
}

// Aceitação: Pinned model missing.
func TestSelectPinnedMissing(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:   SelectorModePinned,
		Pinned: []string{"nonexistent"},
	}, nil)
	r, _ := s.Select(context.Background(), []Provider{p})
	if len(r.Decisions) != 1 {
		t.Errorf("count")
	}
	if !strings.Contains(r.Decisions[0].Reason, "não tem") {
		t.Errorf("reason: %s", r.Decisions[0].Reason)
	}
}

// Aceitação: Discover mode.
func TestSelectDiscover(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessNone,
		MinModels:    1,
	}, nil)
	r, err := s.Select(context.Background(), []Provider{p})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(r.Decisions) == 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: Discover distinct providers.
func TestSelectDistinctProviders(t *testing.T) {
	p1 := NewOpenAIProvider("https://a", "").WithProviderName("a")
	p1.WithStaticModels([]ModelInfo{{ID: "x", SupportsJSON: true, Provider: "a"}, {ID: "y", SupportsJSON: true, Provider: "a"}})
	p2 := NewOpenAIProvider("https://b", "").WithProviderName("b")
	p2.WithStaticModels([]ModelInfo{{ID: "x", SupportsJSON: true, Provider: "b"}, {ID: "y", SupportsJSON: true, Provider: "b"}})
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessProvider,
		MinModels:    2,
	}, nil)
	r, err := s.Select(context.Background(), []Provider{p1, p2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(r.Decisions) >= 2 {
		if r.Decisions[0].Provider == r.Decisions[1].Provider {
			t.Errorf("providers distintos")
		}
	}
}

// Aceitação: Discover distinct models.
func TestSelectDistinctModels(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessModel,
		MinModels:    1,
	}, nil)
	r, err := s.Select(context.Background(), []Provider{p})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	seen := make(map[string]bool)
	for _, d := range r.Decisions {
		if seen[d.Model] {
			t.Errorf("model duplicado")
		}
		seen[d.Model] = true
	}
}

// Aceitação: Hybrid mode.
func TestSelectHybrid(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeHybrid,
		Pinned:       []string{"mock-1"},
		Distinctness: DistinctnessNone,
		MinModels:    2,
	}, nil)
	r, err := s.Select(context.Background(), []Provider{p})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(r.Decisions) < 2 {
		t.Errorf("esperado 2, teve %d", len(r.Decisions))
	}
	if r.Decisions[0].Source != "pinned" {
		t.Errorf("primeiro devia ser pinned")
	}
}

// Aceitação: Hybrid com pinned suficiente.
func TestSelectHybridPinnedEnough(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:      SelectorModeHybrid,
		Pinned:    []string{"mock-1", "mock-2"},
		MinModels: 2,
	}, nil)
	r, err := s.Select(context.Background(), []Provider{p})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(r.Decisions) != 2 {
		t.Errorf("count")
	}
	for _, d := range r.Decisions {
		if d.Source != "pinned" {
			t.Errorf("todos pinned")
		}
	}
}

// Aceitação: Mode inválido.
func TestSelectInvalidMode(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{Mode: "bogus"}, nil)
	_, err := s.Select(context.Background(), []Provider{p})
	if err == nil {
		t.Errorf("mode bogus")
	}
}

// Aceitação: ModelFamily.
func TestModelFamily(t *testing.T) {
	cases := map[string]string{
		"gpt-4":         "gpt",
		"claude-3-opus": "claude",
		"gemini-pro":    "gemini",
		"llama-3-70b":   "llama",
		"mistral-7b":    "mistral",
		"mixtral-8x7b":  "mistral",
		"random-model":  "other",
	}
	for model, want := range cases {
		if got := ModelFamily(model); got != want {
			t.Errorf("%s: want %s, got %s", model, want, got)
		}
	}
}

// Aceitação: ModelFamily case insensitive.
func TestModelFamilyCase(t *testing.T) {
	if ModelFamily("GPT-4") != "gpt" {
		t.Errorf("case")
	}
}

// Aceitação: DistinctFamily.
func TestSelectDistinctFamily(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessFamily,
		MinModels:    1,
	}, nil)
	r, err := s.Select(context.Background(), []Provider{p})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(r.Decisions) == 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: DistinctnessNone inclui todos.
func TestSelectDistinctNone(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessNone,
		MinModels:    0,
	}, nil)
	r, _ := s.Select(context.Background(), []Provider{p})
	if len(r.Decisions) != 2 {
		t.Errorf("todos: %d", len(r.Decisions))
	}
}

// Aceitação: FormatResult.
func TestFormatResult(t *testing.T) {
	r := &SelectorResult{
		Mode: SelectorModeHybrid, Policy: DistinctnessProvider,
		Reason:    "test",
		Decisions: []SelectorDecision{{Model: "x", Provider: "y", Reason: "r"}},
	}
	out := FormatResult(r)
	if !strings.Contains(out, "hybrid") {
		t.Errorf("mode")
	}
	if !strings.Contains(out, "x @y") {
		t.Errorf("decision")
	}
	if FormatResult(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: RequireJSON filter.
func TestSelectRequireJSON(t *testing.T) {
	p := NewMockProvider()
	// mock-1 e mock-2 ambos SupportsJSON=true.
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessNone,
		MinModels:    1,
		RequireJSON:  true,
	}, nil)
	r, _ := s.Select(context.Background(), []Provider{p})
	if len(r.Decisions) == 0 {
		t.Errorf("json required devia achar")
	}
}

// Aceitação: MinModels zero = todos.
func TestSelectMinZero(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessNone,
		MinModels:    0,
	}, nil)
	r, _ := s.Select(context.Background(), []Provider{p})
	if len(r.Decisions) != 2 {
		t.Errorf("min=0 = todos")
	}
}

// Aceitação: decisions com rank/score populated.
func TestDecisionsPopulated(t *testing.T) {
	p := NewMockProvider()
	s := NewSelector(SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessNone,
		MinModels:    1,
	}, nil)
	r, _ := s.Select(context.Background(), []Provider{p})
	if r.Decisions[0].Rank == 0 {
		t.Errorf("rank")
	}
	if r.Decisions[0].Score == 0 {
		t.Errorf("score")
	}
}
