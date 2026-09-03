package arbiter

import (
	"strings"
	"testing"
)

// Aceitação: PromptVersion.
func TestPromptVersion(t *testing.T) {
	if PromptVersion == "" {
		t.Errorf("versão vazia")
	}
}

// Aceitação: DefaultPrompt não vazio.
func TestDefaultPrompt(t *testing.T) {
	if DefaultPrompt == "" {
		t.Errorf("default vazio")
	}
	if !strings.Contains(DefaultPrompt, "{{evidence}}") {
		t.Errorf("evidence placeholder")
	}
}

// Aceitação: RequiredVars.
func TestRequiredVars(t *testing.T) {
	vars := RequiredVars()
	if len(vars) != 4 {
		t.Errorf("esperado 4: %v", vars)
	}
	want := map[string]bool{"evidence": true, "peer_a": true, "peer_b": true, "divergence": true}
	for _, v := range vars {
		if !want[v] {
			t.Errorf("var inesperada: %s", v)
		}
	}
}

// Aceitação: ValidateVars OK.
func TestValidateVarsOK(t *testing.T) {
	vars := map[string]string{
		"evidence": "e", "peer_a": "a", "peer_b": "b", "divergence": "d",
	}
	if err := ValidateVars(vars); err != nil {
		t.Errorf("err: %v", err)
	}
}

// Aceitação: ValidateVars missing.
func TestValidateVarsMissing(t *testing.T) {
	if err := ValidateVars(map[string]string{"evidence": "e"}); err == nil {
		t.Errorf("vazio")
	}
	if err := ValidateVars(map[string]string{}); err == nil {
		t.Errorf("map vazio")
	}
}

// Aceitação: BuildPrompt basic.
func TestBuildPrompt(t *testing.T) {
	p, err := BuildPrompt(DefaultPrompt, map[string]string{
		"evidence": "ev", "peer_a": "pa", "peer_b": "pb", "divergence": "dm",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Version != PromptVersion {
		t.Errorf("version")
	}
	if p.Hash == "" {
		t.Errorf("hash vazio")
	}
	if len(p.VarsUsed) == 0 {
		t.Errorf("vars_used vazio")
	}
}

// Aceitação: BuildPrompt template vazio.
func TestBuildPromptEmpty(t *testing.T) {
	if _, err := BuildPrompt("", map[string]string{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: BuildPrompt var usada sem valor.
func TestBuildPromptMissingUsedVar(t *testing.T) {
	tpl := "Hi {{name}}"
	if _, err := BuildPrompt(tpl, map[string]string{}); err == nil {
		t.Errorf("missing var")
	}
}

// Aceitação: Render basic.
func TestRender(t *testing.T) {
	p, _ := BuildPrompt(DefaultPrompt, map[string]string{
		"evidence": "EV", "peer_a": "PA", "peer_b": "PB", "divergence": "DM",
	})
	out, err := p.Render()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "EV") {
		t.Errorf("evidence não renderizado")
	}
	if !strings.Contains(out, "PA") {
		t.Errorf("peer_a não renderizado")
	}
	if !strings.Contains(out, "PB") {
		t.Errorf("peer_b não renderizado")
	}
	if !strings.Contains(out, "DM") {
		t.Errorf("divergence não renderizado")
	}
}

// Aceitação: Render missing var.
func TestRenderMissing(t *testing.T) {
	p := &ArbiterPrompt{
		Version: PromptVersion,
		Content: DefaultPrompt,
		Vars:    map[string]string{"evidence": "x"},
	}
	if _, err := p.Render(); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Render nil.
func TestRenderNil(t *testing.T) {
	var p *ArbiterPrompt
	if _, err := p.Render(); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: RenderWithVars.
func TestRenderWithVars(t *testing.T) {
	p, _ := BuildPrompt(DefaultPrompt, map[string]string{
		"evidence": "EV", "peer_a": "PA", "peer_b": "PB", "divergence": "DM",
	})
	v := ArbiterVars{
		Evidence: "evidence-text",
		PeerA:    "peer-a-text",
		PeerB:    "peer-b-text",
		DivMap:   "div-text",
	}
	out, err := p.RenderWithVars(v)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, s := range []string{"evidence-text", "peer-a-text", "peer-b-text", "div-text"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s", s)
		}
	}
}

// Aceitação: extractUsedVars.
func TestExtractUsedVars(t *testing.T) {
	tpl := "Hi {{name}}, age {{age}}, {{name}} again"
	vars := extractUsedVars(tpl)
	if len(vars) != 2 {
		t.Errorf("dedup: %v", vars)
	}
}

// Aceitação: extractUsedVars dedup + sort.
func TestExtractUsedVarsSort(t *testing.T) {
	tpl := "{{b}} {{a}} {{c}}"
	vars := extractUsedVars(tpl)
	if len(vars) != 3 {
		t.Errorf("count: %v", vars)
	}
	if vars[0] != "a" || vars[1] != "b" || vars[2] != "c" {
		t.Errorf("sort: %v", vars)
	}
}

// Aceitação: extractUsedVars vazio.
func TestExtractUsedVarsEmpty(t *testing.T) {
	vars := extractUsedVars("sem placeholders")
	if len(vars) != 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: extractUsedVars placeholder malformado.
func TestExtractUsedVarsMalformed(t *testing.T) {
	vars := extractUsedVars("hi {{ unclosed")
	if len(vars) != 0 {
		t.Errorf("malformed")
	}
}

// Aceitação: Hash determinístico.
func TestHashDeterministic(t *testing.T) {
	h1 := computeTemplateHash(DefaultPrompt)
	h2 := computeTemplateHash(DefaultPrompt)
	if h1 != h2 {
		t.Errorf("stable")
	}
}

// Aceitação: Hash muda com template.
func TestHashSensitive(t *testing.T) {
	h1 := computeTemplateHash(DefaultPrompt)
	h2 := computeTemplateHash(DefaultPrompt + " ")
	if h1 == h2 {
		t.Errorf("sensitive")
	}
}

// Aceitação: HashMatches.
func TestHashMatches(t *testing.T) {
	if !HashMatches("abc", "abc") {
		t.Errorf("igual")
	}
	if HashMatches("abc", "xyz") {
		t.Errorf("diff")
	}
	if HashMatches("", "abc") {
		t.Errorf("vazio")
	}
}

// Aceitação: PlaceholdersForRequired.
func TestPlaceholdersForRequired(t *testing.T) {
	phs := PlaceholdersForRequired()
	if len(phs) != 4 {
		t.Errorf("count: %v", phs)
	}
	for _, p := range phs {
		if !strings.HasPrefix(p, "{{") || !strings.HasSuffix(p, "}}") {
			t.Errorf("placeholder: %s", p)
		}
	}
}

// Aceitação: LoadPrompt default.
func TestLoadPromptDefault(t *testing.T) {
	p, err := LoadPrompt("")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Version != PromptVersion {
		t.Errorf("version")
	}
}

// Aceitação: FormatVars.
func TestFormatVars(t *testing.T) {
	p, _ := BuildPrompt(DefaultPrompt, map[string]string{
		"evidence": "very long evidence string that should be truncated for display purposes",
		"peer_a":   "PA", "peer_b": "PB", "divergence": "DM",
	})
	out := FormatVars(p)
	if !strings.Contains(out, "ArbiterPrompt") {
		t.Errorf("header")
	}
	if !strings.Contains(out, "evidence=") {
		t.Errorf("var key")
	}
	if !strings.Contains(out, "truncated") {
		t.Errorf("truncate")
	}
	if FormatVars(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: Render idempotência (vars não vazam).
func TestRenderNoLeak(t *testing.T) {
	p, _ := BuildPrompt(DefaultPrompt, map[string]string{
		"evidence": "EV", "peer_a": "PA", "peer_b": "PB", "divergence": "DM",
	})
	out, _ := p.Render()
	if strings.Contains(out, "{{") {
		t.Errorf("placeholder não substituído: %s", out)
	}
}
