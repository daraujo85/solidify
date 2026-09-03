// Model discovery (SAI-061).
//
// Cache de modelos por provider + expiry + normalized IDs.
// Store interface — produção usa SQLite (SAI-022), testes usam
// in-memory.
package ai

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// DiscoveryStore persiste cache de modelos.
type DiscoveryStore interface {
	Get(key string) ([]byte, error)
	Put(key string, data []byte, expiresAt time.Time) error
	Delete(key string) error
}

// MemoryStore implementação in-memory.
type MemoryStore struct {
	mu sync.Mutex
	m  map[string]memEntry
}

type memEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewMemoryStore cria store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{m: make(map[string]memEntry)}
}

// Get retrieve.
func (s *MemoryStore) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[key]
	if !ok {
		return nil, errors.New("not found")
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		delete(s.m, key)
		return nil, errors.New("expired")
	}
	out := make([]byte, len(e.data))
	copy(out, e.data)
	return out, nil
}

// Put armazena.
func (s *MemoryStore) Put(key string, data []byte, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := make([]byte, len(data))
	copy(d, data)
	s.m[key] = memEntry{data: d, expiresAt: expiresAt}
	return nil
}

// Delete remove.
func (s *MemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

// DefaultDiscoveryTTL expiry default.
const DefaultDiscoveryTTL = 24 * time.Hour

// Discovery resultado de descoberta.
type Discovery struct {
	Provider  string      `json:"provider"`
	Models    []ModelInfo `json:"models"`
	FetchedAt time.Time   `json:"fetched_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// NormalizeID normaliza id de modelo.
func NormalizeID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ToLower(id)
	// Remove prefixos redundantes tipo "openai/" ou "models/".
	for _, prefix := range []string{"openai/", "anthropic/", "models/", "provider/"} {
		if strings.HasPrefix(id, prefix) {
			id = strings.TrimPrefix(id, prefix)
		}
	}
	return id
}

// NormalizeModels normaliza lista.
func NormalizeModels(models []ModelInfo) []ModelInfo {
	out := make([]ModelInfo, 0, len(models))
	seen := make(map[string]bool)
	for _, m := range models {
		m.ID = NormalizeID(m.ID)
		if m.Name == "" {
			m.Name = m.ID
		}
		if m.Provider != "" {
			m.Provider = NormalizeID(m.Provider)
		}
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	return out
}

// DiscoveryResult output público.
type DiscoveryResult struct {
	Models    []ModelInfo `json:"models"`
	FromCache bool        `json:"from_cache"`
	FetchedAt time.Time   `json:"fetched_at"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// Discover busca modelos. Cache primeiro; senão chama provider.
func Discover(ctx context.Context, p Provider, store DiscoveryStore, ttl time.Duration) (*DiscoveryResult, error) {
	if p == nil {
		return nil, errors.New("discover: provider nil")
	}
	if ttl <= 0 {
		ttl = DefaultDiscoveryTTL
	}
	cacheKey := "models:" + p.Name()
	if store != nil {
		if data, err := store.Get(cacheKey); err == nil {
			var d Discovery
			if err := decodeJSON(data, &d); err == nil {
				return &DiscoveryResult{
					Models:    d.Models,
					FromCache: true,
					FetchedAt: d.FetchedAt,
					ExpiresAt: d.ExpiresAt,
				}, nil
			}
		}
	}
	models, err := p.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	models = NormalizeModels(models)
	now := time.Now()
	expires := now.Add(ttl)
	d := Discovery{
		Provider:  p.Name(),
		Models:    models,
		FetchedAt: now,
		ExpiresAt: expires,
	}
	if store != nil {
		if data, err := encodeJSON(d); err == nil {
			_ = store.Put(cacheKey, data, expires)
		}
	}
	return &DiscoveryResult{
		Models:    models,
		FromCache: false,
		FetchedAt: now,
		ExpiresAt: expires,
	}, nil
}

// Invalidate limpa cache de provider.
func Invalidate(p Provider, store DiscoveryStore) error {
	if store == nil {
		return nil
	}
	return store.Delete("models:" + p.Name())
}

// IsExpired checa se discovery está expirado.
func IsExpired(d *DiscoveryResult) bool {
	if d == nil || d.ExpiresAt.IsZero() {
		return true
	}
	return time.Now().After(d.ExpiresAt)
}

// RemainingTTL devolve ttl restante.
func RemainingTTL(d *DiscoveryResult) time.Duration {
	if d == nil || d.ExpiresAt.IsZero() {
		return 0
	}
	r := time.Until(d.ExpiresAt)
	if r < 0 {
		return 0
	}
	return r
}

// MergeModels une discoveries de múltiplos providers.
func MergeModels(results []*DiscoveryResult) []ModelInfo {
	out := make([]ModelInfo, 0)
	seen := make(map[string]bool)
	for _, r := range results {
		if r == nil {
			continue
		}
		for _, m := range r.Models {
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			out = append(out, m)
		}
	}
	return out
}

// Encode/decode helpers (stdlib JSON).
func encodeJSON(v any) ([]byte, error) {
	return jsonMarshal(v)
}

func decodeJSON(data []byte, v any) error {
	return jsonUnmarshal(data, v)
}
