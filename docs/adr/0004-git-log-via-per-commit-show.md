# ADR 0004 — git log via per-commit show

## Status

Aceito. 2026-08-19.

## Contexto

SAI-009 pede metadados completos do range: SHA, autor/committer,
Conventional Commit (type/scope/breaking), issue keys.

Tentativa 1: `git log --format=%H%x1f%an%x1f%ae...%B%x1f` em uma
chamada só, parseando o output como 8N+1 tokens separados.

## Problema

O `git log --format` adiciona automaticamente `\n` ao final de cada
commit. Pior: ele **absorve o último separador antes do `\n`** — `\x00`,
`\x1f`, e sentinels multi-char como `<<<>>>` se comportam igual. Em
git 2.47.3 (docker) e 2.50.1 (host Apple Git), o output é sempre
"commits × (campos-1) + 1" tokens em vez de "commits × campos".

Fora isso, `git log` por padrão usa pager (`cat`/`less`), que trunca
linhas longas na largura do terminal — emails longos viram
`pratadigit..` com reticências, corrompendo o parse.

## Decisão

Em vez de lutar com o format string, aceitar O(N) e separar em duas
fases:

1. `git rev-list FIRST..HEAD` para listar SHAs (exclui FIRST, fazer
   prepend manual).
2. `git log -1 --format=%H%x1f%an...%B <SHA>` para cada SHA.

Comandos sempre precedidos de `--no-pager` para não cair no pager.

## Consequências

**Positivas:**

- Parser fica trivial: 7 separadores entre 8 campos, sem magia.
- Funciona igual em qualquer versão de git.
- Cada commit é independente — SHA inválido em uma posição não
  corrompe as outras.

**Negativas:**

- O(N) chamadas ao git. Para 100 commits ~5s. Aceitável para release
  reports; para gerar diff incremental faz diferença.
- A inclusão do commit FIRST depende de `git rev-list` retornar
  `^FIRST HEAD`, e a função `revList` prepend o FIRST manualmente
  baseado no parsing da string `rng`.

## Alternativas consideradas

- **Sentinel multi-char**: testado `<<<>>>END` — git 2.47 ainda absorve
  o último sentinel. Mesma classe de bug.
- **`-z` flag**: `git log -z` apenas põe NUL entre commits; o format
  dentro de cada commit segue igual, então mesma fragilidade.
- **go-git**: rejeitado no ADR 0003 (zero deps, git no PATH).
