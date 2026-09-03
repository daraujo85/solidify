# ADR 0118 — Terminal/MCP Peer A (SAI-118)

Status: **Aceito**. 2026-08-21. Pré-requisitos
SAI-116/117 fechados. Smoke contractual executado.

> **Atualização SAI-119**: stub `peer_a_response.json`
> foi removido em favor do transporte MCP real via
> `PeerReviewStore` polling. Ver ADR-0119.
>
> **Atualização SAI-120**: stub legado foi totalmente
> removido. `executeTerminalMCP` agora confia 100% no
> `PeerReviewStore`. Quem dependia de
> `<artifact_dir>/peer_a_response.json` migra para
> `solidify_submit_peer_review` via MCP — setup em
> `solidify mcp setup --target claude-code`. Ver
> ADR-0120.

## Contexto

SAI-118: `mode: "single_peer_phase0"` no report porque
peer_a não rodava. peer_a é o "terminal MCP interativo"
— em tese o agente principal (sessão Claude Code/OpenCode)
age como revisor primário, não outro modelo HTTP.

Estado pré-SAI-118: peer_a skipped no orchestrator
(`run.go:189` literalmente `PeerAOutput: ""`).
Config `ai.peer_a.source: "terminal-mcp"` existia mas
nada a executava.

Pré-requisitos fechados:
- **SAI-116**: `CanonicalSchema` + sem número silencioso
  (schema falhou → `score_status=unavailable`, `grade=?`).
- **SAI-117**: independence gate real derivado dos atores
  efetivamente executados; subgates quality + independence.

## Decisão concreta (refinada pós 116/117)

### 1. Dispatcher de peer_a (`internal/peer/peer_a.go`)

Source enum com 3 valores, todos validados em
`config/validate.go`:

| Source | Comportamento | Quando usar |
|---|---|---|
| `terminal-mcp` | Stub: lê `<artifact_dir>/peer_a_response.json`; ausente → skipped. SAI-119 substitui por transporte MCP real (depende SAI-044 SDK choice). | Humano/IA escreve o JSON de review num arquivo; SAI-019 futura |
| `http-combo` | `peer.Executor` padrão com combo `solidai-peer-a` via 9Router. Mesma máquina (timeout/repair/score_status) do peer_b. | Smoke automatizado, CI, qualquer contexto não-interativo |
| `auto` | Resolve pra `http-combo` na Fase 1. Default. | Quando usuário não quer pensar nisso |

Validação rejeita qualquer outro valor com hint listando
os 4 válidos (`terminal-mcp`, `http-combo`, `external` legacy, `auto`).

`ResolveSource(string) Source` normaliza default desconhecido
pra `auto` (graceful degradation — usuário erra o nome,
run não quebra).

### 2. Config

`ai.selection.peer_a` adicionado (mesmo padrão de `peer_b`):
```json
{
  "ai": {
    "peer_a": {"source": "auto"},
    "selection": {
      "peer_a": {"preferred": ["solidai-peer-a"], "exclude": []}
    }
  }
}
```

`PeerA.Source` valores legacy preservados:
`"external"` vira alias de `"http-combo"` (backwards compat).

### 3. Wire no orchestrator (`run.go`)

Sequência após `peer_b done`:
1. `runPeerA(ctx, cfg, builder, ...)` — monta request,
   resolve modelo via `Selection.PeerA.Preferred`, dispara
   `peer.ExecutePeerA`.
2. `peer_a.Content` alimenta `arbiter.ExecutorOptions.PeerAOutput`.
3. `gate.IndependenceInput.PeerACalled` e `PeerAModel`
   derivados do resultado (não hardcoded mais).
4. `report.ai_review.mode` = `multi_peer_v1` se peer_a rodou,
   senão `single_peer_phase0`.
5. `report.ai_review.actors[]` ganha `peer_a` actor com
   status correto: `ok` / `schema_failed` / `error` / `skipped`
   (era hardcoded `"ok"` pra peer_b — corrigido).

### 4. Independence gate pós-SAI-118

Smoke contractual (`profile=contractual`):

**SAI-117b (pré):**
```json
{
  "peer_a_called": false,
  "distinct_models": 1,
  "required": ["peer_a", "distinct_external_models>=2"]
}
```

**SAI-118 (pós):**
```json
{
  "peer_a_called": true,
  "peer_b_called": true,
  "arbiter_called": false,
  "distinct_models": 2,
  "required": ["arbiter"]
}
```

peer_a + distinct OK. Restante: arbiter (não relacionado
a SAI-118 — bug pré-existente HTTPClient.Timeout=60s em
`openai.go` revela quando pipeline cresce).

## Não-objetivos (neste ADR — diferidos)

- **MCP transport real** (SAI-119): SDK choice
  (depende SAI-044). Stub atual lê arquivo; transporte
  oficial substitui.
- **Auto-aprovação** (peer_a sem humano): out of scope.
  peer_a SEMPRE requer sessão ativa (humana ou stub
  que escreva o arquivo).
- **Múltiplos peer_a paralelos**: 1 terminal MCP por run.
- **Distinct via hash de provider+model**: hoje é
  string-equality (`PeerAModel != PeerBModel`). Suficiente
  pra combos 9Router; futuro pode exigir hash.

## Consequências

- `mode` sobe de `single_peer_phase0` pra `multi_peer_v1`
  quando peer_a rodou.
- `ai_review.actors[]` agora reflete status real de cada
  ator (corrige bug onde peer_b ficava `"ok"` mesmo com
  schema_failed).
- Run latency cresce ~30-60s (peer_a sequential após peer_b).
- Stub `terminal-mcp` permite testes futuros sem provider
  HTTP configurado.
- HTTPClient.Timeout bug fica exposto; ADR-0119 deve
  cobrir.

## Próximo passo

- ADR 0119: MCP transport real pra `terminal-mcp`
  (depende SAI-044 SDK choice).
- Bug separado: HTTPClient.Timeout em openai.go
  precisa crescer (peer_b+peer_a sequential estoura 60s).
