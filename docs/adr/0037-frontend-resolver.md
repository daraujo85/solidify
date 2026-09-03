# ADR 0037 — Frontend target resolver

Status: Aceito. 2026-08-20.

## Contexto

SAI-043: resolve URL frontend antes de Lighthouse (SAI-044).
Múltiplas fontes possíveis: config estática, runtime hook
dinâmico, env var. Backend-only fixture NÃO tem frontend — resolver
precisa devolver sentinel sem falhar.

## Decisão

`Mode` enum: `config | runtime | disabled | failed | unset`.
`disabled` = backend-only explicit (sentinel). `unset` =
nenhuma fonte encontrada (suave, não erro). `failed` = tentativa
explicou erro (URL inválida).

`Config` carrega URL + HeadersEnv (nomes de env vars a propagar
como headers) + Disabled + HealthPath/Code/Timeout.

`Resolver.Resolve(ctx)` ordem:
1. Se `Disabled` → ModeDisabled, OK
2. Se `URL` → parse, resolve headers, ModeConfig
3. Loop hooks (ordem de adição) — primeiro que devolver `*Target` vence
4. Fallthrough → ModeUnset

`RuntimeHook` interface { Name() string; Resolve(ctx) (*Target, error) }.
Implementação default: `EnvHook{Var string}` lê env var.

`parseTarget` valida scheme (http/https only) + host presente.
Outros schemes (ftp, file, etc) rejeitados.

`IsLocalhost` checa URL OU Origin (test friendly).

`Healthcheck` (placeholder estrutural) — não executa IO real;
valida shape. Caller passa result via http client real quando
integrar.

`resolveEnvHeaders` transforma env names em "Name: Value";
envs vazias silenciosamente skipadas.

## Consequências

- 21 testes (resolver config/disabled/unset/invalid/bad-scheme/
  no-host/runtime/empty/config-beats-hook/first-hook-wins,
  env headers, parseTarget cases, IsLocalhost, IsHTTPS, EnvHook
  name/empty/bad-url, Healthcheck no-target/no-path/real-shape,
  disabled-beats-url).
- SAI-044 (Lighthouse) usa `Resolver.Resolve` pra obter target;
  se Mode=ModeDisabled ou Mode=ModeUnset → skip scanner.
- `Headers` populado a partir de env vars — header value nunca
  logado, redaction em logs (responsabilidade downstream).

## Trade-offs

- Healthcheck não implementa IO real nesta task (placeholder
  estrutural). Caller injeta result real quando integrar.
- Runtime hooks não-discovery: caller adiciona explicitamente.
  Auto-discovery via plugin system seria overkill aqui.
- Headers via env vars não fazem `redaction` automática no struct
  `Target` — quem serializa/loga precisa truncar/mascarar.