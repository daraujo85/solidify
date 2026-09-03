package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Aceitação: NewOpenAIProvider defaults.
func TestNewOpenAIProvider(t *testing.T) {
	p := NewOpenAIProvider("https://api.openai.com", "key")
	if p.BaseURL != "https://api.openai.com" {
		t.Errorf("trailing slash removido: %s", p.BaseURL)
	}
	if p.ProviderName != "openai" {
		t.Errorf("name")
	}
	if !p.Metadata().RequiresKey {
		t.Errorf("requires_key")
	}
	if !p.Metadata().Streaming {
		t.Errorf("streaming")
	}
}

// Aceitação: Name / Metadata.
func TestOpenAIMetadata(t *testing.T) {
	p := NewOpenAIProvider("https://x", "")
	m := p.Metadata()
	if m.Name != "openai" {
		t.Errorf("name")
	}
	if m.RequiresKey {
		t.Errorf("sem key = not required")
	}
}

// Aceitação: WithProviderName.
func TestWithProviderName(t *testing.T) {
	p := NewOpenAIProvider("https://x", "").WithProviderName("9router")
	if p.Name() != "9router" {
		t.Errorf("name")
	}
	if p.Metadata().Name != "9router" {
		t.Errorf("meta name")
	}
}

// Aceitação: StaticModels (skip /v1/models).
func TestStaticModelsList(t *testing.T) {
	p := NewOpenAIProvider("https://x", "").WithStaticModels([]ModelInfo{
		{ID: "a", SupportsJSON: true},
		{ID: "b", SupportsJSON: false},
	})
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("count")
	}
}

// Aceitação: /v1/models via httptest.
func TestListModelsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Errorf("auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"gpt-x","owned_by":"org"}]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "testkey")
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(models) != 1 || models[0].ID != "gpt-x" {
		t.Errorf("models: %+v", models)
	}
}

// Aceitação: ListModels erro de status.
func TestListModelsStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"auth"}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Errorf("esperava erro")
	}
}

// Aceitação: ListModels parse error.
func TestListModelsParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("garbage"))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Errorf("parse")
	}
}

// Aceitação: ListModels network error.
func TestListModelsNetwork(t *testing.T) {
	p := NewOpenAIProvider("http://127.0.0.1:1", "")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Errorf("network")
	}
}

// Aceitação: ListModels baseURL vazio.
func TestListModelsNoURL(t *testing.T) {
	p := NewOpenAIProvider("", "")
	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Errorf("url vazia")
	}
}

// Aceitação: CompleteJSON httptest.
func TestCompleteJSONHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		var req oaiChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "gpt-x" {
			t.Errorf("model")
		}
		if len(req.Messages) != 1 {
			t.Errorf("messages")
		}
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Errorf("auth")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "x",
			"model": "gpt-x",
			"choices": [{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
			"usage": {"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}
		}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "testkey")
	res, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:    "gpt-x",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Content != "hello" {
		t.Errorf("content: %s", res.Content)
	}
	if res.FinishReason != "stop" {
		t.Errorf("finish")
	}
	if res.TotalTokens != 8 {
		t.Errorf("usage")
	}
}

// Aceitação: CompleteJSON com json_schema força response_format.
func TestCompleteJSONSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req oaiChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.ResponseFmt == nil || (req.ResponseFmt.Type != "json_object" && req.ResponseFmt.Type != "json_schema") {
			t.Errorf("response_format: %+v", req.ResponseFmt)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "")
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:      "x",
		Messages:   []Message{{Role: RoleUser, Content: "hi"}},
		JSONSchema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

// Aceitação: CompleteJSON status error.
func TestCompleteJSONStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("oops"))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "")
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:    "x",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Errorf("status")
	}
}

// Aceitação: CompleteJSON empty choices.
func TestCompleteJSONEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "")
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:    "x",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Errorf("empty choices")
	}
}

// Aceitação: CompleteJSON baseURL vazio.
func TestCompleteJSONNoURL(t *testing.T) {
	p := NewOpenAIProvider("", "")
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:    "x",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Errorf("url vazia")
	}
}

// Aceitação: CompleteJSON validate fail.
func TestCompleteJSONValidate(t *testing.T) {
	p := NewOpenAIProvider("https://x", "")
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model: "x",
	})
	if err == nil {
		t.Errorf("validate")
	}
}

// Aceitação: OrgID header.
func TestOrgIDHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("OpenAI-Organization") != "myorg" {
			t.Errorf("org header")
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "k")
	p.OrgID = "myorg"
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:    "x",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

// Aceitação: WithHTTPClient nil-safe.
func TestWithHTTPClient(t *testing.T) {
	p := NewOpenAIProvider("https://x", "").WithHTTPClient(nil)
	if p.HTTPClient == nil {
		t.Errorf("client nil")
	}
}

// Aceitação: sem auth header quando key vazia.
func TestNoAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("auth esperado vazio")
		}
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "")
	if _, err := p.ListModels(context.Background()); err != nil {
		t.Errorf("err: %v", err)
	}
}

// Aceitação: trailing slash no BaseURL é strippado.
func TestBaseURLTrim(t *testing.T) {
	p := NewOpenAIProvider("https://x/v1/", "")
	if strings.HasSuffix(p.BaseURL, "/") {
		t.Errorf("trailing slash: %s", p.BaseURL)
	}
}

// Aceitação: temperature propagado.
func TestCompleteJSONTemperature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req oaiChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Temperature == nil || *req.Temperature != 0.7 {
			t.Errorf("temperature: %v", req.Temperature)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "")
	temp := 0.7
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:       "x",
		Messages:    []Message{{Role: RoleUser, Content: "hi"}},
		Temperature: &temp,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

// Aceitação: stop sequences.
func TestCompleteJSONStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req oaiChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Stop) != 2 {
			t.Errorf("stop: %v", req.Stop)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider(srv.URL, "")
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:    "x",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Stop:     []string{"foo", "bar"},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}
