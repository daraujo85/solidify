# ADR 0125 — Dashboard widget: v1_fraction trend (SAI-125)

Status: Aceito. 2026-08-21.

## Contexto

SAI-121 (ADR-0121) deu `peer-reviews stats` agregado
point-in-time. SAI-123 (ADR-0123) deu canary check que
também é point-in-time (`peer_review_canary` lê
`SummarizeMetrics(events, from)`).

Pergunta concreta: **como operator acompanha o
rollout v2 ao longo do tempo** sem rodar `stats` todo
dia e diffar mentalmente?

Estado da arte (ver `internal/dashboard/` + `web/`):

- Mux do dashboard expõe 2 endpoints (`/api/runs`,
  `/api/run/<id>`). Zero `/api/peer-reviews/*`.
- `dashboard` não importa `mcpserver`/`doctor` —
  não conhece o JSONL de telemetria.
- `mcpserver.SummarizeMetrics(events, from)` aceita
  só `from`, sem `to`. Série temporal exige N chamadas
  ou parâmetro `to`.
- Repo é zero-deps JS (ADR-0074): sem chart lib,
  sem SVG. Sparkline = SVG inline.
- Volume atual do JSONL: ~1 evento/run. 1000 runs ≈
  80KB. Binning O(N) é trivial.

## Decisão

### 1. `mcpserver.ComputeTrend(events, days, now, threshold)`

Novo arquivo `internal/mcpserver/trend.go`:

```go
type TrendPoint struct {
    Date       string  `json:"date"`         // YYYY-MM-DD UTC
    V1Count    int     `json:"v1_count"`
    V2Count    int     `json:"v2_count"`
    Total      int     `json:"total"`
    V1Fraction float64 `json:"v1_fraction"`
}
type PeerReviewTrend struct {
    WindowDays int          `json:"window_days"`
    Threshold  float64      `json:"threshold"`
    Points     []TrendPoint `json:"points"`
}
func ComputeTrend(events []Metric, days int, now time.Time, threshold float64) PeerReviewTrend
```

Para cada dia D ∈ [now-days+1, now]:
`SummarizeMetrics(events, D-30d, D)`. Último ponto
usa `now` direto → bate com `peer_review_canary`.

`SummarizeMetrics` ganha 3º param `to time.Time`;
zero = sem teto (= `time.Now()`). Caller existente
(`peer_review_canary.go:55` + `peer_reviews.go:69`)
passa zero — backward-compat.

### 2. Endpoint `/api/peer-reviews/trend`

`internal/dashboard/dashboard.go`, antes do
catch-all `FileServer`:

```go
mux.HandleFunc("/api/peer-reviews/trend", func(w, r) {
    events, err := mcpserver.ReadMetrics()
    if err != nil { http.Error(w, err.Error(), 500); return }
    trend := mcpserver.ComputeTrend(events, 30, time.Time{}, doctor.CanaryFractionThreshold)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(trend)
})
```

`dashboard` ganha imports `mcpserver` + `doctor`.
Honra `mcpserver.MetricsPath` global (mesmo path
que canary tests + `peer-reviews stats`).

### 3. View JS + nav

`web/src/main.js`:

- `async function loadTrend()` → fetch endpoint.
- `viewTrend(data)` → `div.card.col` com SVG
  sparkline (viewBox 600×80) + tabela compacta dos
  últimos 14 dias.
- Sparkline: polyline conectando os 30 pontos, dot no
  último, threshold line tracejada em 5%, tooltips via
  `<title>` nativo. Sem JS animation, sem deps.
- Cores: linha `--pass` se último ponto ≤ threshold,
  `--fail` caso contrário.
- Registrada em `render()` na rota `#/peer-reviews`.

`web/public/index.html:13-19` adiciona
`<a href="#/peer-reviews">Peer reviews</a>` no `<nav>`.

`web/public/assets/app.css` adiciona
`.spark { width:100%; height:auto }`,
`.spark .threshold { stroke:var(--border);
stroke-dasharray:4 4 }` + estilos `td.pos/neg`.

### 4. Sem nova storage

Compute on-demand a cada request. YAGNI: volume
esperado é baixo (~80KB/1000 runs); cache em
memória seria prematura. Threshold = `doctor.
CanaryFractionThreshold` (mesma linha que canary
reporta) — UI e CLI sempre consistentes.

### 5. Naming

Função `ComputeTrend` (não `PeerReviewTrend`) porque
o **tipo** `PeerReviewTrend` também vive no package;
não dá pra ter função e tipo com mesmo nome.

## Não-objetivos

- Snapshots pré-computados / SQLite de séries —
  ADR-0121 §"não-objetivos".
- Drill-down por dia (lista de events) — fora de
  escopo. SAI-126+ se virar demanda real.
- Múltiplas janelas (7d, 90d) — 30d bate com canary.
- Threshold dinâmico por dia — usa
  `CanaryFractionThreshold` fixo.
- Caching HTTP — request é O(N) sobre JSONL,
  aceitável pro volume esperado.

## Consequências

- **Visibilidade temporal**: operator acompanha
  rollout v2 sem diff manual.
- **Mesmo número que canary**: último ponto do trend
  bate com `peer_review_canary` no momento da
  requisição. Threshold line consistente.
- **Zero storage extra**: aproveita o JSONL existente.
- **Zero deps JS**: SVG inline respeita ADR-0074.
- **Compute O(N×days)**: N=eventos, days=30. Em 1000
  events = 30000 ops por request. Aceitável.

## Verificação

- `go build ./...` → 0 erros
- `internal/mcpserver/trend_test.go` 5 testes:
  - `Empty` (sem events → 30 pontos zerados)
  - `DaysAreOldestFirst` (ordem cronológica)
  - `SingleBucketWithEvents` (eventos só em 1 dia)
  - `MixedV1V2` (frac ≈ 2/3 no último ponto)
  - `DefaultsDaysTo30` (days=0 → 30)
- `internal/dashboard/dashboard_test.go` 2 testes:
  - `TestHandlerPeerReviewsTrend` (200 + 30 pontos)
  - `TestHandlerPeerReviewsTrend_WithEvents`
    (último ponto com Total>0 após 12 events)
- Smoke: `curl /api/peer-reviews/trend` retorna 30
  pontos; com 8 events reais (3 v1 + 5 v2
  espalhados em 28 dias) → fração decresce
  monotonicamente de 1.0 (dia 25/jul) → 0.375 (dia
  21/ago, hoje).
- Regressão: `peer_review_canary` + `peer-reviews
  stats` continuam funcionando com novo signature
  3-arg de `SummarizeMetrics`.

## Próximo passo

1. SAI-126: schema "3" com `detailed_findings` por
   pilar SOLID — bump após field usage confirmar
   demanda (ver ADR-0121 §"próximo passo").
2. Widget drill-down (click num dia → lista de
   events) — fora de escopo SAI-125; vira ADR
   separada se virar demanda.