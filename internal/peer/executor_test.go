package peer

import (
	"context"
	"strings"
	"testing"

	"github.com/diegoaraujo/solidify/internal/ai"
)

func TestExecutor_AttemptsLogged(t *testing.T) {
	// mock placeholder para compilar
}

// scriptedProvider devolve uma resposta por chamada, na ordem — usado
// pra testar o fluxo de repair sem bater em provider real.
type scriptedProvider struct {
	calls     []ai.CompleteOptions
	responses []*ai.CompleteResult
}

func (p *scriptedProvider) Name() string { return "scripted" }
func (p *scriptedProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	return nil, nil
}
func (p *scriptedProvider) Metadata() ai.ProviderMetadata {
	return ai.ProviderMetadata{Name: "scripted"}
}
func (p *scriptedProvider) CompleteJSON(ctx context.Context, opts ai.CompleteOptions) (*ai.CompleteResult, error) {
	idx := len(p.calls)
	p.calls = append(p.calls, opts)
	if idx < len(p.responses) {
		return p.responses[idx], nil
	}
	return p.responses[len(p.responses)-1], nil
}

// TestExecute_RepairPromptIncludesValidationErrors reproduz achado real
// (dogfooding em projeto real, 2026-09-02): o LLM confundiu o vocabulário do
// heuristic hint ("CLEARLY_APPLICABLE", pré-análise) com o enum final do
// schema ("APPLICABLE"), e ecoou o hint no campo `applicability` — JSON
// sintaticamente válido mas rejeitado por validateCanonical. O repair
// antigo mandava só "The following JSON is malformed", sem dizer QUAL
// erro — o modelo não tinha como saber o que corrigir. O repair precisa
// incluir os erros de validação reais.
func TestExecute_RepairPromptIncludesValidationErrors(t *testing.T) {
	invalid := `{"solid":{"S":{"applicability":"APPLICABLE","score":82,"evidence_refs":["x"]},"O":{"applicability":"NOT_APPLICABLE"},"L":{"applicability":"NOT_APPLICABLE"},"I":{"applicability":"NOT_APPLICABLE"},"D":{"applicability":"CLEARLY_APPLICABLE"}},"quality_score":82}`
	repaired := `{"solid":{"S":{"applicability":"APPLICABLE","score":82,"evidence_refs":["x"]},"O":{"applicability":"NOT_APPLICABLE"},"L":{"applicability":"NOT_APPLICABLE"},"I":{"applicability":"NOT_APPLICABLE"},"D":{"applicability":"APPLICABLE","score":70,"evidence_refs":["y"]}},"quality_score":76}`

	p := &scriptedProvider{responses: []*ai.CompleteResult{
		{Content: invalid},
		{Content: repaired},
	}}
	e := NewExecutor()
	res, err := e.Execute(context.Background(), ExecutorOptions{
		Request:  &PeerRequest{Prompt: "review"},
		Provider: p,
		Model:    "m",
	})
	if err != nil {
		t.Fatalf("Execute retornou erro: %v", err)
	}
	if len(p.calls) != 2 {
		t.Fatalf("esperava 2 chamadas ao provider (original + repair), veio %d", len(p.calls))
	}
	var repairText strings.Builder
	for _, m := range p.calls[1].Messages {
		repairText.WriteString(m.Content)
	}
	if !strings.Contains(repairText.String(), "solid.D.applicability invalid") {
		t.Fatalf("repair prompt não inclui o erro de validação real: %q", repairText.String())
	}
	if res.ScoreStatus != ScoreStatusAvailable {
		t.Fatalf("ScoreStatus = %q, esperava available (repair corrigiu)", res.ScoreStatus)
	}
}

// TestParseJSONContent_StripsThinkBlocks (SAI-129D): providers como
// mimo/MiniMax-M2.1 (rota do claude-coder) emitem `<think>...</think>`
// antes do JSON. Sem o fallback de extractJSONBounds, parseJSONContent
// falhava e o report saía com score_status=unavailable mesmo com peer
// gerando resposta válida (achado do cenário violator).
func TestParseJSONContent_StripsThinkBlocks(t *testing.T) {
	content := "<think>\nThe user wants valid JSON. Let me think...\n</think>\n" +
		`{"solid":{"S":{"applicability":"APPLICABLE","score":40,"evidence_refs":["x.go:1"]}},"quality_score":40,"confidence":0.9,"summary":"ok","issues":[{"id":"X-1","severity":"high","title":"t"}]}`

	parsed, ok := parseJSONContent(content)
	if !ok {
		t.Fatalf("parseJSONContent falhou: %q", content)
	}
	if _, ok := parsed["solid"]; !ok {
		t.Fatalf("parsed sem chave 'solid': %#v", parsed)
	}
	if qs, ok := parsed["quality_score"].(float64); !ok || qs != 40 {
		t.Fatalf("quality_score = %v, esperava 40", parsed["quality_score"])
	}
}

// TestParseJSONContent_PlainJSONUnchanged: entrada sem ruído continua
// parseando igual ao caminho feliz original (sem regressão).
// TestParseJSONContent_SkipsJSONFragmentsInThinkBlocks reproduz 5da4cab6:
// claude-coder escreveu o literal `{}` dentro de <think> antes do JSON
// canônico. extractJSONBounds escolhia esse fragmento vazio e o executor
// devolvia score_status=unavailable por "missing solid".
func TestParseJSONContent_SkipsJSONFragmentsInThinkBlocks(t *testing.T) {
	content := "<think>getPreferences returns `{}` when empty</think>\n" +
		`{"solid":{"S":{"applicability":"APPLICABLE","score":80,"evidence_refs":["x.go:1"]}},"quality_score":80}`

	parsed, ok := parseJSONContent(content)
	if !ok {
		t.Fatalf("parseJSONContent falhou: %q", content)
	}
	if _, ok := parsed["solid"]; !ok {
		t.Fatalf("parsed selecionou fragmento em vez do JSON canônico: %#v", parsed)
	}
}

func TestParseJSONContent_PlainJSONUnchanged(t *testing.T) {
	content := `{"solid":{"S":{}},"quality_score":50}`
	parsed, ok := parseJSONContent(content)
	if !ok {
		t.Fatalf("plain JSON falhou: %q", content)
	}
	if parsed["quality_score"].(float64) != 50 {
		t.Fatalf("quality_score errado: %v", parsed["quality_score"])
	}
}

// TestExtractJSONBounds_BalancedBraces: o extrator não confunde chaves
// dentro de strings literais (sem `extractJSONBounds` isso seria
// frágil em content com aspas escapadas).
func TestExtractJSONBounds_BalancedBraces(t *testing.T) {
	s := `prefix {"a":"}{","b":{"c":1}} suffix`
	start, end := extractJSONBounds(s)
	if start < 0 || end <= start {
		t.Fatalf("bounds inválidos: start=%d end=%d", start, end)
	}
	got := s[start : end+1]
	if !strings.Contains(got, `"c":1`) {
		t.Fatalf("extração pegou range errado: %q", got)
	}
}
