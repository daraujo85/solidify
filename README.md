# Solidify

Quality gate for releases. Analyzes diff, runs
multi-peer review with role-swap, computes a release
score, and emits a JSON + HTML dashboard + PDF report
with `PASS`/`WARN`/`FAIL` verdict.

## Quickstart

Build (no host deps — runs in container):

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine \
  sh -c "CGO_ENABLED=0 go build -o /out/solidify ./cmd/solidify"
```

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
- `solidify doctor` — environment diagnostics.
- `solidify report --format pdf|json` — emit artifacts.

## Architecture

- `internal/ai/` — provider abstraction.
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
