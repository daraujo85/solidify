# ADR 0117 — Real Independence Gate (SAI-117)

Status: Aceito. 2026-08-20. `IndependenceRule`/`Subgates` implementados
e testados em `internal/gate/gate.go` (SAI-118). **Correção 2026-09-01**:
status anterior dizia "implementado em run.go" — falso. `internal/app/run.go:runRun`
era stub Fase 0 (sempre retornava erro de quota fake) e NÃO chamava `gate.Evaluate`
nem `report.NewBuilder`. Nenhum caller de produção usava `IndependenceInput` —
só `gate_test.go`.

**SAI-127 (2026-09-01) ✅**: wiring real feito. `runRun` agora orquestra
peer_a → peer_b → arbiter → `gate.Evaluate` → `report.NewBuilder` de
verdade (`internal/app/run.go`), populando `GateInput.Independence` a
partir dos atores efetivamente executados (não mais hardcoded). Smoke
test em `internal/app/run_test.go`.

**SAI-128 (2026-09-01) ✅**: teste e2e do SAI-127 contra 9Router real
(combos `solidai-peer-a/b/arbiter`) revelou que o Arbiter falhava
sempre que havia divergência real entre peer_a/peer_b — o único caso
em que ele é chamado pra valer. Causa: `internal/arbiter/executor.go`
montava um prompt inline sem shape de JSON, e `arbiterSchema` era
dead code (`{"type":"object"}`, nunca passado ao provider). Corrigido:
`Execute()` agora usa o motor de prompt já existente (SAI-068,
`LoadPrompt`/`RenderWithVars` em `prompt.go`, que já documentava o
shape mas nunca era chamado), e `arbiterSchema` ganhou o shape real
(`verdict`+`resolutions` obrigatórios, espelhando `buildVerdict`),
passado nas 2 chamadas a `CompleteJSON` (principal + repair).
Validado e2e: `--profile release` com diff real gera `ai_review.actors`
com arbiter `status: available` e divergência efetivamente arbitrada.
Pendente (SAI-129+): `ai.Selector`/`DistinctnessPolicy` real (hoje lê
só `Selection.*.Preferred[0]`, sem probing de distinctness).

## Contexto

SAI-117: hoje `internal/app/run.go:259` seta
`IndependenceOK: true` hardcoded, independente do
perfil. O report exibe `independence_degraded: false`
mesmo quando `mode = "single_peer_phase0"` e
`peer_a` não rodou. Config `require_distinct_external_models=true`
(contractual) é ignorada.

Bug semântico grave: o report promete "2+ peers
distintos" e o gate não checa. Em contractual isso
é a principal garantia do perfil — sem ela, o nome
é marketing.

## Decisão

### 1. IndependenceRule computado dos atores

`internal/gate/gate.go` ganha `IndependenceRule`:

```go
type IndependenceInput struct {
    PeerACalled       bool     // peer_a executou (não skipped)
    PeerBCalled       bool
    ArbiterCalled     bool
    PeerAModel        string   // "" se skipped
    PeerBModel        string
    RequirePeerA            bool   // profile config
    RequirePeerB            bool
    RequireArbiter          bool
    RequireDistinctModels   bool   // contractual
}

distinct := unique([peerAModel, peerBModel] filtrando "")
independent :=
    (!requirePeerA || peerACalled) &&
    (!requirePeerB || peerBCalled) &&
    (!requireArbiter || arbiterCalled) &&
    (!requireDistinctModels || len(distinct) >= 2)
```

### 2. Gate split em 3 níveis

`gate.GateResult` ganha `Subgates`:

```go
type GateResult struct {
    Status      GateStatus  // PASS|WARN|FAIL|INCOMPLETE
    Reason      string
    Subgates    Subgates
    IsBlocking  bool
}

type Subgates struct {
    Quality      SubgateResult  // score vs threshold
    Independence SubgateResult  // distinct + required actors
}
```

Regras de bloqueio:
- `final_gate = FAIL` se qualquer subgate blocking
  falhar
- `final_gate = WARN` se nenhum blocking falha mas
  algum subgate degrada
- `final_gate = PASS` se tudo passa

### 3. Report mostra 3 gates

`release-report.json` ganha:

```json
"quality_gate":      {"status": "PASS",  "rule": "G1"},
"independence_gate": {"status": "FAIL",  "rule": "G2", "reason": "peer_a not run"},
"final_gate":        {"status": "FAIL",  "rules": ["G1", "G2"]}
```

Auditável: reviewer vê QUAL foi o gate que falhou
e POR QUÊ. Não mais "score abaixo do threshold"
quando o real problema é peer_a skipped.

### 4. Orchestrator wire

`internal/app/run.go` para de hardcodar. Track
efetivo de quem rodou:

```go
peerACalled := cfg.AI.PeerA.Source != "" &&
    !strings.Contains(peerAStatus, "skipped")
peerBCalled := peerRes != nil
arbiterCalled := arbiterRes != nil && arbiterRes.Verdict != nil
```

Passa para `gate.IndependenceInput`.

## Consequências

- Report reflete realidade, não aspiração.
- `contractual` falha explicitamente em
  `single_peer_phase0` (que é Fase 0, esperado).
- Comparação entre perfis fica honesta: quick
  com 1 peer tem `independence_gate: WARN`,
  contractual com 1 peer tem `independence_gate: FAIL`.
- Caller pode escolher rodar `--profile quick`
  sabendo que独立性 é WARN, não FAIL blocking.

## Trade-offs

- Report schema cresce (3 gates em vez de 1).
  Mitigação: schema versionado, `quality_gate`
  preservado para retro-compat.
- Mais campos no `GateInput` = mais fácil errar.
  Mitigação: struct com `Validate()` no construtor.
- Não checa `model family` (ex: 2 modelos do mesmo
  provedor são "independentes"?). Fase 1: só
  `distinct ID`. Fase 2: heurística por provider
  family se necessário.

## Não-objetivos

- Detectar collusion entre modelos via embedding
  similarity. Fora de escopo (Fase 3+).
- Garantir que peer_a e peer_b usaram prompts
  diferentes. Out of scope pra este ADR.
