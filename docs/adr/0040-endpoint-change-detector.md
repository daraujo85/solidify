# ADR 0040 — Endpoint change detector

Status: Aceito. 2026-08-20.

## Contexto

SAI-046: load test (k6) precisa saber quais endpoints priorizar.
Mudou contrato OpenAPI? Adicionou rota nova? Removido endpoint?
Detector agrega fontes: OpenAPI/Swagger spec + scan de
controllers (gin/echo/express) + lista manual em config.

## Decisão

`Endpoint{Method, Path, Source}` com `Fingerprint()` =
`normalizeMethod + normalizePath`. normalize = uppercase +
trim + leading "/" + collapse "//".

`DiffSet{Added, Removed, Changed, Unchanged}` + `HasChanges()` +
`IsEmpty()` + `Summary()` ("+N -N ~N =N" ou "no changes").

`Diff(old, new)` — set diff via fingerprint map. Stable sort
em Added/Removed.

`Detector{manualEndpoints}` + `AddManual(eps)` + `AllEndpoints()`
+ `SnapshotFingerprint()` (sha256 ordenado por fingerprint;
ordem não muda o hash).

`ParseOpenAPIEndpoints(data)` subset — reusa padrão do
internal/zap. Detecta 3.x vs 2.0, filtra non-http methods.

`DetectControllerEndpoints(content)` string scan — patterns
`.GET("/...")`, `.POST("/...")`, etc. Extrai path até próxima
aspas. Heurística barata pra monorepos.

`Fingerprint(method, path)` / `HashToFingerprint(method, path)`
helpers.

## Consequências

- 19 testes (fingerprint, normalize, diff empty/added/removed/
  has-changes/mixed, detector manual, snapshot stable/changes,
  parse 3.x/2.0/invalid/nil/unknown/filters, controller gin/
  none/source, helpers, summary, toMap, empty detector).
- SAI-047 (k6) usa `Detector.AllEndpoints()` ou `DiffSet.Added`
  pra definir `scenarios`.
- Detector barato: controller scan sem AST. Falsos positivos
  possíveis (string em comentário). Aceitável pra heurística.

## Trade-offs

- Controller scan é string-based, não regex completa nem AST.
  Casos como `.Get(` (lowercase, .NET) ou `route.get(` (Python
  FastAPI) não capturados.
- DiffSet.Changed não implementado nesta v1 (path mudou mas
  method+base equivalente) — placeholder no struct.
- SnapshotFingerprint não inclui Source — endpoints manuais e
  OpenAPI com mesmo fingerprint colidem. Aceitável: o objetivo
  é detectar mudança de contrato, não fonte.