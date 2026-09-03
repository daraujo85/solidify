# ADR 0055 — Model Discovery

Status: Aceito. 2026-08-20.

## Contexto

SAI-061: cache de modelos por provider com TTL + normalização
de IDs. `GET /v1/models` retorna lista crua; precisamos
normalizar (case, prefixos redundantes), deduplicar, e
armazenar com expiry para evitar hammering no provider.

## Decisão

`DiscoveryStore` interface (Get/Put/Delete + expiresAt) em
`internal/ai/discovery.go`. `MemoryStore` implementação para
testes e fallback.

`DefaultDiscoveryTTL = 24h`.

`NormalizeID(id)`:
- trim whitespace
- lowercase
- remove prefixos `openai/`, `anthropic/`, `models/`, `provider/`

`NormalizeModels(models)` dedup por ID + Name default = ID +
Provider normalizado.

`Discover(ctx, provider, store, ttl)`:
- cache key = `"models:" + provider.Name()`
- se cache válido (não-expirado) → retorna FromCache=true
- senão → `provider.ListModels(ctx)` + normaliza + cache
- TTL default 24h quando ttl≤0
- cache corrompido (decode fail) → refetch silencioso

`DiscoveryResult{Models, FromCache, FetchedAt, ExpiresAt}`.

`Invalidate(provider, store)` limpa cache.

`IsExpired`, `RemainingTTL` helpers.

`MergeModels([]*DiscoveryResult)` une + dedup.

## Consequências

- 21 testes (normalize ID variants, dedup, name default,
  provider normalizado, memory store put/get/missing/expiry/
  delete/no-expiry, discover no cache, cache miss → fill,
  nil provider, default TTL, custom TTL, invalidate, invalidate
  no store, is expired, remaining TTL, merge models, merge
  nil-safe, fetched monotonic, corrupted cache refetch).
- Cache abstrato via interface — produção pode trocar
  MemoryStore por SQLite sem mexer em callers.
- Normalização remove prefixos comuns (`openai/gpt-4` → `gpt-4`)
  que 9Router adiciona; deduplicação trata case-insensitive.
- Corrupted cache recovery automático — decode fail cai pra
  fetch fresh, sem erro propagado.

## Trade-offs

- Sem lock para invalidar+refetch simultâneo (race condition
  teórica) — aceitável: TTL 24h + GET barato.
- Sem versionamento de cache schema — campo adição futura
  quebra decoders antigos. ADR seguinte pode adicionar
  schema version.
- TTL fixo 24h — caller pode passar custom mas não suporta
  "stale-while-revalidate" (refetch em background enquanto
  serve cache). ADR seguinte pode adicionar.
- Normalização atual não cobre Unicode (fullwidth chars, etc).
  Caller pode normalizar antes se necessário.
