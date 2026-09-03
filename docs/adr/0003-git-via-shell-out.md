# 0003 — Acesso a Git via shell-out, não via go-git

- Status: aceito
- Data: 2026-08-19

## Contexto

O Solidify precisa resolver refs, listar arquivos alterados, ler diffs e
histórico de um repositório Git. Existem duas opções principais:

1. **Shell-out** para o binário `git` instalado no host.
2. **go-git** (`github.com/go-git/go-git/v5`), implementação pura em Go.

A spec é firme em "Go core, zero dependência externa no MVP" (ARCHITECTURE
§4). `go-git` é uma biblioteca pesada, com API instável entre versões e
sem cobertura 100% dos comandos CLI do git real (especialmente `--diff-filter`,
rename detection, worktrees).

## Decisão

Shell-out para `git`. Implementação isolada em `internal/gitx`.

- Cada chamada vira um comando `git <args>...` com `exec.Command` (sem shell).
- Argumentos são passados diretamente, sem interpolação — entradas do usuário
  (refs, paths) entram como argumentos, nunca como string de shell.
- Erros do git são embrulhados com `errs.CodeGit` e a saída stderr do git.
- `EnsureBinary()` roda no startup da CLI para falhar cedo se git não existir
  no PATH (mensagem acionável em vez de `executable file not found`).
- Testes sobem repos reais via `git init` em `t.TempDir()`: o único jeito
  fiel de testar o resolver é exercitar git de verdade.

## Consequências

- O binário `git` precisa estar no PATH em qualquer host que rode o Solidify.
  Em CI isso é trivial (a imagem base de qualquer runner tem git); em dev
  também (Xcode CLT, homebrew, apt). A verificação via `EnsureBinary` falha
  com `errs.CodeGit` e dica "instale git ou ajuste PATH".
- A Makefile troca o alvo `test`/`bench` da imagem alpine pra debian:
  alpine não traz git; debian sim. Build de produção continua no alpine
  (binário final é estático e menor).
- Performance: 1 fork+exec por operação git. Aceitável — análise é I/O bound
  e o overhead é < 5% em qualquer workload real. Se virar gargalo,
  cachear resultados por `(range, config_hash)`.

## Alternativas consideradas

- **go-git v5**: dep externa pesada, e várias features que vamos precisar
  (`merge-base`, `--diff-filter`, `log --first-parent`) têm bugs conhecidos.
  Rejeitada pela regra de zero deps no MVP.
- **libgit2 via cgo**: binário nativo maior, exige toolchain C em runtime.
  Rejeitada — custo de build e empacotamento sem benefício claro.
- **Bundlar git no binário**: ~30 MiB extras, freeze de versão, quebra UX.
  Rejeitada.