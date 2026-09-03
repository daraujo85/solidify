# ADR 0054 — 9Router Preset + Network Handling

Status: Aceito. 2026-08-20.

## Contexto

SAI-060: 9Router (gateway local em `http://localhost:20128`)
tem comportamento diferente conforme contexto de execução:
host nativo usa `127.0.0.1`, container usa
`host.docker.internal` (macOS/Win) ou gateway address
(Linux nativo). Preset deve resolver isso transparente.

## Decisão

`NetworkContext` enum: `host`, `docker`, `auto`.

Constantes:
- `NineRouterDefaultPort = "20128"`
- `NineRouterDefaultHost = "127.0.0.1"`
- `NineRouterDockerHost = "host.docker.internal"`
- `NineRouterTokenEnv = "ANTHROPIC_AUTH_TOKEN"`
- `NineRouterDefaultLabel = "9router"`

`DetectNetworkContext()`:
- env `SOLIDIFY_NETWORK` (host/native/docker/container)
- senão `/.dockerenv` presente → docker
- senão `DOCKER_DESKTOP` env → docker
- fallback: host

`ResolveBaseURL(ctx, port)` retorna `http://{host}:{port}`
(host = 127.0.0.1 ou host.docker.internal).

`ResolveBaseURLWithHost(ctx, host, port)` customiza host
(defaults aplicados conforme ctx).

`ResolveToken()` lê `ANTHROPIC_AUTH_TOKEN`.

`BuildPreset(opts)`:
- defaults: port=20128, host=127.0.0.1, label=9router
- `Network=auto` → `DetectNetworkContext()`
- `Network=docker && Host=127.0.0.1` → rewrite para host.docker.internal
- token: opts.Token → fallback env

`NineRouterPreset{BaseURL, Token, Network, Host, Port, Label, RequiresKey}`.

`NewProviderFromPreset(preset)` → `OpenAIProvider` com
`WithProviderName(Label)`.

`FormatPreset(p)` → `"9Router[network] base (auth)"`.

`PlatformDockerHost()` switch runtime.GOOS:
- linux → 172.17.0.1 (Docker bridge)
- macOS/Windows → host.docker.internal (Docker Desktop)

## Consequências

- 22 testes (resolve base host/docker/default, custom host,
  token env/empty, detect network via env host/docker/garbage,
  build preset basic/docker/auto/env-token/label/port,
  new provider from preset, IsDockerNetwork, default host,
  format preset auth/no-auth/nil, platform docker host,
  docker auto-rewrite, docker custom host, custom label).
- Auto-detecção via /.dockerenv permite zero-config quando
  Solidify roda em container.
- Preset + provider factory simplifica wiring:
  `preset, _ := BuildPreset(PresetOptions{}); p := NewProviderFromPreset(preset)`.
- `RequiresKey` flag permite UI exibir "configure ANTHROPIC_AUTH_TOKEN"
  quando preset sem token.

## Trade-offs

- `172.17.0.1` em Linux nativo é frágil — networks customizadas
  usam bridge address diferente. Caller deve preferir
  `host-gateway` (Compose) ou override explícito.
- `SOLIDIFY_NETWORK` env sobrepõe detecção — bom pra CI, ruim
  se caller esquece de unsetar.
- `RequiresKey = token != ""` — provider pode aceitar requests
  sem auth em modo dev. Não validamos response 401 aqui.
- Sem timeout/retry no preset — caller adiciona via
  `provider.WithHTTPClient(...)`.
