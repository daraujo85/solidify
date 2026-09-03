# Uso pelo agente de terminal

Este documento é a referência canônica para um agente (Claude Code, Codex, 
ou outro terminal MCP-compatible) operar o Solidify via CLI. Leia isto
antes de `spec/AGENT_HANDOFF.md` se seu objetivo é *usar* o binário, não
implementá-lo.

## Contrato de invocação

- Binário único: `solidify`. Sem daemon obrigatório.
- stdout carrega dados (JSON quando `--json`/`--format json`); stderr carrega logs e erros.
- Exit code estável, definido por `internal/errs` (0 = sucesso).
- `--log-format json` também formata erros de stderr em JSON — use isso se for parsear a saída programaticamente.

## Sequência mínima que o agente deve executar

```bash
# 1. Diagnóstico do ambiente antes de qualquer coisa.
solidify doctor --json

# 2. Orquestra a análise completa (Git diff, analyzers aplicáveis,
#    peer review, gate). Ver nota de status abaixo.
solidify run --profile release

# 3. Exporta o artefato canônico já persistido.
solidify report --format json --run <RUN_ID>

# 4. (opcional) Gera o laudo em PDF a partir do mesmo artefato.
solidify report --format pdf --run <RUN_ID> --out ./release-<RUN_ID>.pdf
```

`<RUN_ID>` vem da saída de `solidify run` (ou de `solidify dashboard`).

## Status de cada comando (não invente comportamento)

| Comando | Status | Observação |
|---|---|---|
| `solidify doctor` | estável | checa docker, git, 9router, peer review store. Use `--only=NAME` ou `--canary` para subconjuntos. |
| `solidify run` | **em construção** | hoje só demonstra o caminho de falha parcial (`release-report.json` INCOMPLETE); não orquestra o pipeline completo ainda. Não assuma que produz um report PASS real. |
| `solidify report --format json --run ID` | estável | lê `<store>/<ID>.json` (default `./out/runs`) e reimprime formatado. `--run` obrigatório. |
| `solidify report --format pdf --run ID --out PATH` | estável | sobe dashboard efêmero, renderiza PDF, valida seções obrigatórias. `--run` e `--out` obrigatórios. |
| `solidify dashboard --port 8080` | estável | serve UI + API de reports para inspeção humana. |
| `solidify mcp` | ver `spec/CONTRACTS.md` | expõe as mesmas operações via MCP stdio, para o agente chamar como tool em vez de subprocess. |

Se `solidify run` falhar/for insuficiente, isso é esperado no estado atual do projeto — não tente compensar gerando dados fake; reporte a lacuna.

## CLI vs MCP — quando usar qual

- **CLI** (`solidify <comando>`): use quando o agente só precisa disparar uma etapa e ler o resultado (scripts, CI, verificação rápida).
- **MCP** (`solidify mcp`, stdio): use quando o agente quer orquestrar o fluxo completo com round-trips (`begin_review` → Evidence Bundle → `submit_peer_review` → Peer B/Arbiter → report), como descrito em `spec/AGENT_HANDOFF.md` e `spec/CONTRACTS.md`. Ambos os caminhos operam sobre o mesmo `.solidify/` e o mesmo `release-report.json` — não são fontes de verdade diferentes.

## Build/test (sem toolchain no host)

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine \
  sh -c "CGO_ENABLED=0 go build -o /out/solidify ./cmd/solidify"

docker run --rm -v "$PWD":/src -w /src golang:1.26 \
  sh -c "go test ./... -count=1"
```
