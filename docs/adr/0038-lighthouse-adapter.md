# ADR 0038 — Lighthouse adapter

Status: Aceito. 2026-08-20.

## Contexto

SAI-044/045: importar scores oficiais Lighthouse (performance/
accessibility/best-practices/SEO) e agregar pillar frontend.
Backend-only fixture NÃO tem frontend — adapter não pode ser
invocado.

## Decisão

`Score` (0-1) com `ToPercent()` e `IsZero()`. `CategoryKey`
enum (perf/a11y/bp/seo/pwa).

`Pillar` carrega os 5 scores. `DefaultPillarWeights` com pesos
0.4/0.3/0.3/0.0 — SEO=0 = informativo (default não conta).
`AggregatePillar` pondera e normaliza: soma `score*peso` /
soma pesos. Zero pesos → 0.

`ParseLighthouseJSON(data, url, mode)` subset do JSON oficial
Lighthouse v10+: `categories{}.score` direto; `audits` ignorados
nesta versão (categoria-score já resume). Mode default = container.

`Report` carrega URL/FinalURL/FetchTime/UA + Categories + Pillar +
AggregateScore + Error. `EmptyReport(url, reason)` sentinel
para skip.

`ShouldRun(mode)` — Mode=Disabled OR "" → false. Demais modes
(container, local) → true.

`CategoryScoreByKey`, `HasPerformance`, `FailedAudits(threshold)`,
`IsInformativeOnly` (só SEO), `IsPassing(threshold)` — helpers.

`SanitizeUserAgent` corta em `(` (URLs internas do Chrome ficam
dentro de parens).

## Consequências

- 21 testes (score percent/iszero, default weights sum=1, perf
  perfeito, weighted 0.4, zero weights, SEO informative, parse
  basic/invalid/no-cats/SEO-only/not-informative/mode-default/PWA,
  ShouldRun cases, CategoryScoreByKey, HasPerformance,
  FailedAudits, IsPassing, EmptyReport, SanitizeUserAgent,
  DefaultConfig, mode/category constants).
- SAI-045 (pillar score) usa `AggregatePillar` + `IsInformativeOnly`.
- Backend-only fixture: `lighthouse.ShouldRun` retorna false →
  pipeline skipa sem alocar Report.

## Trade-offs

- Audits individuais não populados no parse (escopo desta task).
  Caller pode parsear via `lhJSON.Audits` direto se precisar.
- Pillar score mistura escala (0-1) com thresholds de gate
  (0-100). Conversion via `ToPercent()` no caller.
- Sem execução real do Chrome — adapter só parseia JSON.