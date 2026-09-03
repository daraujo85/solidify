package mcpserver

import (
	"strings"
	"testing"
)

// Aceitação: DefaultSetupOptions.
func TestDefaultSetupOptions(t *testing.T) {
	opts := DefaultSetupOptions()
	if opts.BinaryPath != "solidify" {
		t.Errorf("binary default")
	}
	if opts.Target != TargetClaudeCode {
		t.Errorf("target default")
	}
}

// Aceitação: GenerateSetupDoc Claude Code.
func TestGenerateClaudeCode(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: TargetClaudeCode, BinaryPath: "solidify"})
	if doc.Target != TargetClaudeCode {
		t.Errorf("target")
	}
	if !strings.Contains(doc.Snippet, "solidify") {
		t.Errorf("snippet")
	}
	if !strings.Contains(doc.Snippet, "mcp") {
		t.Errorf("snippet tem mcp")
	}
	if len(doc.Steps) == 0 {
		t.Errorf("steps")
	}
}

// Aceitação: GenerateSetupDoc Codex.
func TestGenerateCodex(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: TargetCodex})
	if doc.Target != TargetCodex {
		t.Errorf("target")
	}
	if !strings.Contains(doc.Snippet, "mcp") {
		t.Errorf("snippet")
	}
}

// Aceitação: GenerateSetupDoc Generic.
func TestGenerateGeneric(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: TargetGeneric})
	if doc.Target != TargetGeneric {
		t.Errorf("target")
	}
	if len(doc.Steps) == 0 {
		t.Errorf("steps")
	}
}

// Aceitação: Markdown render.
func TestMarkdownRender(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: TargetClaudeCode})
	md := doc.Markdown()
	if !strings.Contains(md, "# Solidify MCP setup") {
		t.Errorf("title")
	}
	if !strings.Contains(md, "## Steps") {
		t.Errorf("steps section")
	}
	if !strings.Contains(md, "## Snippet") {
		t.Errorf("snippet section")
	}
}

// Aceitação: PlainText render.
func TestPlainTextRender(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: TargetClaudeCode})
	pt := doc.PlainText()
	if !strings.Contains(pt, "Solidify MCP setup") {
		t.Errorf("title")
	}
	if !strings.Contains(pt, "Steps:") {
		t.Errorf("steps label")
	}
}

// Aceitação: binary path vazio vira default.
func TestEmptyBinaryPath(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: TargetClaudeCode, BinaryPath: ""})
	if doc.Binary != "solidify" {
		t.Errorf("default")
	}
}

// Aceitação: target vazio vira default.
func TestEmptyTarget(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: ""})
	if doc.Target != TargetClaudeCode {
		t.Errorf("default claude")
	}
}

// Aceitação: target unknown vira generic handling.
func TestUnknownTarget(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: SetupTarget("unknown")})
	if doc.Target != SetupTarget("unknown") {
		t.Errorf("preserva target")
	}
	if len(doc.Steps) == 0 {
		t.Errorf("generic steps")
	}
}

// Aceitação: ExtraNotes append.
func TestExtraNotes(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{
		Target:     TargetClaudeCode,
		ExtraNotes: []string{"custom note"},
	})
	found := false
	for _, n := range doc.Notes {
		if strings.Contains(n, "custom") {
			found = true
		}
	}
	if !found {
		t.Errorf("extra notes appended")
	}
}

// Aceitação: AllTargets.
func TestAllTargets(t *testing.T) {
	targets := AllTargets()
	if len(targets) != 3 {
		t.Errorf("3 targets")
	}
}

// Aceitação: snippet sem path destrutivo.
func TestNoDestructivePath(t *testing.T) {
	for _, tgt := range AllTargets() {
		doc := GenerateSetupDoc(SetupOptions{Target: tgt})
		if strings.Contains(doc.Snippet, "rm -rf") {
			t.Errorf("%s tem path destrutivo: %s", tgt, doc.Snippet)
		}
	}
}

// Aceitação: buildResolveHint não vazio.
func TestBuildResolveHint(t *testing.T) {
	h := buildResolveHint()
	if h == "" {
		t.Errorf("vazio")
	}
}

// Aceitação: advanced hints populated.
func TestAdvancedHints(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: TargetClaudeCode})
	if len(doc.AdvancedHints) == 0 {
		t.Errorf("advanced")
	}
}

// Aceitação: BinaryResolve populated.
func TestBinaryResolve(t *testing.T) {
	doc := GenerateSetupDoc(SetupOptions{Target: TargetClaudeCode})
	if doc.BinaryResolve == "" {
		t.Errorf("resolve hint")
	}
}
