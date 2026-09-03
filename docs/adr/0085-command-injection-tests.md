# ADR 0085 — Command injection tests (SAI-102)

Status: Aceito. 2026-08-20.

## Contexto

SAI-102: paths e commit subjects maliciosos não
podem entrar em shell. Defesa em profundidade
antes do `exec.Command`.

## Decisão

`internal/shellguard/`:

- `DangerousChars`: `;&|`<>$`\n\r\\'"`
  (sem `()` — conventional commits usam).
- `ShellMetachars`: `&& || >> << $() ${ }; |;`.
- `ValidateArg(arg)`:
  1. Empty → `ErrEmptyArg`.
  2. Qualquer char ou metachar → `ErrInjection`.
- `ValidateArgs(args)` — checa todos.
- `SanitizeSubject(subject)` — ≤200 chars, sem
  dangerous/metachar.
- `SanitizePath(p)` — sem `..`, sem dangerous chars.
- `HasMetachar(arg)` helper.
- Erros sentinel: `ErrEmptyArg`, `ErrInjection`.

## Consequências

- 13 testes: arg empty/valid/dangerous/metachars/
  multi, subject ok/empty/inj/long, path
  ok/traversal/dangerous, hasmetachar.
- `(` e `)` liberados — conventional commits
  `feat(api): ...` passam.
- `$()` bloqueado — command substitution não.

## Trade-offs

- Lista de deny, não allowlist — risco de bypass
  com char novo. Mitigação: caller DEVE usar
  `exec.Command(name, args...)` em vez de
  `sh -c "..."`.
- Sem rate-limit — múltiplas tentativas de bypass
  não bloqueiam caller. Trade-off: simplicidade >
  rate-limit.
- `SanitizePath` permite `/` — paths absolutos são
  tratados por `pathguard` separadamente.
