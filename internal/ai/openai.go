// OpenAI-compatible HTTP adapter (SAI-059).
//
// Implementa ai.Provider usando `net/http` e structs mínimas.
// Funciona com OpenAI, Together, Groq, OpenRouter, 9Router
// (todos expõem /v1/chat/completions + /v1/models).
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider adapter.
type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	// OrgID opcional (OpenAI).
	OrgID string
	// HTTPClient customizado (opcional).
	HTTPClient *http.Client
	// StaticModels quando o provider não expõe /v1/models.
	StaticModels []ModelInfo
	// ProviderName label.
	ProviderName string
}

// NewOpenAIProvider constrói adapter.
//
// Default HTTPClient.Timeout = 180s (cobre peer_b ~60s + peer_a ~33s +
// arbiter ~30s sequenciais). Caller pode sobrepor via WithHTTPClient.
func NewOpenAIProvider(baseURL, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		APIKey:       apiKey,
		HTTPClient:   &http.Client{Timeout: 180 * time.Second},
		ProviderName: "openai",
	}
}

// Name impl.
func (p *OpenAIProvider) Name() string {
	return p.ProviderName
}

// Metadata impl.
func (p *OpenAIProvider) Metadata() ProviderMetadata {
	return ProviderMetadata{
		Name:        p.ProviderName,
		Version:     "1.0.0",
		BaseURL:     p.BaseURL,
		RequiresKey: p.APIKey != "",
		Streaming:   true,
	}
}

// oaiModelsResponse resposta de /v1/models.
type oaiModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// oaiChatRequest payload.
type oaiChatRequest struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	Temperature *float64     `json:"temperature,omitempty"`
	MaxTokens   *int         `json:"max_tokens,omitempty"`
	Stop        []string     `json:"stop,omitempty"`
	// Stream=false é obrigatório: 9Router combos devolvem SSE (data: ...) por
	// default se o cliente não disser o contrário, e nosso parser não é SSE.
	Stream      bool         `json:"stream"`
	ResponseFmt *oaiRespFmt  `json:"response_format,omitempty"`
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiRespFmt struct {
	Type   string         `json:"type"`             // "json_object" ou "json_schema"
	Schema map[string]any `json:"schema,omitempty"` // SAI-116: JSON Schema quando type=json_schema
}

// oaiChatResponse resposta.
type oaiChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ListModels impl.
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if len(p.StaticModels) > 0 {
		out := make([]ModelInfo, len(p.StaticModels))
		copy(out, p.StaticModels)
		return out, nil
	}
	if p.BaseURL == "" {
		return nil, errors.New("openai: BaseURL vazio")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	p.applyAuth(req)
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: GET /v1/models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai: /v1/models status %d: %s", resp.StatusCode, string(body))
	}
	var r oaiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("openai: parse: %w", err)
	}
	out := make([]ModelInfo, 0, len(r.Data))
	for _, m := range r.Data {
		out = append(out, ModelInfo{
			ID:       m.ID,
			Name:     m.ID,
			Provider: p.ProviderName,
			// Capabilities conservadoras: desconhece até prova em contrário.
			SupportsJSON: true,
		})
	}
	return out, nil
}

// CompleteJSON impl.
func (p *OpenAIProvider) CompleteJSON(ctx context.Context, opts CompleteOptions) (*CompleteResult, error) {
	if err := ValidateOptions(opts); err != nil {
		return nil, err
	}
	if p.BaseURL == "" {
		return nil, errors.New("openai: BaseURL vazio")
	}
	msgs := make([]oaiMessage, len(opts.Messages))
	for i, m := range opts.Messages {
		msgs[i] = oaiMessage{Role: string(m.Role), Content: m.Content}
	}
	req := oaiChatRequest{
		Model:       opts.Model,
		Messages:    msgs,
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		Stop:        opts.Stop,
	}
	if opts.JSONSchema != nil {
		// SAI-116: pede json_schema nativo quando provider suporta
		// (OpenAI, OpenRouter, Gemini). Provider que ignora cai na
		// validação client-side do peer executor.
		req.ResponseFmt = &oaiRespFmt{Type: "json_schema", Schema: opts.JSONSchema}
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.applyAuth(httpReq)
	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: POST: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: status %d: %s", resp.StatusCode, string(respBody))
	}
	var r oaiChatResponse
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, fmt.Errorf("openai: parse: %w", err)
	}
	if len(r.Choices) == 0 {
		return nil, ErrResponseInvalid
	}
	c := r.Choices[0]
	return &CompleteResult{
		Content:          c.Message.Content,
		Model:            r.Model,
		FinishReason:     c.FinishReason,
		PromptTokens:     r.Usage.PromptTokens,
		CompletionTokens: r.Usage.CompletionTokens,
		TotalTokens:      r.Usage.TotalTokens,
	}, nil
}

// applyAuth injeta Authorization header.
func (p *OpenAIProvider) applyAuth(req *http.Request) {
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	if p.OrgID != "" {
		req.Header.Set("OpenAI-Organization", p.OrgID)
	}
}

// WithHTTPClient injeta client customizado.
func (p *OpenAIProvider) WithHTTPClient(c *http.Client) *OpenAIProvider {
	if c != nil {
		p.HTTPClient = c
	}
	return p
}

// WithStaticModels injeta lista estática (quando /v1/models ausente).
func (p *OpenAIProvider) WithStaticModels(models []ModelInfo) *OpenAIProvider {
	p.StaticModels = models
	return p
}

// WithProviderName seta label.
func (p *OpenAIProvider) WithProviderName(name string) *OpenAIProvider {
	if name != "" {
		p.ProviderName = name
	}
	return p
}
