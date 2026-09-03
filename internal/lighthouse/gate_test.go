package lighthouse

import (
	"strings"
	"testing"
)

// Aceitação: DefaultPillarThreshold conservador.
func TestDefaultPillarThreshold(t *testing.T) {
	t1 := DefaultPillarThreshold()
	if t1.MinPerformance < 0.8 {
		t.Errorf("perf min baixo demais")
	}
	if t1.MinSEO != 0 {
		t.Errorf("SEO default informativo")
	}
	if t1.MinAggregate <= 0 {
		t.Errorf("agg min")
	}
}

// Aceitação: LenientPillarThreshold permissivo.
func TestLenientPillarThreshold(t *testing.T) {
	t1 := LenientPillarThreshold()
	if t1.MinPerformance != 0.5 {
		t.Errorf("lenient perf = 0.5")
	}
}

// Aceitação: gate perfeitos pilares passam.
func TestEvaluatePillarGatePerfect(t *testing.T) {
	rep := &Report{
		Mode:           ModeContainer,
		Pillar:         Pillar{Performance: 1, Accessibility: 1, BestPractices: 1},
		AggregateScore: 1.0,
	}
	res := EvaluatePillarGate(rep, DefaultPillarThreshold())
	if !res.Allow {
		t.Errorf("perfeito devia passar: %s", res.Reason)
	}
}

// Aceitação: gate com pilares baixos falha.
func TestEvaluatePillarGateFail(t *testing.T) {
	rep := &Report{
		Mode:           ModeContainer,
		Pillar:         Pillar{Performance: 0.5, Accessibility: 0.9, BestPractices: 0.9},
		AggregateScore: 0.7,
	}
	res := EvaluatePillarGate(rep, DefaultPillarThreshold())
	if res.Allow {
		t.Errorf("perf baixo devia bloquear")
	}
	if len(res.Failed) == 0 {
		t.Errorf("failed vazio")
	}
}

// Aceitação: gate skip se Mode=Disabled.
func TestEvaluatePillarGateDisabled(t *testing.T) {
	rep := &Report{Mode: ModeDisabled, Error: "backend-only"}
	res := EvaluatePillarGate(rep, DefaultPillarThreshold())
	if !res.Allow {
		t.Errorf("disabled devia skip + allow")
	}
	if !res.Skipped {
		t.Errorf("skipped flag")
	}
	if !strings.Contains(res.SkippedReason, "backend") {
		t.Errorf("reason = %q", res.SkippedReason)
	}
}

// Aceitação: gate skip se Report nil.
func TestEvaluatePillarGateNil(t *testing.T) {
	res := EvaluatePillarGate(nil, DefaultPillarThreshold())
	if !res.Allow || !res.Skipped {
		t.Errorf("nil = skip allow")
	}
}

// Aceitação: gate lenient permite score baixo.
func TestEvaluatePillarGateLenient(t *testing.T) {
	rep := &Report{
		Mode:           ModeContainer,
		Pillar:         Pillar{Performance: 0.5, Accessibility: 0.5, BestPractices: 0.5},
		AggregateScore: 0.5,
	}
	res := EvaluatePillarGate(rep, LenientPillarThreshold())
	if !res.Allow {
		t.Errorf("lenient devia permitir: %s", res.Reason)
	}
}

// Aceitação: SEO informativo (zero) não bloqueia.
func TestEvaluatePillarGateSEOInformative(t *testing.T) {
	rep := &Report{
		Mode:           ModeContainer,
		Pillar:         Pillar{Performance: 1, Accessibility: 1, BestPractices: 1, SEO: 0.3},
		AggregateScore: 1.0,
	}
	res := EvaluatePillarGate(rep, DefaultPillarThreshold())
	if !res.Allow {
		t.Errorf("SEO baixo não devia bloquear (informativo): %s", res.Reason)
	}
}

// Aceitação: SEO com threshold > 0 bloqueia.
func TestEvaluatePillarGateSEOStrict(t *testing.T) {
	th := DefaultPillarThreshold()
	th.MinSEO = 0.9
	rep := &Report{
		Mode:           ModeContainer,
		Pillar:         Pillar{Performance: 1, Accessibility: 1, BestPractices: 1, SEO: 0.5},
		AggregateScore: 1.0,
	}
	res := EvaluatePillarGate(rep, th)
	if res.Allow {
		t.Errorf("SEO strict devia bloquear")
	}
	if !containsString(res.Failed, "seo") {
		t.Errorf("failed = %v", res.Failed)
	}
}

// Aceitação: GateDefault / GateLenient helpers.
func TestGateHelpers(t *testing.T) {
	rep := &Report{
		Mode:           ModeContainer,
		Pillar:         Pillar{Performance: 1, Accessibility: 1, BestPractices: 1},
		AggregateScore: 1.0,
	}
	if !GateDefault(rep) {
		t.Errorf("default gate devia passar")
	}
	if !GateLenient(rep) {
		t.Errorf("lenient gate devia passar")
	}
}

// Aceitação: WorstPillar.
func TestWorstPillar(t *testing.T) {
	p := Pillar{Performance: 0.9, Accessibility: 0.5, BestPractices: 0.8}
	key, score := WorstPillar(p)
	if key != CatAccessibility || score != 0.5 {
		t.Errorf("got %v=%f, quero accessibility=0.5", key, score)
	}
}

// Aceitação: WorstPillar vazio.
func TestWorstPillarEmpty(t *testing.T) {
	key, score := WorstPillar(Pillar{})
	if key != "" || score != 0 {
		t.Errorf("got %v=%f", key, score)
	}
}

// Aceitação: WorstPillar só SEO (informativo).
func TestWorstPillarSEOOnly(t *testing.T) {
	key, score := WorstPillar(Pillar{SEO: 0.5})
	if key != CatSEO || score != 0.5 {
		t.Errorf("got %v=%f", key, score)
	}
}

// Aceitação: AggregatePercent.
func TestAggregatePercent(t *testing.T) {
	if AggregatePercent(&Report{AggregateScore: 0.85}) != 85 {
		t.Errorf("85")
	}
	if AggregatePercent(nil) != 0 {
		t.Errorf("nil = 0")
	}
}

// Aceitação: gate multiple failures.
func TestEvaluatePillarGateMultiFail(t *testing.T) {
	rep := &Report{
		Mode:           ModeContainer,
		Pillar:         Pillar{Performance: 0.3, Accessibility: 0.4, BestPractices: 0.5},
		AggregateScore: 0.4,
	}
	res := EvaluatePillarGate(rep, DefaultPillarThreshold())
	if res.Allow {
		t.Errorf("tudo baixo devia bloquear")
	}
	if len(res.Failed) < 3 {
		t.Errorf("failed = %v", res.Failed)
	}
}

// Aceitação: joinStrings.
func TestJoinStrings(t *testing.T) {
	if joinStrings(nil, ",") != "" {
		t.Errorf("nil")
	}
	if joinStrings([]string{"a"}, ",") != "a" {
		t.Errorf("single")
	}
	if joinStrings([]string{"a", "b", "c"}, ",") != "a,b,c" {
		t.Errorf("multi")
	}
}

func containsString(parts []string, want string) bool {
	for _, p := range parts {
		if p == want {
			return true
		}
	}
	return false
}
