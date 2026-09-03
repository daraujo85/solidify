# ADR 0045 — MCP server (begin_review + submit + evidence)

Status: Aceito. 2026-08-20.

## Contexto

SAI-051/052/053: server MCP Solidify com tools
`solidify_begin_review`, `solidify_submit_peer_review`,
`solidify_evidence_get`. Usa SDK MCP v1.0.0 (ADR 0044) com
transporte stdio.

## Decisão

`Server` wrapper thin sobre `*mcp.Server` do SDK.
`New()` cria com identity Solidify. `Run(ctx)` roda em
`&mcp.StdioTransport{}`.

`AddTool` SDK v1.0.0:
```
mcp.AddTool(s, *Tool, func(ctx, *CallToolRequest, In) (*CallToolResult, Out, error))
```
genérico In/Out. Wrapper usa In=`map[string]any`,
Out=`map[string]any` (zero). Faz JSON round-trip manual
pra inputs/outputs tipados.

3 tools expostos:
1. `solidify_begin_review` — RunID/CreateAt/Reused/Schema/
   AnalyzerIDs/HasEvidence/Limits/Instructions. Recebe
   `RunID?`, `BaseRef?`, `HeadRef?`, `RepoPath?`, `Resume`.
2. `solidify_submit_peer_review` — RunID/Actor/Schema/
   EvidenceHash/Verdict/Findings/Notes. Retorna
   Accepted/ReviewID/SavedAt/ValidationOK/Errors.
3. `solidify_evidence_get` — RunID/Kind/Path/Offset/Limit/Query.
   Kind: manifest|diff|context|symbols|analyzer|release.
   Retorna Items/Total/Truncated/NextOffset/Budget.

`mustJSON(v)` helper de serialização.

## Consequências

- 8 testes (New, BeginReview defaults, Register handlers
  não-panic, Submit input, mustJSON, RegisterSubmit, Run
  smoke).
- API consistente entre tools: mesma shape
  `(ctx, *XxxInput) (*XxxOutput, error)`.
- `Register*` desacopla SDK type — caller passa handler limpo.
- Tools listadas todas de uma vez via SAI-054 setup command.

## Trade-offs

- `map[string]any` em vez de typed generics — JSON round-trip
  extra (Marshal+Unmarshal) mas trivial; tipagem forte fica
  nos Input/Output structs.
- Output final é `mcp.CallToolResult` com `TextContent` JSON;
  SDK v1.0.0 também permite structured Content — diferido.
- `Register*` acumula no mesmo `*mcp.Server`; pattern
  Document/Discover do MCP lida com name conflicts (v1.0+
  tem DuplicateNoPanic test).
