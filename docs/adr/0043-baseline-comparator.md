# ADR 0043 — Baseline comparator

Status: Aceito. 2026-08-20.

## Contexto

SAI-049: persistir LoadResult (SAI-048) como baseline canônico
por fingerprint; comparar novo run contra baseline selecionado
por branch policy; decisão de promoção/update.

## Decisão

`Branch` enum (main/release/develop/feature) com
`promotionPriority` map (main=100, release=80, develop=50,
feature=10).

`LoadResultMeta{Result, CapturedAt, RefRunID, Branch, Notes}`
envolve `load.LoadResult`. `BaselineStore{dir, mu}` com
ops: `NewBaselineStore`, `Path`, `Save` (atomic via tmp+rename),
`Load`, `Delete` (missing = no-op), `List(branches...)`
(filter por branch opcional, sort Desc por CapturedAt).

`SelectBaselineForBranch(fp, branches)` tenta primeiro branch
do array (priority preference). `SelectHighestPriorityBaseline`
pega promotionPriority maior.

`DecisionKind` (promote/keep/reject/require_new_run).
`Decision{Kind, Baseline, Current, Delta, Reason, ComparedAt}`.

`CompareDecision(current, branches, th)`:
- nil current → reject
- sem baseline match → promote
- comparou + regression → reject
- comparou + passou → promote
- comparação erro → require_new_run

`PromoteIfBetter(decision, refRunID, branch, notes)`:
só salva se Decision=Promote AND (sem baseline OR
current.P95 < baseline.P95). Update only if better.

`Validate`, `ShouldRun`, `AgingDays`, `IsStale(maxAge)`,
`SortByCapturedAt`, `FilterByFingerprint`, `DecisionToReason`.

## Consequências

- 28 testes (NewBaselineStore empty, Save nil/empty-fp/
  result-nil, Load missing/empty, Delete + DeleteMissing,
  List + Filter, SelectBaselineForBranch match/no-match/empty,
  SelectHighestPriority match/no-match, CompareDecision
  no-baseline/nil/regression/pass/incompatible, PromoteIfBetter
  better/not-better/reject/nil, Validate cases, ShouldRun,
  AgingDays, IsStale, DecisionToReason, SortByCapturedAt,
  FilterByFingerprint, Path, JSONRoundTrip).
- `*LoadResultMeta` nil pointer-checks em AgingDays/IsStale
  (Go permite method call em nil receiver quando função
  checa `b == nil`).
- Atomic write (tmp + rename) previne baseline corrompido
  em crash mid-write.

## Trade-offs

- Single-level filesystem (sem external storage) — simples,
  test-friendly, suficiente pra CI/cache local.
- Branch priority hard-coded (não config) — promote policy
  raramente muda; aceito.
- `SelectHighestPriorityBaseline` usa map lookup O(N); N
  tipicamente < 100 baselines; aceitável.
- `PromoteIfBetter` rer-write sempre se melhor — sem
  versionamento; v0 OK.
