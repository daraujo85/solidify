# ADR 0069 — Quality Gate

Status: Aceito. 2026-08-20.

## Contexto

SAI-075: combinar score global, risk e confidence em
veredito ship/no-ship por perfil.

## Decisão

`internal/gate/gate.go`:

- 3 perfis: `quick` (threshold 60), `release` (75),
  `contractual` (85).
- 5 status: `PASS`, `WARN`, `FAIL`, `BLOCKED`,
  `INCOMPLETE`.
- `GateInput{RunID, Profile, GlobalScore,
  ConfidenceScore, ImmutableCritical, IndependenceOK,
  ArbiterResolved, EvidenceComplete}`.
- `GateResult{RunID, Profile, Status, Reason, Score,
  Threshold}`.

`Evaluate(in)`:

Precedência (alta → baixa):

1. `ImmutableCritical > 0` → BLOCKED.
2. `!IndependenceOK` → BLOCKED.
3. `!EvidenceComplete` → INCOMPLETE.
4. `!ArbiterResolved` → INCOMPLETE.
5. `score < threshold` → FAIL.
6. `score < threshold+5` → WARN.
7. senão → PASS.

WARN band = 5 pontos entre threshold e PASS. Margem
estreita = release apertado, não ruído.

`IsBlocking()`: FAIL/BLOCKED/INCOMPLETE.
`IsPassing()`: PASS/WARN.

`ValidProfile(p)` consulta `ProfileThresholds`.

## Consequências

- 19 testes: pass/warn/fail por profile, blocked por
  immutable/independence, incomplete por evidence/
  arbiter, precedência blocked/incomplete > fail,
  ValidProfile, RenderGate, helpers, validações.
- Gate é puramente determinístico — entrada produz
  mesma saída sempre (sem I/O, sem clock).
- Score sozinho não basta: evidence incompleta para
  gate mesmo com score alto (INCOMPLETE > FAIL).
- Risk/Confidence engines (SAI-073/074) alimentam
  `GateInput` mas gate não conhece seus internals.

## Trade-offs

- WARN band hardcoded em 5 pontos — caller pode querer
  margem custom por domínio.
- Sem suporte explícito a overrides por projeto — só
  3 perfis globais. ADR futuro pode expor config.
- Threshold único por perfil — release não diferencia
  backend crítico vs UI experimental. Trade-off:
  simplicidade > granularidade no MVP.
- `ConfidenceScore` está no input mas não decide
  status — reservado p/ ADR futuro (combinar
  risk+confidence).