# ADR 0082 — Tool resource limits (SAI-099)

Status: Aceito. 2026-08-20.

## Contexto

SAI-099: profiles compose-style com limites
configuráveis. Heavy tools (k6, lighthouse, e2e)
com concurrency=1.

## Decisão

`internal/limits/`:

- `Profile{Name, CPU, Memory, Concurrency, Heavy,
  Notes}`.
- `StandardProfiles`:
  - `light`: 0.25 CPU, 256m, concurrency=4.
  - `medium`: 0.5 CPU, 512m, concurrency=2.
  - `heavy`: 2.0 CPU, 2g, concurrency=1 (forçado).
- `Limits{Tool, Profile}` + `DefaultLimits` por tool:
  - k6, lighthouse, playwright → heavy.
  - jest, go-test → medium.
  - eslint, prettier → light.
- `EffectiveConcurrency(p)` força 1 se `Heavy`.
- `Resolve(tool)` retorna profile expandido.
- `ComposeFragment(p)` gera YAML-style fragment.
- `ValidateProfile(p)` — CPU/Memory obrigatórios;
  Heavy com Concurrency>1 falha.

## Consequências

- 10 testes: effective heavy/light/zero, resolve
  known/unknown, compose fragment, validate
  ok/no-cpu/no-mem/heavy-bad, itoaL.
- `stringErr` type privado p/ error sem `errors`
  (zero import).
- ComposeFragment é textual — caller formata YAML
  real (compose/k8s/etc.).

## Trade-offs

- Sem integração direta com docker-compose — caller
  usa ComposeFragment como input. Trade-off:
  portabilidade > acoplamento.
- Profiles hard-coded (light/medium/heavy) — sem
  tuning runtime via config. Trade-off: zero config
  > flex.
- `Resolve` lookup simples em map — sem fallthrough
  p/ prefix matching. Caller deve usar nome exato.
