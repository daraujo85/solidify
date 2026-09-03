# ADR 0072 — Artifact immutability/hash

Status: Aceito. 2026-08-20.

## Contexto

SAI-078: finalized report não é modificado. Rerun produz
novo run_id + content_hash. CI/CD e auditoria precisam
garantir que report no disco = report registrado.

## Decisão

`internal/report/store.go`:

- `Store` append-only com `sync.RWMutex`.
- 2 índices: `byID` (run_id → StoredReport) e `byHash`
  (content_hash → []run_id).
- `Finalize(r, path)` registra `Report` calculando hash
  via `r.HashContent()` (canônico).
- `FinalizeBytes(runID, data, path)` registra bytes
  brutos — hash = SHA-256 do raw.
- Re-finalize do mesmo run_id com mesmo conteúdo é
  idempotente (retorna entrada existente).
- Re-finalize do mesmo run_id com conteúdo divergente
  → `ErrReportMutated`.
- `VerifyHash(runID, data)` e `VerifyReport(r)` — checa
  integridade sob demanda.
- `Get(runID)` retorna **cópia rasa** — caller não pode
  mutar store acidentalmente.
- `MarshalSnapshot()` exporta índice leve (sem `Report`).
- `NewRunID(prefix, time)` gerador determinístico
  (`{prefix}-{UnixNano}`).

Erros sentinel: `ErrDuplicateRunID`, `ErrReportMutated`,
`ErrHashMismatch` — caller pode discriminar com
`errors.Is`.

## Consequências

- 16 testes: finalize basic/idempotent/mutado/nil/
  vazio, finalize bytes idempotent/mutado, get/byhash,
  verify bytes/report, list ordenado, marshal snapshot,
  new runid, get cópia.
- Cópia rasa em `Get` evita surpresa de mutação
  externa — store efetivamente imutável.
- Hash canônico (SAI-077) garante que mesma estrutura
  → mesmo hash, independente de ordem de keys.
- Erros wrapping via `fmt.Errorf("%w: ...", ...)` —
  compat com `errors.Is`.

## Trade-offs

- Cópia rasa: mutar nested slice (ex: `Report.Git.Commits`)
  ainda afeta store. Trade-off: simplicidade > cópia
  profunda. ADR futuro pode trocar por deep copy se
  necessário.
- Sem persistência em disco — store é in-memory. CLI
  orquestrador persiste via filesystem; esta package
  garante lógica de imutabilidade. ADR seguinte pode
  adicionar disk-backed store.
- Snapshot omite `Report` — caller precisa do report
  completo via `Get`. Trade-off: snapshot leve p/
  indexação vs duplicação.
- Sem locking por run_id individual — mutex global.
  Aceitável p/ volume típico (runs/dia).