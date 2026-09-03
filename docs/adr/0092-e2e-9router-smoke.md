# ADR 0092 — E2E 9Router adapter smoke (SAI-109)

Status: Aceito. 2026-08-20.

## Contexto

SAI-109: smoke test do adapter 9Router. Health
endpoint, combo routing, fallback chain.

## Decisão

`internal/ninesmoke/`:

- `Gateway{BaseURL, AuthToken, Combos}` stub.
- `Health()` — online se URL setada; retorna
  combos/models counts.
- `ResolveCombo(combo, failed)` — primeiro modelo
  não-failed.
- `RegisterCombo(name, models)` adiciona/atualiza.
- `RunSmoke(combo)`:
  1. Health → `HealthOK`.
  2. ResolveCombo sem failed → `ComboResolved`.
  3. ResolveCombo com primário failed → diferente
     → `FallbackWorked`.

## Consequências

- 9 testes: health empty/ok, resolve OK/missing/
  exhausted, fallback, RunSmoke ok/bad, contains
  helper.
- Sem HTTP real — stub. Integração com 9Router
  live fica p/ ADR de CI.
- AuthToken armazenado — caller zera após uso.

## Trade-offs

- Gateway é stub — testes não cobrem network
  real. Trade-off: portabilidade > fidelity.
- Health check trivial (URL non-empty). Detecção
  real de outage fica p/ HTTP probe.
- `failed` é slice — sem rate-limit por modelo.
