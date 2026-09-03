# ADR 0001 — Versão do Go e ambiente de build

- Status: proposta
- Data: 2026-08-19
- Tasks afetadas: SAI-001, SAI-002

## Contexto

`TASKS.md` (SAI-001) exige `go.mod` com **Go 1.27**. `AGENT_HANDOFF.md` também impõe Docker-first e `CGO_ENABLED=0`.

## Incompatibilidade encontrada

1. **Go 1.27 não existe.** A release estável mais recente disponível é 1.26.6. Declarar `go 1.27` no `go.mod` torna o módulo não compilável em qualquer toolchain existente.
2. **Restrição do ambiente do projeto:** o host não pode ter Docker nem toolchains instalados. Logo não há runtime de build/teste permanente disponível para o core Go.

## Opções

1. `go 1.27` no `go.mod` — módulo não compila hoje. Descartada.
2. `go 1.26` — versão maior estável disponível; sem `toolchain` pin, para não forçar download automático em máquinas restritas. **Escolhida.**
3. Fixar `go 1.25` por conservadorismo — perde melhorias sem ganho, e a spec pede a versão mais nova. Descartada.

Para o ambiente de build:

1. Toolchain permanente no host — vetado pelo usuário.
2. Toolchain efêmero no host (instalar, verificar, desinstalar) — funciona, mas é lento e suja o host a cada task.
3. **Toolchain em container `golang:1.26-alpine` com bind mount do repo.** **Escolhida:** o host não recebe nada, a versão do Go fica fixada por tag (build reproduzível), e o binário resultante é estático.

## Decisão proposta

- `go.mod` declara `go 1.26`; subir para `1.27` quando a release existir, apenas se algum recurso da versão for necessário.
- Verificação roda em container: `docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine`. Confirmada em SAI-001 com Go 1.26.7: `gofmt` limpo, `go vet` limpo, 6 testes verdes, `CGO_ENABLED=0 go build` ok, `solidify version` (humano e `--json`) ok.
- O Makefile expõe os alvos de forma agnóstica: por padrão delegam ao container; `NATIVE=1` usa o `go` do host quando existir.
- Quando entrarem dependências (SAI-022), montar volume persistente em `/go/pkg/mod` para não re-baixar o módulo em cada verificação.

## Impacto em performance/compatibilidade

Nenhum: 1.26 atende `CGO_ENABLED=0` e binário estático. Sem dependências externas ainda.

## Invariantes preservados

- Core em Go, sem dependência externa em SAI-001.
- `CGO_ENABLED=0`.
- Nenhuma mudança arquitetural silenciosa.
