# ADR 0032 — OSV-Scanner adapter

Status: Aceito. 2026-08-20.

## Contexto

SAI-038: vulnerabilidades em dependências são blockers comuns em
release. OSV (Open Source Vulnerabilities) é DB distribuído da
Google; osv-scanner CLI analisa lockfiles e retorna JSON com vulns
mapeados (CVE/GHSA), severity CVSS, fixed version.

## Decisão

`osv.Report` normalizado:

- `Vuln.ID` (GHSA-xxx), `Aliases` (CVE-xxx), `Package/Version/Ecosystem`,
  `Severity`, `CVSS`, `Summary`, `FixedIn`, `IsFixable`.
- `Severity`: critical (≥9.0), high (≥7.0), medium (≥4.0), low (>0),
  info (=0). Função `CVSStoSeverity`.
- `Report.HasCriticalOrHigh()` — gate release bloqueia.
- `SortVulns()` — critical primeiro, depois package, id.

`DetectLockfiles(root)`: varre root + 1 nível pra monorepos. Lista
hardcoded cobre npm/yarn/pnpm/Gemfile/composer/Cargo/go.sum/Pip/
poetry/maven/gradle/nuget. Sem `filepath.Walk` recursivo (perf).

`ParseOSVScannerJSON(data)`: parser do output CLI. Schema subset:
results[].{source.path, package, vulns[]}. Cada vuln com severity[],
affected[].ranges[].events[].fixed.

`extractCVSS`: aceita "9.8" ou vector "CVSS:3.1/...:9.8". Extrai
último token numérico após último `:`.

`parseCVSSString` retorna 0,nil pra vazio (sem erro).

`DetectEcosystemByLockfile(path)`: inferência por filename.

## Consequências

- 19 testes (CVSS mapping, lockfile detection, monorepo, JSON
  parsing, fix detection, ecosystem inference, counts, sort).
- Gate SAI-040+ usa `Report.HasCriticalOrHigh()`.
- Sem dependência externa — só stdlib.

## Trade-offs

- Sem `filepath.Walk` recursivo (limite 1 nível) — projetos com
  lockfiles em subdirs profundos (ex: `frontend/node_modules`)
  não detectados; monorepo raso OK.
- ParseFloat custom (não strconv) — evita import mas só inteiros
  + decimais simples.
- Affected matching por nome+ecosystem só (sem version range) —
  osv-scanner já filtra, mas pode haver ruído em shared deps.
