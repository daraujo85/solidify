# ADR 0019 — Sharding v1

Status: Aceito. 2026-08-20.

## Contexto

SAI-025: peer A/B executam análise paralela e precisam particionar
mudanças de forma reprodutível. Sem shards estáveis, A e B podem
analisar conjuntos diferentes e divergir.

## Decisão

`sharding.Cluster(changes []Change) []Shard` agrupa changes por
component. Cada cluster vira um `Shard` com:

- **ID**: SHA-256(component + sorted(paths) + content) truncado a 16 hex.
- **Hash**: SHA-256 hex completo.
- **Paths**: ordenados alfabeticamente.
- **Reasons**: tags de auditoria (ex: "component-cluster").

Content dentro de um cluster é concatenação ordenada (path asc) dos
conteúdos das changes. Peer A/B recebem mesmo content porque a ordem
é determinística.

`sharding.Merge` deduplica por ID — útil pra combinar shards de
múltiplos peers em B peer review.

## Consequências

- 22 testes (determinismo, sort, dedupe, peer A/B equality).
- SHA-256 garante estabilidade cross-machine.
- ID curto (16 chars) para legibilidade em logs.
- Sem dependência externa (stdlib `crypto/sha256`).
- Truncar hash a 16 chars tem colisão ~1 em 2^64 — aceitável pra
  identificador visual, hash completo fica no `Shard.Hash`.
