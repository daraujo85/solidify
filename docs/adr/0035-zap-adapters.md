# ADR 0035 — ZAP Baseline + API/active adapters

Status: Aceito. 2026-08-20.

## Contexto

SAI-041/042: OWASP ZAP dois modos — baseline (passive scan) e
API/active (scan ativo contra OpenAPI). Permitir target errado
= scan em produção = incidente de segurança. Default tem que ser
deny.

## Decisão

`ClassifyTarget(url)` heurístico — host → ClassLocal/Test/Staging/
Prod. PRODUCTION conservative default: se nada bate, é Prod.

`Allowlist` — entries `{host, class}` com wildcard `*.test`. Match
exato OU suffix wildcard. Class upgrade permitido (local entry
aceita class Test — promotable).

PROD nunca passa pela `Allows` a menos que entry tenha class
explicitamente Prod. `TargetGuard.Validate` adiciona segunda
camada: `AllowProd` flag default=false, mesmo com entry Prod
configurada o guard bloqueia.

`Alert` ZAP format, `Finding` normalizado com Severity (high/
medium/low/info via riskToSeverity), Source="zap-baseline"|"zap-api".

`Report` agrega counts por severity. `ParseZAPReportJSON` extrai
`site[].alerts[]`.

`TargetGuard` é wrapper que aplica DefaultAllowlist + AllowProd
flag — caller usa pra validar antes de scan.

## Consequências

- 25 testes (classify URLs, allowlist match/wildcard/class-upgrade,
  risk severity, normalize, parse, guard prod block, guard prod
  override, allowlist custom).
- Demais packages usam `TargetGuard{Allowlist: DefaultAllowlist()}.Validate(target)` antes de scan.
- SAI-042 constrói o scan ativo sobre esse mesmo schema (mesma
  `Report`, diferente `Source`).

## Trade-offs

- Heurística de class é heuristic (substring `.test`, `staging`).
  `api.example.com` com TLD `.test` no nome bate — overkill pra
  URL improvável, mas entradas allowlist explícitas sobrescrevem.
- AllowProd flag é separado da entry Prod — duas chaves de bypass
  são redundantes mas defensivas (guard layer).
- ZAP report JSON schema subset só (`site[].alerts[]`). ZAP pode
  ter mais campos — ignoramos silenciosamente.