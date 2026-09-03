# ADR 0028 — Sonar Web API client

Status: Aceito. 2026-08-20.

## Contexto

SAI-034: SonarQube/SonarCloud Web API mudou entre versões (v9 → v10 →
2025-edition). Endpoints estáveis (`/api/system/status`,
`/api/measures/component`, `/api/issues/search`,
`/api/qualitygates/project_status`) mas JSON pode ganhar campos.

Auth dual: SonarCloud moderno usa `Authorization: Bearer <token>`,
SonarQube ≤9.x usa `Authorization: Basic base64(login:token)`.

## Decisão

`sonar.Client`:

- Auth via `applyAuth`: se `Login != ""` usa Basic, senão Bearer.
- Timeout default 30s (`ClientConfig.Timeout`).
- User-Agent `solidify/0.1 (sonar)` (visibilidade no server).
- Limite de body 16MB (`io.LimitReader`).
- `get()` categoriza erros: 401/403/404/429/5xx com mensagens
  específicas (token inválido, sem permissão, rate limit).
- `GetServerVersion` popula `c.Version` (info, não bloqueia uso).

Tolerância a versão: structs Go só declaram campos conhecidos;
campos extras são ignorados pelo `encoding/json` decoder.

`APIVersionsSupported` devolve cópia defensiva.

## Consequências

- 26 testes (auth basic/bearer, ping, get project, measures,
  issues, QG, version, 401/403/404/429/500, context cancel,
  user-agent, query params, API-version tolerance, defensive copy).
- Client reusado por SAI-035 (normalizer) e SAI-036 (scanner runner).
- Sem SDK externo — `net/http` + `encoding/json` da stdlib.

## Trade-offs

- Não suporta paginação além de `ps=100` (issues grandes ficam
  incompletos). Suficiente pra gate release — issues > 100 já
  indica problema sério.
- Não usa cache de token (SonarQube tokens não expiram rápido).
- Sem retry exponencial — caller decide (SRE pode preferir falhar
  rápido em CI).
