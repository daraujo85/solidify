# ADR 0029 — Sonar result normalizer

Status: Aceito. 2026-08-20.

## Contexto

SAI-035: Web API retorna issues com 5 severities Sonar (BLOCKER/
CRITICAL/MAJOR/MINOR/INFO), QG com conditions aninhadas, e issues
sem distinção entre "causado pelo diff" vs "histórico do projeto".

Score/gate (SAI-040+) precisa de escala uniforme e filtro por scope
pra não reprovar release por débito técnico pré-existente.

## Decisão

`Severity` interno: `critical/high/medium/low/info` (5 níveis).
Mapeamento Sonar→interno: BLOCKER→critical, CRITICAL→high,
MAJOR→medium, MINOR→low, INFO→info.

`Issue.Scope`:
- `project-wide` (sem file OU status RESOLVED/CLOSED)
- `diff` (file bate em diffPaths)
- `unknown` (file existe mas não tá no diff)

**Status RESOLVED/CLOSED tem precedência sobre diff** — issue
resolvido não é diff-relevant mesmo se file foi tocado.

`MeasureN` (normalizada) separa do `Measure` do client (string→float64).
`Value` float64 + `Raw` string original (audit).

`QGConditionN.Threshold/Actual` parseados como float64.

Sort issues por severity rank (critical primeiro), estável por key.

`Report.Counts`: agregações por severity/scope/type — gate usa pra
threshold matching.

## Consequências

- 24 testes (severity mapping, scope precedence, project-wide
  override, sort stability, blocking issues, counts, QG parsing,
  measure normalization, parseFloat edge cases).
- Gate release usa `Report.BlockingIssues()` (critical+high abertas)
  pra decidir block/allow.

## Trade-offs

- ParseFloat custom (não `strconv.ParseFloat`) — evita import mas
  não suporta notação científica. Suficiente pra values Sonar.
- Scope `unknown` não é "safe" — gate precisa contar como
  potencial diff-relevant pra ser conservador.
