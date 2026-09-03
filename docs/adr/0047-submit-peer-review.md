# ADR 0047 — submit_peer_review

Status: Aceito. 2026-08-20.

## Contexto

SAI-053: tool `solidify_submit_peer_review` precisa validar
schema, actor metadata, evidence hash, e atomic save.

## Decisão

`PeerReviewSchemaVersion = "1"`.

`PeerReviewValidator{MinFindings, AllowedVerdicts["approve",
"request_changes","comment"]}`. Default valida:
- run_id, actor, schema, evidence_hash, verdict required
- schema == "1"
- evidence_hash: 64 chars hex
- actor: ≤ 200 chars
- verdict ∈ allowed
- findings ≥ MinFindings (default 0)

`ValidationError{Field, Message}` + `ValidationErrors[]`
implementa `error` + `Errors()`.

`PeerReviewRecord{ReviewID, RunID, Actor, Schema, EvidenceHash,
Verdict, Findings, Notes, SubmittedAt, ContentHash}`.

`ComputeContentHash(in)` sha256(JSON(struct)) — campo
"content_hash" do record; detecta mudanças.

`PeerReviewStore{dir, mu}` — Save (atomic tmp+rename), Load.

`DefaultSubmitPeerReviewHandler(store, validator)`:
1. nil input → error
2. Validate → if errors: return `[Accepted=false, ValidationOK=false, Errors]`
3. deriveReviewID = sha256(runID|actor|contentHash)[:16]
4. Save record atomic
5. return `[Accepted=true, ReviewID, SavedAt, ValidationOK]`

`ValidateEvidenceHash(in, expected)` comparador externo.

## Consequências

- 22 testes (Validate valid/missing fields/unsupported schema/
  unknown verdict/hash not-hex/hash len/long actor/min findings,
  ComputeContentHash stable/sensitive, NewPeerReviewStore
  empty/valid, Save/Load round-trip/Save nil/Save empty-id/
  Load missing/Load empty, DefaultSubmitPeerReviewHandler
  accept/reject/nil-input, ValidateEvidenceHash match/mismatch,
  ValidationErrors.Error(), DeriveReviewID unique).
- Errors incluídos no output (não fatal) — caller vê lista
  em vez de erro exception.
- Review ID determinístico a partir de content + metadata —
  re-submit idempotente.

## Trade-offs

- Review ID = first 16 chars of sha256 — collision risk
  ~2^64 (suficiente); legado uuid seria mais seguro.
- `AllowedVerdicts` hard-coded na validator — sem config
  runtime; aceito.
- ComputeContentHash usa `json.Marshal(map[string]any)`
  ordem não-determinística em findings — content_hash pode
  variar entre Go versions; aceitável (sigla "best-effort").
- Save é atomic single-file; concorrência via mutex —
  suficiente pra CI low-traffic.
