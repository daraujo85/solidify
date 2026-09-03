# Setup local do Solidify

Guia para rodar o Solidify contra qualquer repositório Git local,
usando o 9Router como gateway de modelos e Docker como runtime
do binário (constraint "no install no host").

Pré-requisitos: macOS/Linux com Docker Desktop, 9Router rodando
em `localhost:20128`, token em env (`ANTHROPIC_AUTH_TOKEN`).

## 1. Visão geral

```
[cwd do repo]──┐                          ┌──[9Router combos]──┐
               │   bin/solidify            │                    │
               ├──docker run────solidify──▶│  peer_a (modelo X) │
               │   -v $PWD:/work          │  peer_b (modelo Y) │
               │   -e ANTHROPIC_AUTH_TOKEN│  arbiter_c (Z)     │
               │                          │                    │
               └◀──report JSON────────────┴────────────────────┘
```

- O wrapper `bin/solidify` monta o cwd em `/work` dentro do container.
- O binário (non-root alpine) lê `solidify.json` do git root.
- Cada peer HTTP é uma chamada independente ao 9Router — sem memória
  compartilhada entre roles.

## 2. Pré-requisitos

```bash
docker --version                  # Docker Desktop instalado
curl -sf -H "Authorization: Bearer $ANTHROPIC_AUTH_TOKEN" \
  http://localhost:20128/v1/models | head -c 200  # 9router up
```

`ANTHROPIC_AUTH_TOKEN` deve estar no env. Forma rápida de carregar:

```bash
export ANTHROPIC_AUTH_TOKEN=$(grep ANTHROPIC_AUTH_TOKEN ~/.claude/settings.json | cut -d'"' -f4)
```

(O token nunca é commitado — sempre via env.)

## 3. Build da imagem

Na raiz do repo Solidify:

```bash
docker build -f Dockerfile.solidify -t solidify:local .
```

Valida:

```bash
docker run --rm solidify:local version --json
```

Deve imprimir versão + commit. Imagem fica em `solidify:local` (cache).

## 4. Criar combos no 9Router

3 combos dedicados (chain de fallback pra diversidade + redundância):

| Combo | Fallback chain |
|---|---|
| `solidai-peer-a` | `gc/gemini-3.1-pro-preview` → `cc/claude-sonnet-5` → `gemini/gemini-3.1-pro-preview` → `cx/gpt-5.6-sol` |
| `solidai-peer-b` | `cc/claude-opus-5` → `gc/gemini-3-pro-preview` → `cx/gpt-5.6-sol` → `gc/gemini-3.1-flash-lite-preview` |
| `solidai-arbiter` | `cc/claude-opus-5` → `gc/gemini-3.1-pro-preview` → `cx/gpt-5.6-sol` |

Login (cookie) + POST via skill `9router-combos`:

```bash
GATEWAY=http://localhost:20128
curl -c /tmp/9r.txt -X POST "$GATEWAY/api/auth/login" \
  -H "Content-Type: application/json" -d '{"password":"dimome092526"}'

# Criar combo peer-a
curl -b /tmp/9r.txt -X POST "$GATEWAY/api/combos" \
  -H "Content-Type: application/json" \
  -d '{"name":"solidai-peer-a","models":[
    "gc/gemini-3.1-pro-preview",
    "cc/claude-sonnet-5",
    "gemini/gemini-3.1-pro-preview",
    "cx/gpt-5.6-sol"
  ]}'

# Repetir para peer-b e arbiter.
```

> **REGRA PÉTREA**: nunca tocar `~/.9router/data/db/data.sqlite`
> direto enquanto o container 9router estiver rodando — sempre via API.

## 5. Inicializar config num repo alvo

Em qualquer repositório Git (não precisa ser o repo Solidify):

```bash
cd /caminho/do/meu-repo
docker run --rm -v "$PWD":/work solidify:local init
# → cria solidify.json + .solidify/ no git root
```

Sobrescrever config com o template de setup local:

```bash
# A partir da raiz do repo Solidify (este), copia o exemplo.
cp examples/solidify-config.json /caminho/do/meu-repo/solidify.json
```

Ou edite `solidify.json` e ajuste:

- `ai.external_provider.base_url_docker` →
  `http://host.docker.internal:20128/v1` (macOS/Win);
  `http://172.17.0.1:20128/v1` (Linux).
- `ai.selection.peer_b.preferred` → `["solidai-peer-b"]`.
- `ai.selection.arbiter.preferred` → `["solidai-arbiter"]`.

## 6. Wrapper `bin/solidify`

O wrapper vive na raiz do repo Solidify. Pra usar fora:

```bash
# (a) copia pra um repo:
cp /caminho/solidfy/bin/solidify /caminho/do/meu-repo/bin/solidify

# (b) ou instala em ~/.local/bin (global):
ln -sf /caminho/solidfy/bin/solidify ~/.local/bin/solidify
```

Uso:

```bash
bin/solidify version
bin/solidify doctor
bin/solidify dashboard --port 8765
bin/solidify run --profile release .
```

Variáveis:

| Var | Default | Função |
|---|---|---|
| `ANTHROPIC_AUTH_TOKEN` | (env) | token do 9Router |
| `NINEROUTER_URL` | (env) | override da URL do gateway |
| `SOLIDIFY_IMAGE` | `solidify:local` | tag da image Docker |
| `SOLIDIFY_NETWORK` | `auto` | `auto`/`host`/`docker` |
| `DOCKER_HOST_HOST_GATEWAY` | (vazio) | `true` força `--add-host` no Linux |

## 7. Smoke test

```bash
bin/solidify doctor
```

Saída esperada:

```
OK    docker        docker disponível
OK    git           git disponível
OK    disk_space    wd acessível: meu-repo
OK    network       resolv presente
OK    9router       OK http://host.docker.internal:20128/v1/models (200 OK)
5 checks: 5 OK, 0 FAIL
```

Se o check `9router` falhar:
- `NINEROUTER_URL não setada` → exporte `ANTHROPIC_AUTH_TOKEN`.
- `401 Unauthorized` → token expirou. Re-exporte de `~/.claude/settings.json`.
- `probe falhou: …` → 9router caiu. `docker ps --filter name=9router`.

## 8. Primeiro run

```bash
bin/solidify run --profile release .
```

(Atualmente `solidify run` retorna erro claro dizendo que o
orchestrator ainda está em construção. Quando wired, devolve
report JSON + PDF em `.solidify/`. Veja "Estado atual" abaixo.)

## 9. Auto-avaliação via sessão IA

O wrapper é um binário comum — qualquer sessão IA (Claude Code,
OpenCode, SessionFlow worker) pode invocá-lo via tool Bash:

```bash
# Dentro de qualquer sessão:
bin/solidify run --profile release .   # ou /caminho/do/repo
```

O Solidify faz chamadas HTTP independentes ao 9Router — não passa
pelo contexto da sessão que invocou. Cada peer recebe contexto
limpo. Resultado volta como JSON no stdout.

## 10. Estado atual (2026-08-20)

| Comando | Status |
|---|---|
| `version` | ✅ wirado |
| `init` | ✅ wirado |
| `doctor` | ✅ wirado (probe real do 9Router) |
| `dashboard` | ✅ wirado |
| `run` | ⚠️ stub — orchestrator em construção |

A imagem Docker funciona. `doctor` valida o setup ponta-a-ponta.
O comando `run` é a próxima task do plano
(`/Users/diegoaraujo/.claude/plans/gentle-crunching-boole.md`).

## 11. Troubleshooting

| Sintoma | Causa provável | Fix |
|---|---|---|
| `exec format error` ao rodar `bin/solidify` | shell wrapper rodando em arch errada | use `bash bin/solidify …` |
| `bind: address already in use` no dashboard | port 8765 ocupada | `--port 9000` |
| `host.docker.internal` não resolve (Linux) | Docker bridge não mapeia | `DOCKER_HOST_HOST_GATEWAY=true bin/solidify …` |
| `9router` FAIL: connection refused | container 9router caiu | `docker start 9router` |
| `9router` FAIL: 401 | token expirado | re-exporte `ANTHROPIC_AUTH_TOKEN` |
| Image > 50MB compressed | alguma dep nova no go.mod | `docker images solidify:local`; revisar ADR 0081 |
| `init` retorna `CodeGit` | cwd não é repo Git | rode dentro de repo, ou `--dir=/caminho/repo` |

## 12. Constraints preservadas

- **No install no host** — Go toolchain vive dentro do container;
  o host só tem Docker + curl.
- **Token nunca commitado** — sempre via env, nunca em
  `solidify.json` ou `.env.example`.
- **Combos via API only** — nunca tocar `data.sqlite` direto.
- **Binário roda non-root** — Dockerfile cria usuário `solidify`.
