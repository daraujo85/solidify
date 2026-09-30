// Package jev é o client do JEV System One (TypeSafe AI).
//
// JEV é um motor de decisão determinístico (System One) que responde
// perguntas schema-enforced em ~100-300ms com zero token de output.
// Primitivas suportadas: choice (escolher 1 opção), noul (sim/não),
// score (1-10).
//
// Contrato do endpoint (POST /v1/systemone):
//
//	{ "model": "jev-latest", "state": "...", "questions": {
//	    "k1": { "type": "choice", "instructions": "...", "criteria": {...} },
//	    "k2": { "type": "noul",   "instructions": "..." },
//	    "k3": { "type": "score",  "instructions": "...", "min":1, "max":10 }
//	}}
//
// Resposta:
//
//	{ "answers": { "k1": { "type":"choice", "choice":"x", "confidence":0.9, "probabilities":{...} },
//	                "k2": { "type":"noul", "noul":true, "probability":0.95 },
//	                "k3": { "type":"score", "score":8.2, "confidence":0.8 } }, ... }
//
// Em qualquer falha (timeout, HTTP, parse) o client devolve erro —
// quem chama decide o fallback (hooks do Solidify sempre caem pro
// caminho determinístico, nunca quebram o run).
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/diegoaraujo/solidify/internal/jsonx"
)

// DefaultEndpoint é o endpoint canônico do JEV System One.
const DefaultEndpoint = "https://api.typesafe.ai/v1/systemone"

// EnvKeyAPIKey é a variável de ambiente default para a chave.
const EnvKeyAPIKey = "JEV_API_KEY"

// EnvKeyGateway é a variável de ambiente para override de endpoint.
const EnvKeyGateway = "JEV_GATEWAY_URL"

// QuestionType enum das primitivas JEV.
type QuestionType string

const (
	TypeChoice QuestionType = "choice"
	TypeNoul   QuestionType = "noul"
	TypeScore  QuestionType = "score"
)

// Question é uma pergunta no protocolo System One.
type Question struct {
	Type         QuestionType      `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"` // choice: opção -> descrição
	Min          *float64          `json:"min,omitempty"`      // score
	Max          *float64          `json:"max,omitempty"`      // score
}

// ChoiceAnswer resposta da primitiva choice.
type ChoiceAnswer struct {
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

// NoulAnswer resposta da primitiva noul.
type NoulAnswer struct {
	Noul        bool    `json:"noul"`
	Probability float64 `json:"probability"`
}

// ScoreAnswer resposta da primitiva score.
type ScoreAnswer struct {
	Score      float64 `json:"score"`
	Confidence float64 `json:"confidence"`
}

// Answer é a resposta normalizada de uma pergunta.
type Answer struct {
	Type   QuestionType  `json:"type"`
	Choice *ChoiceAnswer `json:"choice,omitempty"`
	Noul   *NoulAnswer   `json:"noul,omitempty"`
	Score  *ScoreAnswer  `json:"score,omitempty"`
}

// Result é o resultado de Evaluate.
type Result struct {
	Model     string            `json:"model"`
	LatencyMS int64             `json:"latency_ms"`
	Answers   map[string]Answer `json:"answers"`
}

// Client é o client HTTP do JEV System One.
type Client struct {
	Endpoint   string
	APIKey     string
	HTTPClient *http.Client
	// Model usado no payload; vazio = "jev-latest".
	Model string
}

// NewClient constrói com defaults. Se apiKey == "", resolve de
// JEV_API_KEY ou TYPESAFE_API_KEY (nessa ordem). Endpoint vazio usa
// DefaultEndpoint; override via JEV_GATEWAY_URL.
func NewClient(endpoint, apiKey string) *Client {
	if endpoint == "" {
		endpoint = os.Getenv(EnvKeyGateway)
	}
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if apiKey == "" {
		apiKey = os.Getenv(EnvKeyAPIKey)
	}
	if apiKey == "" {
		apiKey = os.Getenv("TYPESAFE_API_KEY")
	}
	return &Client{
		Endpoint:   strings.TrimRight(endpoint, "/"),
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 1500 * time.Millisecond},
		Model:      "jev-latest",
	}
}

// KeyConfigured devolve true se há chave disponível (env ou explícita).
func (c *Client) KeyConfigured() bool {
	return c != nil && c.APIKey != ""
}

// Enabled devolve true se o client pode ser usado (chave presente).
func (c *Client) Enabled() bool { return c.KeyConfigured() }

// requestPayload payload do POST /v1/systemone.
type requestPayload struct {
	Model     string              `json:"model"`
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// rawAnswer é a resposta crua por chave (variante por primitiva).
type rawAnswer struct {
	Type          QuestionType       `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	Noul          *flexBool          `json:"noul"`
	Probability   float64            `json:"probability"`
	Score         *float64           `json:"score"`
}

// flexBool aceita `true`, `false`, `1`, `0` ou um número 0–1 no campo
// noul (a API JEV real devolve noul como probabilidade, ex.: 0.51).
// Preserva o valor numérico original em V para o caller usar como
// probabilidade quando o campo `probability` vier zerado.
type flexBool struct {
	v bool
	// V é o valor numérico original quando o campo veio como número
	// (0–1); zero quando veio como boolean explícito.
	V float64
}

func (f *flexBool) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch s {
	case "true":
		f.v = true
		f.V = 1
		return nil
	case "false":
		f.v = false
		f.V = 0
		return nil
	case "1", "1.0":
		f.v = true
		f.V = 1
		return nil
	case "0", "0.0":
		f.v = false
		f.V = 0
		return nil
	}
	if num, err := strconv.ParseFloat(s, 64); err == nil {
		f.V = num
		f.v = num >= 0.5
		return nil
	}
	return fmt.Errorf("flexBool: valor inesperado %q", s)
}

func (f *flexBool) Bool() bool {
	if f == nil {
		return false
	}
	return f.v
}

// rawResponse envelope da resposta.
type rawResponse struct {
	Model     string               `json:"model"`
	LatencyMS int64                `json:"latency_ms"`
	Answers   map[string]rawAnswer `json:"answers"`
}

// Evaluate envia state + questions ao JEV e devolve respostas tipadas.
// Falha (timeout/HTTP/parse) => erro; caller decide fallback.
func (c *Client) Evaluate(ctx context.Context, state string, questions map[string]Question) (*Result, error) {
	if c == nil {
		return nil, errors.New("jev: client nil")
	}
	if c.APIKey == "" {
		return nil, errors.New("jev: APIKey ausente")
	}
	if len(questions) == 0 {
		return nil, errors.New("jev: questions vazio")
	}

	payload := requestPayload{Model: c.Model, State: state, Questions: questions}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("jev: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jev: criar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jev: request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("jev: ler resposta: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jev: status %d: %s", resp.StatusCode, truncate(string(data), 300))
	}

	var raw rawResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("jev: parse resposta: %w", err)
	}

	return normalize(raw), nil
}

// normalize converte rawAnswer (variante por primitiva) em Answer tipado.
func normalize(raw rawResponse) *Result {
	out := &Result{Model: raw.Model, LatencyMS: raw.LatencyMS, Answers: map[string]Answer{}}
	for k, ra := range raw.Answers {
		switch ra.Type {
		case TypeChoice:
			out.Answers[k] = Answer{
				Type: TypeChoice,
				Choice: &ChoiceAnswer{
					Choice:        ra.Choice,
					Confidence:    ra.Confidence,
					Probabilities: ra.Probabilities,
				},
			}
		case TypeNoul:
			n := false
			prob := ra.Probability
			if ra.Noul != nil {
				n = ra.Noul.Bool()
				if prob == 0 && ra.Noul.V != 0 {
					prob = ra.Noul.V
				}
			} else if ra.Probability >= 0.5 {
				n = true
			}
			out.Answers[k] = Answer{
				Type: TypeNoul,
				Noul: &NoulAnswer{Noul: n, Probability: prob},
			}
		case TypeScore:
			s := 0.0
			if ra.Score != nil {
				s = *ra.Score
			}
			out.Answers[k] = Answer{
				Type:  TypeScore,
				Score: &ScoreAnswer{Score: s, Confidence: ra.Confidence},
			}
		default:
			// Answer desconhecida — tenta inferir por campos presentes.
			out.Answers[k] = inferAnswer(ra)
		}
	}
	return out
}

// inferAnswer cobre providers que não mandam "type" explícito.
func inferAnswer(ra rawAnswer) Answer {
	switch {
	case ra.Choice != "":
		return Answer{Type: TypeChoice, Choice: &ChoiceAnswer{Choice: ra.Choice, Confidence: ra.Confidence, Probabilities: ra.Probabilities}}
	case ra.Score != nil:
		return Answer{Type: TypeScore, Score: &ScoreAnswer{Score: *ra.Score, Confidence: ra.Confidence}}
	default:
		n := false
		prob := ra.Probability
		if ra.Noul != nil {
			n = ra.Noul.Bool()
			if prob == 0 && ra.Noul.V != 0 {
				prob = ra.Noul.V
			}
		} else if ra.Probability >= 0.5 {
			n = true
		}
		return Answer{Type: TypeNoul, Noul: &NoulAnswer{Noul: n, Probability: prob}}
	}
}

// truncate limita string pra diagnóstico.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// ParseContent reexporta jsonx.ParseContent para quem precisa extrair
// JSON tolerante a ruído (ex.: mock/parser de resposta manual).
func ParseContent(s string) (map[string]any, bool) { return jsonx.ParseContent(s) }
