// Model selector (SAI-063).
//
// Modos: pinned (configurado), discover (probe + rank),
// hybrid (pinned + discovery para fallback).
// Distinctness policy: garante que 2 modelos selecionados
// não sejam do mesmo provider (independência).
// SelectionReason: audit trail explicando escolha.
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SelectorMode enum.
type SelectorMode string

const (
	SelectorModePinned   SelectorMode = "pinned"
	SelectorModeDiscover SelectorMode = "discover"
	SelectorModeHybrid   SelectorMode = "hybrid"
)

// DistinctnessPolicy enum.
type DistinctnessPolicy string

const (
	DistinctnessNone     DistinctnessPolicy = "none"     // aceita qualquer
	DistinctnessProvider DistinctnessPolicy = "provider" // providers distintos
	DistinctnessModel    DistinctnessPolicy = "model"    // modelos distintos
	DistinctnessFamily   DistinctnessPolicy = "family"   // famílias distintas
)

// SelectorOptions opções.
type SelectorOptions struct {
	Mode         SelectorMode       `json:"mode"`
	Pinned       []string           `json:"pinned,omitempty"`
	Distinctness DistinctnessPolicy `json:"distinctness"`
	MinModels    int                `json:"min_models"`
	// CapabilityRequired requirements.
	RequireJSON   bool `json:"require_json"`
	RequireVision bool `json:"require_vision"`
	RequireAudio  bool `json:"require_audio"`
}

// DefaultSelectorOptions defaults.
func DefaultSelectorOptions() SelectorOptions {
	return SelectorOptions{
		Mode:         SelectorModeDiscover,
		Distinctness: DistinctnessProvider,
		MinModels:    2,
	}
}

// SelectorDecision uma seleção específica.
type SelectorDecision struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
	Reason   string `json:"reason"`
	Source   string `json:"source"` // "pinned"|"discover"|"fallback"
	Rank     int    `json:"rank,omitempty"`
	Score    int    `json:"score,omitempty"`
}

// SelectorResult output.
type SelectorResult struct {
	Decisions []SelectorDecision `json:"decisions"`
	Mode      SelectorMode       `json:"mode"`
	Policy    DistinctnessPolicy `json:"policy"`
	Reason    string             `json:"reason"`
}

// Selector engine.
type Selector struct {
	opts   SelectorOptions
	probes *ProbeCache
}

// NewSelector constrói selector.
func NewSelector(opts SelectorOptions, probes *ProbeCache) *Selector {
	if probes == nil {
		probes = NewProbeCache()
	}
	return &Selector{opts: opts, probes: probes}
}

// Select seleciona modelos conforme opts.
func (s *Selector) Select(ctx context.Context, providers []Provider) (*SelectorResult, error) {
	if len(providers) == 0 {
		return nil, errors.New("selector: providers vazio")
	}
	switch s.opts.Mode {
	case SelectorModePinned:
		return s.selectPinned(providers)
	case SelectorModeDiscover:
		return s.selectDiscover(ctx, providers)
	case SelectorModeHybrid:
		return s.selectHybrid(ctx, providers)
	}
	return nil, fmt.Errorf("selector: mode inválido: %s", s.opts.Mode)
}

// selectPinned usa lista fixa.
func (s *Selector) selectPinned(providers []Provider) (*SelectorResult, error) {
	decisions := make([]SelectorDecision, 0)
	for _, m := range s.opts.Pinned {
		norm := NormalizeID(m)
		found := false
		for _, p := range providers {
			models, _ := p.ListModels(context.Background())
			for _, mi := range models {
				if mi.ID == norm {
					decisions = append(decisions, SelectorDecision{
						Model:    mi.ID,
						Provider: p.Name(),
						Reason:   "pinned na config",
						Source:   "pinned",
					})
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			decisions = append(decisions, SelectorDecision{
				Model:  norm,
				Reason: "pinned mas provider não tem modelo",
				Source: "pinned",
			})
		}
	}
	return &SelectorResult{
		Decisions: decisions,
		Mode:      SelectorModePinned,
		Policy:    s.opts.Distinctness,
		Reason:    "pinned mode: config fixa",
	}, nil
}

// selectDiscover probe + rank.
func (s *Selector) selectDiscover(ctx context.Context, providers []Provider) (*SelectorResult, error) {
	summaries, err := s.collectSummaries(ctx, providers)
	if err != nil {
		return nil, err
	}
	ranked := RankByCapability(summaries)
	decisions := s.applyDistinctness(ranked)
	return &SelectorResult{
		Decisions: decisions,
		Mode:      SelectorModeDiscover,
		Policy:    s.opts.Distinctness,
		Reason:    "discover mode: probe + rank + distinctness",
	}, nil
}

// selectHybrid pinned + discovery.
func (s *Selector) selectHybrid(ctx context.Context, providers []Provider) (*SelectorResult, error) {
	pinnedResult, err := s.selectPinned(providers)
	if err != nil {
		return nil, err
	}
	if len(pinnedResult.Decisions) >= s.opts.MinModels {
		return pinnedResult, nil
	}
	discoverResult, err := s.selectDiscover(ctx, providers)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]bool)
	for _, d := range pinnedResult.Decisions {
		existing[d.Model] = true
	}
	merged := append([]SelectorDecision{}, pinnedResult.Decisions...)
	for _, d := range discoverResult.Decisions {
		if existing[d.Model] {
			continue
		}
		d.Source = "fallback"
		merged = append(merged, d)
		if len(merged) >= s.opts.MinModels {
			break
		}
	}
	return &SelectorResult{
		Decisions: merged,
		Mode:      SelectorModeHybrid,
		Policy:    s.opts.Distinctness,
		Reason:    fmt.Sprintf("hybrid: %d pinned + fallback discover", len(pinnedResult.Decisions)),
	}, nil
}

// collectSummaries probe todos modelos de todos providers.
func (s *Selector) collectSummaries(ctx context.Context, providers []Provider) ([]ProbeSummary, error) {
	out := make([]ProbeSummary, 0)
	for _, p := range providers {
		models, err := p.ListModels(ctx)
		if err != nil {
			continue
		}
		for _, m := range models {
			if s.opts.RequireJSON && !m.SupportsJSON {
				continue
			}
			if s.opts.RequireVision && !m.SupportsVision {
				continue
			}
			if s.opts.RequireAudio && !m.SupportsAudio {
				continue
			}
			cached := s.probes.Get(m.ID)
			var results []ProbeResult
			if len(cached) > 0 {
				results = cached
			} else {
				results, _ = ProbeMany(ctx, p, ProbeOptions{Model: m.ID}, 3)
				for _, r := range results {
					s.probes.Put(r)
				}
			}
			out = append(out, Summarize(results))
		}
	}
	return out, nil
}

// applyDistinctness aplica policy.
func (s *Selector) applyDistinctness(ranked []RankedModel) []SelectorDecision {
	out := make([]SelectorDecision, 0)
	seenProvider := make(map[string]bool)
	seenModel := make(map[string]bool)
	seenFamily := make(map[string]bool)
	for i, r := range ranked {
		switch s.opts.Distinctness {
		case DistinctnessNone:
			out = append(out, SelectorDecision{
				Model:    r.Model,
				Provider: r.Summary.Provider,
				Reason:   fmt.Sprintf("rank #%d score=%d", i+1, r.Score),
				Source:   "discover",
				Rank:     i + 1,
				Score:    r.Score,
			})
		case DistinctnessProvider:
			if seenProvider[r.Summary.Provider] {
				continue
			}
			seenProvider[r.Summary.Provider] = true
		case DistinctnessModel:
			if seenModel[r.Model] {
				continue
			}
			seenModel[r.Model] = true
		case DistinctnessFamily:
			fam := ModelFamily(r.Model)
			if seenFamily[fam] {
				continue
			}
			seenFamily[fam] = true
		}
		if s.opts.Distinctness != DistinctnessNone {
			out = append(out, SelectorDecision{
				Model:    r.Model,
				Provider: r.Summary.Provider,
				Reason:   fmt.Sprintf("rank #%d score=%d (distinct=%s)", i+1, r.Score, s.opts.Distinctness),
				Source:   "discover",
				Rank:     i + 1,
				Score:    r.Score,
			})
		}
		if len(out) >= s.opts.MinModels && s.opts.MinModels > 0 {
			break
		}
	}
	return out
}

// ModelFamily extrai família de modelo (heurística).
func ModelFamily(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "gpt"):
		return "gpt"
	case strings.HasPrefix(m, "claude"):
		return "claude"
	case strings.HasPrefix(m, "gemini"):
		return "gemini"
	case strings.HasPrefix(m, "llama"):
		return "llama"
	case strings.HasPrefix(m, "mistral") || strings.HasPrefix(m, "mixtral"):
		return "mistral"
	}
	return "other"
}

// FormatResult renderiza decisão.
func FormatResult(r *SelectorResult) string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Selector[%s, %s]: %s\n", r.Mode, r.Policy, r.Reason))
	for i, d := range r.Decisions {
		sb.WriteString(fmt.Sprintf("  %d. %s @%s — %s\n", i+1, d.Model, d.Provider, d.Reason))
	}
	return sb.String()
}
