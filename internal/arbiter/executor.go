package arbiter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
	"github.com/diegoaraujo/solidify/internal/peer"
)

// Constantes de Decision para o Verdict.
const (
	DecisionAcceptPeerA = "accept_peer_a"
	DecisionAcceptPeerB = "accept_peer_b"
	DecisionAcceptBoth  = "accept_both"
	DecisionRejectBoth  = "reject_both"
)

// Resolution reflete uma decisão sobre um tópico divergente.
type Resolution struct {
	Topic     string `json:"topic"`
	Decision  string `json:"decision"`
	Reasoning string `json:"reasoning"`
}

// ApplicabilityVerdict veredito do arbiter sobre a applicability de UM
// princípio SOLID (S/O/L/I/D) em disputa (SAI-129C) — resolve
// peer.ApplicableMismatch. Roda ANTES de score: um princípio sem
// applicability resolvida não pode ter score arbitrado (§9 do plano).
type ApplicabilityVerdict struct {
	Principle     string   `json:"principle"`     // S|O|L|I|D
	Applicability string   `json:"applicability"` // APPLICABLE|NOT_APPLICABLE|INSUFFICIENT_EVIDENCE
	EvidenceRefs  []string `json:"evidence_refs"`
	Reasoning     string   `json:"reasoning"`
}

// ScoreVerdict veredito do arbiter sobre qual peer "venceu" o score de
// UM princípio SOLID (SAI-129C). Separado de Resolutions (que é por
// finding/issue, não por princípio) — decisão do usuário no checkpoint
// de design: campo próprio em vez de sobrecarregar Resolutions ou
// ApplicabilityVerdicts. Score é o valor numérico que run.go usa pra
// recompor globalScore via peer.AggregateSolidScore; nil quando
// reject_both (princípio sai da agregação, não incluído).
type ScoreVerdict struct {
	Principle string   `json:"principle"` // S|O|L|I|D
	Decision  string   `json:"decision"`  // Decision* consts
	Score     *float64 `json:"score,omitempty"`
}

// Verdict é a decisão final do árbitro.
type Verdict struct {
	RunID                 string                 `json:"run_id"`
	Actor                 string                 `json:"actor"`
	Verdict               string                 `json:"verdict"` // "resolved", etc.
	Resolutions           []Resolution           `json:"resolutions"`
	Reasoning             string                 `json:"reasoning"`
	ApplicabilityVerdicts []ApplicabilityVerdict `json:"applicability_verdicts,omitempty"`
	ScoreVerdicts         []ScoreVerdict         `json:"score_verdicts,omitempty"`
}

// ExecutorOptions opções de execução do Arbiter.
type ExecutorOptions struct {
	RunID       string
	Evidence    string
	PeerAOutput string
	PeerBOutput string
	DivMap      string
	Provider    ai.Provider
	Model       string
	Timeout     time.Duration
	MaxRetries  int
	// RequirePrincipleVerdicts (SAI-129C): true quando o caller (run.go)
	// detectou peer.DivergenceMap.ApplicableMismatch não-vazio — força o
	// schema a exigir applicability_verdicts+score_verdicts, não deixa o
	// arbiter responder só com resolutions genéricas quando há disputa
	// de applicability real.
	RequirePrincipleVerdicts bool
}

// ExecutorResult contém o resultado do Arbiter.
type ExecutorResult struct {
	Verdict        *Verdict
	Content        string
	Model          string
	RequestedModel string
	// ResolvedModel é o model que o provider de fato executou
	// (ai.CompleteResult.Model) — distinto de RequestedModel quando um
	// combo do 9router cai pro fallback.
	ResolvedModel    string
	Provider         string
	RetryCount       int
	RepairCount      int
	LatencyMS        int64
	StartedAt        time.Time
	CompletedAt      time.Time
	Errors           []string
	ValidationErrors []string
	ScoreStatus      string
	Attempts         []peer.Attempt
}

// Executor engine.
type Executor struct {
	mu   sync.Mutex
	opts ExecutorOptions
}

// NewExecutor cria um Executor configurado.
func NewExecutor(opts ExecutorOptions) (*Executor, error) {
	if opts.RunID == "" {
		return nil, errors.New("arbiter: RunID nil ou vazio")
	}
	if opts.Provider == nil {
		return nil, errors.New("arbiter: Provider nil")
	}
	if opts.Model == "" {
		return nil, errors.New("arbiter: Model vazio")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}
	return &Executor{opts: opts}, nil
}

func parseVerdictContent(content string) (map[string]any, error) {
	content = ai.StripCodeFences(content)
	var m map[string]any
	if err := json.Unmarshal([]byte(content), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func buildVerdict(m map[string]any, defaultRunID string, requirePrincipleVerdicts bool) (*Verdict, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var v Verdict
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	if v.RunID == "" {
		if defaultRunID == "" {
			return nil, errors.New("run_id obrigatório")
		}
		v.RunID = defaultRunID
	}
	if v.Verdict == "" {
		return nil, errors.New("verdict obrigatório")
	}
	if len(v.Resolutions) == 0 {
		return nil, errors.New("resolutions obrigatórias")
	}
	if err := validatePrincipleVerdicts(v, requirePrincipleVerdicts); err != nil {
		return nil, err
	}
	return &v, nil
}

// validatePrincipleVerdicts (SAI-129C): quando requirePrincipleVerdicts
// (peer.DivergenceMap tinha ApplicableMismatch), exige que o arbiter
// tenha de fato resolvido applicability+score por princípio — não
// aceita a resposta genérica de resolutions sozinha. Mesma evidência
// mínima do peer review (evidence_refs não-vazio quando APPLICABLE,
// §8 do plano) — justificativa genérica sem evidência é rejeitada.
func validatePrincipleVerdicts(v Verdict, required bool) error {
	if required && len(v.ApplicabilityVerdicts) == 0 {
		return errors.New("applicability_verdicts obrigatório: divergência de applicability detectada")
	}
	if required && len(v.ScoreVerdicts) == 0 {
		return errors.New("score_verdicts obrigatório: divergência de applicability detectada")
	}
	for _, av := range v.ApplicabilityVerdicts {
		if av.Principle == "" {
			return errors.New("applicability_verdicts: principle vazio")
		}
		switch av.Applicability {
		case peer.ApplicabilityApplicable:
			if len(av.EvidenceRefs) == 0 {
				return errors.New("applicability_verdicts: " + av.Principle + " APPLICABLE sem evidence_refs")
			}
		case peer.ApplicabilityNotApplicable, peer.ApplicabilityInsufficientEvidence:
			// ok, sem evidência obrigatória.
		default:
			return errors.New("applicability_verdicts: " + av.Principle + " applicability inválida: " + av.Applicability)
		}
	}
	validDecisions := map[string]bool{
		DecisionAcceptPeerA: true, DecisionAcceptPeerB: true,
		DecisionAcceptBoth: true, DecisionRejectBoth: true,
	}
	for _, sv := range v.ScoreVerdicts {
		if sv.Principle == "" {
			return errors.New("score_verdicts: principle vazio")
		}
		if !validDecisions[sv.Decision] {
			return errors.New("score_verdicts: " + sv.Principle + " decision inválida: " + sv.Decision)
		}
		if sv.Decision != DecisionRejectBoth && sv.Score == nil {
			return errors.New("score_verdicts: " + sv.Principle + " decision=" + sv.Decision + " sem score")
		}
	}
	return nil
}

func (e *Executor) repairOutput(ctx context.Context, broken string) (*ai.CompleteResult, error) {
	repairPrompt := fmt.Sprintf("The following JSON is malformed. Fix it and output only the corrected JSON:\n\n%s", broken)
	res, err := e.opts.Provider.CompleteJSON(ctx, ai.CompleteOptions{
		Model: e.opts.Model,
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a JSON repair assistant. Output only valid JSON."},
			{Role: ai.RoleUser, Content: repairPrompt},
		},
		JSONSchema: schemaFor(e.opts.RequirePrincipleVerdicts),
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// buildPromptContent renderiza o template de prompt do arbiter (SAI-068,
// internal/arbiter/prompt.go) com a evidência/outputs desta execução.
// SAI-128: antes disso Execute() montava um prompt inline sem shape de
// JSON — o motor de prompt já existia mas nunca era chamado.
func (e *Executor) buildPromptContent() (string, error) {
	p, err := LoadPrompt("")
	if err != nil {
		return "", err
	}
	return p.RenderWithVars(ArbiterVars{
		Evidence: e.opts.Evidence,
		PeerA:    e.opts.PeerAOutput,
		PeerB:    e.opts.PeerBOutput,
		DivMap:   e.opts.DivMap,
	})
}

func (e *Executor) Execute(ctx context.Context) (*ExecutorResult, error) {
	opts := e.opts
	result := &ExecutorResult{
		Model:          opts.Model,
		RequestedModel: opts.Model,
		Provider:       opts.Provider.Name(),
		StartedAt:      time.Now(),
	}

	promptContent, perr := e.buildPromptContent()
	if perr != nil {
		result.CompletedAt = time.Now()
		return result, fmt.Errorf("arbiter: montar prompt: %w", perr)
	}
	msgs := []ai.Message{
		{Role: ai.RoleSystem, Content: "You are the Arbiter. Output only JSON matching the requested schema."},
		{Role: ai.RoleUser, Content: promptContent},
	}

	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			result.RetryCount++
		}

		pctx, cancel := context.WithTimeout(ctx, opts.Timeout)
		start := time.Now()

		res, err := opts.Provider.CompleteJSON(pctx, ai.CompleteOptions{
			Model:      opts.Model,
			Messages:   msgs,
			JSONSchema: schemaFor(opts.RequirePrincipleVerdicts),
		})

		cancel()
		latency := time.Since(start)
		result.LatencyMS += latency.Milliseconds()

		att := peer.Attempt{
			Attempt:   attempt + 1,
			Provider:  opts.Provider.Name(),
			Model:     opts.Model,
			LatencyMS: latency.Milliseconds(),
		}

		if err != nil {
			lastErr = err
			result.Errors = append(result.Errors, err.Error())
			att.Result = "error"
			att.ErrorCategory = string(ai.ClassifyProviderError(err).Category)
			result.Attempts = append(result.Attempts, att)
			continue
		}

		att.Result = "success"
		result.Attempts = append(result.Attempts, att)
		result.ResolvedModel = res.Model

		parsed, err := parseVerdictContent(res.Content)
		if err != nil {
			if result.RepairCount == 0 {
				repaired, rerr := e.repairOutput(ctx, res.Content)
				if rerr == nil {
					res = repaired
					result.ResolvedModel = res.Model
					parsed, err = parseVerdictContent(res.Content)
					result.RepairCount++
				}
			}
			if err != nil {
				lastErr = fmt.Errorf("output não é JSON válido: %v", err)
				result.Errors = append(result.Errors, lastErr.Error())
				result.ScoreStatus = peer.ScoreStatusUnavailable
				result.CompletedAt = time.Now()
				result.Content = res.Content
				return result, nil
			}
		}

		v, err := buildVerdict(parsed, opts.RunID, opts.RequirePrincipleVerdicts)
		if err != nil {
			result.ValidationErrors = []string{err.Error()}
			result.ScoreStatus = peer.ScoreStatusUnavailable
			result.CompletedAt = time.Now()
			result.Content = res.Content
			return result, nil
		}

		result.Content = res.Content
		result.Verdict = v
		result.CompletedAt = time.Now()
		result.ScoreStatus = peer.ScoreStatusAvailable
		return result, nil
	}

	result.CompletedAt = time.Now()
	result.ScoreStatus = peer.ScoreStatusError
	return result, fmt.Errorf("arbiter: %d tentativas falharam: %w", opts.MaxRetries+1, lastErr)
}

func computeResultHash(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// arbiterSchema JSON Schema do Verdict — espelha exatamente o que
// buildVerdict valida (verdict + resolutions obrigatórios; actor/run_id/
// reasoning opcionais). SAI-128: antes era dead code ({"type":"object"}),
// nunca passado ao provider. SAI-129C: variante base (sem disputa de
// applicability) — schemaFor() estende com applicability_verdicts/
// score_verdicts obrigatórios quando há ApplicableMismatch.
var arbiterSchema = map[string]any{
	"type":       "object",
	"required":   []string{"verdict", "resolutions"},
	"properties": arbiterSchemaProperties,
}

// arbiterSchemaProperties compartilhado entre a variante base e a
// variante com principle verdicts obrigatórios (schemaFor) — evita
// duas cópias do bloco `properties` divergirem.
var arbiterSchemaProperties = map[string]any{
	"run_id":  map[string]any{"type": "string"},
	"actor":   map[string]any{"type": "string"},
	"verdict": map[string]any{"type": "string"},
	"resolutions": map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":     "object",
			"required": []string{"topic", "decision"},
			"properties": map[string]any{
				"topic":     map[string]any{"type": "string"},
				"decision":  map[string]any{"type": "string", "enum": []any{DecisionAcceptPeerA, DecisionAcceptPeerB, DecisionAcceptBoth, DecisionRejectBoth}},
				"reasoning": map[string]any{"type": "string"},
			},
		},
	},
	"reasoning": map[string]any{"type": "string"},
	"applicability_verdicts": map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":     "object",
			"required": []string{"principle", "applicability", "evidence_refs"},
			"properties": map[string]any{
				"principle":     map[string]any{"type": "string", "enum": []any{"S", "O", "L", "I", "D"}},
				"applicability": map[string]any{"type": "string", "enum": []any{peer.ApplicabilityApplicable, peer.ApplicabilityNotApplicable, peer.ApplicabilityInsufficientEvidence}},
				"evidence_refs": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"reasoning":     map[string]any{"type": "string"},
			},
		},
	},
	"score_verdicts": map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":     "object",
			"required": []string{"principle", "decision"},
			"properties": map[string]any{
				"principle": map[string]any{"type": "string", "enum": []any{"S", "O", "L", "I", "D"}},
				"decision":  map[string]any{"type": "string", "enum": []any{DecisionAcceptPeerA, DecisionAcceptPeerB, DecisionAcceptBoth, DecisionRejectBoth}},
				"score":     map[string]any{"type": "number"},
			},
		},
	},
}

// schemaFor escolhe a variante do schema (SAI-129C): quando
// requirePrincipleVerdicts é true (run.go detectou
// peer.DivergenceMap.ApplicableMismatch não-vazio), applicability_verdicts
// e score_verdicts passam de opcionais a obrigatórios — o arbiter não
// pode resolver uma disputa de applicability só com resolutions
// genéricas.
func schemaFor(requirePrincipleVerdicts bool) map[string]any {
	if !requirePrincipleVerdicts {
		return arbiterSchema
	}
	return map[string]any{
		"type":       "object",
		"required":   []string{"verdict", "resolutions", "applicability_verdicts", "score_verdicts"},
		"properties": arbiterSchemaProperties,
	}
}
