// Agregação determinística de score SOLID (SAI-129A).
//
// AggregateSolidScore roda FORA do LLM — nunca lê `quality_score`
// (autorreportado, vira só telemetria). Substitui a leitura do campo
// top-level como fonte de verdade do score oficial.
package peer

import "strings"

// SolidScoreStatus enum: status da agregação (AggregateSolidScore).
// Distinto de ScoreStatus — este descreve suficiência de evidência
// pra formar um score, não validade de schema/chamada.
const (
	SolidScoreAvailable            = "AVAILABLE"             // ≥1 princípio APPLICABLE com score, nenhum INSUFFICIENT_EVIDENCE
	SolidScorePartial              = "PARTIAL"                // ≥1 princípio APPLICABLE com score, mas há INSUFFICIENT_EVIDENCE
	SolidScoreNotApplicable        = "NOT_APPLICABLE"          // 0 APPLICABLE, sem INSUFFICIENT_EVIDENCE (5/5 N/A)
	SolidScoreInsufficientEvidence = "INSUFFICIENT_EVIDENCE"   // 0 APPLICABLE, mas há INSUFFICIENT_EVIDENCE
)

// AggregateSolidScore calcula o score global (média simples dos
// princípios APPLICABLE com score não-nulo) e o status da agregação.
// score=nil quando nenhum princípio contribui.
func AggregateSolidScore(parsed map[string]any) (*float64, string) {
	solid, _ := parsed["solid"].(map[string]any)

	var sum float64
	var n int
	var hasInsufficient bool

	for _, k := range []string{"S", "O", "L", "I", "D"} {
		pm, _ := solid[k].(map[string]any)
		applicability, score, ok := PrincipleApplicabilityAndScore(pm)
		switch applicability {
		case ApplicabilityInsufficientEvidence:
			hasInsufficient = true
		case ApplicabilityApplicable:
			if ok {
				sum += score
				n++
			}
		}
	}

	if n == 0 {
		if hasInsufficient {
			return nil, SolidScoreInsufficientEvidence
		}
		return nil, SolidScoreNotApplicable
	}
	avg := sum / float64(n)
	if hasInsufficient {
		return &avg, SolidScorePartial
	}
	return &avg, SolidScoreAvailable
}

// PrincipleApplicabilityAndScore lê applicability+score de um nó
// solid.<P>. Fallback de compat: nó sem `applicability` (registro
// pré-SAI-129) usa o formato antigo (after_score.value/number) e é
// tratado como APPLICABLE implícito quando há score numérico —
// preserva leitura de registros já armazenados sem exigir migração.
//
// Exportado (SAI-129C) pra ser a ÚNICA fonte de verdade de leitura de
// nó solid.<P> — internal/app.parsedToPeerScores reusa esta função em
// vez de duplicar a lógica de leitura+fallback (evita as duas cópias
// divergirem, que era exatamente o bug do shape legado).
func PrincipleApplicabilityAndScore(pm map[string]any) (applicability string, score float64, ok bool) {
	if pm == nil {
		return ApplicabilityInsufficientEvidence, 0, false
	}

	if rawApp, present := pm["applicability"]; present {
		applicability = normalizeApplicability(rawApp)
		if applicability != ApplicabilityApplicable {
			return applicability, 0, false
		}
		if v, sok := pm["score"].(float64); sok {
			return applicability, v, true
		}
		// APPLICABLE sem score numérico válido: validateCanonical já
		// deveria ter rejeitado antes; aqui trata como sem evidência
		// suficiente pra não inflar/contaminar a agregação.
		return ApplicabilityInsufficientEvidence, 0, false
	}

	// Legado: sem `applicability`, cai no formato antigo.
	if v, lok := legacyScore(pm); lok {
		return ApplicabilityApplicable, v, true
	}
	return ApplicabilityInsufficientEvidence, 0, false
}

// legacyScore extrai o score do formato pré-SAI-129 (after_score.value
// objeto ou number direto — formato compacto).
func legacyScore(pm map[string]any) (float64, bool) {
	after, ok := pm["after_score"]
	if !ok {
		return 0, false
	}
	switch a := after.(type) {
	case map[string]any:
		if v, ok := a["value"].(float64); ok {
			return v, true
		}
	case float64:
		return a, true
	}
	return 0, false
}

// normalizeApplicability tolera variação de caixa/espaço vinda do
// provider; qualquer valor fora do enum vira INSUFFICIENT_EVIDENCE
// (nunca inventa nota pra estado desconhecido).
func normalizeApplicability(raw any) string {
	s, _ := raw.(string)
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case ApplicabilityApplicable:
		return ApplicabilityApplicable
	case ApplicabilityNotApplicable:
		return ApplicabilityNotApplicable
	case ApplicabilityInsufficientEvidence:
		return ApplicabilityInsufficientEvidence
	default:
		return ApplicabilityInsufficientEvidence
	}
}
