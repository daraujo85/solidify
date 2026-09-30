# Solidify

[![Go Report Card](https://goreportcard.com/badge/github.com/diegoaraujo/solidify)](https://goreportcard.com/report/github.com/diegoaraujo/solidify)
![Go version](https://img.shields.io/github/go-mod/go-version/daraujo85/solidify)
![Last commit](https://img.shields.io/github/last-commit/daraujo85/solidify)

Quality gate for releases. Analyzes diff, runs
multi-peer review with role-swap, computes a release
score, and emits a JSON + HTML dashboard + PDF report
with `PASS`/`WARN`/`FAIL` verdict.

## Build

No host Go toolchain needed — everything runs in container (see `docs/adr/0001`).

```bash
git clone https://github.com/daraujo85/solidify.git
cd solidify
make install-host   # builds bin/solidify-darwin-arm64, installs to ~/bin/solidify
```

Other targets: `make build` (container-only build, no install), `make verify`
(full local CI: build+test+lint). Override `GOOS`/`GOARCH` in `install-host`
for other platforms — see `Makefile`.

## Usage

```bash
solidify init                                  # scaffold solidify.json in a repo
solidify run --profile quick --dir . --base <ref> --head HEAD
solidify report --format pdf --run <run-id> --out report.pdf
solidify dashboard --port 8080                 # serve local UI over .solidify/out
solidify doctor                                # environment diagnostics
```

- `--profile`: `quick` (single peer, fast) | `release` (2 peers + arbiter) |
  `contractual` (strictest, generates PDF).
- Exit code `0` = gate PASS; non-zero = FAIL/BLOCKED/INCOMPLETE (see
  `internal/errs/errs.go` for codes).
- Result: `<repo>/.solidify/out/release-report.json`.

Run smoke tests:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.26 \
  sh -c "go test ./... -count=1"
```

## Profiles

| Profile | Threshold | Use |
|---|---|---|
| `quick` | 60 | dev/CI smoke |
| `release` | 75 | standard releases |
| `contractual` | 85 | regulated/audit |

## Commands

- `solidify run --profile release` — full pipeline.
- `solidify dashboard --port 8080` — serve local UI.
- `solidify doctor` — environment diagnostics (inclui check `jev`).
- `solidify report --format pdf|json` — emit artifacts.

## JEV (TypeSafe System One) — optional decision engine

Solidify pode usar o **JEV System One** (motor de decisão determinístico
sub-300ms da TypeSafe AI) para dois caminhos opcionais. Basta ter a chave
no ambiente — sem chave, tudo continua funcionando com a heurística pura.

```bash
export JEV_API_KEY="apikey_..."   # opt-in explícito
```

### 1. Applicability dos gates via JEV

`applicability.jev.enabled=true` faz o run decidir os 8 gates
(sonar/tests/security/lighthouse/zap/k6/migration/env) com o JEV em vez do
LLM generativo — mais rápido e mais barato. Modo `advisory` (default)
enriquece o `Reason` sem override; `enforce` permite override do verdict.

```json
{
  "applicability": {
    "jev": { "enabled": true, "api_key_env": "JEV_API_KEY", "mode": "advisory" }
  }
}
```

No report, `applicability_source` passa a valer `"jev"` nos gates
avaliados (fallback → `"jev-fallback"`).

### 2. Analyzer anti-regressão `jev_regression`

`analyzers.jev_regression.enabled=true` avalia o diff do release com o JEV
contra 4 sinais de regressão antes do gate:

- quebra de assinatura/contrato público (critical)
- enfraquecimento/deleção de lógica funcional (high)
- testes enfraquecidos para fazer a esteira passar (high)
- efeito colateral em estado compartilhado/schema (medium)

Findings aparecem no analyzer `jev_regression` do report; score derruba
15 pontos por sinal detectado. Em qualquer falha (sem chave, timeout,
HTTP) o analyzer vira `skipped` com motivo — nunca fabrica findings nem
quebra o run.

```json
{
  "analyzers": {
    "jev_regression": { "enabled": true, "api_key_env": "JEV_API_KEY" }
  }
}
```

## Architecture

- `internal/ai/` — provider abstraction.
- `internal/jev/` — JEV System One client (choice/noul/score, flexBool).
- `internal/applicability/` — heuristic + LLM/JEV enriched decisions.
- `internal/score/` — SOLID + pillars aggregation.
- `internal/peer/` — multi-peer + role-swap + robustness.
- `internal/gate/` — quality gate (release/quick/contractual).
- `internal/report/` — canonical report (JSON/HTML/PDF).
- `internal/cache/` — analyzer result cache.
- `internal/print/` — PDF generation pipeline.

## E2E

```bash
fixtures/fullstack-sample/  # backend+frontend sample
```

## Security

- `internal/pathguard/` — path traversal defense.
- `internal/shellguard/` — command injection defense.
- `internal/targetguard/` — ZAP/k6 active scan allowlist.
- `internal/secretcorpus/` — secret redaction fixtures.

## ADR index

See [docs/adr/](docs/adr/) — 1 per task (SAI-001..SAI-115).
