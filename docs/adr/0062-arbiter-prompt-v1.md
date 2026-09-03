# ADR 0062 — Arbiter prompt v1

Status: Aceito. 2026-08-20.

## Contexto

SAI-068: template neutro que recebe evidence + Peer A + Peer B +
divergence map. Inputs explícitos pra auditoria.

## Decisão

`internal/arbiter/prompt.go` com:

- `PromptVersion = "1"`.
- `DefaultPrompt` template neutro (não rotula Peer A como
  ground truth).
- 4 vars obrigatórias: `evidence`, `peer_a`, `peer_b`,
  `divergence`.
- `RequiredVars()` canônico.
- `ValidateVars` checa presença.
- `BuildPromptRaw` (load-time, sem validação de valores) +
  `BuildPrompt` (validate valores pra cada var usada no
  template).
- `ArbiterPrompt{Version, Hash, Content, Vars, VarsUsed,
CreatedAt}`.
- `Render()` substitui `{{var}}` em ordem sorted (determinístico).
- `RenderWithVars(ArbiterVars)` helper.
- `LoadPrompt("")` retorna DefaultPrompt via BuildPromptRaw.
- `extractUsedVars` dedup + sort, ignora malformados.
- `computeTemplateHash` SHA256[:8] (8 bytes hex).
- `HashMatches` helper.
- `FormatVars` debug com truncate 60 chars.

## Consequências

- 22 testes: version, default non-empty, required vars count/
set, validate OK/missing, build basic/empty/missing-used-var,
render basic/missing/nil, render-with-vars, extractUsedVars
dedup/sort/empty/malformed, hash determinístico/sensitivo,
HashMatches, placeholders, load default, format vars c/
truncate, render no-leak.
- Template neutro evita viés de ancoragem (Peer A não é
  truth por default).
- Render idempotente: 2 chamadas com mesmas vars produzem mesmo
  output.
- BuildPromptRaw/BuildPrompt split permite carregar template
  sem saber todas as vars — útil pra inspeção de schema.
- Vars obrigatórias espelham ADR 0050 (peer review) — mesmo
  conjunto expandido com `divergence`.

## Trade-offs

- Template hardcoded em código (não em `prompts/arbiter.md`).
  Trade-off: zero FS dep + versionado em Go. ADR seguinte
  pode externalizar com FS read opcional.
- Hash truncado pra 16 chars (8 bytes hex) — colisão improvável
  mas possível. Suficiente pra audit log, não pra cripto.
- `extractUsedVars` é best-effort (regex-like manual) —
  ignora placeholders malformados em vez de falhar.
- Sem persistência — prompt é construído on-demand a cada
  arbiter call.
