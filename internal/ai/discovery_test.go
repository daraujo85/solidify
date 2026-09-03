package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Aceitação: NormalizeID básico.
func TestNormalizeID(t *testing.T) {
	if NormalizeID("  GPT-4  ") != "gpt-4" {
		t.Errorf("trim+lower")
	}
	if NormalizeID("openai/gpt-4") != "gpt-4" {
		t.Errorf("prefix openai")
	}
	if NormalizeID("anthropic/claude-3") != "claude-3" {
		t.Errorf("prefix anthropic")
	}
	if NormalizeID("models/x") != "x" {
		t.Errorf("prefix models")
	}
}

// Aceitação: NormalizeID sem prefix.
func TestNormalizeIDPlain(t *testing.T) {
	if NormalizeID("claude-3-opus") != "claude-3-opus" {
		t.Errorf("plain")
	}
}

// Aceitação: NormalizeModels dedup.
func TestNormalizeModelsDedup(t *testing.T) {
	models := []ModelInfo{
		{ID: "GPT-4", Name: ""},
		{ID: "gpt-4", Name: "x"},
		{ID: "claude-3"},
	}
	out := NormalizeModels(models)
	if len(out) != 2 {
		t.Errorf("dedup esperado 2, teve %d", len(out))
	}
	for _, m := range out {
		if m.ID != strings.ToLower(m.ID) {
			t.Errorf("id lowercase: %s", m.ID)
		}
	}
}

// Aceitação: NormalizeModels Name default.
func TestNormalizeModelsName(t *testing.T) {
	models := []ModelInfo{{ID: "x"}}
	out := NormalizeModels(models)
	if out[0].Name != "x" {
		t.Errorf("name default = id")
	}
}

// Aceitação: NormalizeModels provider normalizado.
func TestNormalizeModelsProvider(t *testing.T) {
	models := []ModelInfo{{ID: "x", Provider: "OpenAI"}}
	out := NormalizeModels(models)
	if out[0].Provider != "openai" {
		t.Errorf("provider")
	}
}

// Aceitação: MemoryStore Put/Get.
func TestMemoryStorePutGet(t *testing.T) {
	s := NewMemoryStore()
	if err := s.Put("k", []byte("v"), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("put: %v", err)
	}
	data, err := s.Get("k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(data) != "v" {
		t.Errorf("content")
	}
}

// Aceitação: MemoryStore Get missing.
func TestMemoryStoreMissing(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.Get("nope"); err == nil {
		t.Errorf("missing")
	}
}

// Aceitação: MemoryStore expiry.
func TestMemoryStoreExpiry(t *testing.T) {
	s := NewMemoryStore()
	if err := s.Put("k", []byte("v"), time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, err := s.Get("k"); err == nil {
		t.Errorf("expirado devia falhar")
	}
}

// Aceitação: MemoryStore Delete.
func TestMemoryStoreDelete(t *testing.T) {
	s := NewMemoryStore()
	s.Put("k", []byte("v"), time.Time{})
	s.Delete("k")
	if _, err := s.Get("k"); err == nil {
		t.Errorf("deleted")
	}
}

// Aceitação: MemoryStore Get sem expiry (zero).
func TestMemoryStoreNoExpiry(t *testing.T) {
	s := NewMemoryStore()
	s.Put("k", []byte("v"), time.Time{})
	data, _ := s.Get("k")
	if string(data) != "v" {
		t.Errorf("no expiry")
	}
}

// Aceitação: Discover sem cache.
func TestDiscoverNoCache(t *testing.T) {
	p := NewMockProvider()
	r, err := Discover(context.Background(), p, nil, time.Hour)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.FromCache {
		t.Errorf("not from cache")
	}
	if len(r.Models) != 2 {
		t.Errorf("models count")
	}
}

// Aceitação: Discover com cache miss → fill.
func TestDiscoverCacheMiss(t *testing.T) {
	store := NewMemoryStore()
	p := NewMockProvider()
	r, err := Discover(context.Background(), p, store, time.Hour)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.FromCache {
		t.Errorf("miss")
	}
	// Segunda chamada vem do cache.
	r2, _ := Discover(context.Background(), p, store, time.Hour)
	if !r2.FromCache {
		t.Errorf("hit")
	}
}

// Aceitação: Discover provider nil.
func TestDiscoverNilProvider(t *testing.T) {
	if _, err := Discover(context.Background(), nil, nil, 0); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: Discover TTL default.
func TestDiscoverDefaultTTL(t *testing.T) {
	p := NewMockProvider()
	r, _ := Discover(context.Background(), p, nil, 0)
	if r.ExpiresAt.IsZero() {
		t.Errorf("expires vazio")
	}
	if r.ExpiresAt.Sub(r.FetchedAt) < time.Hour {
		t.Errorf("ttl default = 24h, teve %v", r.ExpiresAt.Sub(r.FetchedAt))
	}
}

// Aceitação: Discover TTL custom.
func TestDiscoverCustomTTL(t *testing.T) {
	p := NewMockProvider()
	r, _ := Discover(context.Background(), p, nil, 5*time.Minute)
	d := r.ExpiresAt.Sub(r.FetchedAt)
	if d != 5*time.Minute {
		t.Errorf("custom ttl: %v", d)
	}
}

// Aceitação: Invalidate limpa cache.
func TestInvalidate(t *testing.T) {
	store := NewMemoryStore()
	p := NewMockProvider()
	Discover(context.Background(), p, store, time.Hour)
	if err := Invalidate(p, store); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	r, _ := Discover(context.Background(), p, store, time.Hour)
	if r.FromCache {
		t.Errorf("invalidado")
	}
}

// Aceitação: Invalidate sem store.
func TestInvalidateNoStore(t *testing.T) {
	if err := Invalidate(NewMockProvider(), nil); err != nil {
		t.Errorf("sem store")
	}
}

// Aceitação: IsExpired.
func TestIsExpired(t *testing.T) {
	var nilD *DiscoveryResult
	if !IsExpired(nilD) {
		t.Errorf("nil = expired")
	}
	d := &DiscoveryResult{ExpiresAt: time.Now().Add(-time.Hour)}
	if !IsExpired(d) {
		t.Errorf("past = expired")
	}
	d2 := &DiscoveryResult{ExpiresAt: time.Now().Add(time.Hour)}
	if IsExpired(d2) {
		t.Errorf("future = fresh")
	}
}

// Aceitação: RemainingTTL.
func TestRemainingTTL(t *testing.T) {
	if RemainingTTL(nil) != 0 {
		t.Errorf("nil")
	}
	d := &DiscoveryResult{ExpiresAt: time.Now().Add(time.Hour)}
	if RemainingTTL(d) <= 0 {
		t.Errorf("future tem ttl")
	}
	d2 := &DiscoveryResult{ExpiresAt: time.Now().Add(-time.Hour)}
	if RemainingTTL(d2) != 0 {
		t.Errorf("past tem 0")
	}
}

// Aceitação: MergeModels.
func TestMergeModels(t *testing.T) {
	r1 := &DiscoveryResult{Models: []ModelInfo{{ID: "a"}, {ID: "b"}}}
	r2 := &DiscoveryResult{Models: []ModelInfo{{ID: "b"}, {ID: "c"}}}
	merged := MergeModels([]*DiscoveryResult{r1, r2})
	if len(merged) != 3 {
		t.Errorf("merged count: %d", len(merged))
	}
}

// Aceitação: MergeModels nil-safe.
func TestMergeModelsNilSafe(t *testing.T) {
	merged := MergeModels(nil)
	if len(merged) != 0 {
		t.Errorf("empty")
	}
	r := &DiscoveryResult{Models: nil}
	merged = MergeModels([]*DiscoveryResult{nil, r})
	if len(merged) != 0 {
		t.Errorf("nil entries")
	}
}

// Aceitação: Discover preserva FetchedAt monotônico.
func TestDiscoverFetchedAt(t *testing.T) {
	p := NewMockProvider()
	r1, _ := Discover(context.Background(), p, nil, time.Hour)
	time.Sleep(2 * time.Millisecond)
	r2, _ := Discover(context.Background(), p, nil, time.Hour)
	if !r2.FetchedAt.After(r1.FetchedAt) {
		t.Errorf("monotonic")
	}
}

// Aceitação: Discover com store mas erro decode → refetch.
func TestDiscoverCorruptedCache(t *testing.T) {
	store := NewMemoryStore()
	store.Put("models:mock", []byte("garbage"), time.Now().Add(time.Hour))
	p := NewMockProvider()
	r, err := Discover(context.Background(), p, store, time.Hour)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.FromCache {
		t.Errorf("cache corrompido devia re-fetch")
	}
	if len(r.Models) != 2 {
		t.Errorf("models")
	}
}
