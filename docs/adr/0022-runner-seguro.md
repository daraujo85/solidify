# ADR 0022 — External process runner seguro

Status: Aceito. 2026-08-20.

## Contexto

SAI-028: analyzers rodam binários externos (lighthouse, sonar-scanner,
gitleaks, jest). CVE risk: shell injection via commit message em
shell string, env leak (POSTGRES_PASSWORD no env do child), DoS via
stdout ilimitado, cwd escape (acessa /etc/passwd).

## Decisão

`runner.Run(ctx, name, args, cfg)`:

1. **Binário via `exec.CommandContext`**: args é slice literal —
   nunca `bash -c "..."`. Whitespace e NUL no path rejeitados.
   Path literal requer `filepath.IsLocal` ou absoluto.
2. **Cwd restrito**: `cfg.AllowedDirs` é whitelist opcional. Cwd
   fora → erro. Default = os.Getwd.
3. **Env allowlist**: `cfg.EnvAllow` filtra `os.Environ()`; só nomes
   permitidos passam pro child. `cfg.EnvBase` injeta pares
   `KEY=value` adicionais. Rejeita keys com `=` ou vazias.
4. **Bounded buffers**: stdout/stderr cada um capado em
   `cfg.MaxOutput` (default 1MB). Writer que dropa após limite — não
   bloqueia child.
5. **Timeout**: `cfg.Timeout` via `context.WithTimeout`. Marca
   `TimedOut=true` e `ExitCode=-1`.
6. **Audit**: `Result.Command` é a linha reproduzível (com
   shell-quote seguro só pra display).

## Consequências

- 27 testes: OK, timeout, exit code, bounded, env, cwd, traversal,
  cancel, audit, etc.
- Whitelist de env é o default seguro: nada vaza a não ser que
  analyzer peça explicitamente.
- Bounded buffer é defesa contra DoS de child travado que cospe
  output infinito (ataque ou bug).
- Cwd allowlist + path traversal check mitigam LFI/escape.
- Stdout bounded a 1MB pode cortar logs críticos em análise
  verbose — analyzer pode pedir mais via `Config.MaxOutput`.

## Trade-offs

- Resolve binário via `exec.LookPath` em vez de literal `PATH=...`:
  PATH do child pode ser diferente. Aceitável (analyzer tem PATH
  correto).
- `shellQuote` é só pra audit (display), não usado pra execução —
  execução é slice nativo de args, immune a shell metachars.
