# ADR 0030 — Sonar scanner runner opcional

Status: Aceito. 2026-08-20.

## Contexto

SAI-036: Web API adapter (SAI-034) lê dados já analisados. Em CI/
release local, gate pode precisar rodar scanner pra popular o
SonarQube antes de consultar. Solidify NÃO sobe Sonar Server
(princípio: dependência externa), mas pode rodar CLI scanner se
usuário já tem server disponível.

## Decisão

`ScannerMode`: Disabled | Local | Docker.

Disabled = default. Gate release funciona só com Web API.

Local = `exec.CommandContext("sonar-scanner")` no ProjectDir,
herda env do processo + overrides do `cfg.Env`.

Docker = `docker run --rm -v $PWD:/src -w /src image args...`.
Image default: `sonarsource/sonar-scanner-cli:latest`. Mount
readonly por default.

`ShouldRun` valida pré-condições (mode, ProjectDir existe + é dir,
binário no PATH pra local). Falha = skip com reason (não erro).

`RunScanner` timeout 15min default, `context.WithTimeout`. Exit
code preservado no `ScannerResult`.

## Consequências

- 16 testes (mode parse, should-run checks, skip paths, run local
  echo/fail/cancel, mergeEnv, available).
- Gate release não trava se scanner falhar — skip explícito.

## Trade-offs

- Docker mount `-v src:/src` é rw — bom pra scanner gerar reports;
  env vars com `-e` podem vazar em `ps` no host durante run.
- Sem retry em transient failures (network blip no sonar scanner).
- Sem suporte a `-Dsonar.token=...` via CLI arg (vai por env
  pra evitar expor token em `ps` e em logs).
