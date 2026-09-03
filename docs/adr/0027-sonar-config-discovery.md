# ADR 0027 — Sonar config discovery

Status: Aceito. 2026-08-20.

## Contexto

SAI-033: gate "sonar-applicable" precisa decidir se projeto tem Sonar
configurado. Sem config → skip; com config incompleta → skip; com
config completa (host + token + projectKey) → run.

Arquivo canônico Sonar: `sonar-project.properties` (formato Java
properties). Override interno: `.solidify/sonar.json` (JSON). Secrets
nunca em disco → env refs (`sonar.token=env:SONAR_TOKEN`).

## Decisão

`sonar.Discoverer` carrega config com precedência:

1. `.solidify/sonar.json` (override Solidify)
2. `sonar-project.properties` (padrão Sonar)
3. env vars (`SONAR_HOST_URL`, `SONAR_TOKEN`) como fallback

`Config.TokenRef` (string `env:NAME`) é separado de `Token` (valor).
ResolveEnv é explícito — caller decide quando expandir env (depois de
checar permissões).

`parseProperties`: scanner linha-a-linha, separador `=` ou `:`, ignora
`#`/`!`. Keys desconhecidas com prefixo `sonar.` vão pra `Extra`.

`splitComma` divide arrays (`sonar.sources`, `sonar.coverageReportPaths`).

## Consequências

- 23 testes (properties, override, env refs, fallback, edge cases).
- Gate SAI-034+ usa `HasMinimumConfig()` pra decidir skip vs run.
- Sem leak: EnvRefs devolve `{Key, Env}` (sem valor); ResolveEnv só
  roda após autorização explícita.

## Trade-offs

- Java properties aceita `\` multi-line e continuação — não
  suportamos (properties Sonar raramente usam).
- Override JSON substitui campos inteiros (não merge profundo) —
  match com properties parser que também é overwrite.
- Sem validação de URL host (deixa pra gate validar conectividade).
