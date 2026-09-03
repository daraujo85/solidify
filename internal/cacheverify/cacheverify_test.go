package cacheverify

import (
	"testing"

	"github.com/diegoaraujo/solidify/internal/cache"
)

// Aceitação: Verify hit quando hash bate.
func TestVerifyHit(t *testing.T) {
	k := cache.Key{Analyzer: "a", Version: "1", ConfigHash: "c", Base: "b", Head: "h", InputHash: "i"}
	h := k.Hash()
	r := Verify(SameRunScenario("a", "1", "c", "b", "h", "i"), h)
	if !r.Passed || !r.Hit {
		t.Errorf("hit: %+v", r)
	}
}

// Aceitação: Verify miss quando prev vazio.
func TestVerifyMissEmpty(t *testing.T) {
	sc := Scenario{
		Name:  "empty_prev",
		Key:   baseKey("a", "1", "c", "b", "h", "i"),
		ExpectHit: false,
		MutationReason: "prev vazio",
	}
	r := Verify(sc, "")
	if r.Hit {
		t.Errorf("empty prev")
	}
	if !r.Passed {
		t.Errorf("miss esperado: %+v", r)
	}
}

// Aceitação: Verify miss quando hashes diferem.
func TestVerifyMissDifferent(t *testing.T) {
	r := Verify(ChangedToolScenario(cache.Key{Analyzer: "a", InputHash: "i"}), "fakehash")
	if r.Hit {
		t.Errorf("hit")
	}
	if !r.Passed {
		t.Errorf("miss esperado")
	}
}

// Aceitação: SameRunScenario hit quando prev bate.
func TestSameRunScenario(t *testing.T) {
	k := baseKey("a", "1", "c", "b", "h", "i")
	h := k.Hash()
	sc := SameRunScenario("a", "1", "c", "b", "h", "i")
	r := Verify(sc, h)
	if !r.Passed || r.CacheHits != 1 {
		t.Errorf("same: %+v", r)
	}
}

// Aceitação: ChangedToolScenario invalida.
func TestChangedTool(t *testing.T) {
	prev := baseKey("a", "1", "c", "b", "h", "i")
	sc := ChangedToolScenario(prev)
	if sc.ExpectHit {
		t.Errorf("expected miss")
	}
	r := Verify(sc, prev.Hash())
	if !r.Passed {
		t.Errorf("tool change: %+v", r)
	}
}

// Aceitação: ChangedConfigScenario invalida.
func TestChangedConfig(t *testing.T) {
	prev := baseKey("a", "1", "c", "b", "h", "i")
	sc := ChangedConfigScenario(prev)
	r := Verify(sc, prev.Hash())
	if !r.Passed {
		t.Errorf("config change: %+v", r)
	}
}

// Aceitação: ChangedInputScenario invalida.
func TestChangedInput(t *testing.T) {
	prev := baseKey("a", "1", "c", "b", "h", "i")
	sc := ChangedInputScenario(prev)
	r := Verify(sc, prev.Hash())
	if !r.Passed {
		t.Errorf("input change: %+v", r)
	}
}

// Aceitação: ChangedBaseScenario invalida.
func TestChangedBase(t *testing.T) {
	prev := baseKey("a", "1", "c", "b", "h", "i")
	sc := ChangedBaseScenario(prev)
	r := Verify(sc, prev.Hash())
	if !r.Passed {
		t.Errorf("base change: %+v", r)
	}
}

// Aceitação: VerifyHashPair same.
func TestPairSame(t *testing.T) {
	k := baseKey("a", "1", "c", "b", "h", "i")
	res := VerifyHashPair(VerifyPair{RunA: k, RunB: k, ExpectSameHash: true, Description: "same"})
	if !res.Passed {
		t.Errorf("pair same: %+v", res)
	}
}

// Aceitação: VerifyHashPair different.
func TestPairDifferent(t *testing.T) {
	k1 := baseKey("a", "1", "c", "b", "h", "i")
	k2 := baseKey("a", "1", "c", "b", "h", "i2")
	res := VerifyHashPair(VerifyPair{RunA: k1, RunB: k2, ExpectSameHash: false, Description: "different"})
	if !res.Passed {
		t.Errorf("pair diff: %+v", res)
	}
}

// Aceitação: AggregateResults.
func TestAggregate(t *testing.T) {
	rs := []Result{
		{Passed: true, CacheHits: 1},
		{Passed: true, CacheHits: 1},
		{Passed: false, Misses: 1},
	}
	agg := AggregateResults(rs)
	if agg.Total != 3 || agg.Passed != 2 || agg.Failed != 1 {
		t.Errorf("agg: %+v", agg)
	}
	if agg.Hits != 2 || agg.Misses != 1 {
		t.Errorf("counts: %+v", agg)
	}
}
