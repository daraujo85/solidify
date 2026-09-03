# ADR 0120 — Canonical payload nativo + schema v2 (SAI-120)

Status: Aceito. 2026-08-21.

## Contexto

SAI-119 (ADR-0119) implantou o transporte MCP real para
peer_a via `PeerReviewStore`. Solução funcionou, mas
pagou um preço de compat:

- `PeerReviewRecord.Notes` (string) carregava o
  payload canônico JSON-serializado
- Schema versioning era implícito: `"1"` constante
  sem caminho de evolução
- Stub legado `<artifact_dir>/peer_a_response.json`
  ficou como fallback em `executeTerminalMCP`,
  marcado pra remoção em SAI-120

Custos:

1. **Marshal/unmarshal overhead** — peer_a parseia
   `Notes` toda vez pra extrair `quality_score` /
   `solid.*.after_score.value`. JSON dentro de JSON.
2. **Sem schema field first-class** — gate computa
   score sobre `ParsedContent` que pode ser qualquer
   coisa. Validação falha silenciosa.
3. **Debt explícita** — ADR-0119 marcou stub
   `<artifact_dir>/peer_a_response.json` pra remoção
   neste ADR.

## Decisão

### 1. Schema version bump "1" → "2"

`PeerReviewSchemaVersion` em
`internal/mcpserver/peer.go` passa de `"1"` para `"2"`.
Validador aceita ambos:

```go
if in.Schema != "" && in.Schema != PeerReviewSchemaVersion && in.Schema != "1" {
    errs = append(errs, ValidationError{...})
}
```

Submissions com `schema: "1"` continuam válidos
(mas logam warning no output sinalizando
deprecation).

### 2. Campo nativo `canonical_payload`

`SubmitPeerReviewInput` e `PeerReviewRecord` ganham
`CanonicalPayload map[string]any` — peer review
canônico (SAI-116 schema: `quality_score`, `solid`,
`confidence`, etc.) como objeto nativo.

```json
{
  "run_id": "r20260821-103012",
  "actor": "peer_a",
  "schema": "2",
  "evidence_hash": "<sha256>",
  "verdict": "comment",
  "canonical_payload": {
    "quality_score": 85.0,
    "confidence": 0.9,
    "solid": {
      "S": {"after_score": {"value": 85.0}},
      "O": {"after_score": {"value": 80.0}},
      ...
    }
  },
  "notes": "comentário humano livre (markdown)"
}
```

`Notes` é preservado como campo livre pra
comentário humano (markdown), separado do payload
canônico.

### 3. Backward compat v1

`recordToExecutorResult` em `peer_a.go` lê
`CanonicalPayload` primeiro; se vazio, parseia
`Notes` como JSON (v1). Records existentes do
SAI-119 continuam legíveis sem migração.

```go
parsed := rec.CanonicalPayload
if parsed == nil && rec.Notes != "" {
    var vp map[string]any
    if jerr := json.Unmarshal([]byte(rec.Notes), &vp); jerr == nil {
        parsed = vp
    }
}
```

### 4. `ComputeContentHash` inclui canonical_payload

Hash estável do conteúdo (usado pra
`review_id = sha256(...)[:16]`) agora inclui
`CanonicalPayload`. Records v1 têm mesmo
`ContentHash` que antes (Notes ainda conta);
records v2 divergem (campo nativo presente).

### 5. Remoção do stub SAI-118

`executeTerminalMCP` para de ler
`<artifact_dir>/peer_a_response.json`. Função
`legacyFileFallback` deletada. ADR-0118 status
passa de "parcial" pra **Aceito** definitivo.

Quem dependia do stub migra usando
`solidify_submit_peer_review` (MCP) — setup doc em
`solidify mcp setup --target claude-code` imprime
snippet pronto.

### 6. Deprecation warning no output

`DefaultSubmitPeerReviewHandler` adiciona warning
em `SubmitPeerReviewOutput.Warnings` quando
submissions v1 chegam:

```
warnings: ["schema v1 deprecated: use canonical_payload (schema v2)"]
```

Não bloqueia save — apenas sinaliza. SAI-121 pode
virar warning em erro quando telemetria confirmar
que ninguém mais usa v1.

## Não-objetivos

- Migration automática de records v1 → v2 (co-exist
  já basta; cleanup async via task separada)
- Telemetria de quantos clientes ainda mandam v1
  (futuro SAI-121+)
- Validação de schema canônico completa via
  `jsonschema-go` (subset no `validateCanonical`
  cobre o essencial; Draft 7 strict é overkill
  agora)
- Auto-fail v1 submissions (warning-only por
  enquanto)

## Consequências

- **Payload nativo** elimina marshal/unmarshal
  redundante. `ParsedContent` já vem pronto.
- **Schema field first-class** — futuras mudanças
  no canônico (SAI-122+) versionam explicitamente.
- **v1 compat mantida** — ninguém quebra.
  Records antigos continuam legíveis.
- **Stub sai** — quem rodava smoke contra
  `peer_a_response.json` precisa atualizar pro MCP
  flow. ADR-0118 marca fim do período de transição.
- **Warning visible** — operator vê deprecation em
  output do submit, sem precisar grep logs.

## Verificação

- `go build ./...` → 0 erros
- `peer_a_test.go` 6/6 testes passam:
  - V2 PollFindsRecord
  - V1 Fallback
  - NoSubmission skipped
  - RunIDMismatch
  - ValidatorAcceptsV1AndV2
  - ResolveSource
- Smoke `solidify mcp setup --target claude-code`
  renderiza snippet (regression check pós-edits)

## Próximo passo

1. ~~SAI-121: telemetria de uso v1 vs v2 + ponto de
   cutoff pra auto-fail v1.~~ ✅ Ver ADR-0121.
2. ~~SAI-122: `solidify peer-reviews migrate` —
   converte records v1 → v2 in-place.~~ ✅ Ver ADR-0122.
3. SAI-123+: schema canônico ganha campos novos
   (ex: `detailed_findings[]` por pilar SOLID)
   sem quebrar v2 — bump pra "3".
