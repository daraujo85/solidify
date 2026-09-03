// Motor heurístico de applicability (SAI-129B).
//
// Roda ANTES de qualquer chamada a LLM, sobre o diff bruto
// (gitx.DiffFile) — nunca sobre output de peer. Decide, por
// princípio SOLID, se há sequer candidatura a avaliação:
//
//   - HeuristicNotApplicable: sinal forte de que o hunk não pode
//     acionar o princípio (rename puro, comentário/doc, whitespace).
//     Só decide sozinho com sinal FORTE — nunca por ausência de sinal.
//   - HeuristicApplicable: sinal estrutural de que o princípio está
//     em jogo (novo import, nova interface pública, novo branch
//     condicional, múltiplos domínios tocados). Passa pro LLM já
//     com o sinal — pede score, não applicability.
//   - HeuristicAmbiguous: default seguro quando nenhum sinal forte
//     bate — LLM decide applicability antes de score (§8 do plano).
//
// Nota deliberada de escopo (flagged pro usuário no checkpoint):
// o plano/ADR-0129 propunha `internal/applicability` como pacote
// novo. Esse nome colide com o pacote internal/applicability JÁ
// existente (gates de CI/CD — Lighthouse/Sonar/Security/etc.,
// ARCHITECTURE.md §8.4), que não tem relação nenhuma com SOLID.
// Pra evitar colisão de nome/import cycle, o motor entra aqui
// dentro de internal/peer (mesmo padrão de aggregate.go, SAI-129A).
package peer

import (
	"sort"
	"strings"

	"github.com/diegoaraujo/solidify/internal/gitx"
)

// Applicability heurística (distinto de peer.Applicability* — aquele
// é o veredito FINAL vindo do schema; este é só o sinal pré-LLM).
const (
	HeuristicNotApplicable = "CLEARLY_NOT_APPLICABLE"
	HeuristicApplicable    = "CLEARLY_APPLICABLE"
	HeuristicAmbiguous     = "AMBIGUOUS"
)

// PrincipleHint sinal heurístico de um princípio, auditável.
type PrincipleHint struct {
	Principle string `json:"principle"` // S|O|L|I|D
	Hint      string `json:"hint"`      // HeuristicNotApplicable|HeuristicApplicable|HeuristicAmbiguous
	Reason    string `json:"reason"`
}

// principleOrder ordem canônica S,O,L,I,D usada em toda iteração.
var principleOrder = []string{"S", "O", "L", "I", "D"}

// ClassifyApplicability roda a heurística sobre o diff inteiro e
// devolve um hint por princípio. Nunca retorna hint vazio — default
// é sempre AMBIGUOUS (nunca inventa NOT_APPLICABLE sem sinal forte).
func ClassifyApplicability(files []gitx.DiffFile) map[string]PrincipleHint {
	hints := defaultAmbiguousHints()

	if len(files) == 0 {
		return hints
	}

	allTrivial := true
	allTestFiles := true
	domains := map[string]bool{}
	sawImport, sawPublicSymbol, sawConditional := false, false, false

	for _, f := range files {
		fileTrivial := isTrivialFile(f)
		if !isTestFilePath(f.Path) {
			allTestFiles = false
		}
		if len(f.Hunks) == 0 {
			// Sem hunks (rename puro, mode-only change): só conta como
			// trivial se isTrivialFile já confirmou; senão, sinal
			// desconhecido não pode se passar por trivial (conservador).
			if !fileTrivial {
				allTrivial = false
			}
			continue
		}
		for _, h := range f.Hunks {
			// allTrivial exige TODO hunk de TODO arquivo trivial — nível
			// certo é o hunk (skeletonEqual), não o arquivo: um arquivo
			// "não-doc" ainda pode ter só hunks triviais (ex: rename de
			// identificador dentro de um .go comum), que é exatamente o
			// golden case do plano.
			if !(fileTrivial || isTrivialHunk(h)) {
				allTrivial = false
			}
			if fileTrivial || isTrivialHunk(h) {
				continue
			}
			sig := fileSignals(f.Path, h)
			if sig.hasImport {
				sawImport = true
				for d := range sig.importDomains {
					domains[d] = true
				}
			}
			if sig.hasPublicSymbol {
				sawPublicSymbol = true
			}
			if sig.hasConditional {
				sawConditional = true
			}
		}
	}

	if allTrivial {
		// Rename/comentário/whitespace puro: nenhum princípio é
		// sequer candidato. Sinal mais forte do motor.
		for _, p := range principleOrder {
			hints[p] = PrincipleHint{Principle: p, Hint: HeuristicNotApplicable,
				Reason: "diff trivial (rename/comentário/whitespace) — sem mudança de símbolo ou dependência"}
		}
		return hints
	}

	if allTestFiles {
		// Regra da tabela do plano §8: diff só em teste, sem tocar
		// produção → O/L/I ficam N/A; S/D seguem AMBIGUOUS (podem
		// captar duplicação de setup em S ou nova dependency em D).
		for _, p := range []string{"O", "L", "I"} {
			hints[p] = PrincipleHint{Principle: p, Hint: HeuristicNotApplicable,
				Reason: "diff restrito a arquivo(s) de teste, sem tocar produção"}
		}
	}

	if sawImport {
		hints["D"] = PrincipleHint{Principle: "D", Hint: HeuristicApplicable,
			Reason: "novo import/dependency edge detectado no diff"}
	}
	// allTestFiles já fixou O/L/I como N/A (regra da tabela do plano §8:
	// sinal estrutural dentro de arquivo de teste não promove O/L/I de
	// volta a applicable — só produção real aciona esses 3).
	if sawPublicSymbol && !allTestFiles {
		hints["I"] = PrincipleHint{Principle: "I", Hint: HeuristicApplicable,
			Reason: "novo símbolo público (interface/struct/método exportado) detectado"}
		hints["L"] = PrincipleHint{Principle: "L", Hint: HeuristicApplicable,
			Reason: "novo símbolo público pode alterar contrato de substituição"}
	}
	if sawConditional && !allTestFiles {
		hints["O"] = PrincipleHint{Principle: "O", Hint: HeuristicApplicable,
			Reason: "novo branch condicional sobre tipo/comportamento detectado"}
	}
	if len(domains) > 1 {
		hints["S"] = PrincipleHint{Principle: "S", Hint: HeuristicApplicable,
			Reason: "diff toca múltiplos domínios de import — possível múltipla responsabilidade"}
	}

	return hints
}

// AllNotApplicable true quando os 5 princípios vieram CLEARLY_NOT_APPLICABLE
// — condição de skip total do LLM (golden case do rename).
func AllNotApplicable(hints map[string]PrincipleHint) bool {
	if len(hints) != len(principleOrder) {
		return false
	}
	for _, p := range principleOrder {
		h, ok := hints[p]
		if !ok || h.Hint != HeuristicNotApplicable {
			return false
		}
	}
	return true
}

func defaultAmbiguousHints() map[string]PrincipleHint {
	hints := make(map[string]PrincipleHint, len(principleOrder))
	for _, p := range principleOrder {
		hints[p] = PrincipleHint{Principle: p, Hint: HeuristicAmbiguous,
			Reason: "sem sinal heurístico forte — LLM decide applicability"}
	}
	return hints
}

// --- detecção de trivialidade (rename/comment/whitespace) ---

// isTrivialFile true quando status/path não indicam mudança de
// conteúdo real (ex: puro rename sem hunks).
func isTrivialFile(f gitx.DiffFile) bool {
	if f.Status == gitx.ChangeRenamed && len(f.Hunks) == 0 {
		return true
	}
	if isDocPath(f.Path) {
		for _, h := range f.Hunks {
			if !isTrivialHunk(h) {
				return false
			}
		}
		return true
	}
	return false
}

// isTrivialHunk true quando TODA linha alterada (+/-) do hunk é
// comentário/whitespace, OU quando +/- casam por "skeleton" (mesmo
// esqueleto de tokens — rename de identificador sem mudar
// estrutura/operadores). Ver skeletonEqual: rejeita falso-negativo
// de mudança de operador/condição de contorno.
func isTrivialHunk(h gitx.Hunk) bool {
	added, removed := changedLines(h.Content)
	if len(added) == 0 && len(removed) == 0 {
		return true // hunk só de contexto (raro, mas seguro tratar como trivial)
	}
	if allCommentOrBlank(added) && allCommentOrBlank(removed) {
		return true
	}
	if len(added) != len(removed) {
		return false
	}
	for i := range added {
		if !skeletonEqual(removed[i], added[i]) {
			return false
		}
	}
	return true
}

// changedLines separa linhas +/- do conteúdo bruto do hunk (prefixo
// preservado por gitx.Hunk.Content).
func changedLines(content string) (added, removed []string) {
	for _, line := range strings.Split(content, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case '+':
			if !strings.HasPrefix(line, "+++") {
				added = append(added, line[1:])
			}
		case '-':
			if !strings.HasPrefix(line, "---") {
				removed = append(removed, line[1:])
			}
		}
	}
	return added, removed
}

func allCommentOrBlank(lines []string) bool {
	for _, l := range lines {
		if !isCommentLine(l) {
			return false
		}
	}
	return true
}

func isCommentLine(l string) bool {
	t := strings.TrimSpace(l)
	if t == "" {
		return true
	}
	for _, prefix := range []string{"//", "#", "*", "/*", "\"\"\"", "'''"} {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

func isDocPath(path string) bool {
	lower := strings.ToLower(path)
	for _, suffix := range []string{".md", ".txt", ".rst", ".adoc"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return strings.Contains(lower, "/docs/") || strings.HasPrefix(lower, "docs/")
}

func isTestFilePath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, "_test.go") ||
		strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") ||
		strings.HasSuffix(lower, ".test.ts") || strings.HasSuffix(lower, ".test.js") ||
		strings.HasSuffix(lower, ".spec.ts") || strings.HasSuffix(lower, ".spec.js") ||
		strings.HasPrefix(lower, "test_") || strings.Contains(lower, "/test_")
}

// skeletonEqual compara duas linhas ignorando identificadores (runs
// de letra/dígito/_) — só identificadores podem diferir; qualquer
// outro byte (operadores, pontuação, dígitos fora de identificador)
// precisa casar exatamente. Isso é o que rejeita o falso-negativo:
// "if x > 0" vira esqueleto diferente de "if x >= 0" (o ">" vs ">="
// é token não-identificador, não pode variar), mas "if oldName > 0"
// casa com "if newName > 0" (só o identificador variou).
func skeletonEqual(a, b string) bool {
	ta, tb := tokenizeSkeleton(a), tokenizeSkeleton(b)
	if len(ta) != len(tb) {
		return false
	}
	for i := range ta {
		// Ambos tokens de identificador: sempre "casam" (é o que pode
		// variar). Ambos não-identificador: precisam ser idênticos.
		// Um de cada tipo: não casa (mudança estrutural real).
		if ta[i].ident != tb[i].ident {
			return false
		}
		if !ta[i].ident && ta[i].text != tb[i].text {
			return false
		}
	}
	return true
}

type skeletonToken struct {
	text  string
	ident bool
}

func tokenizeSkeleton(s string) []skeletonToken {
	var out []skeletonToken
	var cur strings.Builder
	flushIdent := func() {
		if cur.Len() > 0 {
			out = append(out, skeletonToken{text: cur.String(), ident: true})
			cur.Reset()
		}
	}
	for _, r := range s {
		if isIdentByte(r) {
			cur.WriteRune(r)
			continue
		}
		flushIdent()
		if r == ' ' || r == '\t' {
			continue // whitespace nunca é diferenciador
		}
		out = append(out, skeletonToken{text: string(r), ident: false})
	}
	flushIdent()
	return out
}

func isIdentByte(r rune) bool {
	return r == '_' ||
		(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// --- detecção de sinal estrutural (applicable) ---

type hunkSignals struct {
	hasImport bool
	// importDomains: um hunk pode conter várias linhas de import (bloco
	// multi-linha) — cada uma pode citar um domínio distinto (o próprio
	// caso do teste multi-domínio de S). Guardar só a 1ª descartava as
	// demais e nunca acionava len(domains)>1.
	importDomains   map[string]bool
	hasPublicSymbol bool
	hasConditional  bool
}

// fileSignals varre linhas adicionadas do hunk por sinais estruturais.
// Conservador de propósito: falso-positivo aqui só manda o princípio
// pro LLM com um sinal (barato); falso-negativo silencioso é que
// seria grave — por isso NUNCA usado para decidir NOT_APPLICABLE.
func fileSignals(path string, h gitx.Hunk) hunkSignals {
	var sig hunkSignals
	added, _ := changedLines(h.Content)
	for _, line := range added {
		t := strings.TrimSpace(line)
		if t == "" || isCommentLine(t) {
			continue
		}
		if isImportLine(t) {
			sig.hasImport = true
			if d := extractImportDomain(t); d != "" {
				if sig.importDomains == nil {
					sig.importDomains = map[string]bool{}
				}
				sig.importDomains[d] = true
			}
		}
		if isPublicSymbolLine(t) {
			sig.hasPublicSymbol = true
		}
		if isConditionalLine(t) {
			sig.hasConditional = true
		}
	}
	_ = path
	return sig
}

func isImportLine(t string) bool {
	switch {
	case strings.HasPrefix(t, "import "):
		return true
	case strings.HasPrefix(t, "from ") && strings.Contains(t, " import "):
		return true
	case strings.HasPrefix(t, "require(") || strings.Contains(t, "= require("):
		return true
	case strings.HasPrefix(t, "\"") && strings.HasSuffix(t, "\"") && !strings.Contains(t, " "):
		// linha solta entre parênteses de import Go multi-linha: `"pkg/path"`
		return true
	}
	return false
}

// extractImportDomain extrai um domínio grosseiro (1º segmento não
// trivial do path importado) — só pra detectar "múltiplos domínios
// tocados" (sinal S), não pra resolver dependências de verdade.
func extractImportDomain(t string) string {
	q := quotedDomain(t)
	if q == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(q, "/"), "/")
	// Pega o segmento mais específico disponível (evita "internal"/"github.com"
	// genéricos demais pra servir de domínio).
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && parts[i] != "internal" && parts[i] != "pkg" {
			return parts[i]
		}
	}
	return q
}

func quotedDomain(t string) string {
	start := strings.Index(t, "\"")
	if start < 0 {
		return ""
	}
	end := strings.Index(t[start+1:], "\"")
	if end < 0 {
		return ""
	}
	return t[start+1 : start+1+end]
}

func isPublicSymbolLine(t string) bool {
	for _, prefix := range []string{"type ", "func ", "export ", "public ", "class ", "interface "} {
		if strings.HasPrefix(t, prefix) {
			name := extractSymbolName(t, prefix)
			if name == "" {
				continue
			}
			if isExportedIdent(name) {
				return true
			}
		}
	}
	return false
}

func extractSymbolName(t, prefix string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(t, prefix))
	rest = strings.TrimPrefix(rest, "(") // func (recv Type) Name(...) — ignora receiver, aproximação
	fields := strings.FieldsFunc(rest, func(r rune) bool { return !isIdentByte(r) })
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}
	r := name[0]
	return r >= 'A' && r <= 'Z'
}

// isConditionalLine detecta introdução de branch condicional sobre
// tipo/comportamento. Deliberadamente permissivo (qualquer `if`/
// `switch`/`case` novo conta) — nunca deve suprimir avaliação de O.
func isConditionalLine(t string) bool {
	for _, kw := range []string{"if ", "if(", "switch ", "switch(", "case ", "else if"} {
		if strings.HasPrefix(t, kw) || strings.Contains(t, " "+kw) {
			return true
		}
	}
	return false
}

// FormatHeuristicHints renderiza os hints como bloco auditável,
// explicitamente não-vinculante, pra injeção no prompt via
// {{heuristic_hints}}. O LLM só pode divergir de um hint
// CLEARLY_NOT_APPLICABLE se tiver evidence_refs concretos de
// violação real — texto deixa isso explícito.
func FormatHeuristicHints(hints map[string]PrincipleHint) string {
	if len(hints) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Pre-analysis heurístico (não vinculante)\n\n")
	b.WriteString("Sinais determinísticos calculados sobre o diff, ANTES de qualquer leitura por você. ")
	b.WriteString("Você pode divergir de um hint, mas só com evidence_refs concretos ")
	b.WriteString("(symbol/hunk/dependency/interface específicos) — nunca com justificativa genérica.\n\n")
	keys := make([]string, 0, len(hints))
	for k := range hints {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h := hints[k]
		b.WriteString("- ")
		b.WriteString(h.Principle)
		b.WriteString(": ")
		b.WriteString(h.Hint)
		b.WriteString(" — ")
		b.WriteString(h.Reason)
		b.WriteString("\n")
	}
	return b.String()
}

// SynthesizeHeuristicResult monta um ExecutorResult sem chamar LLM,
// pro caso AllNotApplicable (golden case do rename). ScoreStatus usa
// SolidScoreNotApplicable ("NOT_APPLICABLE") — não um dos 3 valores
// de ScoreStatus (available/unavailable/error), que descrevem
// validade de schema/chamada, não suficiência de evidência. Isso é
// deliberado: gate.Evaluate() precisa reconhecer esse valor (ver
// SynthesizeGateScoreStatus / internal/gate) pra não FAILar um score
// zero que na verdade significa "nada a avaliar".
func SynthesizeHeuristicResult(actor, provider, model string, hints map[string]PrincipleHint) *ExecutorResult {
	solid := map[string]any{}
	for _, p := range principleOrder {
		h := hints[p]
		solid[p] = map[string]any{
			"applicability": ApplicabilityNotApplicable,
			"score":         nil,
			"reason":        h.Reason,
		}
	}
	parsed := map[string]any{
		"solid":         solid,
		"quality_score": nil,
		"summary":       "heurística determinística: todos os 5 princípios SOLID CLEARLY_NOT_APPLICABLE — LLM não foi chamado",
	}
	return &ExecutorResult{
		Provider:      provider,
		Model:         model,
		RequestedModel: model,
		ParsedContent: parsed,
		ScoreStatus:   SolidScoreNotApplicable,
		QualityScore:  0,
	}
}
