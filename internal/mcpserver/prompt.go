// Prompt Peer v1 (SAI-056).
//
// Lê `prompts/peer-review.md`, aplica placeholders estruturados,
// computa prompt hash/version. Loader + Render helpers.
package mcpserver

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// PeerReviewPromptVersion versão atual.
const PeerReviewPromptVersion = "1"

// promptPath usado quando embed não usado.
const defaultPromptPath = "prompts/peer-review.md"

// LoadPromptOptions opções de carregamento.
type LoadPromptOptions struct {
	// Path filesystem path; optional (default: prompts/peer-review.md).
	Path string
	// Vars placeholders {name: value} — substituem {{name}} no template.
	Vars map[string]string
}

// PeerPrompt struct carregado.
type PeerPrompt struct {
	Version  string            `json:"version"`
	Hash     string            `json:"hash"` // sha256(content)
	Vars     map[string]string `json:"vars,omitempty"`
	Content  string            `json:"content"`
	VarsUsed []string          `json:"vars_used,omitempty"`
}

// RequiredVars lista vars default.
func DefaultRequiredVars() []string {
	return []string{"run_id", "actor", "evidence_hash", "schema_version"}
}

// LoadPrompt lê do filesystem (path). Vars[required] faltando
// resultam em error.
func LoadPrompt(opts LoadPromptOptions) (*PeerPrompt, error) {
	path := opts.Path
	if path == "" {
		path = defaultPromptPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("prompt: read: %w", err)
	}
	return renderPrompt(string(data), opts.Vars)
}

// LoadPromptFromContent renderiza a partir de content string.
func LoadPromptFromContent(content string, vars map[string]string) (*PeerPrompt, error) {
	return renderPrompt(content, vars)
}

// renderPrompt aplica vars + computa hash.
func renderPrompt(content string, vars map[string]string) (*PeerPrompt, error) {
	if content == "" {
		return nil, errors.New("prompt: content vazio")
	}
	rendered := content
	used := make([]string, 0)
	missing := make([]string, 0)
	for _, k := range DefaultRequiredVars() {
		v, ok := vars[k]
		if !ok || v == "" {
			missing = append(missing, k)
			continue
		}
		used = append(used, k)
		rendered = strings.ReplaceAll(rendered, "{{"+k+"}}", v)
	}
	// Vars extras além das required.
	for k, v := range vars {
		if containsStr(DefaultRequiredVars(), k) {
			continue
		}
		used = append(used, k)
		rendered = strings.ReplaceAll(rendered, "{{"+k+"}}", v)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("prompt: missing required vars: %s", strings.Join(missing, ", "))
	}
	sum := sha256.Sum256([]byte(rendered))
	return &PeerPrompt{
		Version:  PeerReviewPromptVersion,
		Hash:     hex.EncodeToString(sum[:]),
		Vars:     vars,
		Content:  rendered,
		VarsUsed: used,
	}, nil
}

// ValidateVars checa se todas required vars presentes.
func ValidateVars(vars map[string]string) []string {
	var missing []string
	for _, k := range DefaultRequiredVars() {
		if v, ok := vars[k]; !ok || v == "" {
			missing = append(missing, k)
		} else {
			_ = v
		}
	}
	return missing
}

// FormatVars renderiza como template usando fmt.Sprintf.
// Útil pra ver vars formatados.
func FormatVars(p *PeerPrompt) string {
	if p == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PeerPrompt v%s hash=%s\n", p.Version, p.Hash[:8]))
	for _, k := range p.VarsUsed {
		sb.WriteString(fmt.Sprintf("  %s=%s\n", k, p.Vars[k]))
	}
	return sb.String()
}

// HashMatches compara hash de dois prompts.
func HashMatches(a, b *PeerPrompt) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Hash == b.Hash
}

// PlaceholdersForRequired devolve {{name}} markers para required vars.
func PlaceholdersForRequired() []string {
	out := make([]string, 0)
	for _, v := range DefaultRequiredVars() {
		out = append(out, "{{"+v+"}}")
	}
	return out
}
