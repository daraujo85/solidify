# ADR 0031 — Secret scanner adapter (Gitleaks)

Status: Aceito. 2026-08-20.

## Contexto

SAI-037: detect secrets em código/diff antes do release. Gitleaks é
a opção mais usada (open source, regex+entropy). Output JSON mudou
entre v7 (NDJSON) e v8+ (array JSON).

## Decisão

`secrets.Report` normalizado:

- `Finding.Rule/Severity/FilePath/Line/Secret/Entropy/Commit/Author`
- `Severity`: high (aws/private-key/description "high"), medium
  (default), low (description "low"), info.
- `Report.HasHighSeverity()` — gate release bloqueia se true.
- `SortFindings()` — high primeiro, insertion sort.

`ParseGitleaksJSON` autodetecta formato: prefix `[` → array,
senão → NDJSON linha-a-linha (linhas inválidas puladas).

`RedactSecret(s)`: primeiros 4 + "***" + últimos 2. Trunca a 64
chars pra log.

`IsLikelySecret(s)`: heurística fallback (length 16-256 + entropy
≥ 4.0). Shannon entropy via `math.Log2` (stdlib).

`Counts.BySeverity`: agregação por severity.

## Consequências

- 17 testes (array/NDJSON/invalid/empty, severity mapping, redact,
  entropy, sort, counts).
- Gate SAI-040+ usa `Report.HasHighSeverity()`.
- Sem dependência externa — só stdlib.

## Trade-offs

- Sem integração direta com `gitleaks` CLI (chamada fica pra
  SAI-039+ quando integrar com analyzer framework). Adapter só
  parseia output.
- Heurística entropy é fraca — falsos positivos em hashes SHA-256.
  Gitleaks próprio tem rules melhores.
