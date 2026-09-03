# ADR 0015 — Artifact store filesystem

## Status

Aceito. 2026-08-20.

## Contexto

SAI-021 pede store de artefatos por run com:

- Run dir estável
- Atomic writes (sem arquivo parcial visível durante escrita)
- SHA-256 de cada artefato
- Manifest JSON para auditoria/reproducibilidade

§11.1: artefatos são determinísticos e auditáveis.

## Decisão

Pacote `internal/artifacts` com `Store` thread-safe.

**Layout:**

```
<baseDir>/runs/<run-id>/manifest.json
<baseDir>/runs/<run-id>/<artifact-name>
```

**Operações:**

- `New(baseDir, runID)` — runID vazio gera `YYYYMMDDTHHMMSS-<6hex>`.
- `Write(name, data)` / `WriteReader(name, io.Reader)` — atomic via
  temp + rename.
- `Read(name)`.
- `List()` — metadados (sem conteúdo).
- `SetMeta(key, value)` — proveniência (branch, commit, base, head).
- `FlushManifest()` / `SaveManifest()` — persiste `manifest.json`
  atomicamente.

**Atomic write:** arquivo `name.tmp-<random>` no mesmo diretório,
`WriteFile` permissions 0o644, `Rename` (atômico em POSIX por inode).

**SHA-256:** computado durante leitura do Reader (tee em `sha256.New`).
Bug potencial: Read parcial pode requerer retry — mas para o caso
comum (arquivo único em memória) sempre lê tudo.

**`filepath.IsLocal`** bloqueia path traversal (`../`, absoluto).
`name == "manifest.json"` é reservado — caller não pode usar.

**Manifest schema (v1):**

```json
{
  "run_id": "...",
  "created_at": "RFC3339",
  "schema_version": 1,
  "artifacts": [
    {"name": "...", "size": N, "sha256": "hex", "written": "RFC3339"}
  ],
  "meta": {"key": "value"}
}
```

Ordenado por `name` (reprodutibilidade).

## Consequências

**Positivas:**

- 22 testes (atomicidade, SHA-256, traversal, concorrência, manifest).
- Thread-safe (mutex global no Store). Concurrent writes somam N itens.
- Re-abertura via `FromDir` lê manifest existente.
- Sem dependências externas (apenas stdlib).
- Schema versionado permite evolução.

**Negativas:**

- SHA-256 computado em buffer — para arquivos >RAM não escala. SAI-021
  não tem caso de uso de big files; SAI-024 (budget) lida com
  truncamento upstream.
- `manifest.json` é um único arquivo — corrompê-lo quebra o Store.
  Mitigação: backup antes de `Rename` (TODO se virar problema).
- Path traversal via `filepath.IsLocal` (Go 1.20+) — coberto para
  `../`, absoluto, mas não para symlinks intencionalmente plantados
  no run dir. SAI-021 não escreve symlinks.

## Alternativas consideradas

- **SQLite (BLOB)**: tudo no DB. Coberto por SAI-022 (storage). Para
  artefatos binários puros, filesystem é mais simples. Rejeitado.
- **Content-addressed storage (sha256 = path)**: dedup automático.
  Complexidade: collisions, autorização. Rejeitado para SAI-021.
- **Append-only log**: bom para audit, ruim para Read/Ls. Rejeitado.
- **S3/cloud**: speach de cloud. §11 prefere local. Rejeitado.
