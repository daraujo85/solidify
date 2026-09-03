// Arbiter prompt (SAI-068).
//
// v1: input = evidence + Peer A + Peer B + divergence map.
// Template neutro (não menciona "Peer A" como ground truth).
package arbiter

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

// PromptVersion número.
const PromptVersion = "1"

// DefaultPrompt template default.
const DefaultPrompt = `You are an impartial arbiter resolving a dispute between two peer reviewers.

# Inputs

## Evidence (ground truth)
{{evidence}}

## Peer A verdict
{{peer_a}}

## Peer B verdict
{{peer_b}}

## Divergence map
{{divergence}}

# Task

Resolve the divergence between Peer A and Peer B. For each
disputed finding or principle, produce a verdict:
- accept_peer_a
- accept_peer_b
- accept_both
- reject_both

If the divergence map lists an applicable_mismatch (the two peers
disagreed on whether a SOLID principle S/O/L/I/D even applies to this
diff), you MUST resolve applicability FIRST, separately from score,
before resolving anything else for that principle:

1. applicability_verdicts: for each disputed principle, decide
   APPLICABLE, NOT_APPLICABLE or INSUFFICIENT_EVIDENCE. APPLICABLE
   requires concrete evidence_refs (symbol, hunk, dependency or
   interface actually touched by the diff) — a generic architectural
   opinion with no evidence_refs is invalid.
2. score_verdicts: only for principles you marked APPLICABLE, decide
   which peer's score to accept (or reject_both) — same accept_peer_a/
   accept_peer_b/accept_both/reject_both vocabulary. Omit score when
   decision is reject_both.

Return JSON with shape:
{
  "run_id": "...",
  "actor": "arbiter",
  "verdict": "resolved",
  "applicability_verdicts": [
    {
      "principle": "S",
      "applicability": "APPLICABLE",
      "evidence_refs": ["internal/service/tms.go:42-58"],
      "reasoning": "Diff introduces a second responsibility (billing + notification) in the same function."
    }
  ],
  "score_verdicts": [
    {
      "principle": "S",
      "decision": "accept_peer_b",
      "score": 62
    }
  ],
  "resolutions": [
    {
      "topic": "SRP:f1",
      "decision": "accept_peer_b",
      "reasoning": "Peer B cited concrete SRP violation; Peer A's high severity was unsupported."
    }
  ]
}

Omit applicability_verdicts/score_verdicts entirely when the
divergence map has no applicable_mismatch — resolutions alone is
enough for score/severity/finding disputes.`

// ArbiterPrompt template + hash + vars.
type ArbiterPrompt struct {
	Version   string            `json:"version"`
	Hash      string            `json:"hash"`
	Content   string            `json:"content"`
	Vars      map[string]string `json:"vars"`
	VarsUsed  []string          `json:"vars_used"`
	CreatedAt int64             `json:"created_at,omitempty"`
}

// ArbiterVars vars esperadas.
type ArbiterVars struct {
	Evidence string `json:"evidence"`
	PeerA    string `json:"peer_a"`
	PeerB    string `json:"peer_b"`
	DivMap   string `json:"divergence"`
}

// RequiredVars lista canônica.
func RequiredVars() []string {
	return []string{"evidence", "peer_a", "peer_b", "divergence"}
}

// ValidateVars checa vars obrigatórias.
func ValidateVars(vars map[string]string) error {
	for _, k := range RequiredVars() {
		if _, ok := vars[k]; !ok {
			return errors.New("arbiter prompt: var obrigatória ausente: " + k)
		}
	}
	return nil
}

// LoadPrompt carrega template (default se path vazio).
func LoadPrompt(templatePath string) (*ArbiterPrompt, error) {
	if templatePath == "" {
		return BuildPromptRaw(DefaultPrompt)
	}
	// Spec não exige FS read aqui; ADR seguinte pode plugar.
	return BuildPromptRaw(DefaultPrompt)
}

// BuildPromptRaw constrói sem validar vars (load-time).
func BuildPromptRaw(template string) (*ArbiterPrompt, error) {
	if template == "" {
		return nil, errors.New("arbiter prompt: template vazio")
	}
	return &ArbiterPrompt{
		Version:  PromptVersion,
		Hash:     computeTemplateHash(template),
		Content:  template,
		Vars:     map[string]string{},
		VarsUsed: extractUsedVars(template),
	}, nil
}

// BuildPrompt constrói ArbiterPrompt a partir do template+vars.
func BuildPrompt(template string, vars map[string]string) (*ArbiterPrompt, error) {
	if template == "" {
		return nil, errors.New("arbiter prompt: template vazio")
	}
	hash := computeTemplateHash(template)
	used := extractUsedVars(template)
	// Validate required used vars têm valor.
	for _, v := range used {
		if _, ok := vars[v]; !ok {
			return nil, errors.New("arbiter prompt: var usada sem valor: " + v)
		}
	}
	return &ArbiterPrompt{
		Version:  PromptVersion,
		Hash:     hash,
		Content:  template,
		Vars:     vars,
		VarsUsed: used,
	}, nil
}

// Render aplica vars no template.
func (p *ArbiterPrompt) Render() (string, error) {
	if p == nil {
		return "", errors.New("arbiter prompt: nil")
	}
	if err := ValidateVars(p.Vars); err != nil {
		return "", err
	}
	out := p.Content
	keys := make([]string, 0, len(p.Vars))
	for k := range p.Vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		placeholder := "{{" + k + "}}"
		out = strings.ReplaceAll(out, placeholder, p.Vars[k])
	}
	return out, nil
}

// RenderWithVars helper.
func (p *ArbiterPrompt) RenderWithVars(v ArbiterVars) (string, error) {
	if p == nil {
		return "", errors.New("arbiter prompt: nil")
	}
	p.Vars["evidence"] = v.Evidence
	p.Vars["peer_a"] = v.PeerA
	p.Vars["peer_b"] = v.PeerB
	p.Vars["divergence"] = v.DivMap
	return p.Render()
}

// extractUsedVars extrai {{var}} do template.
func extractUsedVars(template string) []string {
	seen := make(map[string]bool)
	var out []string
	for i := 0; i < len(template); i++ {
		if i+1 >= len(template) || template[i] != '{' || template[i+1] != '{' {
			continue
		}
		end := strings.Index(template[i+2:], "}}")
		if end < 0 {
			continue
		}
		name := template[i+2 : i+2+end]
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			i = i + 2 + end + 1
			continue
		}
		seen[name] = true
		out = append(out, name)
		i = i + 2 + end + 1
	}
	sort.Strings(out)
	return out
}

// computeTemplateHash SHA256 do template.
func computeTemplateHash(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:8])
}

// HashMatches compara 2 hashes.
func HashMatches(a, b string) bool {
	return a != "" && a == b
}

// PlaceholdersForRequired vars obrigatórias como placeholders.
func PlaceholdersForRequired() []string {
	keys := RequiredVars()
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = "{{" + k + "}}"
	}
	return out
}

// FormatVars debug.
func FormatVars(p *ArbiterPrompt) string {
	if p == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("ArbiterPrompt[")
	sb.WriteString(p.Version)
	sb.WriteString(" hash=")
	sb.WriteString(p.Hash)
	sb.WriteString("]\n")
	sb.WriteString("  vars_used: ")
	sb.WriteString(strings.Join(p.VarsUsed, ","))
	sb.WriteString("\n")
	for _, k := range RequiredVars() {
		v, ok := p.Vars[k]
		sb.WriteString("  ")
		sb.WriteString(k)
		sb.WriteString("=")
		if !ok {
			sb.WriteString("<missing>")
		} else {
			sb.WriteString(truncate(v, 60))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
