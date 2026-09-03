# ADR 0064 — Arbitration merge

Status: Aceito. 2026-08-20.

## Contexto

SAI-070: aplicar veredito do árbitro (SAI-069) sobre findings
de ambos os peers. Derivar accepted/rejected/merged + status
final S/O/L/I/D.

**Aceitação crítica:** árbitro NÃO pode alterar resultado
objetivo de teste/Sonar/ZAP (imutável).

## Decisão

`internal/arbiter/merge.go`:

- `Finding{ID, Kind, Principle, Severity, Symbol, Note,
  Immutable, Accepted, Rejected, Merged, SourcePeer}`.
- `Kind` const: `test|sonar|zap|solid|subjective`.
- `Status` const: `S/O/L/I/D`.
- `MergeInput{RunID, PeerAFindings, PeerBFindings, Verdict}`.
- `MergeResult{RunID, Accepted, Rejected, Merged, Immutable,
  FinalStatus, Reasoning}`.

`Merge(in)`:
1. Coleta immutable findings ANTES do loop (test/sonar/zap).
2. Aplica resolutions do verdict:
   - `accept_peer_a`/`accept_peer_b` → move pra Accepted
   - `accept_both` → cria Merged
   - `reject_both` → move pra Rejected
3. Dedup por ID.
4. `ComputeFinalStatus(r)`:
   - immutable critical/high → `L`
   - accepted/merged critical/high → `L`
   - medium → `O`
   - tudo low + sem findings → `S`
   - resto → `D`

`isImmutableKind(k)` whitelist: `test|sonar|zap`.

`acceptFromPeer(target, source, topic)` parseia topic
`principle:id`.

`mergeBoth(a, b, topic)` tenta A primeiro, fallback B.

`HasImmutableCritical`, `TotalFindings`, `AcceptedCount`,
`MergedCount`, `RejectedCount`, `ImmutableCount` helpers.

`RenderMerge` textual.

## Consequências

- 22 testes: merge OK/empty-runid/no-verdict, immutable preserv
ado, immutable NÃO alterado pelo verdict (aceitação!), accept
peer A/B/both, reject both, topic inválido, isImmutableKind,
final status S/L/L/O/D/merged-critical, hasImmutableCritical,
counters, render, dedup, sort, itoaCount, acceptA not found.
- Immutable findings são **imunes** ao verdict — coletadas
  antes e nunca movidas pra Accepted/Rejected/Merged.
- `ComputeFinalStatus` exportada — caller pode re-derivar
  status após mutação de findings.
- Topic format `principle:id` (e.g. `SRP:f1`) — extensão
  natural pra findings multi-id.

## Trade-offs

- Imutabilidade é convencional (campo `Immutable` + whitelist
  de `Kind`) — caller pode mentir. ADR seguinte pode impor
  via interface segregada.
- Topic parsing simples (1 split) — não suporta IDs com `:`.
  Trade-off: SOLID IDs não têm `:`. ADR seguinte pode usar
  regex/encoding.
- Severity mapping hardcoded (critical/high/medium/low) —
  caller deve usar esses valores. ADR seguinte pode aceitar
  custom severity ladder.
- `D` (defer) é catch-all — pode ser refinado em ADR futuro
  pra distinguir "needs human review" vs "blocked".
- `SourcePeer` apenas em merged — ADR seguinte pode popular
  em accepted também pra audit log completo.
