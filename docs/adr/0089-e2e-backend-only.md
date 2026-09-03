# ADR 0089 — E2E backend-only release (SAI-106)

Status: Aceito. 2026-08-20.

## Contexto

SAI-106: validar comportamento E2E quando fixture
tem só backend. Lighthouse N/A, backend analyzers
aplicáveis, PDF correto.

## Decisão

`internal/e2e/`:

- `Scenario{HasFrontend, HasBackend, HasMigrations,
  HasEnvChanges, ExpectedGate, ExpectedLighthse}`.
- `StandardScenarios`: backend_only, frontend_only,
  fullstack, minimal.
- `CheckScenario(s)` valida: gate match,
  lighthouse applicability match, migrations,
  env_changes, PDF válido.
- `CheckResult{GateOK, LighthouseOK, MigrationsOK,
  EnvChangesOK, PDFValid, Passed, Reasons}`.
- `BackendOnlyExpected{LighthouseNA, BackendActive,
  PDFValidMagic, JSONParses}`.
- `VerifyBackendOnly(pdf, json)`:
  - PDF deve começar com `%PDF`.
  - JSON não vazio.

## Consequências

- 8 testes: backend_only, frontend_only, fullstack,
  minimal (gate=INCOMPLETE), mismatch custom,
  verify OK, bad PDF, empty JSON.
- Cenários cobrem combinações de frontend/backend
  com expectations explícitas.
- `Reasons` lista o que falhou — log/debug rico.

## Trade-offs

- Sem fixture real — testes sintéticos. Integração
  E2E completa fica p/ ADR de CI real.
- PDF check é só magic bytes — não valida
  estrutura completa. Trade-off: smoke check >
  full PDF parse.
- `ExpectedLighthse` é bool direto — sem semântica
  p/ "should run but failed".
