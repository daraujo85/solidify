// Package ai define interface para provedores de IA (SAI-058).
//
// Provider interface: ListModels + CompleteJSON + metadata.
// Adapters (OpenAI-compat, 9Router, etc) implementam essa interface.
package ai

import (
	"context"
	"errors"
	"strings"
)

// Role representa o papel de uma mensagem.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message conteúdo de mensagem.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ModelInfo metadata de modelo.
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	ContextSize int    `json:"context_size,omitempty"`
	MaxOutput   int    `json:"max_output,omitempty"`
	// SupportsJSON indica se o model retorna JSON estruturado.
	SupportsJSON bool `json:"supports_json"`
	// SupportsVision indica entrada de imagem.
	SupportsVision bool `json:"supports_vision"`
	// SupportsAudio indica entrada de áudio.
	SupportsAudio bool `json:"supports_audio"`
}

// CompleteOptions opções para CompleteJSON.
type CompleteOptions struct {
	// Model id do modelo a usar.
	Model string `json:"model"`
	// Messages conversa.
	Messages []Message `json:"messages"`
	// Temperature 0.0–2.0 (opcional).
	Temperature *float64 `json:"temperature,omitempty"`
	// MaxTokens limite (opcional).
	MaxTokens *int `json:"max_tokens,omitempty"`
	// JSONSchema se setado, força output conforme schema (opcional).
	JSONSchema map[string]any `json:"json_schema,omitempty"`
	// Stop sequences (opcional).
	Stop []string `json:"stop,omitempty"`
}

// CompleteResult resposta.
type CompleteResult struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	FinishReason string `json:"finish_reason,omitempty"`
	// Usage stats (se reportadas pelo provider).
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// Provider interface implementada por adapters.
type Provider interface {
	// Name nome do provider (ex: "openai", "9router").
	Name() string
	// ListModels lista modelos disponíveis.
	ListModels(ctx context.Context) ([]ModelInfo, error)
	// CompleteJSON executa completion e devolve JSON.
	CompleteJSON(ctx context.Context, opts CompleteOptions) (*CompleteResult, error)
	// Metadata info do provider.
	Metadata() ProviderMetadata
}

// ProviderMetadata metadados.
type ProviderMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	BaseURL     string `json:"base_url,omitempty"`
	RequiresKey bool   `json:"requires_key"`
	Streaming   bool   `json:"streaming"`
}

// Errors.
var (
	ErrModelNotFound   = errors.New("ai: model não encontrado")
	ErrEmptyMessages   = errors.New("ai: messages vazio")
	ErrProviderNotImpl = errors.New("ai: provider não implementado")
	ErrResponseInvalid = errors.New("ai: resposta inválida")
)

// ValidateOptions valida opts.
func ValidateOptions(opts CompleteOptions) error {
	if opts.Model == "" {
		return errors.New("ai: model vazio")
	}
	if len(opts.Messages) == 0 {
		return ErrEmptyMessages
	}
	for i, m := range opts.Messages {
		if m.Role == "" {
			return errors.New("ai: message sem role")
		}
		if m.Content == "" {
			return errors.New("ai: message sem content")
		}
		_ = i
	}
	return nil
}

// FilterByCapability filtra models.
func FilterByCapability(models []ModelInfo, jsonOK, visionOK, audioOK bool) []ModelInfo {
	out := make([]ModelInfo, 0)
	for _, m := range models {
		if jsonOK && !m.SupportsJSON {
			continue
		}
		if visionOK && !m.SupportsVision {
			continue
		}
		if audioOK && !m.SupportsAudio {
			continue
		}
		out = append(out, m)
	}
	return out
}

// FindModel busca por ID.
func FindModel(models []ModelInfo, id string) (ModelInfo, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return ModelInfo{}, false
}

// SupportsJSON helper.
func SupportsJSON(models []ModelInfo, id string) bool {
	m, ok := FindModel(models, id)
	if !ok {
		return false
	}
	return m.SupportsJSON
}

// RoleIsValid valida role.
func RoleIsValid(r Role) bool {
	switch r {
	case RoleSystem, RoleUser, RoleAssistant:
		return true
	}
	return false
}

// BuildMessages helper.
func BuildMessages(system, user string) []Message {
	out := make([]Message, 0, 2)
	if system != "" {
		out = append(out, Message{Role: RoleSystem, Content: system})
	}
	if user != "" {
		out = append(out, Message{Role: RoleUser, Content: user})
	}
	return out
}

// TruncateContent trunca se exceder max.
func TruncateContent(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// StripCodeFences remove ```json fences se presentes.
func StripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// remove first line
		if i := strings.Index(s, "\n"); i > 0 {
			s = s[i+1:]
		}
		// remove trailing ```
		if strings.HasSuffix(s, "```") {
			s = s[:len(s)-3]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

// MockProvider provider fake para testes.
type MockProvider struct {
	models []ModelInfo
}

// NewMockProvider constrói mock.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		models: []ModelInfo{
			{ID: "mock-1", Name: "Mock 1", Provider: "mock", SupportsJSON: true, ContextSize: 8000},
			{ID: "mock-2", Name: "Mock 2", Provider: "mock", SupportsJSON: true, SupportsVision: true, ContextSize: 16000},
		},
	}
}

// Name impl.
func (m *MockProvider) Name() string { return "mock" }

// ListModels impl. Devolve cópia defensiva.
func (m *MockProvider) ListModels(_ context.Context) ([]ModelInfo, error) {
	out := make([]ModelInfo, len(m.models))
	copy(out, m.models)
	return out, nil
}

// CompleteJSON impl.
func (m *MockProvider) CompleteJSON(_ context.Context, opts CompleteOptions) (*CompleteResult, error) {
	if err := ValidateOptions(opts); err != nil {
		return nil, err
	}
	if _, ok := FindModel(m.models, opts.Model); !ok {
		return nil, ErrModelNotFound
	}
	return &CompleteResult{
		Content:      "{\"mock\":true}",
		Model:        opts.Model,
		FinishReason: "stop",
	}, nil
}

// Metadata impl.
func (m *MockProvider) Metadata() ProviderMetadata {
	return ProviderMetadata{
		Name: "mock", Version: "0.1.0",
		RequiresKey: false, Streaming: false,
	}
}
