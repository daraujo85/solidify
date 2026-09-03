# ADR 0023 — Cache engine

Status: Aceito. 2026-08-20.

## Contexto

SAI-029: analyzer results são reprodutíveis se inputs são iguais.
Re-rodar lighthouse em cada commit quando só `internal/api/auth.go`
mudou desperdiça minutos. Cache key determinístico permite skip
inteligente.

## Decisão

`cache.Key` com 6 componentes:
- `Analyzer` (nome)
- `Version` (versão do analyzer)
- `ConfigHash` (SHA-256 da config do analyzer)
- `Base` (git base SHA)
- `Head` (git head SHA)
- `InputHash` (SHA-256 do input — diff, shard, etc)
- `Tags` (extras como stack/lang, sorted)

`Hash()` produz SHA-256 canônico (json manual em ordem fixa, tags
sorted). Storage: filesystem, sharded em 2 níveis (`<dir>/<a>/<b>/<ab...json>`).

Atomic write: temp + rename. Entry metadata em JSON inclui key,
created, expires (se TTL), payload, analyzer, version, inputHash.

TTL opcional via `Config.TTL`. Entry expirada é removida no Get.
`InvalidateByTag` remove por nome de analyzer (ex: invalida
"lighthouse" após bump de versão).

## Consequências

- 26 testes (determinismo, mudança de campo, TTL, sharding,
  overwrite, binary payload, tag order).
- Path sharding evita milhares de entries em um único dir (ext4
  degrada >10k inodes no mesmo dir).
- Mutex protege map de paths + filesystem ops; múltiplos readers OK.
- Cache hit é O(filesystem read + JSON unmarshal); miss é O(SHA-256
  calc).
- Sem dependência externa (stdlib).
