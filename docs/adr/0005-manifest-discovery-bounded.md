# ADR 0005 — Manifest discovery bounded

## Status

Aceito. 2026-08-19.

## Contexto

SAI-011 pede localizar manifests sem walk integral de monorepo. Para
um repo com `apps/web/package.json`, `apps/api/go.mod`,
`packages/ui/package.json` e um root `package.json`, com changed paths
em `apps/api/cmd/main.go`, `packages/ui/src/Button.tsx` e
`docs/README.md`, o resultado correto são **4 manifests** — não
passar por `unrelated/` ou `baz/large-tree/`.

## Decisão

Dois caminhos limitados, ambos terminam cedo:

1. **Walk-up por changed path**: para cada `changedPath`, subir a
   cadeia `path.Dir` até achar manifest ou chegar em `""`. Custo
   proporcional à profundidade do path, não ao tamanho do repo.
2. **Scan por ManifestRoot**: cada raiz configurada é descida
   recursivamente (depth ≤ 8) pulando lixo conhecido
   (`node_modules`, `.git`, `vendor`, `target`, `build`, `dist`,
   `.next`, `.nuxt`, `.venv`, `venv`, `__pycache__`). Sem
   ManifestRoots, nada é descido.

`lookupManifests` devolve **todos** os manifests num diretório —
monorepos polyglot (`package.json` + `Cargo.toml` no root) precisam
disso.

`path.Dir("foo")` retorna `"."` — a normalização para `""` precisa
acontecer **a cada iteração** do walk-up, não só na entrada.

## Consequências

**Positivas:**

- 4 changed paths × ~3 níveis de walk-up = ~12 `os.Stat` por caminho
  alterado. Sem `find`/`fs.WalkDir` no monorepo inteiro.
- Comportamento determinístico: o conjunto de manifests depende só
  dos changed paths e ManifestRoots.
- Dedup por Path — mesma instância encontrada por dois changed
  paths vira 1 entry.

**Negativas:**

- Profundidade 8 é arbitrária. Monorepos muito profundos (>8 níveis
  sem o manifest aparecer) ficam sem coverage. Aceitável para o
  universo Go/Node/Python/Ruby/Java/PHP/Rust; Flutter/Dart pubspec
  raramente passa de 3.
- Hardcoded junk-dirs. Se um projeto usar `third-party/` ou
  `external/`, não pula. Configurável depois via config se virar
  problema.

## Alternativas consideradas

- **`filepath.Walk`/`fs.WalkDir`**: custo O(repo inteiro) — proibido
  pelo spec. Rejeitado.
- **Só ManifestRoots**: deixa de fora qualquer manifest fora das
  raízes configuradas, mesmo que ancestre de changed path. Rejeitado.
- **Só walk-up**: perde manifests em subtrees adjacentes
  (`apps/web/package.json` quando o changed path é em `apps/api`).
  Rejeitado.