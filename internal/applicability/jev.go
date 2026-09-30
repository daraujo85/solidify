// JEV-enriched applicability (opt-in, TypeSafe System One).
//
// Decide() puro continua sendo o caminho determinístico (zero-custo).
// Este arquivo adiciona um caminho opcional JEV que decide os MESMOS
// 8 gates via primitivas schema-enforced (choice/noul) do JEV System
// One — sub-300ms, custo ~$0.04/M input, zero output.
//
// Regras de merge JEV vs heurística (mesmo contrato do LLMDecider):
//   - advisory (default): JEV só enriquece `Reason`; Verdict sempre
//     = heurística. Override deliberado desativado por segurança.
//   - enforce: JEV pode override Verdict; divergência é registrada
//     em Source = "jev" no resultado.
//
// Em qualquer erro de provider/parse/schema: fallback silencioso
// para a heurística (Source = "jev-fallback"). Decide() nunca falha —
// este caminho também.
package applicability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/diegoaraujo/solidify/internal/jev"
)

// JEVDecider aplica JEV System One sobre applicability.Profile.
//
// Seguro chamar nil: DecideWithContext devolve heurística pura sem
// panic. Sem cache próprio: JEV é determinístico e sub-300ms, cache
// do LLMDecider não se aplica (decisions não vêm de texto generativo).
type JEVDecider struct {
	Client  *jev.Client
	Timeout time.Duration
	Mode    Mode
}

// NewJEVDecider constrói com defaults seguros. Client pode ser nil
// (→ DecideWithContext cai pra heurística pura).
func NewJEVDecider(client *jev.Client) *JEVDecider {
	return &JEVDecider{
		Client:  client,
		Timeout: 2 * time.Second,
		Mode:    ModeAdvisory,
	}
}

// DecideWithContext devolve as 8 decisions para o profile. Se o JEV
// não está configurado (client nil / sem chave) ou falha, devolve
// heurística pura marcada com Source = SourceJEVFallback. Caller não
// precisa tratar erro — DecideWithContext nunca quebra o run.
func (j *JEVDecider) DecideWithContext(ctx context.Context, p Profile) []Decision {
	heuristic := Decide(p)
	if j == nil || j.Client == nil || !j.Client.Enabled() {
		return markSource(heuristic, SourceHeuristic)
	}

	jevDecisions, err := j.callJEV(ctx, p)
	if err != nil {
		return markSource(heuristic, SourceJEVFallback)
	}

	return mergeJEV(heuristic, jevDecisions, j.Mode)
}

// jevVerdict é o verdict por gate esperado do JEV (mesma taxonomia
// da heurística, uppercase consistente).
const (
	jevVerdictApplicable    = "APPLICABLE"
	jevVerdictNotApplicable = "NOT_APPLICABLE"
	jevVerdictConditional   = "CONDITIONAL"
)

// callJEV envia as 8 perguntas de applicability como primitivas
// choice (uma por gate) e devolve o verdict escolhido.
func (j *JEVDecider) callJEV(ctx context.Context, p Profile) (map[Gate]string, error) {
	pctx, cancel := context.WithTimeout(ctx, j.Timeout)
	defer cancel()

	criteria := map[string]string{
		jevVerdictApplicable:    "O gate roda neste release",
		jevVerdictNotApplicable: "O gate não se aplica a este release",
		jevVerdictConditional:   "O gate se aplica apenas se configurado",
	}

	questions := map[string]jev.Question{}
	for _, g := range allGates() {
		questions[string(g)] = jev.Question{
			Type:         jev.TypeChoice,
			Instructions: gateInstruction(Gate(g)),
			Criteria:     criteria,
		}
	}

	state := fmt.Sprintf("Perfil do release:\n%s", formatProfileForJEV(p))
	res, err := j.Client.Evaluate(pctx, state, questions)
	if err != nil {
		return nil, err
	}

	out := make(map[Gate]string, len(questions))
	for _, g := range allGates() {
		ans, ok := res.Answers[string(g)]
		if !ok || ans.Choice == nil {
			return nil, fmt.Errorf("jev: gate %q sem resposta", g)
		}
		choice := ans.Choice.Choice
		if choice == "" {
			return nil, fmt.Errorf("jev: gate %q com choice vazio", g)
		}
		out[Gate(g)] = choice
	}
	return out, nil
}

// gateInstruction descreve o gate pro JEV em linguagem curta.
func gateInstruction(g Gate) string {
	switch g {
	case GateSonar:
		return "Qual a applicability do Sonar (qualidade estática) para este release?"
	case GateTests:
		return "Qual a applicability dos testes automatizados para este release?"
	case GateSecurity:
		return "Qual a applicability da análise de segurança para este release?"
	case GateLighthouse:
		return "Qual a applicability do Lighthouse (performance frontend) para este release?"
	case GateZAP:
		return "Qual a applicability do ZAP (DAST) para este release?"
	case GateK6:
		return "Qual a applicability do k6 (load test) para este release?"
	case GateMigration:
		return "Qual a applicability da verificação de migrações para este release?"
	case GateEnv:
		return "Qual a applicability da verificação de variáveis de ambiente para este release?"
	}
	return "Qual a applicability deste gate para este release?"
}

// formatProfileForJEV serializa o Profile de forma compacta e legível.
func formatProfileForJEV(p Profile) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("componentes: %v\n", p.Components))
	sb.WriteString(fmt.Sprintf("stacks: %v\n", p.Stacks))
	sb.WriteString(fmt.Sprintf("paths alterados: %v\n", p.ChangedPaths))
	sb.WriteString(fmt.Sprintf("tem migrations: %t\n", p.HasMigrations))
	sb.WriteString(fmt.Sprintf("tem mudanças de env: %t\n", p.HasEnvChanges))
	sb.WriteString(fmt.Sprintf("lighthouse target: %q\n", p.LighthouseTarget))
	sb.WriteString(fmt.Sprintf("sonar configurado: %t\n", p.SonarConfigured))
	sb.WriteString(fmt.Sprintf("zap configurado: %t\n", p.ZAPConfigured))
	sb.WriteString(fmt.Sprintf("k6 configurado: %t", p.K6Configured))
	return sb.String()
}

// mergeJEV funde heurística + JEV conforme Mode (mesmo contrato do
// merge do LLMDecider).
func mergeJEV(heuristic []Decision, jevVerdicts map[Gate]string, mode Mode) []Decision {
	out := make([]Decision, 0, len(heuristic))
	for _, h := range heuristic {
		jv, hasJEV := jevVerdicts[h.Gate]
		jevOpinionated := hasJEV && validJEVVerdict(jv)

		verdict := h.Verdict
		reason := h.Reason
		jevAssessed := false

		if jevOpinionated {
			jevAssessed = true
			if mode == ModeEnforce {
				if v := normalizeJEVVerdict(jv); v != "" {
					verdict = v
				}
			}
			reason = mergeJEVReason(h.Reason, jv, mode)
		}

		out = append(out, Decision{
			Gate:        h.Gate,
			Verdict:     verdict,
			Reason:      reason,
			Source:      sourceFor(jevAssessed),
			JEVAssessed: jevAssessed,
		})
	}
	return out
}

// sourceFor devolve o Source da decision conforme participação do JEV.
func sourceFor(jevAssessed bool) Source {
	if jevAssessed {
		return SourceJEV
	}
	return SourceHeuristic
}

// mergeJEVReason anexa a opinião JEV ao Reason (advisory) ou a usa
// como base (enforce), sem duplicar o texto da heurística.
func mergeJEVReason(hReason, jVerdict string, mode Mode) string {
	if jVerdict == "" {
		return hReason
	}
	jevNote := "JEV: " + jVerdict
	if mode == ModeEnforce {
		if hReason == "" {
			return jevNote
		}
		return jevNote + " | " + hReason
	}
	if hReason == "" {
		return jevNote
	}
	if strings.Contains(hReason, jevNote) {
		return hReason
	}
	return hReason + " | " + jevNote
}

func validJEVVerdict(v string) bool {
	switch v {
	case jevVerdictApplicable, jevVerdictNotApplicable, jevVerdictConditional:
		return true
	}
	return false
}

func normalizeJEVVerdict(v string) Verdict {
	switch v {
	case jevVerdictApplicable:
		return Applicable
	case jevVerdictNotApplicable:
		return NotApplicable
	case jevVerdictConditional:
		return Conditional
	}
	return ""
}
