// Canonical peer review schema (SAI-116).
//
// Schema único que o peer reviewer deve devolver. Garante
// que o orchestrator não precise adivinhar formato —
// `quality_score` direto, `solid.{S,O,L,I,D}.after_score.value`
// como redundância.
//
// Provider que honra `response_format: json_schema` recebe o
// schema nativo (OpenAI, OpenRouter, Gemini). Provider que
// ignora cai em validação client-side + repair único.
package peer

// CanonicalSchema JSON Schema Draft 7 (subset).
// Required: `solid` + `quality_score`. Optional: confidence,
// summary, issues.
var CanonicalSchema = map[string]any{
	"type":     "object",
	"required": []string{"solid", "quality_score"},
	"additionalProperties": true,
	"properties": map[string]any{
		"solid": map[string]any{
			"type":     "object",
			"required": []string{"S", "O", "L", "I", "D"},
			"properties": map[string]any{
				"S": principleSchema,
				"O": principleSchema,
				"L": principleSchema,
				"I": principleSchema,
				"D": principleSchema,
			},
		},
		"quality_score": map[string]any{
			"type":    "number",
			"minimum": 0.0,
			"maximum": 100.0,
		},
		"confidence": map[string]any{
			"type":    "number",
			"minimum": 0.0,
			"maximum": 1.0,
		},
		"summary": map[string]any{"type": "string"},
		"issues": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":     "object",
				"required": []string{"id", "severity", "title"},
				"properties": map[string]any{
					"id":       map[string]any{"type": "string"},
					"severity": map[string]any{"type": "string", "enum": []any{"low", "medium", "high", "critical"}},
					"title":    map[string]any{"type": "string"},
					"evidence_refs": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
				},
			},
		},
	},
}

// principleSchema (SAI-129A): `applicability` obrigatório, `score`
// condicional (Draft 7 if/then — hinting pro provider; enforcement real
// é client-side em validatePrincipleNode). Formato antigo
// (`applicable`/`after_score`) não é mais oferecido ao LLM daqui pra
// frente — só sobrevive como fallback de leitura em validatePrincipleNode
// para registros MCP já armazenados antes do SAI-129.
var principleSchema = map[string]any{
	"type":     "object",
	"required": []string{"applicability"},
	"properties": map[string]any{
		"applicability": map[string]any{
			"type": "string",
			"enum": []any{ApplicabilityApplicable, ApplicabilityNotApplicable, ApplicabilityInsufficientEvidence},
		},
		"score": map[string]any{
			"type":    []any{"number", "null"},
			"minimum": 0.0,
			"maximum": 100.0,
		},
		"confidence": map[string]any{
			"type":    "number",
			"minimum": 0.0,
			"maximum": 1.0,
		},
		"reason": map[string]any{"type": "string"},
		"evidence_refs": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		},
	},
	"if": map[string]any{
		"properties": map[string]any{
			"applicability": map[string]any{"const": ApplicabilityApplicable},
		},
	},
	"then": map[string]any{
		"required": []string{"score", "evidence_refs"},
	},
}

// ScoreStatus enum. Descreve validade de schema/chamada ao provider —
// NÃO suficiência de evidência (isso é SolidScoreStatus, ver aggregate.go).
const (
	ScoreStatusAvailable   = "available"   // schema bateu, score real
	ScoreStatusUnavailable = "unavailable" // schema falhou mesmo após repair
	ScoreStatusError       = "error"       // chamada ao provider falhou
)

// Applicability enum (SAI-129A): estado por princípio SOLID.
const (
	ApplicabilityApplicable           = "APPLICABLE"
	ApplicabilityNotApplicable        = "NOT_APPLICABLE"
	ApplicabilityInsufficientEvidence = "INSUFFICIENT_EVIDENCE"
)
