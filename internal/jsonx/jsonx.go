// JSON helpers tolerantes a ruído de LLM (think blocks, code fences,
// prosa antes do objeto canônico). Extraído de internal/peer para
// reuso entre peer.Executor e applicability.LLM.
package jsonx

import (
	"encoding/json"
	"strings"
)

// ParseContent tenta parsear s como objeto JSON.
//
// Tolerâncias:
//   - strip ``` ```json fences (ai.StripCodeFences);
//   - extração balanceada de objetos {...} ignorando strings escapadas
//     (lida com `<think>...</think>` antes do JSON canônico);
//   - primeiro candidato não-vazio que parseia vence.
//
// Retorna (parsed, true) ou (nil, false).
func ParseContent(s string) (map[string]any, bool) {
	s = strings.TrimSpace(s)
	s = stripFences(s)
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err == nil && len(v) > 0 {
		return v, true
	}
	for _, c := range Candidates(s) {
		if err := json.Unmarshal([]byte(s[c[0]:c[1]+1]), &v); err == nil && len(v) > 0 {
			return v, true
		}
	}
	return nil, false
}

// Candidates devolve cada par balanceado {…} em s, na ordem em que
// abrem. Strings literais escapadas não confundem o balanceamento.
// A maior das candidatas costuma ser o JSON canônico e descarta
// fragmentos vazios que aparecem como prosa.
func Candidates(s string) [][2]int {
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

// FirstBounds kept for backwards compatibility: pega o 1º objeto
// balanceado. Para extração robusta com múltiplos candidatos,
// prefira Candidates.
func FirstBounds(s string) (int, int) {
	cands := Candidates(s)
	if len(cands) == 0 {
		return -1, -1
	}
	return cands[0][0], cands[0][1]
}

// MatchBrace devolve o índice do `}` que fecha o `{` em start, ou -1.
func MatchBrace(s string, start int) int {
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

// stripFences remove ```json...``` wrappers.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.Index(s, "\n"); i > 0 {
		s = s[i+1:]
	}
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}