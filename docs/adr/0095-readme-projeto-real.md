# ADR 0095 — README do projeto real (SAI-112)

Status: Aceito. 2026-08-20.

## Contexto

SAI-112: README canônico do projeto Solidify.
Quickstart via Docker (sem deps no host), perfis,
comandos, arquitetura, segurança.

## Decisão

`README.md` na raiz com seções:

- **Quickstart**: build via `docker run golang:1.26-alpine`,
  tests via `golang:1.26`.
- **Profiles**: quick=60, release=75, contractual=85.
- **Commands**: `run`, `gate`, `dashboard`, `doctor`,
  `report`.
- **Architecture**: mapeamento internal/* → função.
- **E2E**: referência à fixture fullstack.
- **Security**: pathguard, shellguard, targetguard,
  secretcorpus.
- **ADR index**: link p/ docs/adr/.

## Consequências

- 65 linhas, leitura < 2min.
- Não duplica conteúdo de docs/adr/ — só linka.
- Quickstart via Docker alinha com constraint
  do projeto (zero install no host).

## Trade-offs

- Sem badges (coverage, version) — caller adiciona
  via CI. Trade-off: zero CI config > cosmetic.
- Sem GIF/demo — texto puro. Trade-off: leveza >
  visual.
