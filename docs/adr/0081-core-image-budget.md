# ADR 0081 — Core image budget (SAI-098)

Status: Aceito. 2026-08-20.

## Contexto

SAI-098: image core multi-stage Alpine non-root com
git+ca-certs. Limites: 50MB compressed / 200MB
uncompressed.

## Decisão

`internal/imagebudget/`:

- `Budget{MaxCompressedBytes=50MB,
  MaxUncompressed=200MB}`.
- `InspectResult{Image, CompressedSize, Uncompressed,
  User, Workdir, HasGit, HasCACerts}`.
- `InspectImage(img)` — shell em `docker image
  inspect`.
- `CompressedSize(img)` — `docker save` + `gzip -c`
  → bytes.
- `hasBinary(img, bin)` / `hasPath(img, path)` —
  `docker run --rm which` / `test -f`.
- `CheckBudget(r, b)` valida limites.
- `CheckSecurity(r)` valida: User≠root, HasGit,
  HasCACerts.
- `MultiStageDockerfile()` — template canônico:
  - stage 1: `golang:1.26-alpine AS build` +
    `CGO_ENABLED=0 go build`.
  - stage 2: `alpine:3.20` + apk add git/ca-certs +
    addgroup/adduser + USER solidify.

## Consequências

- 11 testes: inspect empty/missing, budget
  nil/uncompressed-over/compressed-over/ok,
  security root/no-git/no-certs/ok/nil, dockerfile
  contém estágios/non-root/git+ca-certs.
- `hasPath` retorna `false` em test env (sem
  docker) — caller deve rodar com docker disponível.
- `CompressedSize` fallback: se `gzip` falhar,
  retorna uncompressed length.

## Trade-offs

- Shell-out p/ docker — não usa Go Docker SDK
  (zero dep). Trade-off: precisa CLI no PATH.
- Limites hard-coded (50MB/200MB) — tune via env
  fica p/ ADR futuro.
- `hasPath` simplificado no test path — produção
  requer docker. Trade-off: testes rápidos > mock
  completo.
