# ADR 0123 — Doctor canary: peer review v1 (SAI-123)

Status: Aceito. 2026-08-21.

## Contexto

SAI-121 deu a ferramenta de decisão (`peer-reviews
stats` mostra V1Count real) + cutoff config.
SAI-122 deu a alavanca de cleanup (`peer-reviews
migrate`). Mas operador precisa **lembrar** de rodar
stats periodicamente e interpretar v1_fraction.

Pergunta concreta: **como transformar "v1 morrendo"
em ação automática?**

Dois checks canários:

1. `peer_review_canary` — observa v1_fraction na
   janela de 30 dias. Sugere cutoff quando v1 está
   quase extinto.
2. `peer_review_store_v1` — conta records v1 no
   store. Sugere migrate quando > 0.

Operator roda `solidify doctor` no CI ou pré-deploy
e vê o estado de rollout v2 sem precisar lembrar de
nada.

## Decisão

### 1. Check `peer_review_canary`

`internal/doctor/peer_review_canary.go::checkPeerReviewCanary`
lê JSONL via `mcpserver.ReadMetrics()` e computa
`SummarizeMetrics(events, time.Now() - CanaryWindow)`.

Thresholds (vars package-level, ajustáveis):

| Var | Default | Significado |
|---|---|---|
| `CanaryWindow` | 30 dias | janela de observação |
| `CanaryFractionThreshold` | 5% | abaixo disso, v1 "morrendo" |
| `CanaryMinEvents` | 10 | mínimo pra sugestão ser confiável |

Lógica:

```
events = ReadMetrics()
if len(events) == 0               → OK ("sem submissões")
elif events_in_window < 10        → OK ("dados insuficientes")
elif v1_fraction < 5%             → OK ("considere setar v1_cutoff=<+30d>")
else                              → FAIL ("rollout v2 pendente")
```

Mensagem do OK inclui `ai.peer_review.v1_cutoff=<RFC3339>`
calculado como `now + 30 dias` em UTC — operador
copia/cola direto no config.

### 2. Check `peer_review_store_v1`

`checkPeerReviewStoreV1` resolve `PeerReviewStore`
dir (env override `SOLIDIFY_PEER_REVIEW_STORE_DIR`
pra testes; default `~/.solidify/peer_reviews/`)
e conta records onde `Schema == "1"`.

```
v1_count = sum(records where Schema == "1")
if store não existe                → OK ("store vazio")
elif v1_count == 0                  → OK ("clean")
else                                → FAIL ("rode peer-reviews migrate --dry-run")
```

Mensagem inclui count exato + comando. Operator
sabe quantos records pendentes e o que rodar.

### 3. Thresholds ajustáveis

`CanaryWindow`, `CanaryFractionThreshold`,
`CanaryMinEvents` ficam como vars no package
`doctor` — testes setam valores diferentes pra
edge cases (ex: testar threshold alto), produção
usa defaults. Sem necessidade de flag CLI — quem
quiser customizar seta no código (raríssimo).

### 4. Não-failures

`peer_review_canary` retorna FAIL **só** quando
v1_fraction >= threshold **e** >= CanaryMinEvents
events. Sem dados = OK silencioso (não quebra CI).
Filosofia: doctor fail = bloqueante; OK + mensagem
informativa = sinal mas não bloqueia.

## Não-objetivos

- Auto-aplicar cutoff ou auto-rodar migrate — doctor
  reporta, humano decide. SAI-124+ pode adicionar
  `--apply` opcional em comando separado.
- Métricas externas (Prometheus push, OTLP) — fica
  fora do escopo; ADR-0121 já cobre telemetria local.
- Threshold configurável via CLI/env — vars de
  package bastam. Customização real é mudança de
  política (vai pra ADR nova).
- Histórico de "v1_fraction ao longo do tempo" —
  só point-in-time. Tendência fica pra dashboard
  futuro.

## Consequências

- **Operator awareness sem esforço**: `solidify
  doctor` no CI mostra estado de rollout v2.
- **Threshold conservador**: 5% evita ações
  prematuras (10 clientes v1 ainda é 5% em 200).
- **Mensagens acionáveis**: OK com cutoff sugerido
  + FAIL com comando. Operator não precisa pensar
  o que fazer.
- **Sem dados = OK**: zero submissões não bloqueia
  ninguém. Fresh install roda doctor limpo.
- **Dual check**: canary (telemetria) + store
  (records) cobrem ângulos diferentes — telemetria
  mede tráfego recente, store mede backlog.

## Verificação

- `go build ./...` → 0 erros
- `internal/doctor/peer_review_canary_test.go` 8/8:
  - Canary_NoMetrics (OK sem dados)
  - Canary_TooFewEvents (OK < 10 events)
  - Canary_LowV1Fraction (OK + sugestão cutoff)
  - Canary_HighV1Fraction (FAIL rollout pendente)
  - Canary_OldEventsIgnored (eventos > 30d não contam)
  - StoreV1_Empty (OK sem store)
  - StoreV1_HasV1Records (FAIL + sugere migrate)
  - StoreV1_OnlyV2 (OK clean)
- Regression: 6/6 migrate + 9/5 SAI-121 + 6/6 SAI-120
- Smoke CLI:
  - `doctor --only=peer_review_store_v1` com v1
    record → FAIL "1 records v1 no store — rode
    `solidify peer-reviews migrate --dry-run`"
  - `doctor --only=peer_review_canary` sem
    metrics → OK "sem submissões registradas"

## Próximo passo

1. ~~SAI-124: `solidify doctor` ganha flag `--canary`
   que roda só os checks v1 (atalho).~~ ✅ Ver ADR-0124.
2. ~~SAI-125: dashboard widget mostra v1_fraction
   trend over time.~~ ✅ Ver ADR-0125.
3. SAI-126: schema "3" com detailed_findings
   por pilar SOLID — bump após field usage
   confirmar demanda.
