# ADR 0087 — Active target safety tests (SAI-104)

Status: Aceito. 2026-08-20.

## Contexto

SAI-104: ZAP active / k6 destructive não pode rodar
em host fora da allowlist. Previne scan acidental
em produção.

## Decisão

`internal/targetguard/`:

- `Mode` enum: `ModeZAPActive`, `ModeK6`,
  `ModeDestruct`, mais modos safe.
- `DangerousModes` = [zap, k6, destruct].
- `ProductionSuffixes`: `.amazonaws.com`,
  `.azure.com`, `.googleapis.com`,
  `.cloudfront.net`, `.herokuapp.com`, `.prod.`,
  `.production.`.
- `Guard{AllowHosts}` + `NewGuard(allow)`.
- `Validate(host, mode)`:
  1. Se mode não-dangerous → passa.
  2. Host vazio → `ErrNotAllowed`.
  3. Match em ProductionSuffixes (case-insensitive)
     → `ErrProduction`.
  4. Allowlist check (case-insensitive). Match →
     ok. Sem match → `ErrNotAllowed`.
- `IsDangerous(m)` + `CheckProductionOnly(host)`
  helpers.

## Consequências

- 9 testes: safe mode passa, localhost allowed,
  empty host, not allowed, production heuristic
  (3 hosts), k6 dangerous, IsDangerous helper,
  CheckProductionOnly, case-insensitive allowlist.
- Heurística de produção é defensiva — false
  positives (ex: `prod.` em dev URL) bloqueiam.
  Trade-off documentado.
- Allowlist vazia = bloqueia tudo (fail-closed).

## Trade-offs

- Heurística de produção por substring — pode
  bloquear host legítimo com `.prod.` no nome.
  Trade-off: false positive > false negative.
- Allowlist é lista simples — sem wildcards/glob.
  Trade-off: simplicidade > expressividade.
- Sem timeout/circuit-breaker — caller controla
  tempo de scan.
