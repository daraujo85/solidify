# ADR 0025 — Generic test executor

Status: Aceito. 2026-08-20.

## Contexto

SAI-031: commands descobertos em SAI-030 precisam rodar, capturar
exit/duration/output, e normalizar resultados de frameworks de teste
(JUnit XML é o formato cross-stack mais comum).

## Decisão

`testexec.Executor` consome `testexec.Command` e delega execução
para `runner.Run` (SAI-028, secure). Após execução, parseia JUnit XML
se:

1. `ReportPath` explícito é passado.
2. `ReportDir` é setado — procura `junit.xml`, `report.xml`,
   `TEST-*.xml`.

Suporta ambos formatos de JUnit XML:

- `<testsuites><testsuite>...</testsuite></testsuites>` (multi-suite,
  comum em Gradle/maven)
- `<testsuite>...</testsuite>` root direto (jest, mocha)

`Outcome` normaliza counts (total, passed, failed, skipped, errored,
duration). `Passed` é true se exit=0 E failed=0 E errored=0 E não
timed-out E sem error.

`Merge([]Outcome)` soma counts — útil pra monorepos com múltiplos
components.

## Consequências

- 22 testes (execute, junit parse, merge, report discovery, pass/fail
  logic, edge cases).
- Reuso de runner.Run mantém segurança (sem shell, env allowlist,
  bounded output).
- TRX (.NET) não suportado nesta versão — pode ser adicionado como
  parser adicional em SAI-031+ se demandado.
- Stdout/stderr incluídos em Outcome pra debug; bounded em 5MB
  default (runner.MaxOutput).

## Trade-offs

- Format JUnit parsing é XML estrito (encoding/xml). Variações de
  vendor (Surefire vs Gradle vs NUnit) podem falhar parse mas pelo
  menos exit code ainda funciona como fallback.
- Merge é O(n) sobre outcomes — fine pra dezenas de components.
