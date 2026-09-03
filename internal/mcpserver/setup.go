// Agent setup docs (SAI-054).
//
// Gera instruções para registrar MCP Solidify em Claude Code
// e Codex SEM hardcode de home path destrutivo. Caller decide
// onde aplicar (print, save, copy-to-clipboard).
package mcpserver

import (
	"fmt"
	"runtime"
	"strings"
)

// SetupTarget enum.
type SetupTarget string

const (
	TargetClaudeCode SetupTarget = "claude-code"
	TargetCodex      SetupTarget = "codex"
	TargetGeneric    SetupTarget = "generic"
)

// SetupDoc estrutura do snippet retornado.
type SetupDoc struct {
	Target        SetupTarget `json:"target"`
	Binary        string      `json:"binary"`
	BinaryResolve string      `json:"binary_resolve"`
	Steps         []string    `json:"steps"`
	Snippet       string      `json:"snippet"`
	Notes         []string    `json:"notes,omitempty"`
	AdvancedHints []string    `json:"advanced_hints,omitempty"`
}

// SetupOptions configurações.
type SetupOptions struct {
	BinaryPath string // default "solidify"
	Target     SetupTarget
	// ExtraNotes (advanced) — caller-provided.
	ExtraNotes []string
}

// DefaultSetupOptions.
func DefaultSetupOptions() SetupOptions {
	return SetupOptions{
		BinaryPath: "solidify",
		Target:     TargetClaudeCode,
	}
}

// GenerateSetupDoc monta instruções não-destrutivas. Caller
// decide o que fazer com o resultado (print, save, copy).
func GenerateSetupDoc(opts SetupOptions) SetupDoc {
	if opts.BinaryPath == "" {
		opts.BinaryPath = "solidify"
	}
	if opts.Target == "" {
		opts.Target = TargetClaudeCode
	}
	doc := SetupDoc{
		Target:        opts.Target,
		Binary:        opts.BinaryPath,
		BinaryResolve: buildResolveHint(),
	}
	switch opts.Target {
	case TargetClaudeCode:
		doc.Steps = claudeCodeSteps(opts.BinaryPath)
		doc.Snippet = claudeCodeSnippet(opts.BinaryPath)
		doc.Notes = claudeCodeNotes()
		doc.AdvancedHints = advancedHints()
	case TargetCodex:
		doc.Steps = codexSteps(opts.BinaryPath)
		doc.Snippet = codexSnippet(opts.BinaryPath)
		doc.Notes = codexNotes()
		doc.AdvancedHints = advancedHints()
	default:
		doc.Steps = genericSteps(opts.BinaryPath)
		doc.Snippet = genericSnippet(opts.BinaryPath)
		doc.Notes = genericNotes()
		doc.AdvancedHints = advancedHints()
	}
	if len(opts.ExtraNotes) > 0 {
		doc.Notes = append(doc.Notes, opts.ExtraNotes...)
	}
	return doc
}

// Markdown renderiza doc como markdown.
func (d SetupDoc) Markdown() string {
	var sb strings.Builder
	sb.WriteString("# Solidify MCP setup — " + string(d.Target) + "\n\n")
	sb.WriteString("Binary: `" + d.Binary + "`\n\n")
	sb.WriteString("Resolve: " + d.BinaryResolve + "\n\n")
	if len(d.Steps) > 0 {
		sb.WriteString("## Steps\n\n")
		for i, s := range d.Steps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		sb.WriteString("\n")
	}
	if d.Snippet != "" {
		sb.WriteString("## Snippet\n\n```json\n" + d.Snippet + "\n```\n\n")
	}
	if len(d.Notes) > 0 {
		sb.WriteString("## Notes\n\n")
		for _, n := range d.Notes {
			sb.WriteString("- " + n + "\n")
		}
		sb.WriteString("\n")
	}
	if len(d.AdvancedHints) > 0 {
		sb.WriteString("## Advanced\n\n")
		for _, h := range d.AdvancedHints {
			sb.WriteString("- " + h + "\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// PlainText renderiza como plain text (sem markdown).
func (d SetupDoc) PlainText() string {
	var sb strings.Builder
	sb.WriteString("Solidify MCP setup — " + string(d.Target) + "\n")
	sb.WriteString("Binary: " + d.Binary + "\n")
	if len(d.Steps) > 0 {
		sb.WriteString("Steps:\n")
		for i, s := range d.Steps {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
		}
	}
	if d.Snippet != "" {
		sb.WriteString("Snippet:\n")
		sb.WriteString(d.Snippet + "\n")
	}
	if len(d.Notes) > 0 {
		sb.WriteString("Notes:\n")
		for _, n := range d.Notes {
			sb.WriteString("  - " + n + "\n")
		}
	}
	return sb.String()
}

// buildResolveHint dica de resolução de path.
func buildResolveHint() string {
	switch runtime.GOOS {
	case "windows":
		return "use `where solidify` (cmd) or `Get-Command solidify` (pwsh) para localizar o binário."
	case "darwin", "linux":
		return "use `which solidify` ou `command -v solidify` para localizar o binário."
	default:
		return "use `which solidify` para localizar o binário."
	}
}

func claudeCodeSteps(bin string) []string {
	return []string{
		"Localize o binário: `which " + bin + "` (ou `command -v`)",
		"Edite `~/.claude.json` (ou via `claude mcp add` na CLI interativa)",
		"Adicione o server Solidify na seção `mcpServers`",
		"Reinicie a sessão / reinicie o CLI",
		"Verifique digitando no Claude: `tools/list` deve incluir `solidify_begin_review`",
	}
}

func claudeCodeSnippet(bin string) string {
	return `{
  "mcpServers": {
    "solidify": {
      "command": "` + bin + `",
      "args": ["mcp", "serve"],
      "env": {}
    }
  }
}`
}

func claudeCodeNotes() []string {
	return []string{
		"alterar `command` para o path absoluto detectado em (1) se necessário",
		"`args` deve bater com o subcomando do Solidify (default: `mcp serve`)",
		"em Windows, troque `~` por `%USERPROFILE%`",
		"este comando NÃO modifica nada sozinho; copie o snippet e cole manualmente",
	}
}

func codexSteps(bin string) []string {
	return []string{
		"Localize o binário: `which " + bin + "`",
		"Edite o config MCP do Codex (geralmente `~/.codex/mcp.json`)",
		"Adicione ou substitua a entrada `solidify`",
		"Reinicie o Codex",
		"Confirme via `/mcp` ou comando equivalente",
	}
}

func codexSnippet(bin string) string {
	return `{
  "mcp": {
    "servers": {
      "solidify": {
        "command": "` + bin + `",
        "args": ["mcp", "serve"],
        "env": {}
      }
    }
  }
}`
}

func codexNotes() []string {
	return []string{
		"formato JSON do Codex pode divergir em versões diferentes — confira docs",
		"`env` opcional: use para SOLIDIFY_* env vars (auth, paths)",
		"este comando NÃO escreve o config; caller decide onde aplicar",
	}
}

func genericSteps(bin string) []string {
	return []string{
		"Localize o binário: `which " + bin + "`",
		"Adicione entry MCP conforme documentação do seu client",
		"Reinicie o client",
	}
}

func genericSnippet(bin string) string {
	return `{
  "command": "` + bin + `",
  "args": ["mcp", "serve"]
}`
}

func genericNotes() []string {
	return []string{
		"clientes diferentes têm schemas diferentes; adapte o JSON",
		"não hardcode home path absoluto — use `which`/`command -v` no passo 1",
	}
}

func advancedHints() []string {
	return []string{
		"em CI, prefira `command -v` + path resolution em vez de hardcode de `/usr/local/bin`",
		"se quiser autostart, registre via `systemd --user` (Linux) ou `launchd` (macOS)",
		"para sandbox do Solidify, exponha apenas `mcp serve` — nunca expõe a CLI completa",
		"versione o config MCP junto com o repo (`mcp.servers.json` versionado)",
	}
}

// AllTargets retorna todos targets disponíveis.
func AllTargets() []SetupTarget {
	return []SetupTarget{TargetClaudeCode, TargetCodex, TargetGeneric}
}
