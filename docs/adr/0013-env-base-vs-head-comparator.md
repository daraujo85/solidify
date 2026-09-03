# ADR 0013 — Env base-vs-head comparator

## Status

Aceito. 2026-08-20.

## Contexto

SAI-019 pede o comparador que recebe Usages do base e do head (do
SAI-018) e devolve o conjunto union de Diffs com states:

- added/removed/changed-use (mudança de uso)
- documented/missing-doc (rastreabilidade)
- required/optional (default inference)
- likely_secret (heurística de nome)

Não carrega valores (§11.1).

## Decisão

Função pura `Compare(base, head []Usage, documentedNames []string, secretChecker func(string) bool) []Diff`.

Algoritmo:

1. Indexa base e head por `Name`.
2. Union dos nomes (preserva adicionados e removidos).
3. Para cada nome:
   - `inBase`/`inHead` determinam o state primário (added/removed/
     changed-use/nenhum).
   - `sameUsage(b, h)` compara (Form, HasDefault) — paths não contam
     (refactor trivial não é "changed-use").
   - `documented` é true se nome ∈ `documentedNames`.
   - `HasDefault` é true se algum uso no head traz default.
   - `Required` = `inHead && !HasDefault`.
   - `LikelySecret` via `secretChecker` (nil-safe).

`DocumentedNames(src)` parseia .env-style e yaml-style:

- `NAME=value`, `export NAME=value` (shell)
- `NAME: value` (yaml/properties — exige VALOR após `:`)
- `# NAME=value` (comentário — ainda conta)

Exclui namespaces yaml sem valor (`app:` como parent).

`IsLikelySecret(name)` heurística case-insensitive para sufixos
comuns: KEY, SECRET, TOKEN, PASSWORD, PASS, ACCESS, PRIVATE,
CREDENTIAL, AUTH, CERT.

## Consequências

**Positivas:**

- Estados múltiplos por var (added + + documentation-missing é
  comum para vars novas).
- 27 testes cobrindo cada state, edge cases (nil checker, nil docs,
  sameUsage mudanças sutis).
- `DocumentedNames` robusto contra yaml parents (sem valor).
- `sameUsage` ignora paths — refactor de path não é "changed-use".

**Negativas:**

- "Changed-use" só detecta mudança de Form ou HasDefault — não
  detecta adição de novo call site (mais usos). Isso entra como
  HeadUses vs BaseUses count. Caller decide se threshold alto é
  problema.
- `DocumentedNames` exige valor explícito após `:` — secrets
  marcados como `# API_KEY=` (placeholder) contam como
  documentados (intencional).
- Heurística de likely_secret é ampla — gera falsos positivos
  (ex.: `PUBLIC_KEY` em crypto context). Caller pode refinar.

## Alternativas consideradas

- **State único por var** (não lista): caller perderia informação
  composta ("added E não documentado"). Rejeitado.
- **Detecção de "changed-use" via diff textual do source**: mais
  preciso mas caro. Para MVP, (Form, HasDefault) basta. Rejeitado
  para SAI-019; pode evoluir.
- **Lint config por projeto** (`required: [API_KEY]`): moveria
  inferência para config do repo. §11 prefere inferência a
  config. Rejeitado.