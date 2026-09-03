// Cache verification (SAI-100).
//
// Verifica comportamento esperado:
// 1. Re-run idêntico reaproveita analyzer results.
// 2. Mudança de tool/config invalida.
//
// Estratégia: roda pipeline N vezes com mesma key,
// confirma hash hit; muda 1 componente, confirma miss.
package cacheverify

import (
	"errors"

	"github.com/diegoaraujo/solidify/internal/cache"
)

// Scenario caso de teste.
type Scenario struct {
	Name           string
	Key            cache.Key
	ExpectHit      bool
	MutationReason string // descreve o que mudou
}

// Result resultado da verificação.
type Result struct {
	Scenario  string
	Hit       bool
	KeyHash   string
	CacheHits int
	Misses    int
	Passed    bool
	Reason    string
}

// Verify roda cenário e compara hit/miss esperado.
func Verify(s Scenario, prevHash string) Result {
	h := s.Key.Hash()
	hit := (h == prevHash && prevHash != "")
	res := Result{
		Scenario: s.Name,
		Hit:      hit,
		KeyHash:  h,
		Passed:   hit == s.ExpectHit,
		Reason:   s.MutationReason,
	}
	if hit {
		res.CacheHits = 1
	} else {
		res.Misses = 1
	}
	return res
}

// VerifyPair valida comportamento de par de runs.
type VerifyPair struct {
	RunA   cache.Key
	RunB   cache.Key
	ExpectSameHash bool
	Description    string
}

// PairResult resultado.
type PairResult struct {
	Description string
	SameHash    bool
	Passed      bool
	HashA       string
	HashB       string
}

// VerifyHashPair valida dois keys contra expectativa.
func VerifyHashPair(p VerifyPair) PairResult {
	hA := p.RunA.Hash()
	hB := p.RunB.Hash()
	return PairResult{
		Description: p.Description,
		SameHash:    hA == hB,
		Passed:      (hA == hB) == p.ExpectSameHash,
		HashA:       hA,
		HashB:       hB,
	}
}

// SameRunScenario cenário p/ re-run idêntico.
func SameRunScenario(analyzer, version, cfgHash, base, head, inputHash string) Scenario {
	return Scenario{
		Name:           "same_run",
		Key:            baseKey(analyzer, version, cfgHash, base, head, inputHash),
		ExpectHit:      true,
		MutationReason: "nenhuma",
	}
}

// ChangedToolScenario: mudou nome do analyzer.
func ChangedToolScenario(prev cache.Key) Scenario {
	return Scenario{
		Name: "changed_tool",
		Key: cache.Key{
			Analyzer:   prev.Analyzer + "_v2", // tool name mudou
			Version:    prev.Version,
			ConfigHash: prev.ConfigHash,
			Base:       prev.Base,
			Head:       prev.Head,
			InputHash:  prev.InputHash,
			Tags:       prev.Tags,
		},
		ExpectHit:      false,
		MutationReason: "analyzer mudou",
	}
}

// ChangedConfigScenario: mudou ConfigHash.
func ChangedConfigScenario(prev cache.Key) Scenario {
	return Scenario{
		Name: "changed_config",
		Key: cache.Key{
			Analyzer:   prev.Analyzer,
			Version:    prev.Version,
			ConfigHash: prev.ConfigHash + "_x", // config mudou
			Base:       prev.Base,
			Head:       prev.Head,
			InputHash:  prev.InputHash,
			Tags:       prev.Tags,
		},
		ExpectHit:      false,
		MutationReason: "config hash mudou",
	}
}

// ChangedInputScenario: input mudou.
func ChangedInputScenario(prev cache.Key) Scenario {
	return Scenario{
		Name: "changed_input",
		Key: cache.Key{
			Analyzer:   prev.Analyzer,
			Version:    prev.Version,
			ConfigHash: prev.ConfigHash,
			Base:       prev.Base,
			Head:       prev.Head,
			InputHash:  prev.InputHash + "_y",
			Tags:       prev.Tags,
		},
		ExpectHit:      false,
		MutationReason: "input hash mudou",
	}
}

// ChangedBaseScenario: base SHA mudou.
func ChangedBaseScenario(prev cache.Key) Scenario {
	return Scenario{
		Name: "changed_base",
		Key: cache.Key{
			Analyzer:   prev.Analyzer,
			Version:    prev.Version,
			ConfigHash: prev.ConfigHash,
			Base:       prev.Base + "_z",
			Head:       prev.Head,
			InputHash:  prev.InputHash,
			Tags:       prev.Tags,
		},
		ExpectHit:      false,
		MutationReason: "base SHA mudou",
	}
}

// ErrInvalidScenario scenario sem name.
var ErrInvalidScenario = errors.New("cacheverify: scenario sem name")

func baseKey(a, v, cfg, b, h, in string) cache.Key {
	return cache.Key{
		Analyzer:   a,
		Version:    v,
		ConfigHash: cfg,
		Base:       b,
		Head:       h,
		InputHash:  in,
	}
}

// Aggregate sumariza múltiplos results.
type Aggregate struct {
	Total   int
	Passed  int
	Failed  int
	Hits    int
	Misses  int
}

// AggregateResults agrega N results.
func AggregateResults(rs []Result) Aggregate {
	agg := Aggregate{Total: len(rs)}
	for _, r := range rs {
		if r.Passed {
			agg.Passed++
		} else {
			agg.Failed++
		}
		agg.Hits += r.CacheHits
		agg.Misses += r.Misses
	}
	return agg
}
