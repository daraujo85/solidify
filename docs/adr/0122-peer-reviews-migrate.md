# ADR 0122 — Peer-reviews migrate v1 → v2 (SAI-122)

Status: Aceito. 2026-08-21.

## Contexto

SAI-120 (ADR-0120) bumpou schema "1"→"2" e moveu o
payload canônico de `Notes` (string JSON) pra
`CanonicalPayload` (map nativo). Records existentes
no `PeerReviewStore` ainda são v1 — legíveis via
fallback em `recordToExecutorResult`, mas warnings
constantes em telemetria e ninguém quer carregar
schema deprecated indefinidamente.

SAI-121 (ADR-0121) deu a ferramenta de decisão
(`peer-reviews stats` mostra V1Count real) + o
cutoff (data após a qual v1 = hard fail). Falta a
**migração concreta** que limpa os records antigos
quando operador decide.

## Decisão

### 1. Subcommand `solidify peer-reviews migrate`

Novo subcommand em `internal/app/peer_reviews.go`,
dispatch em `app.go`. Reusa o `PeerReviewStore`
existente (atomic save via tmp+rename).

```
solidify peer-reviews migrate [--store-dir PATH] [--actor NAME]
                                [--dry-run] [--json]
```

Flags:

- `--store-dir PATH` — override do dir (default
  `~/.solidify/peer_reviews/`)
- `--actor NAME` — filtra (`peer_a`, `peer_b`,
  vazio = todos)
- `--dry-run` — não persiste, só mostra o que seria
  migrado
- `--json` — output estruturado pra scripts

### 2. Algoritmo

`MigrateV1ToV2(store, opts)` em
`internal/mcpserver/migrate.go`:

```go
for rec in records {
    if opts.Actor != "" && rec.Actor != opts.Actor { continue }
    res.Scanned++
    if rec.Schema != "1" { res.SkippedV2++; continue }
    if rec.Notes == "" { res.SkippedBad++; continue }
    payload := json.Unmarshal(rec.Notes)
    if err { res.SkippedBad++; continue }
    if !opts.DryRun {
        rec.Schema = "2"
        rec.CanonicalPayload = payload
        rec.Notes = ""
        rec.ContentHash = ComputeContentHash(equiv)
        store.Save(rec)
    }
    res.Migrated++
}
```

Pontos importantes:

- **ContentHash recomputado**: v1 hash ≠ v2 hash
  (campo CanonicalPayload agora conta). Se não
  recomputar, `deriveReviewID(in)` no próximo
  submit divergiria e geraria IDs duplicados.
- **Notes limpo**: `rec.Notes = ""` após
  migration. SAI-120 deixou Notes como campo
  "livre pra comentário humano" — record v1 só
  tinha o payload canônico ali, então pós-migrate
  não sobra nada útil.
- **Skip silencioso de v2**: record já migrado não
  é tocado. Permite rodar `migrate` múltiplas
  vezes sem efeito colateral (idempotente).
- **Skip silencioso de bad JSON**: record v1 com
  Notes não-JSON é impossível migrar sem perder
  dados. Conta em `SkippedBad` mas não falha a
  operação — operador pode inspecionar manualmente
  e decidir.

### 3. Relatório

`MigrateResult` struct (JSON-serializável):

```json
{
  "total_in_store": 142,
  "scanned": 142,
  "migrated": 8,
  "skipped_v2": 130,
  "skipped_bad_json": 4,
  "errors": []
}
```

CLI output (default human):

```
Total in store: 142
Scanned:        142 (após filtro)
Migrated:       8 applied
Skipped v2:     130 (já migrados)
Skipped bad:    4 (Notes não-JSON ou vazio)
```

Quando `len(Errors) > 0`, lista cada erro
(save failures). Operator sabe exatamente o que
não foi migrado e por quê.

### 4. Atomicidade

Reusa `PeerReviewStore.Save` (já atomic via
tmp+rename). Cada record é independente — uma
falha no record N não afeta N-1, N+1, etc.

Não há "transaction" global; se migrate for
interrompido (Ctrl-C), records já salvos ficam em
v2, restantes em v1. Re-rodar completa o trabalho.

## Não-objetivos

- Backup automático antes de migrate — operador
  decide se quer snapshot do store. ADR-0123+
  pode adicionar `migrate --backup` que copia
  `<store>/*.json` pra `<store>.backup-<ts>/`.
- Migrate em background / daemon — operator roda
  manualmente quando stats mostra V1Count alto.
- Schema downgrade (v2 → v1) — sem razão prática;
  clients antigos podem continuar lendo v2 (parser
  é tolerant).
- Validação estrutural do payload antes de migrate
  — SAI-120 já tinha validator que rodava em
  submission; aqui só confiamos no JSON.parse.
  Se parser passa, mantém.

## Consequências

- **Operator tem alavanca concreta**: rodar
  `peer-reviews stats` → ver V1Count → decidir
  cutoff → rodar `peer-reviews migrate` antes ou
  depois do cutoff.
- **Idempotência**: rodar `migrate` múltiplas
  vezes é safe. Records v2 ficam intocados.
- **Sem perda de dados no happy path**: re-parse
  Notes JSON preserva todos os campos canônicos.
- **Perda potencial em edge case**: Notes v1 que
  não parseia como JSON (corrompido, truncado) é
  pulado com warning. Operator vê contagem em
  `SkippedBad` e pode inspecionar manualmente.
- **ContentHash muda**: pós-migrate, `review_id`
  derivado de novos submits com o mesmo payload
  pode mudar (v1 hash ≠ v2 hash). Operador que
  indexa por review_id precisa reindexar — pequeno
  preço da evolução.

## Verificação

- `go build ./...` → 0 erros
- `mcpserver/migrate_test.go` 6/6 testes passam:
  - V1ToV2_Applied: schema bump + payload populado + hash recomputado
  - DryRun: schema/notes/canonical_payload intocados
  - SkipsAlreadyV2: records v2 não são tocados
  - SkipsBadJSON: Notes não-JSON pula
  - ActorFilter: filtro `peer_a` migra só peer_a
  - EmptyNotes: Notes vazio pula (sem payload)
- Smoke CLI: seed r1 (v1) + r2 (v2) em tmp dir;
  `migrate --dry-run` reporta "1 migrated" sem
  mudar disco; `migrate` (sem flag) muda r1 pra
  schema=2 com canonical_payload populado e
  content_hash recomputado; r2 intocado.
- Regression: 9/9 testes SAI-121 + 6/6 testes
  SAI-120 continuam passando.

## Próximo passo

1. SAI-123+: `migrate --backup` opcional — copia
   store pra timestamp dir antes de mutar.
2. SAI-124+: doctor check que alerta se V1Count > 0
   (com sugestão de rodar migrate).
3. Cleanup do JSONL `metrics_path` — SAI-121
   acumula; após cutoff vencido + migrate completo,
   considerar `peer-reviews gc` que apaga entries
   de records já deletados.
4. SAI-125+: schema "3" com detailed_findings por
   pilar SOLID (campos novos) — só após field usage
   confirmar demanda.
