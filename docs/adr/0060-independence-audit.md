# ADR 0060 — Independence Audit

Status: Aceito. 2026-08-20.

## Contexto

SAI-066: registrar model/provider/context metadata de Peer A
e B e marcar `independence_degraded` quando há overlap.

## Decisão

`PeerMetadata{Label, Provider, Model, Family, ContextHash,
RecordedAt, RequestHash}` em `internal/peer/audit.go`.

`Recorder{store map[label]PeerMetadata}` thread-safe.

`Record(m)` valida campos, popula Family (auto via `inferFamily`)
e RecordedAt.

`Audit(opts)`:
- exige A e B registrados
- `DegradedReasons` = `detectDegradation(a, b, cross)`
- `Degraded = len(reasons) > 0`
- `IndependenceOK = !Degraded`

`detectDegradation` razões:
- same provider
- same model
- same family (excluindo "other" para evitar falso positivo)
- same context hash (cross-contamination suspected)
- cross-contamination strings (truncadas 50 chars)

`ComputeContextHash(parts...)` SHA256 dos parts (separador 0).

`DetectCrossContamination(peerAOutput, peerBPrompt)`:
- marker `PEER_A_OUTPUT:`
- substring match primeiros 100 chars

`RenderReport(audit)` human-readable (✓/✗).

`MarkDegraded(audit, reason)` adiciona flag manual.

`HasDegradation(audit)` helper.

## Consequências

- 22 testes (new recorder, record basic/validate/auto-family/
  timestamp/labels, audit OK/missing, same provider/model/
  family/context, cross-contamination OK/empty, context hash
  stable/sensitive, detect cross marker/substring/clean/empty,
  render OK/degraded, mark degraded manual/nil, has degradation,
  infer family, recorded at within margin).
- Recorder thread-safe via mutex — Peer A e Peer B podem
  ser populados por goroutines distintas.
- Family detection espelha `ModelFamily` do ai/selector —
  taxonomia consistente.
- 4 níveis de degradação (provider/model/family/context) +
  cross-contamination strings — audit granular.

## Trade-offs

- Same family exclui "other" para evitar falso positivo em
  modelos desconhecidos. Trade-off: 2 modelos "other" passam
  sem alerta; caller pode forçar via policy custom.
- Context hash é opcional (`omitempty` no json) — caller
  decide se computa e popula.
- `CrossContamination` em `AuditOptions` é manual — caller
  detecta via `DetectCrossContamination` ou outras heurísticas
  e passa resultado. ADR seguinte pode integrar detection.
- Family auto-inferida sobrescreve só se vazia — caller pode
  forçar family custom (ex: "custom-finetune").
- Sem persistência do recorder — em-memory only. Spec não
  exige SQLite; ADR seguinte pode adicionar.
