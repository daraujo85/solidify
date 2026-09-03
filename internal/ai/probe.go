// Capability probe (SAI-062).
//
// Mede JSON compliance + latency + availability por modelo.
// Cache. Não ranqueia "inteligência" — só características
// observáveis.
package ai

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ProbeResult medição.
type ProbeResult struct {
	Model         string    `json:"model"`
	Provider      string    `json:"provider"`
	JSONCompliant bool      `json:"json_compliant"`
	LatencyMS     int64     `json:"latency_ms"`
	Available     bool      `json:"available"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	ProbedAt      time.Time `json:"probed_at"`
	ResponseSize  int       `json:"response_size"`
}

// ProbeSummary agrega por modelo.
type ProbeSummary struct {
	Model         string `json:"model"`
	Provider      string `json:"provider"`
	JSONCompliant bool   `json:"json_compliant"`
	Available     bool   `json:"available"`
	AvgLatencyMS  int64  `json:"avg_latency_ms"`
	P95LatencyMS  int64  `json:"p95_latency_ms"`
	SampleCount   int    `json:"sample_count"`
	SuccessCount  int    `json:"success_count"`
}

// ProbeCache cache de probes.
type ProbeCache struct {
	mu sync.Mutex
	m  map[string][]ProbeResult // key = model
}

// NewProbeCache cria cache.
func NewProbeCache() *ProbeCache {
	return &ProbeCache{m: make(map[string][]ProbeResult)}
}

// Put armazena.
func (c *ProbeCache) Put(r ProbeResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[r.Model] = append(c.m[r.Model], r)
}

// Get resultados.
func (c *ProbeCache) Get(model string) []ProbeResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ProbeResult, len(c.m[model]))
	copy(out, c.m[model])
	return out
}

// All retorna todos.
func (c *ProbeCache) All() map[string][]ProbeResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string][]ProbeResult)
	for k, v := range c.m {
		cp := make([]ProbeResult, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// Reset limpa.
func (c *ProbeCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = make(map[string][]ProbeResult)
}

// ProbeOptions opções.
type ProbeOptions struct {
	// Model id.
	Model string
	// PromptJSON prompt que força output JSON.
	PromptJSON string
	// Timeout.
	Timeout time.Duration
	// Provider customizado (opcional; usa ctx provider senão).
	Provider Provider
}

// DefaultProbePrompt prompt canônico para testar JSON.
const DefaultProbePrompt = `Respond with valid JSON of the form {"ok":true,"answer":"yes"} and nothing else.`

// Probe executa 1 probe.
func Probe(ctx context.Context, p Provider, opts ProbeOptions) (*ProbeResult, error) {
	if p == nil && opts.Provider == nil {
		return nil, errors.New("probe: provider nil")
	}
	prov := p
	if prov == nil {
		prov = opts.Provider
	}
	if opts.Model == "" {
		return nil, errors.New("probe: model vazio")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	prompt := opts.PromptJSON
	if prompt == "" {
		prompt = DefaultProbePrompt
	}
	pctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	start := time.Now()
	res, err := prov.CompleteJSON(pctx, CompleteOptions{
		Model: opts.Model,
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a JSON-producing assistant. Output only valid JSON."},
			{Role: RoleUser, Content: prompt},
		},
	})
	latency := time.Since(start)
	if err != nil {
		return &ProbeResult{
			Model:        opts.Model,
			Provider:     prov.Name(),
			Available:    false,
			ErrorMessage: err.Error(),
			LatencyMS:    latency.Milliseconds(),
			ProbedAt:     start,
		}, nil
	}
	content := StripCodeFences(res.Content)
	ok := IsLikelyJSON(content)
	return &ProbeResult{
		Model:         opts.Model,
		Provider:      prov.Name(),
		JSONCompliant: ok,
		Available:     true,
		LatencyMS:     latency.Milliseconds(),
		ProbedAt:      start,
		ResponseSize:  len(res.Content),
	}, nil
}

// ProbeMany executa N probes paralelos.
func ProbeMany(ctx context.Context, p Provider, opts ProbeOptions, samples int) ([]ProbeResult, error) {
	if samples <= 0 {
		samples = 3
	}
	out := make([]ProbeResult, samples)
	errs := make([]error, samples)
	var wg sync.WaitGroup
	for i := 0; i < samples; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r, err := Probe(ctx, p, opts)
			if err != nil {
				errs[idx] = err
				return
			}
			out[idx] = *r
		}(i)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return out, e
		}
	}
	return out, nil
}

// Summarize agrega resultados.
func Summarize(results []ProbeResult) ProbeSummary {
	if len(results) == 0 {
		return ProbeSummary{}
	}
	r := results[0]
	sum := ProbeSummary{
		Model:    r.Model,
		Provider: r.Provider,
	}
	latencies := make([]int64, 0, len(results))
	for _, x := range results {
		if x.Available {
			sum.SuccessCount++
		}
		if x.JSONCompliant {
			sum.JSONCompliant = true
		}
		if x.Available {
			sum.Available = true
		}
		latencies = append(latencies, x.LatencyMS)
	}
	sum.SampleCount = len(results)
	if sum.SuccessCount > 0 {
		sum.Available = true
	}
	if len(latencies) > 0 {
		sum.AvgLatencyMS = percentile(latencies, 50)
		sum.P95LatencyMS = percentile(latencies, 95)
	}
	return sum
}

// IsLikelyJSON checa se string é provavelmente JSON válido.
func IsLikelyJSON(s string) bool {
	s = StripCodeFences(s)
	if len(s) == 0 {
		return false
	}
	c := s[0]
	if c != '{' && c != '[' {
		return false
	}
	var v any
	if err := jsonUnmarshal([]byte(s), &v); err != nil {
		return false
	}
	return true
}

// IsLikelyJSONObject só objetos.
func IsLikelyJSONObject(s string) bool {
	if !IsLikelyJSON(s) {
		return false
	}
	s = StripCodeFences(s)
	return s[0] == '{'
}

func percentile(values []int64, p int) int64 {
	if len(values) == 0 {
		return 0
	}
	cp := make([]int64, len(values))
	copy(cp, values)
	sortInt64(cp)
	idx := (p * (len(cp) - 1)) / 100
	return cp[idx]
}

func sortInt64(a []int64) {
	// insertion sort simples.
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}

// RankByCapability ranqueia por critérios objetivos (não inteligência).
// Critérios: JSON compliant > Available > Latency (menor melhor).
type RankedModel struct {
	Model   string
	Score   int
	Summary ProbeSummary
}

// RankByCapability ordena summaries.
func RankByCapability(summaries []ProbeSummary) []RankedModel {
	out := make([]RankedModel, len(summaries))
	for i, s := range summaries {
		score := 0
		if s.Available {
			score += 1000
		}
		if s.JSONCompliant {
			score += 500
		}
		// Latência: menor = mais pontos (cap 500).
		if s.AvgLatencyMS > 0 {
			bonus := 500 - int(s.AvgLatencyMS/100)
			if bonus < 0 {
				bonus = 0
			}
			score += bonus
		}
		out[i] = RankedModel{Model: s.Model, Score: score, Summary: s}
	}
	// insertion sort desc.
	for i := 1; i < len(out); i++ {
		v := out[i]
		j := i - 1
		for j >= 0 && out[j].Score < v.Score {
			out[j+1] = out[j]
			j--
		}
		out[j+1] = v
	}
	return out
}
