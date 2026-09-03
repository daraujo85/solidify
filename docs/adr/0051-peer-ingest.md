# ADR 0051 — Peer A Ingestion Scenarios

Status: Aceito. 2026-08-20.

## Contexto

SAI-057: fixtures para o Peer Review Prompt v1 representando
violações SOLID. Cada cenário é um caso canônico que o peer
deve detectar (ou marcar N/A).

## Decisão

`SolidPrinciple` enum: SRP, OCP, LSP, ISP, DIP.

`PeerIngestScenario{Name, Principle, Symbol, Description,
ExpectFindings, ExpectSeverity, Evidence, Reference, Applicable}`.

10 cenários canônicos em `PeerIngestScenarios()`:
- SRP×2 (class_too_many_duties, mixed_io_and_domain)
- OCP×2 (modify_existing_for_extension, flag_parameter)
- LSP×2 (subtype_breaks_contract applicable, no_inheritance N/A)
- ISP×2 (fat_interface, interface_with_io)
- DIP×2 (concrete_dependency, static_factory_call)

Invariant: `Applicable && ExpectFindings > 0` (applicable não
é "no findings"). N/A: `!Applicable && ExpectFindings == 0`.

`ValidateScenario(s)` checa name, principle, applicable/findings
consistency, severity ∈ {low, medium, high, critical}.

`Summarize(scenarios) IngestionSummary{Total, Applicable,
NotApplicable, ByPrinciple, BySeverity}`.

`ScenarioToFinding` converte applicable scenario em finding
estruturado. `ScenariosToFindings` filtra not-applicable.

`ExpectedCoverageAllPrinciples` valida que os 5 princípios
estão cobertos (applicable).

## Consequências

- 19 testes (5 princípios, cobertura, LSP aplic+NA, busca,
  validate ok/error, invariants, severities, evidence,
  unique names, summary, finding conversion, N/A filter,
  edge coverage).
- Fixtures estáticas, não-read-from-disk — determinístico,
  reproduzível, sem IO em testes.
- LSP N/A explícito: codebase sem herança = princípio não
  avaliável. Peer marca "LSP=N/A" em vez de inventar issues.

## Trade-offs

- Apenas 2 cenários por princípio — mínimo para cobrir happy +
  edge. Cobertura completa exigiria 10+ por princípio.
- Severities atribuídas manualmente — pode divergir da opinião
  do peer real. Aceitável: são "expectativas" pra validar
  consistência do prompt, não ground truth.
- Evidence strings inline — não aponta para arquivo real.
  Review real lê `analyzer` output; fixtures só validam o
  pipeline de ingestion.
- Sem versão nos cenários — adicionar novo princípio quebra
  callers. ADR seguinte pode adicionar `SchemaVersion`.
