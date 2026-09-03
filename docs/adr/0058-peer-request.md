# ADR 0058 — Peer B Request Builder

Status: Aceito. 2026-08-20.

## Contexto

SAI-064: Peer B recebe evidence shards equivalentes ao Peer A.
**Nunca** inclui output do Peer A (independência). Builder
deve garantir essa propriedade verificável.

## Decisão

`EvidenceShard{ID, Kind, Content, SourceFile, LineStart, LineEnd}`.

`EvidenceShardSet{RunID, Shards, TotalBytes}` imutável:
- `Hash()` SHA256(RunID || shard.ID || shard.Content) — sorted
- `IDs()` extrai IDs
- `FilterByKind(kind)` subset

`PeerRequest{RunID, Actor, Schema, Prompt, Evidence, Vars, Hash}`.

`RequestBuilder{promptTemplate, schemaVersion}`:
- valida RunID/Actor/Evidence required
- `vars["run_id|actor|evidence_hash|schema_version"]` populado
- extra vars via `opts.Vars`
- render template substituindo `{{name}}`
- `computeHash()` sorted vars iteration → SHA256

`HasPeerAOutput(peerAOutput)`:
- check `PEER_A_OUTPUT:` marker
- check primeiros 100 chars substring

`VerifyIndependence(peerAOutput)` retorna issues:
- prompt contém output do Peer A
- evidence nil/vazio
- run_id vazio

`FormatRequest(r)` debug output.

## Consequências

- 25 testes (new set, empty runid, hash stable/sensitive/nil,
  ids nil/ok, filter kind/nil, build basic/missing fields/hash/
  schema override/extra vars/deterministic, has peer A marker/
  substring/clean/nil, verify independence clean/empty evidence,
  format, marshal JSON, default schema version).
- Hash determinístico via sorted key iteration — caller pode
  reproduzir mesmo request byte-by-byte.
- `HasPeerAOutput` defense-in-depth: marker explícito
  `PEER_A_OUTPUT:` + heurística substring (100 chars).
- `VerifyIndependence` retorna lista de issues — caller decide
  se aborta ou apenas loga warning.

## Trade-offs

- Substring match (100 chars) tem falsos positivos/negativos.
  Trade-off conservador: marker explícito é canônico, substring
  é best-effort.
- Hash exclui `Prompt` raw — evita PII no log. Trade-off:
  auditabilidade de "qual prompt foi enviado" exige log
  separado.
- Vars iteration com insertion sort — O(n²) mas n pequeno
  (4-10 vars). Não vale `sort.Strings` do stdlib.
- Sem signing/encryption do request — confidencialidade é
  responsabilidade do transport (MCP stdio, HTTPS).
- SchemaVersion override por request — Peer A e Peer B podem
  usar schemas diferentes se necessário para A/B testing.
