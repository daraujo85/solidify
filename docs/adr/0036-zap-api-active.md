# ADR 0036 — ZAP API/active scan

Status: Aceito. 2026-08-20.

## Contexto

SAI-042: scan ativo contra OpenAPI/Swagger spec. Complementa
baseline com findings estruturados: cada endpoint da spec vira
alvo, sem spider ad-hoc.

## Decisão

`OpenAPISpec` parsed a partir de OpenAPI 3.x ou Swagger 2.0 JSON.
Suporta 3.0.0, 3.0.1, 3.0.x, 3.1.x. Detecção por prefix do campo
`openapi` ou `swagger`. Não-in-HTTP-method chaves em `paths`
(como `summary`) ignorados.

`APIScanConfig` com Timeout (10min default), MaxEndpoints (100),
Concurrent (5), UserAgent identificável como release-quality-gate.

`RunAPIScan(ctx, target, spec, cfg, guard)`:
1. TargetGuard.Validate (PROD bloqueado, allowlist match)
2. Parse spec
3. Loop pelos endpoints com ctx.Done() check
4. Aplica heurísticas por endpoint:
   - POST/PUT/PATCH sem `{id}` → medium (mass assignment risk)
   - DELETE sem `{id}` → high (provavelmente errado)
   - GET com path `/admin` → high (auth required)

`APIScanResult` carrega Spec + Report + Duration + Scanned/Skipped
+ Err se contexto expirou mid-scan.

`AuthHeaderRedact` substitui valor do header por `***REDACTED***`
preservando o nome (formato `Name: Value`). Headers malformados
(sem `:`) caem pra `***` fallback.

`FilterFindingsBySeverity` + `TotalSeverity` helpers pra UI.

## Consequências

- 25 testes (parse OpenAPI 3.0/3.0.1/3.1, Swagger 2.0, fallback
  sem basePath, endpoints count/by-method/has-path, heurísticas
  delete-no-id/post-no-id/post-with-id/admin/get-normal, run
  blocks-prod/allows-local/no-guard/max-cap/timeout/bad-spec,
  auth redact, filter, severity rank).
- Reusa `Report` + `Severity` do SAI-041 — schema único.
- Heurísticas são conservadoras: podem gerar falso-positivo (medium),
  mas alto risco (high) só em padrões claros (DELETE sem id, GET admin).

## Trade-offs

- Heurísticas simples — não detecta auth ausente em endpoint
  (precisaria ler securitySchemes da spec, não implementado).
- Sem execução real do ZAP daemon — findings são derivados da spec.
  Integração real (rodar ZAP em docker) fica pra tasks futuras.
- Timeout 10min pode não cobrir APIs grandes (>1000 endpoints).
  Caller ajusta MaxEndpoints ou sobe timeout.