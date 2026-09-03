# ADR 0096 — solidify doctor (SAI-113)

Status: Aceito. 2026-08-20.

## Contexto

SAI-113: diagnóstico de ambiente pré-flight. Sem
doctor, erros de setup só aparecem tarde (deep
stack).

## Decisão

`internal/doctor/`:

- `CheckResult{Name, OK, Message}`.
- `AllChecks`: docker, git, go, disk_space, network,
  9router.
- `RunCheck(name)` switch por nome.
- `RunAll()` itera todos.
- `Summary{Total, OK, Fail}` + `Summarize(rs)`.
- Checks individuais:
  - docker/git/go via `exec.LookPath`.
  - disk_space via `os.Stat(wd)`.
  - network via `/etc/resolv.conf` (smoke).
  - 9router via `NINEROUTER_URL` env + `http` prefix.
- Erro sentinel: `ErrCheckFailed`.

## Consequências

- 10 testes: unknown, docker, git, go, disk,
  9router missing/bad/ok, run all, summarize.
- `t.Setenv` permite teste isolado por env var.
- Network check é conservador — não tenta HTTP.

## Trade-offs

- Sem HTTP probe real — só env/config check.
  Trade-off: test-friendly > real connectivity.
- Network check trivial (resolv.conf present). Não
  detecta DNS down. Trade-off: zero cost > accuracy.
- Sem thresholds (disk space MB) — caller adiciona
  em versão futura.
