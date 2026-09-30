// JEV Regression analyzer (opt-in, TypeSafe System One).
//
// Avalia o diff do release com o JEV System One (sub-300ms, zero
// token de output) para detectar riscos de regressão ANTES do gate:
// quebra de contrato público, deleção de lógica funcional,
// enfraquecimento de testes, efeito colateral em estado compartilhado.
//
// Roda só quando analyzers.jev_regression.enabled=true E a chave JEV
// está no ambiente (APIKeyEnv, default JEV_API_KEY). Em qualquer
// falha (sem chave, timeout, HTTP) o analyzer devolve status skip com
// motivo explícito — nunca fabrica findings nem quebra o run.
package app

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/jev"
	"github.com/diegoaraujo/solidify/internal/report"
)

// jevRegressionQuestions avalia 4 sinais clássicos de regressão.
var jevRegressionQuestions = map[string]jev.Question{
	"breaks_public_contract": {
		Type:         jev.TypeNoul,
		Instructions: "Esta alteração quebra, renomeia ou remove assinaturas de funções/classes públicas ou exportadas que outros módulos já utilizam?",
	},
	"weakens_existing_logic": {
		Type:         jev.TypeNoul,
		Instructions: "Esta alteração deleta, desativa, comenta ou enfraquece lógica de negócio funcional pré-existente (em vez de estendê-la com compatibilidade)?",
	},
	"weakens_tests": {
		Type:         jev.TypeNoul,
		Instructions: "Esta alteração comenta, remove ou enfraquece asserções de testes existentes para fazer a esteira passar sem garantir o comportamento?",
	},
	"shared_state_side_effect": {
		Type:         jev.TypeNoul,
		Instructions: "Esta alteração modifica estado compartilhado, schema ou configuração com risco de regressão silenciosa em outros fluxos?",
	},
}

// runJEVRegressionAnalyzer avalia o diff (bruto e paths) contra os 4
// sinais de regressão. Findings têm severidade high/critical e
// identificam o sinal + arquivos suspeitos. O diff é truncado
// (4KB) — evidência mínima para a decisão, sem vazar arquivos.
func runJEVRegressionAnalyzer(ctx context.Context, cfg config.Config, diffText string, changedPaths []string, logger *slog.Logger) report.Analyzer {
	if !cfg.Analyzers.JEVRegression.Enabled {
		return skippedJEVRegression("disabled", "analyzers.jev_regression.enabled=false")
	}
	keyEnv := cfg.Analyzers.JEVRegression.APIKeyEnv
	if keyEnv == "" {
		keyEnv = jev.EnvKeyAPIKey
	}
	client := jev.NewClient(cfg.Analyzers.JEVRegression.Endpoint, os.Getenv(keyEnv))
	if !client.Enabled() {
		return skippedJEVRegression("no_key", "chave JEV ausente (esperado env "+keyEnv+")")
	}

	if diffText == "" {
		return skippedJEVRegression("empty_diff", "sem diff no range")
	}

	state := "DIFF DO RELEASE:\n" + truncateJEV(diffText, 4000) + "\n\nARQUIVOS ALTERADOS:\n" + strings.Join(changedPaths, "\n")
	res, err := client.Evaluate(ctx, state, jevRegressionQuestions)
	if err != nil {
		logger.Debug("jev_regression: falhou", "err", err)
		return skippedJEVRegression("jev_error", "JEV indisponível: "+err.Error())
	}

	var findings []map[string]any
	labels := map[string]struct{ id, sev, msg string }{
		"breaks_public_contract":   {"breaks_public_contract", "critical", "Quebra de assinatura/contrato público"},
		"weakens_existing_logic":   {"weakens_existing_logic", "high", "Enfraquecimento/deleção de lógica funcional"},
		"weakens_tests":            {"weakens_tests", "high", "Testes enfraquecidos para fazer a esteira passar"},
		"shared_state_side_effect": {"shared_state_side_effect", "medium", "Efeito colateral em estado compartilhado/schema"},
	}
	detected := 0
	for key, l := range labels {
		ans, ok := res.Answers[key]
		if !ok || ans.Noul == nil {
			continue
		}
		if ans.Noul.Noul && ans.Noul.Probability >= 0.60 {
			detected++
			findings = append(findings, map[string]any{
				"source":     "jev-regression",
				"rule":       l.id,
				"severity":   l.sev,
				"message":    l.msg,
				"confidence": roundPct(ans.Noul.Probability),
			})
		}
	}
	if detected == 0 {
		findings = []map[string]any{}
	}

	return report.Analyzer{
		ID: "jev_regression", Applicability: "APPLICABLE",
		ExecutionStatus: statusPtr("completed"),
		Score:           scorePtr(100 - float64(detected)*15), // heurística: cada sinal derruba
		Findings:        findings,
		Metrics:         map[string]any{"signals_checked": 4, "signals_detected": detected, "source": "jev"},
	}
}

func skippedJEVRegression(code, reason string) report.Analyzer {
	return report.Analyzer{
		ID: "jev_regression", Applicability: "CONDITIONAL",
		ExecutionStatus: statusPtr("skipped"), Metrics: map[string]any{"skip_code": code},
		Limitations: []string{"jev_regression: " + reason},
	}
}

// truncateJEV limita string mantendo limite de bytes.
func truncateJEV(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncado)"
}

func roundPct(f float64) float64 {
	return float64(int(f*100)) / 100
}

func scorePtr(f float64) *float64 { return &f }
