// Peer B executor (SAI-065).
//
// Nova request/context por execução (independência). Structured
// output (JSON schema). Timeout + retry único + repair quando
// output inválido.
package peer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
)

// ExecutorResult output.
type ExecutorResult struct {
	Request          *PeerRequest   `json:"request"`
	Content          string         `json:"content"`
	ParsedContent    map[string]any `json:"parsed_content,omitempty"`
	Model            string         `json:"model"`
	Provider         string         `json:"provider"`
	RepairCount      int            `json:"repair_count"`
	RetryCount       int            `json:"retry_count"`
	LatencyMS        int64          `json:"latency_ms"`
	StartedAt        time.Time      `json:"started_at"`
	CompletedAt      time.Time      `json:"completed_at"`
	Errors           []string       `json:"errors,omitempty"`
	ValidationErrors []string       `json:"validation_errors,omitempty"`
	// SAI-116: status do score. `available` = schema bateu; `unavailable` =
	// schema falhou mesmo após repair; `error` = provider falhou.
	ScoreStatus    string    `json:"score_status"`
	QualityScore   float64   `json:"quality_score"`
	RequestedModel string    `json:"requested_model,omitempty"`
	FallbackUsed   bool      `json:"fallback_used,omitempty"`
	FallbackCount  int       `json:"fallback_count,omitempty"`
	Attempts       []Attempt `json:"attempts,omitempty"`
}

// Attempt registra uma tentativa de modelo sem payload.
type Attempt struct {
	Attempt       int    `json:"attempt"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Result        string `json:"result"`
	ErrorCategory string `json:"error_category,omitempty"`
	LatencyMS     int64  `json:"latency_ms"`
}

// ExecutorOptions opções.
type ExecutorOptions struct {
	Request    *PeerRequest
	Provider   ai.Provider
	Model      string
	Timeout    time.Duration
	MaxRetries int
	JSONSchema map[string]any
	Validator  func(map[string]any) []string
}

// Executor engine.
type Executor struct {
	mu sync.Mutex
}

// NewExecutor constrói.
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute executa request com timeout/retry/repair.
func (e *Executor) Execute(ctx context.Context, opts ExecutorOptions) (*ExecutorResult, error) {
	if opts.Request == nil {
		return nil, errors.New("executor: request nil")
	}
	if opts.Provider == nil {
		return nil, errors.New("executor: provider nil")
	}
	if opts.Model == "" {
		return nil, errors.New("executor: model vazio")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	}
	result := &ExecutorResult{
		Request:        opts.Request,
		Model:          opts.Model,
		RequestedModel: opts.Model,
		Provider:       opts.Provider.Name(),
		StartedAt:      time.Now(),
	}
	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			result.RetryCount++
		}
		pctx, cancel := context.WithTimeout(ctx, opts.Timeout)
		start := time.Now()
		msgs := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a JSON-producing peer reviewer. Output only valid JSON."},
			{Role: ai.RoleUser, Content: opts.Request.Prompt},
		}
		// Sem isso o peer revisa ar: prompt instrui, mas LLM precisa do material
		// concreto (shards) pra aplicar os princípios. Sem 2ª msg ele responde
		// "no_evidence_provided" e trava o gate.
		if opts.Request.Evidence != nil && len(opts.Request.Evidence.Shards) > 0 {
			msgs = append(msgs, ai.Message{Role: ai.RoleUser, Content: buildEvidenceBody(opts.Request.Evidence)})
		}
		// SAI-116: se caller não passou schema, default é o canônico.
		// Provider que honra `response_format: json_schema` recebe nativo.
		effectiveSchema := opts.JSONSchema
		if effectiveSchema == nil {
			effectiveSchema = CanonicalSchema
		}
		res, err := opts.Provider.CompleteJSON(pctx, ai.CompleteOptions{
			Model:      opts.Model,
			Messages:   msgs,
			JSONSchema: effectiveSchema,
		})
		cancel()
		latency := time.Since(start)
		att := Attempt{
			Attempt:   attempt + 1,
			Provider:  opts.Provider.Name(),
			Model:     opts.Model,
			LatencyMS: latency.Milliseconds(),
		}
		result.LatencyMS += latency.Milliseconds()
		if err != nil {
			lastErr = err
			att.Result = "error"
			att.ErrorCategory = string(ai.ClassifyProviderError(err).Category)
			result.Attempts = append(result.Attempts, att)
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		att.Result = "success"
		result.Attempts = append(result.Attempts, att)

		parsed, ok := parseJSONContent(res.Content)
		if !ok {
			// Repair: 1 tentativa (SAI-116). Falha → ScoreStatus=unavailable,
			// sem fallback silencioso.
			if result.RepairCount == 0 {
				repaired, rerr := e.repairOutput(ctx, opts, res.Content, nil)
				if rerr == nil {
					res = repaired
					parsed, ok = parseJSONContent(res.Content)
					result.RepairCount++
				}
			}
			if !ok {
				lastErr = fmt.Errorf("output não é JSON válido após repair")
				result.Errors = append(result.Errors, lastErr.Error())
				result.ScoreStatus = ScoreStatusUnavailable
				result.CompletedAt = time.Now()
				result.Content = res.Content
				return result, nil // status indica falha, sem error
			}
		}
		// SAI-116: validação estrutural contra CanonicalSchema (se caller
		// passou outro schema, usa o dele).
		if verrs := validateCanonical(parsed); len(verrs) > 0 {
			// Tenta repair uma vez se ainda não tentou.
			if result.RepairCount == 0 {
				repaired, rerr := e.repairOutput(ctx, opts, res.Content, verrs)
				if rerr == nil {
					if reparsed, ok := parseJSONContent(repaired.Content); ok {
						if verrs2 := validateCanonical(reparsed); len(verrs2) == 0 {
							parsed = reparsed
							res = repaired
							result.RepairCount++
							result.ValidationErrors = verrs // guarda original
							goto ok
						}
					}
				}
			}
			result.ValidationErrors = verrs
			result.ScoreStatus = ScoreStatusUnavailable
			result.CompletedAt = time.Now()
			result.Content = res.Content
			result.ParsedContent = parsed
			return result, nil
		}
	ok:
		result.Content = res.Content
		result.ParsedContent = parsed
		result.QualityScore = extractQualityScore(parsed)
		result.ScoreStatus = ScoreStatusAvailable
		if opts.Validator != nil {
			if verrs := opts.Validator(parsed); len(verrs) > 0 {
				result.ValidationErrors = verrs
				lastErr = fmt.Errorf("validation failed: %v", verrs)
				continue
			}
		}
		result.CompletedAt = time.Now()
		return result, nil
	}
	result.CompletedAt = time.Now()
	result.ScoreStatus = ScoreStatusError
	return result, fmt.Errorf("executor: %d tentativas falharam: %w", opts.MaxRetries+1, lastErr)
}

// validateCanonical checa campos obrigatórios do schema canônico
// (subset — sem Draft 7 validator completo, cobre o essencial).
func validateCanonical(p map[string]any) []string {
	var errs []string
	if p == nil {
		return []string{"parsed nil"}
	}
	if _, ok := p["solid"]; !ok {
		errs = append(errs, "missing solid")
	} else if solid, ok := p["solid"].(map[string]any); ok {
		for _, k := range []string{"S", "O", "L", "I", "D"} {
			pnode, ok := solid[k]
			if !ok {
				errs = append(errs, "missing solid."+k)
				continue
			}
			pm, ok := pnode.(map[string]any)
			if !ok {
				errs = append(errs, "solid."+k+" not object")
				continue
			}
			errs = append(errs, validatePrincipleNode(k, pm)...)
		}
	} else {
		errs = append(errs, "solid not object")
	}
	if _, ok := p["quality_score"].(float64); !ok {
		errs = append(errs, "missing or invalid quality_score")
	}
	return errs
}

// validatePrincipleNode valida invariantes de um nó solid.<P>.
//
// Modo novo (SAI-129A, quando `applicability` está presente):
//   - applicability precisa ser um dos 3 estados válidos;
//   - APPLICABLE exige `score` numérico + `evidence_refs` não-vazio;
//   - NOT_APPLICABLE/INSUFFICIENT_EVIDENCE exigem `score` ausente ou
//     null (não pode inventar nota pra princípio não avaliado).
//
// Modo legado (compat, quando `applicability` está ausente): valida
// exatamente como antes do SAI-129 (after_score.value obrigatório) —
// preserva leitura de registros MCP já armazenados sem `applicability`.
func validatePrincipleNode(k string, pm map[string]any) []string {
	prefix := "solid." + k

	rawApp, hasApplicability := pm["applicability"]
	if !hasApplicability {
		after, ok := pm["after_score"]
		if !ok {
			return []string{"missing " + prefix + ".after_score"}
		}
		var errs []string
		switch a := after.(type) {
		case map[string]any:
			if _, ok := a["value"].(float64); !ok {
				errs = append(errs, prefix+".after_score.value not number")
			}
		case float64:
			// ok — formato compacto
		default:
			errs = append(errs, prefix+".after_score not object/number")
		}
		return errs
	}

	applicability, _ := rawApp.(string)
	switch applicability {
	case ApplicabilityApplicable, ApplicabilityNotApplicable, ApplicabilityInsufficientEvidence:
		// ok
	default:
		return []string{fmt.Sprintf("%s.applicability invalid: %v", prefix, rawApp)}
	}

	var errs []string
	scoreRaw, hasScore := pm["score"]
	_, scoreIsNumber := scoreRaw.(float64)

	if applicability == ApplicabilityApplicable {
		if !scoreIsNumber {
			errs = append(errs, prefix+".score required (number) when APPLICABLE")
		}
		refs, ok := pm["evidence_refs"].([]any)
		if !ok || len(refs) == 0 {
			errs = append(errs, prefix+".evidence_refs required (non-empty) when APPLICABLE")
		}
	} else if hasScore && scoreRaw != nil {
		errs = append(errs, prefix+".score must be null/absent when "+applicability)
	}
	return errs
}

// extractQualityScore retorna o score AUTORREPORTADO pelo modelo
// (preferred: quality_score, fallback: média de solid.*.after_score.value).
//
// SAI-129A: deixou de ser fonte de verdade do score oficial — isso
// agora é AggregateSolidScore() (aggregate.go), determinístico e
// nunca lê `quality_score`. Esta função sobrevive só como telemetria
// (ExecutorResult.QualityScore) e para compat de leitura de registros
// antigos; rewiring pra AggregateSolidScore() em run.go/gate/report
// fica pro SAI-129C/D (fora do escopo do SAI-129A).
func extractQualityScore(p map[string]any) float64 {
	if v, ok := p["quality_score"].(float64); ok {
		return v
	}
	if solid, ok := p["solid"].(map[string]any); ok {
		sum, n := 0.0, 0
		for _, k := range []string{"S", "O", "L", "I", "D"} {
			pnode, ok := solid[k].(map[string]any)
			if !ok {
				continue
			}
			after, ok := pnode["after_score"]
			if !ok {
				continue
			}
			switch a := after.(type) {
			case map[string]any:
				if v, ok := a["value"].(float64); ok {
					sum += v
					n++
				}
			case float64:
				sum += a
				n++
			}
		}
		if n > 0 {
			return sum / float64(n)
		}
	}
	return 0
}

// repairOutput tenta consertar JSON malformado ou schema-inválido. verrs
// (opcional) traz os erros reais de validateCanonical — sem isso o modelo
// não sabe o que corrigir e o repair costuma falhar silenciosamente
// (achado real: dogfooding em projeto real, LLM ecoou vocabulário do heuristic
// hint "CLEARLY_APPLICABLE" no campo applicability final).
func (e *Executor) repairOutput(ctx context.Context, opts ExecutorOptions, broken string, verrs []string) (*ai.CompleteResult, error) {
	var repairPrompt string
	if len(verrs) > 0 {
		repairPrompt = fmt.Sprintf("The following JSON failed schema validation with these errors: %v. Fix ONLY these issues and output only the corrected JSON:\n\n%s", verrs, broken)
	} else {
		repairPrompt = fmt.Sprintf("The following JSON is malformed. Fix it and output only the corrected JSON:\n\n%s", broken)
	}
	res, err := opts.Provider.CompleteJSON(ctx, ai.CompleteOptions{
		Model: opts.Model,
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a JSON repair assistant. Output only valid JSON."},
			{Role: ai.RoleUser, Content: repairPrompt},
		},
		JSONSchema: opts.JSONSchema,
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// parseJSONContent tenta parsear (com strip fence).
func parseJSONContent(s string) (map[string]any, bool) {
	s = ai.StripCodeFences(s)
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err == nil && len(v) > 0 {
		return v, true
	}
	// SAI-129D: alguns providers (mimo/MiniMax-M2.1, claude-coder) devolvem
	// `<think>...</think>` antes do JSON. json.Unmarshal é estrito e não
	// extrai do ruído. Acha todos os objetos balanceados (podem ser vários
	// — fragmentos vazios em prosa antes do JSON canônico) e tenta cada
	// um; o primeiro não-vazio que parseia é o retorno. Sem isso o score
	// cai pra unavailable mesmo com peer gerando resposta válida (achado
	// e2e violator; re-achado e2e em commit real de projeto).
	for _, c := range extractJSONCandidates(s) {
		if err := json.Unmarshal([]byte(s[c[0]:c[1]+1]), &v); err == nil && len(v) > 0 {
			return v, true
		}
	}
	return nil, false
}

// extractJSONCandidates retorna cada par balanceado {…} em s, na
// ordem em que abrem. Strings literais escapadas não confundem o
// balanceamento. A maior das candidatas ganha preferência em
// parseJSONContent porque costuma ser a JSON canônico e descarta
// fragmentos vazios que aparecem como prosa (`{}`, `{not JSON}`).
func extractJSONCandidates(s string) [][2]int {
	var out [][2]int
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		end := matchBrace(s, i)
		if end < 0 {
			continue
		}
		out = append(out, [2]int{i, end})
		i = end
	}
	return out
}

func matchBrace(s string, start int) int {
	depth, inStr, escape := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inStr {
			escape = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractJSONBounds kept for backwards compatibility: pega o 1º objeto
// balanceado (mesmo comportamento histórico). Para extração robusta
// com múltiplos candidatos, prefira extractJSONCandidates.
func extractJSONBounds(s string) (int, int) {
	cands := extractJSONCandidates(s)
	if len(cands) == 0 {
		return -1, -1
	}
	return cands[0][0], cands[0][1]
}

// ComputeOutputHash SHA256 do conteúdo.
func ComputeOutputHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// DefaultValidator valida schema mínimo do Peer Review.
func DefaultValidator(parsed map[string]any) []string {
	var errs []string
	for _, field := range []string{"run_id", "actor", "verdict"} {
		if _, ok := parsed[field]; !ok {
			errs = append(errs, "missing "+field)
		}
	}
	if v, ok := parsed["verdict"].(string); ok {
		switch v {
		case "approve", "request_changes", "comment":
		default:
			errs = append(errs, "verdict inválido")
		}
	}
	return errs
}

// FormatResult human-readable.
func FormatResult(r *ExecutorResult) string {
	if r == nil {
		return "<nil>"
	}
	return fmt.Sprintf("PeerExec[%s@%s model=%s latency=%dms retries=%d repairs=%d]",
		r.Provider, r.Model, r.Model, r.LatencyMS, r.RetryCount, r.RepairCount)
}

// buildEvidenceBody serializa os shards como 2ª mensagem do LLM.
// run_id + total bytes no header; cada shard vira seção markdown.
func buildEvidenceBody(set *EvidenceShardSet) string {
	var b strings.Builder
	b.WriteString("## Evidence shards (run_id=")
	b.WriteString(set.RunID)
	b.WriteString(", total=")
	b.WriteString(strconv.Itoa(set.TotalBytes))
	b.WriteString(" bytes)\n\n")
	for _, s := range set.Shards {
		b.WriteString("### [")
		b.WriteString(s.ID)
		b.WriteString("] ")
		if s.SourceFile != "" {
			b.WriteString(s.SourceFile)
		} else {
			b.WriteString(s.Kind)
		}
		b.WriteString("\n\n")
		b.WriteString(s.Content)
		b.WriteString("\n\n")
	}
	return b.String()
}
