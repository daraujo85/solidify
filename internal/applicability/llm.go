// Applicability via LLM (SAI-129C+).
//
// Decide() puro continua sendo o caminho determinístico (zero-custo,
// coberto por applicability_test.go). Este arquivo adiciona um
// caminho opcional LLM-enriquecido que decide os mesmos 8 gates
// olhando o profile (componentes/stacks/paths/flags) e devolve um
// JSON estruturado com {gate, verdict, reason, evidence}.
//
// Regras de merge LLM vs heurística:
//   - advisory (default): LLM só enriquece `Reason`; Verdict sempre
//     = heurística. Override deliberado desativado por segurança.
//   - enforce: LLM pode override Verdict; divergência é registrada
//     em Source = "llm" no resultado.
//
// Em qualquer erro de provider/parse/schema: fallback silencioso
// para a heurística. Decide() nunca falha — este caminho também.
//
// Cache: SHA256(json(profile)) como chave; LRU 64 entradas.
// Profile é imutável durante o run, cache hit garante determinismo.
package applicability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
	"github.com/diegoaraujo/solidify/internal/jsonx"
)

// Source identifica quem produziu a Decision final.
type Source string

const (
	SourceHeuristic     Source = "heuristic"
	SourceLLM           Source = "llm"
	SourceLLMFallback   Source = "llm-fallback"
)

// Mode de operação do LLMDecider.
type Mode string

const (
	ModeAdvisory Mode = "advisory"
	ModeEnforce  Mode = "enforce"
)

// llmVerdict é o verdict canônico esperado pelo LLM (mesma
// taxonomia da heurística, em uppercase consistente).
const (
	llmVerdictApplicable    = "APPLICABLE"
	llmVerdictNotApplicable = "NOT_APPLICABLE"
	llmVerdictConditional   = "CONDITIONAL"
)

// llmDecision é o payload por gate esperado do LLM.
type llmDecision struct {
	Gate     string   `json:"gate"`
	Verdict  string   `json:"verdict"`
	Reason   string   `json:"reason"`
	Evidence []string `json:"evidence,omitempty"`
}

// llmResponse é o envelope esperado: array de 8 decisions.
type llmResponse struct {
	Decisions []llmDecision `json:"decisions"`
}

// LLMDecider aplica LLM sobre applicability.Profile.
//
// Seguro chamar nil: Decide devolve erro mas DecideWithContext
// trata como "sem LLM" e cai pra heurística sem panic.
type LLMDecider struct {
	Provider ai.Provider
	Model    string
	Timeout  time.Duration
	Mode     Mode
	Cache    *DecisionCache // opcional; nil = sem cache

	mu sync.Mutex // serializa Provider.CompleteJSON + cache access
}

// NewLLMDecider constrói com defaults seguros.
func NewLLMDecider(provider ai.Provider, model string) *LLMDecider {
	return &LLMDecider{
		Provider: provider,
		Model:    model,
		Timeout:  30 * time.Second,
		Mode:     ModeAdvisory,
		Cache:    NewDecisionCache(64),
	}
}

// DecideWithContext devolve as 8 decisions para o profile. Se o LLM
// falhar (timeout, parse, schema), devolve heurística pura
// marcada com Source = SourceLLMFallback. Caller não precisa
// tratar erro — Decide() nunca quebra o run.
//
// Decisão: heurística é SEMPRE computada (é o chão); LLM é overlay.
func (l *LLMDecider) DecideWithContext(ctx context.Context, p Profile) []Decision {
	heuristic := Decide(p)
	if l == nil || l.Provider == nil || l.Model == "" {
		return markSource(heuristic, SourceHeuristic)
	}

	llmDecisions, err := l.callLLM(ctx, p)
	if err != nil {
		return markSource(heuristic, SourceLLMFallback)
	}

	merged := merge(heuristic, llmDecisions, l.Mode)
	return merged
}

// callLLM chama o provider com cache + parse + repair 1x.
//
// Profile é serializado como JSON no prompt; o schema força o LLM
// a devolver um array de 8 objetos {gate,verdict,reason,evidence}.
func (l *LLMDecider) callLLM(ctx context.Context, p Profile) ([]llmDecision, error) {
	key := profileHash(p)
	if l.Cache != nil {
		if cached, ok := l.Cache.Get(key); ok {
			return cached, nil
		}
	}

	pctx, cancel := context.WithTimeout(ctx, l.Timeout)
	defer cancel()

	msgs := []ai.Message{
		{Role: ai.RoleSystem, Content: "You are a release-profile classifier. Output only valid JSON matching the provided schema."},
		{Role: ai.RoleUser, Content: buildPrompt(p)},
	}
	res, err := l.Provider.CompleteJSON(pctx, ai.CompleteOptions{
		Model:      l.Model,
		Messages:   msgs,
		JSONSchema: applicabilityJSONSchema(),
	})
	if err != nil {
		return nil, err
	}

	parsed, ok := jsonx.ParseContent(res.Content)
	if !ok {
		return nil, errors.New("applicability: parse falhou")
	}

	decisions, verrs := validateApplicabilityParsed(parsed)
	if len(verrs) > 0 {
		// 1 repair só — sem retry, sem loops.
		repaired, rerr := l.repairOnce(ctx, res.Content, verrs)
		if rerr != nil {
			return nil, rerr
		}
		parsed, ok = jsonx.ParseContent(repaired)
		if !ok {
			return nil, errors.New("applicability: parse falhou após repair")
		}
		decisions, verrs = validateApplicabilityParsed(parsed)
		if len(verrs) > 0 {
			return nil, fmt.Errorf("applicability: schema inválido: %v", verrs)
		}
	}

	if l.Cache != nil {
		l.Cache.Put(key, decisions)
	}
	return decisions, nil
}

// repairOnce tenta consertar JSON malformado ou schema-inválido,
// passando os validation errors reais pro modelo (sem isso o LLM
// não sabe o que corrigir).
func (l *LLMDecider) repairOnce(ctx context.Context, broken string, verrs []string) (string, error) {
	var prompt string
	if len(verrs) > 0 {
		prompt = fmt.Sprintf("The following JSON failed schema validation with these errors: %v. Fix ONLY these issues and output only the corrected JSON:\n\n%s", verrs, broken)
	} else {
		prompt = fmt.Sprintf("The following JSON is malformed. Fix it and output only the corrected JSON:\n\n%s", broken)
	}
	res, err := l.Provider.CompleteJSON(ctx, ai.CompleteOptions{
		Model: l.Model,
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a JSON repair assistant. Output only valid JSON."},
			{Role: ai.RoleUser, Content: prompt},
		},
		JSONSchema: applicabilityJSONSchema(),
	})
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

// profileHash devolve SHA256 do profile serializado (chave de cache).
func profileHash(p Profile) string {
	// Serializa deterministicamente: ordena paths pra evitar reordering
	// entre runs.
	cp := p
	cp.ChangedPaths = append([]string(nil), p.ChangedPaths...)
	sort.Strings(cp.ChangedPaths)
	data, _ := json.Marshal(cp)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// buildPrompt monta o user message: profile serializado + instrução
// curta. Pequeno de propósito — heurística é o chão, LLM só enriquece.
func buildPrompt(p Profile) string {
	body, _ := json.Marshal(p)
	return fmt.Sprintf(`Decide applicability of these 8 CI gates for the following release profile. Return ONLY valid JSON matching the schema ({"decisions":[{"gate","verdict","reason","evidence"}]}).

Rules:
- Use verdict values exactly: APPLICABLE, NOT_APPLICABLE, CONDITIONAL.
- The 8 gates MUST appear exactly once each, in any order: sonar, tests, security, lighthouse, zap, k6, migration, env.
- "reason" ≤ 200 chars, auditável.
- "evidence" is optional list of changed paths that justify the verdict (omit if none).

Profile:
%s`, string(body))
}

// applicabilityJSONSchema é o schema JSON força de saída.
func applicabilityJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"decisions"},
		"properties": map[string]any{
			"decisions": map[string]any{
				"type":     "array",
				"minItems": 8,
				"maxItems": 8,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"gate", "verdict", "reason"},
					"properties": map[string]any{
						"gate": map[string]any{
							"type": "string",
							"enum": []string{"sonar", "tests", "security", "lighthouse", "zap", "k6", "migration", "env"},
						},
						"verdict": map[string]any{
							"type": "string",
							"enum": []string{llmVerdictApplicable, llmVerdictNotApplicable, llmVerdictConditional},
						},
						"reason": map[string]any{"type": "string"},
						"evidence": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}
}

// validateApplicabilityParsed checa envelope e cobertura de 8 gates.
func validateApplicabilityParsed(parsed map[string]any) ([]llmDecision, []string) {
	var errs []string
	if parsed == nil {
		return nil, []string{"parsed nil"}
	}
	raw, ok := parsed["decisions"].([]any)
	if !ok {
		return nil, []string{"missing or invalid 'decisions' array"}
	}
	if len(raw) != 8 {
		errs = append(errs, fmt.Sprintf("decisions length=%d, want 8", len(raw)))
	}
	out := make([]llmDecision, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("decisions[%d] not object", i))
			continue
		}
		var d llmDecision
		if g, ok := m["gate"].(string); ok {
			d.Gate = g
		} else {
			errs = append(errs, fmt.Sprintf("decisions[%d].gate missing", i))
		}
		if v, ok := m["verdict"].(string); ok {
			d.Verdict = v
		} else {
			errs = append(errs, fmt.Sprintf("decisions[%d].verdict missing", i))
		}
		if r, ok := m["reason"].(string); ok {
			d.Reason = r
		}
		if ev, ok := m["evidence"].([]any); ok {
			for _, e := range ev {
				if s, ok := e.(string); ok {
					d.Evidence = append(d.Evidence, s)
				}
			}
		}
		out = append(out, d)
	}
	// Cobertura: todos os 8 gates únicos.
	seen := map[string]int{}
	for _, d := range out {
		seen[d.Gate]++
	}
	for _, g := range allGates() {
		if seen[g] != 1 {
			errs = append(errs, fmt.Sprintf("gate %q apareceu %d vezes, quero 1", g, seen[g]))
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return out, nil
}

func allGates() []string {
	return []string{
		string(GateSonar), string(GateTests), string(GateSecurity),
		string(GateLighthouse), string(GateZAP), string(GateK6),
		string(GateMigration), string(GateEnv),
	}
}

// merge funde heurística + LLM conforme Mode.
//
// advisory (default): Verdict = heurística; Reason = heurística
//                    concatenada com LLM (LLM enriquece sem override).
// enforce:           Verdict = LLM se LLM diverge; heurística vence
//                    só se LLM omitiu o gate.
func merge(heuristic []Decision, llmDecisions []llmDecision, mode Mode) []Decision {
	byGateLLM := map[string]llmDecision{}
	for _, d := range llmDecisions {
		byGateLLM[d.Gate] = d
	}
	out := make([]Decision, 0, len(heuristic))
	for _, h := range heuristic {
		llm, ok := byGateLLM[string(h.Gate)]
		if !ok {
			out = append(out, Decision{Gate: h.Gate, Verdict: h.Verdict, Reason: h.Reason})
			continue
		}
		verdict := h.Verdict
		if mode == ModeAdvisory || !validVerdict(llm.Verdict) {
			verdict = h.Verdict
		} else {
			v := normalizeVerdict(llm.Verdict)
			if v != "" {
				verdict = v
			}
		}
		reason := mergeReason(h.Reason, llm.Reason, llm.Evidence, mode)
		out = append(out, Decision{
			Gate:        h.Gate,
			Verdict:     verdict,
			Reason:      reason,
			Source:      SourceLLM,
			LLMAssessed: true,
		})
	}
	return out
}

// mergeReason combina heurística + LLM conforme o modo.
//
// advisory: LLM Reason é anexado se for novo (heurística primeiro).
// enforce: LLM Reason substitui se existir; senão heurística.
func mergeReason(hReason, lReason string, evidence []string, mode Mode) string {
	lReason = trim(lReason, 200)
	if lReason == "" {
		return hReason
	}
	if mode == ModeAdvisory {
		if lReason == hReason {
			return hReason
		}
		// anexa com separador auditável
		extra := lReason
		if len(evidence) > 0 {
			extra = lReason + " [evidence: " + joinShort(evidence, 3) + "]"
		}
		if hReason == "" {
			return extra
		}
		return hReason + " | LLM: " + extra
	}
	// enforce: LLM vence.
	if len(evidence) > 0 {
		return lReason + " [evidence: " + joinShort(evidence, 3) + "]"
	}
	return lReason
}

func joinShort(items []string, max int) string {
	if len(items) <= max {
		return joinComma(items)
	}
	return joinComma(items[:max]) + ", ..."
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func trim(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

func validVerdict(v string) bool {
	switch v {
	case llmVerdictApplicable, llmVerdictNotApplicable, llmVerdictConditional:
		return true
	}
	return false
}

func normalizeVerdict(v string) Verdict {
	switch v {
	case llmVerdictApplicable:
		return Applicable
	case llmVerdictNotApplicable:
		return NotApplicable
	case llmVerdictConditional:
		return Conditional
	}
	return ""
}

// markSource preenche Source em todas as decisions.
func markSource(ds []Decision, src Source) []Decision {
	out := make([]Decision, 0, len(ds))
	for _, d := range ds {
		d.Source = src
		out = append(out, d)
	}
	return out
}

// ---- DecisionCache (LRU 64) ----

// DecisionCache is a tiny LRU keyed by profile hash → llmDecision[].
// Sync-safe; capacidade fixa.
type DecisionCache struct {
	mu    sync.Mutex
	cap   int
	order []string                  // insertion order, oldest first
	data  map[string][]llmDecision
}

// NewDecisionCache builds a cache with capacity. cap <= 0 ⇒ 64.
func NewDecisionCache(cap int) *DecisionCache {
	if cap <= 0 {
		cap = 64
	}
	return &DecisionCache{
		cap:  cap,
		data: map[string][]llmDecision{},
	}
}

// Get returns cached decisions, true if hit.
func (c *DecisionCache) Get(key string) ([]llmDecision, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.data[key]
	return d, ok
}

// Put stores; evicts oldest if over capacity.
func (c *DecisionCache) Put(key string, decisions []llmDecision) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.data[key]; !ok {
		c.order = append(c.order, key)
	}
	// defensive copy
	cp := make([]llmDecision, len(decisions))
	copy(cp, decisions)
	c.data[key] = cp
	for len(c.order) > c.cap {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.data, oldest)
	}
}