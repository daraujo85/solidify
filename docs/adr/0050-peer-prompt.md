# ADR 0050 — Peer Prompt v1

Status: Aceito. 2026-08-20.

## Contexto

SAI-056: Prompt Peer v1 precisa carregar `prompts/peer-review.md`,
aplicar placeholders estruturados e expor hash/version pra
rastreabilidade. Sem dependência externa — stdlib.

## Decisão

`PeerPrompt{Version, Hash, Vars, Content, VarsUsed}` em
`internal/mcpserver/prompt.go`.

`PeerReviewPromptVersion = "1"`.

`DefaultRequiredVars()` retorna `["run_id", "actor",
"evidence_hash", "schema_version"]` — vars mínimas que o template
consome; faltando qualquer uma → error com lista.

`LoadPrompt(opts)` lê do filesystem (path opcional, default
`prompts/peer-review.md`). `LoadPromptFromContent(content, vars)`
testa sem filesystem.

`renderPrompt(content, vars)`:
- empty content → error
- percorre required vars: missing ou empty → collected
- depois vars extras (não-required)
- aplica `strings.ReplaceAll` para `{{name}}`
- se missing > 0 → error formatado
- senão sha256 do rendered → hash

`ValidateVars(vars) []string` standalone check.

`FormatVars(p)` render para debug.

`HashMatches(a, b)` compara hashes.

`PlaceholdersForRequired()` retorna `{{name}}` markers.

## Consequências

- 19 testes (versão, required vars count, placeholders, render,
  missing, empty, render, vars extras, hash stable, hash sensitive,
  HashMatches, ValidateVars, fs load, missing path, default path,
  FormatVars, empty content, VarsUsed populated, file exists).
- Hash estável: mesmas vars → mesmo hash (idempotência de review).
- Hash sensível: mudança em var ou template → hash diferente.
- Vars extras além de required permitidos — caller pode injetar
  contexto ad-hoc sem mudar schema.

## Trade-offs

- Vars não-required substituídas mas não validadas — caller pode
  mandar qualquer coisa. Aceitável: prompt é texto, validação
  semântica no schema do Peer Review separado (ADR 0049).
- Required vars fixas (não extensíveis sem fork) — ADR seguinte
  pode carregar required vars do schema.
- `LoadPrompt` sem cache — cada call relê disco. Caller pode
  cachear se quiser. Spec não exige cache built-in.
- Vars duplicadas (mesmo nome em required + extras) — required
  ganha, extras ignorados. Verificado por `containsStr` check.
