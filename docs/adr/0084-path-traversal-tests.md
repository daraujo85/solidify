# ADR 0084 — Path traversal tests (SAI-101)

Status: Aceito. 2026-08-20.

## Contexto

SAI-101: MCP context tool não pode ler fora do repo
ou dos allowed roots. Defesa contra path traversal
(`../../`) e leitura fora de escopo.

## Decisão

`internal/pathguard/`:

- `Guard{RepoRoot, AllowedRoots}`.
- `NewGuard(repo, allowed)` constrói.
- `Validate(path)`:
  1. Empty → `ErrEmpty`.
  2. Contains `..` → `ErrTraversal`.
  3. `IsAbs` → `ErrAbsolute`.
  4. `filepath.Rel` confirma que está dentro do
     `RepoRoot`; fora → `ErrOutsideRepo`.
  5. Se `AllowedRoots` setado, prefix deve bater;
     senão → `ErrNotAllowed`.
- `SafeJoin(path)` valida + `filepath.Join` (p/ uso
  imediato).
- `IsSafe(path)` helper estático (só checa 1+2+3).
- Erros sentinel: `ErrEmpty`, `ErrTraversal`,
  `ErrAbsolute`, `ErrOutsideRepo`, `ErrNotAllowed`.

## Consequências

- 10 testes: empty, traversal (3 variants),
  absolute (2 variants), valid (3 variants),
  outside-repo via join, allowed-roots
  enforcement, no-allowed-roots, safe-join ok/err,
  IsSafe helper.
- Ordem de validação documentada — defesa em
  profundidade (cada check rejeita 1 ataque
  específico).
- Sem dependência de filesystem real — testes
  usam paths sintéticos.

## Trade-offs

- Path canonicalization via `filepath.Clean` —
  resolve `.` mas não `..`. Trade-off: simples >
  full resolution.
- Sem symlink resolution — symlink malicioso
  passando validate ainda é risco. Mitigação
  futura via `filepath.EvalSymlinks`.
- `IsSafe` é subconjunto de `Validate` — caller
  escolhe nível.
