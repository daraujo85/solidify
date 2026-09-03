# ADR 0034 — Security score/gates

Status: Aceito. 2026-08-20.

## Contexto

SAI-040: 4 adapters (sonar/secrets/osv/semgrep) produzem findings.
Cada um com sua escala de severity. Gate release precisa de uma
decisão consolidada (allow/block + score 0-100).

## Decisão

`security.Finding` unificado (Source + Severity + Rule + path +
line + message). Severity cobre escala interna (critical/high/
medium/low/info) + Semgrep-style (error/warning).

`CalcScore(findings)`: deduction-based, 100 - soma.
- critical = -25
- high/error = -10
- medium/warning = -3
- low = -1
- info = 0
- Floor 0.

`Threshold`: MaxCritical/High/Medium/Low + BlockOnSecrets +
BlockOnCriticalVuln. Default = conservador (0/0/5/50).

`EvaluateGate(findings, t)`:
1. Source-specific checks PRIMEIRO (BlockOnSecrets, BlockOnCriticalVuln)
2. Thresholds genéricos depois (critical → high → medium → low)

Ordem importa: source-specific pode dar override semântico
(secret é blocker independente do count). Thresholds genéricos
depois aplicam budgets agregados.

`Gate.Allow/Reason/Score/Blocked` — caller sabe o que reprovou.

`SortFindings` (severity → source → file → line) e `MergeFindings`
(concatena sem dedup) helpers.

## Consequências

- 22 testes (calc deductions, score floor, counts, gate pass/block
  por categoria, secret/vuln critical, custom threshold, sort,
  merge).
- Demais packages passam findings via `MergeFindings` antes de
  chamar `EvaluateGate`.

## Trade-offs

- Score dedutivo não pondera por tipo (vuln em dep = mesmo
  critical de bug em código). Diferentes policies poderiam usar
  pesos por Source.
- BlockOnSecrets=high/critical só — secret LOW não bloqueia (pode
  ser falso positivo).
- Sem deduplicação cross-source — mesmo secret detectado por 2
  ferramentas conta 2x. Adequado pra gates release (audit
  downstream).
