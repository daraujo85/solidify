# ADR 0008 — Applicability engine v1

## Status

Aceito. 2026-08-19.

## Contexto

SAI-014 pede decidir applicability (APPLICABLE / NOT_APPLICABLE /
CONDITIONAL) para 8 gates:

Sonar, Tests, Security, Lighthouse, ZAP, k6, Migration, Env.

**Aceite crítico:** release backend-only ⇒ Lighthouse
`NOT_APPLICABLE`.

Baseado em ARCHITECTURE.md §8.1 e §8.4: monorepo não roda Lighthouse
no frontend que não mudou; Lighthouse exige frontend-web **e** target
executável.

## Decisão

Função pura `Decide(Profile) []Decision`. Profile carrega:

- `Components []component.Component` (pode ter mais de um — monorepo)
- `ChangedPaths []string` (paths do range)
- `HasMigrations`, `HasEnvChanges` (flags)
- `LighthouseTarget`, `SonarConfigured`, `ZAPConfigured`, `K6Configured`
  (config do projeto)

`hasCodeChanges(paths)` filtra docs/.env/migrations/test para
identificar se há código real.

**Tabela de decisões:**

| Gate        | APPLICABLE quando…                                       | NOT_APPLICABLE quando…       |
|-------------|----------------------------------------------------------|------------------------------|
| Sonar       | código + Sonar configurado                               | só docs/migrations           |
| Tests       | há código                                                | só docs/config               |
| Security    | backend-api, infra, library ou worker                    | (nunca NOT_APPLICABLE)       |
| Lighthouse  | frontend-web + target URL                                 | sem frontend-web             |
| ZAP         | web/api + ZAP configurado                                | nem web nem api              |
| k6          | backend/worker + k6 configurado                          | nem backend nem worker       |
| Migration   | HasMigrations=true                                       | flag false                   |
| Env         | HasEnvChanges=true                                       | flag false                   |

`CONDITIONAL` cobre o meio termo: gate faria sentido mas falta
config (Sonar sem quality gate, Lighthouse sem target URL, etc).

## Consequências

**Positivas:**

- Aceite crítico coberto: backend-only ⇒ Lighthouse NOT_APPLICABLE.
- 8 gates com regras explícitas e testadas (24 testes).
- Profile struct pequena, fácil de alimentar pelo orquestrador.
- Sem dependência externa (só component + stack).

**Negativas:**

- `hasCodeChanges` é heurística simples (extensão + primeiro
  segmento). Pode dar falso positivo para `Dockerfile` ou
  `.csproj` puros — mitigated pelos manifests que entram antes no
  pipeline.
- Security nunca é NOT_APPLICABLE (até docs-only vira CONDITIONAL
  "análise reduzida"). Decisão consciente: security sempre precisa
  de alguma evidência.

## Alternativas consideradas

- **Regras em YAML**: moveria decisão para config, mas o spec
  pede regras versionadas no código (auditabilidade). Rejeitado.
- **IA decide applicability**: overkill para regras determinísticas.
  Rejeitado.
- **Tudo APPLICABLE sempre**: quebraria o aceite crítico. Rejeitado.