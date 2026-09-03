# ADR 0002 — Logging estruturado, códigos de erro e dois packages fora de §28

- Status: proposta
- Data: 2026-08-19
- Tasks afetadas: SAI-003

## Contexto

SAI-003 pede logger em stderr com níveis e JSON opcional, "error codes estáveis para CLI" e nenhum log de secret. A spec não enumera os códigos nem define onde esses componentes vivem.

## Incompatibilidade encontrada

1. **Taxonomia de códigos ausente.** `ARCHITECTURE.md` menciona `exit code` apenas como campo de resultado de processo externo (§ runner). Não há lista de códigos do próprio binário.
2. **`ARCHITECTURE.md` §28 não prevê packages para logging nem para erros.** A árvore tem `app`, `config`, `redaction`, etc., mas logging e erros tipados são transversais a todos eles — colocá-los em `app` criaria import cycle assim que `config` ou `gitx` precisarem logar.

## Opções

Para os códigos:

1. Só exit codes numéricos — ilegível em logs e não serializável no report. Descartada.
2. Enum de strings (`Code`) com mapa para exit code numérico. **Escolhida:** a string vai para JSON/report, o número vai para o shell.
3. Reusar códigos de erro do Sonar/HTTP — semântica alheia ao domínio. Descartada.

Para o posicionamento:

1. Logging/erros dentro de `internal/app` — cria ciclo. Descartada.
2. Package único `internal/diag` com as duas responsabilidades — acopla concerns distintos. Descartada.
3. **`internal/logging` + `internal/errs`, ambos sem dependências internas.** **Escolhida.**

## Decisão proposta

- Árvore de §28 ganha `internal/logging` e `internal/errs`, ambos folhas do grafo de imports (dependem só da stdlib).
- `errs.Code` é string estável; faixas de exit code:

  | Faixa | Uso |
  |---|---|
  | 0 | sucesso |
  | 1 | `internal` (bug do Solidify) |
  | 2 | `usage` |
  | 3–9 | entrada/ambiente: `config_invalid`, `not_found`, `git`, `io`, `storage`, `security`, `schema` |
  | 10–19 | **reservada** para veredictos de Quality Gate (SAI-075) |
  | 20+ | execução: `analyzer`, `provider`, `timeout`, `canceled` |

  A reserva de 10–19 evita colisão entre "o Solidify falhou" e "o gate reprovou a release" — distinção que scripts de CI precisam fazer.
- Código nunca é renomeado; só se adiciona. Golden test `TestExitCodesAreStable` trava o mapa.
- Logger é `log/slog` da stdlib, sem dependência externa. Handler recebe `ReplaceAttr` que mascara atributos com chave secret-like antes de qualquer escrita — guardrail mínimo; o framework completo (valores, headers, connection strings) é SAI-020.
- Contrato de streams: stdout = dados, stderr = logs e erros. Em `--log-format json` o stderr contém **apenas** JSON — nada de usage em texto colado no fim, senão o payload deixa de ser parseável.

## Impacto em performance/compatibilidade

Zero dependências novas. Binário passou de 1.900.706 para 2.162.850 bytes (+262 KB, `log/slog` + `encoding/json`), bem dentro do budget. `AddSource` só liga em debug, para não pagar stack lookup no caminho normal.

## Invariantes preservados

- Nenhum secret em log ou artifact.
- Sem dependência externa.
- `CGO_ENABLED=0`.
