# ADR 0121 — Telemetria v1 vs v2 + cutoff (SAI-121)

Status: Aceito. 2026-08-21.

## Contexto

SAI-120 (ADR-0120) bumpou schema "1"→"2" e adicionou
campo `CanonicalPayload`. v1 ficou deprecated
(warning-only) por enquanto — depende de telemetria
pra decidir quando endurecer.

Pergunta concreta: **quando v1 vira hard fail?**

Sem dados, decisão arbitrária. SAI-121 adiciona:

1. Telemetria append-only de submissions (JSONL).
2. `solidify peer-reviews stats` — agregado v1/v2.
3. Cutoff config (`ai.peer_review.v1_cutoff` ou
   flag `--v1-cutoff`) — após data, v1 = hard fail.
4. SAI-122+: cleanup migrate quando telemetria
   confirmar queda pra zero.

## Decisão

### 1. JSONL append-only

Storage: `~/.solidify/metrics/peer_reviews.jsonl`
(override: `ai.peer_review.metrics_path` ou flag
`--metrics-path`). Append-only = atômico via
`os.OpenFile(..., O_APPEND|O_CREATE, 0644)`. Sem
rotação de log (volume baixo: ~1 evento/run).

Schema do evento:

```json
{"ts":"2026-08-21T10:00:00Z","schema":"2","actor":"peer_a","review_id":"abc...","verdict":"comment"}
```

Campos grep-friendly (`ts`, `schema`, `actor`,
`review_id`) — operator pode `grep '"schema":"1"'`
pra ver quem ainda usa v1.

`DefaultSubmitPeerReviewHandler` chama `AppendMetric`
depois de `store.Save` com sucesso. Best-effort:
falha no append não bloqueia save (log silencioso).

### 2. `solidify peer-reviews stats`

Subcommand novo em `internal/app/peer_reviews.go`.

```
solidify peer-reviews stats [--since RFC3339] [--metrics-path PATH] [--json]
```

Output (default human):

```
Total:       142
V1 count:    8 (5.6%)
V2 count:    134
Oldest:      2026-07-15T08:00:00Z
Newest:      2026-08-21T11:55:00Z
V1 cutoff:   (não configurado; v1 sempre warning)

By schema:
  2   134
  1   8

By actor:
  peer_a   70
  peer_b   72
```

`--json` emite `MetricsSummary` estruturado pra
scripts/dashboards. `--since` filtra >= RFC3339.

### 3. V1 cutoff config

`ai.peer_review.v1_cutoff` (RFC3339 string) ou flag
`--v1-cutoff` no `solidify mcp serve`. Default vazio
= sem cutoff.

```go
func v1AfterCutoff(schema string) bool {
    if schema != "1" { return false }
    if V1Cutoff.IsZero() { return false }
    return time.Now().After(V1Cutoff)
}
```

Antes do cutoff: v1 = warning (igual SAI-120).
Depois: v1 = hard fail (`accepted: false`,
`errors: ["schema v1 rejected: cutoff date passed; use canonical_payload (schema v2)"]`).

Cutoff é **package-level** (`V1Cutoff time.Time`)
porque validator é instanciado antes do config em
alguns paths (testes isolados). `SetV1Cutoff(s)`
parseia RFC3339; vazio reseta.

### 4. Stats reporta cutoff status

Quando cutoff está setado, stats mostra data e
avisa se está vencido + v1_count > 0:

```
V1 cutoff:   2026-09-01T00:00:00Z
AVISO: cutoff venceu; novos submissions v1 = hard fail
```

Operator sabe rapidamente se precisa agir
(rollout v2 em cliente pendente).

## Não-objetivos

- Telemetria via Prometheus/OTel — JSONL grep-friendly
  basta por ora. SAI-123+ se virem necessidade real.
- Auto-cutoff baseado em heurística (ex: "se v1 < 5%
  por 30 dias"). Decisão fica com operador.
- Cleanup automático de records v1 antigos — SAI-122.
- Hard fail imediato (sem warning period) — warning
  period é importante pra rollout.

## Consequências

- **Decisão data-driven**: operador roda `stats`,
  vê quantos ainda usam v1, decide cutoff.
- **Rollout controlado**: warning period dá tempo
  pra clientes migrarem antes do hard fail.
- **JSONL auditável**: append-only = simples de
  auditar (chain-of-custody). Sem rotação = sem
  surpresa de evento "sumido".
- **Operator visibility**: stats sempre disponível
  sem restart do MCP server.
- **Cutoff reversível**: zerar `v1_cutoff` ou setar
  data futura reverte pra warning-only.

## Verificação

- `go build ./...` → 0 erros
- `mcpserver/metrics_test.go` 9/9 testes passam:
  - AppendMetric_AndRead (round-trip)
  - ReadMetrics_MissingFile (vazio = nil)
  - ReadMetrics_SkipsCorrupt (best-effort)
  - SummarizeMetrics (agregação)
  - SummarizeMetrics_SinceFilter
  - V1Cutoff_ZeroMeansNoCutoff
  - V1Cutoff_AfterDate_Rejects
  - SetV1Cutoff_Parse (RFC3339 + reset + erro)
  - MetricJSONShape (grep-friendly fields)
- Smoke CLI: `solidify peer-reviews stats
  --metrics-path=/tmp/m.jsonl` mostra tabela
  correta (3 events, v1=1, v2=2)
- Regression: SAI-120 tests continuam passando
  (v2 PollFindsRecord, v1 Fallback compat)

## Próximo passo

1. ~~SAI-122: `solidify peer-reviews migrate` —
   converte records v1 → v2 in-place.~~ ✅ Ver ADR-0122.
2. ~~SAI-123: telemetria canário — doctor sugere
   cutoff quando v1_fraction < 5% por 30d.~~ ✅
   Ver ADR-0123.
3. ~~SAI-125: dashboard widget mostra v1_fraction
   trend over time.~~ ✅ Ver ADR-0125.
4. SAI-126: schema "3" com `detailed_findings` por
   pilar SOLID — bump após field usage confirmar
   demanda.
