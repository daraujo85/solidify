// DecisionEnricher abstrai quem pode enriquecer as decisões de
// applicability além da heurística pura: LLMDecider (generativo) ou
// JEVDecider (System One, sub-300ms). buildAnalyzers consome esta
// interface para que o run aceite qualquer um dos dois caminhos sem
// conhecer a implementação.
package applicability

import "context"

// DecisionEnricher enriquece as 8 decisions de applicability.
//
// Implementações (LLMDecider, JEVDecider) seguem o mesmo contrato:
// heurística é sempre o chão; o enricher faz overlay; em falha cai
// pra heurística pura marcada com o Source de fallback — nunca
// quebra o run.
type DecisionEnricher interface {
	// DecideWithContext devolve as decisions de applicability para o
	// profile, enriquecidas pela fonte (LLM ou JEV).
	DecideWithContext(ctx context.Context, p Profile) []Decision
}

// compile-time checks
var (
	_ DecisionEnricher = (*LLMDecider)(nil)
	_ DecisionEnricher = (*JEVDecider)(nil)
)
