package applicability

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
	"github.com/diegoaraujo/solidify/internal/component"
)

// stubProvider devolve um Content programável; conta calls para
// validar cache.
type stubProvider struct {
	name     string
	content  string
	err      error
	calls    int64
	delay    time.Duration
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) ListModels(context.Context) ([]ai.ModelInfo, error) {
	return nil, nil
}
func (s *stubProvider) Metadata() ai.ProviderMetadata {
	return ai.ProviderMetadata{Name: s.name, RequiresKey: false}
}
func (s *stubProvider) CompleteJSON(ctx context.Context, _ ai.CompleteOptions) (*ai.CompleteResult, error) {
	atomic.AddInt64(&s.calls, 1)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return &ai.CompleteResult{Content: s.content, Model: s.name}, nil
}

// buildValidLLMResponse monta JSON com 8 decisions válidas (gates em
// ordem canônica pra parecer LLM bem comportado).
func buildValidLLMResponse() string {
	gates := []string{"sonar", "tests", "security", "lighthouse", "zap", "k6", "migration", "env"}
	d := llmResponse{}
	for _, g := range gates {
		d.Decisions = append(d.Decisions, llmDecision{
			Gate:    g,
			Verdict: llmVerdictApplicable,
			Reason:  "LLM says so",
		})
	}
	b, _ := json.Marshal(d)
	return string(b)
}

func TestLLMDecider_NilProviderFallsBackToHeuristic(t *testing.T) {
	d := NewLLMDecider(nil, "x")
	got := d.DecideWithContext(context.Background(), Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	})
	if len(got) != 8 {
		t.Fatalf("decisões = %d, quero 8", len(got))
	}
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Source != SourceHeuristic {
		t.Errorf("Source = %s, quero heuristic", sonar.Source)
	}
	if sonar.LLMAssessed {
		t.Errorf("LLMAssessed = true, quero false (sem LLM)")
	}
}

// Decider com provider configurado deve chamar LLM 1x e marcar
// LLMAssessed=true.
func TestLLMDecider_CallsProviderAndEnriches(t *testing.T) {
	stub := &stubProvider{name: "stub", content: buildValidLLMResponse()}
	d := NewLLMDecider(stub, "m")
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
		SonarConfigured: true,
	}
	got := d.DecideWithContext(context.Background(), profile)
	if stub.calls != 1 {
		t.Errorf("calls = %d, quero 1", stub.calls)
	}
	for _, dec := range got {
		if !dec.LLMAssessed {
			t.Errorf("gate %s: LLMAssessed = false", dec.Gate)
		}
	}
}

// Modo advisory (heurística vence Verdict, LLM enriquece Reason).
func TestLLMDecider_AdvisoryModePreservesHeuristicVerdict(t *testing.T) {
	stub := &stubProvider{name: "stub", content: buildValidLLMResponse()}
	d := NewLLMDecider(stub, "m")
	d.Mode = ModeAdvisory

	// Backend-only sem Sonar config: heurística = Conditional.
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	got := d.DecideWithContext(context.Background(), profile)
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Verdict != Conditional {
		t.Errorf("advisory: Verdict = %s, quero Conditional (heurística)", sonar.Verdict)
	}
	// Reason deve ter sido enriquecido com "LLM:" marker.
	if !contains(sonar.Reason, "LLM:") {
		t.Errorf("advisory: Reason não-enriquecido: %q", sonar.Reason)
	}
}

// Modo enforce: LLM pode override Verdict (todos APPLICABLE no stub).
func TestLLMDecider_EnforceModeAllowsOverride(t *testing.T) {
	stub := &stubProvider{name: "stub", content: buildValidLLMResponse()}
	d := NewLLMDecider(stub, "m")
	d.Mode = ModeEnforce
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	got := d.DecideWithContext(context.Background(), profile)
	sonar := decisionByGate(t, got, GateSonar)
	// LLM diz APPLICABLE — em enforce, override ganha.
	if sonar.Verdict != Applicable {
		t.Errorf("enforce: Verdict = %s, quero Applicable (LLM)", sonar.Verdict)
	}
}

// Provider com erro ⇒ fallback puro pra heurística.
func TestLLMDecider_ProviderErrorFallsBack(t *testing.T) {
	stub := &stubProvider{name: "stub", err: errors.New("boom")}
	d := NewLLMDecider(stub, "m")
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	got := d.DecideWithContext(context.Background(), profile)
	if stub.calls != 1 {
		t.Errorf("calls = %d, quero 1", stub.calls)
	}
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Source != SourceLLMFallback {
		t.Errorf("Source = %s, quero llm-fallback", sonar.Source)
	}
	if sonar.LLMAssessed {
		t.Errorf("LLMAssessed = true, quero false (fallback)")
	}
	// Verdict deve seguir heurística pura.
	if sonar.Verdict != Conditional {
		t.Errorf("Verdict = %s, quero Conditional (heurística)", sonar.Verdict)
	}
}

// LLM devolve JSON inválido ⇒ fallback (sem panic).
func TestLLMDecider_InvalidJSONFallsBack(t *testing.T) {
	stub := &stubProvider{name: "stub", content: "not json at all"}
	d := NewLLMDecider(stub, "m")
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	got := d.DecideWithContext(context.Background(), profile)
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Source != SourceLLMFallback {
		t.Errorf("Source = %s", sonar.Source)
	}
}

// Cache: 2ª chamada com mesmo profile não toca provider.
func TestLLMDecider_CacheHitAvoidsProviderCall(t *testing.T) {
	stub := &stubProvider{name: "stub", content: buildValidLLMResponse()}
	d := NewLLMDecider(stub, "m")
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	_ = d.DecideWithContext(context.Background(), profile)
	_ = d.DecideWithContext(context.Background(), profile)
	_ = d.DecideWithContext(context.Background(), profile)
	if stub.calls != 1 {
		t.Errorf("calls = %d, quero 1 (cache hit)", stub.calls)
	}
}

// LLM devolve schema inválido (gate faltando) → repair 1x, ainda
// falhando → fallback heurístico.
func TestLLMDecider_InvalidSchemaFallsBackAfterRepair(t *testing.T) {
	stub := &stubProvider{name: "stub", content: `{"decisions":[{"gate":"sonar","verdict":"APPLICABLE","reason":"x"}]}`}
	d := NewLLMDecider(stub, "m")
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	got := d.DecideWithContext(context.Background(), profile)
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Source != SourceLLMFallback {
		t.Errorf("Source = %s, quero llm-fallback", sonar.Source)
	}
	if stub.calls < 2 {
		t.Errorf("calls = %d, quero >= 2 (1 normal + 1 repair)", stub.calls)
	}
}

// LLM devolve JSON com think-block prefix (comportamento real de
// claude-coder / mimo). parse tolerante deve funcionar.
func TestLLMDecider_ThinkBlockPrefix(t *testing.T) {
	raw := "<think>analyzing profile</think>" + buildValidLLMResponse()
	stub := &stubProvider{name: "stub", content: raw}
	d := NewLLMDecider(stub, "m")
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	got := d.DecideWithContext(context.Background(), profile)
	allLLM := true
	for _, dec := range got {
		if !dec.LLMAssessed {
			allLLM = false
		}
	}
	if !allLLM {
		t.Errorf("think-block prefix: nem todas as decisions vieram do LLM")
	}
}

// DecisionCache LRU: capacidade respeitada.
func TestDecisionCache_EvictsOldest(t *testing.T) {
	c := NewDecisionCache(2)
	c.Put("a", []llmDecision{{Gate: "a"}})
	c.Put("b", []llmDecision{{Gate: "b"}})
	c.Put("c", []llmDecision{{Gate: "c"}})
	if _, ok := c.Get("a"); ok {
		t.Error("a deveria ter sido evictado")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("b deveria estar presente")
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("c deveria estar presente")
	}
}

// DecisionCache: re-put no mesmo key não duplica.
func TestDecisionCache_ReputSameKeyNoDuplicate(t *testing.T) {
	c := NewDecisionCache(4)
	c.Put("x", []llmDecision{{Gate: "x"}})
	c.Put("x", []llmDecision{{Gate: "x"}, {Gate: "y"}})
	d, ok := c.Get("x")
	if !ok || len(d) != 2 {
		t.Errorf("x deveria ter 2 entries, got %d", len(d))
	}
}

// DecisionCache: cache nil é no-op.
func TestDecisionCache_NilSafe(t *testing.T) {
	var c *DecisionCache
	if _, ok := c.Get("k"); ok {
		t.Error("nil cache deveria devolver ok=false")
	}
	c.Put("k", []llmDecision{{Gate: "k"}}) // não pode panic
}

// Default cap quando cap <= 0.
func TestDecisionCache_DefaultCap(t *testing.T) {
	c := NewDecisionCache(0)
	if c.cap != 64 {
		t.Errorf("cap = %d, quero 64", c.cap)
	}
	c = NewDecisionCache(-5)
	if c.cap != 64 {
		t.Errorf("cap negative = %d, quero 64", c.cap)
	}
}

// --- helpers ---

func decisionByGate(t *testing.T, ds []Decision, g Gate) Decision {
	t.Helper()
	for _, d := range ds {
		if d.Gate == g {
			return d
		}
	}
	t.Fatalf("gate %s ausente", g)
	return Decision{}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}