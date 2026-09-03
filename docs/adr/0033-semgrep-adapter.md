# ADR 0033 — Semgrep CE adapter

Status: Aceito. 2026-08-20.

## Contexto

SAI-039: SAST (static analysis) com Semgrep CE. CLI retorna JSON
com results[] (check_id, path, start, end, extra.{message,
severity, metadata}). Rules custom em `.semgrep/` ou `.semgrep.yml`.

## Decisão

`semgrep.Report` normalizado:

- `Finding.RuleID/Severity/FilePath/Line/Column/EndLine/Message/CWE/OWASP/Category`
- `Severity`: error/warning/info (built-in) + critical/high/medium/low
  (custom rules em metadata).
- `Report.HasErrorSeverity()` — gate bloqueia se tem error/critical/high.
- `SortFindings()` — error primeiro, depois warning, depois info;
  estável por file/line.

`DetectConfig(root)`: procura `.semgrep.yml`, `.semgrep.yaml`,
`semgrep.yml`, ou arquivos `.yml`/`.yaml` em `.semgrep/`.

`ParseSemgrepJSON(data)`: extrai results[] + errors[]. Metadata
extraction: `cwe`/`owasp` como `[]string` (pode vir string único
ou array no JSON).

`extractStringList` tolera 3 tipos: `[]interface{}`, `string`,
`[]string`.

## Consequências

- 19 testes (severity mapping, config detection, JSON parsing,
  metadata extraction, sort, multi-file counting).
- Gate SAI-040+ usa `Report.HasErrorSeverity()`.
- Sem dependência externa — só stdlib.

## Trade-offs

- Sem chamada ao `semgrep` CLI (adapter só parseia output).
- Sem versionamento de rules — gate não bloqueia se rules mudaram
  entre runs (audit diff fica pra fora).
- Sem detecção de regras deprecated (registry semgrep > 1.0 mudou).
