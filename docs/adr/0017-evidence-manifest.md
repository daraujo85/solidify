# ADR 0017 — Evidence manifest

## Status

Aceito. 2026-08-20.

## Contexto

SAI-023: manifest de evidências para a LLM/auditoria. Refs para:

- Git (base/head SHA, refs, remote, commit count)
- Changes (files, insertions, deletions, by-language)
- Components (nome, framework, language, paths)
- Migrations (file, framework, operations, risk, findings)
- Env (used/documented/added/removed/secrets)
- Limitações (escopo do analyzer)

## Decisão

Estrutura `Evidence` com 8 sub-blocos. Schema versionado ("1").

API builder:
- `New(runID)` cria zerado
- `SetGit`, `SetChanges`, `SetEnv` para blocos
- `AddComponent`, `AddMigration`, `AddLimit` para itens
- `SortedComponents`, `SortedMigrations` para ordem estável
- `Marshal()` JSON

Slices são inicializados para slice vazio (não nil) para serialização
previsível. `uniqSorted` dedupe + sort para reprodutibilidade.

## Consequências

**Positivas:**

- 16 testes (sorted, dedupe, marshaling, init).
- Schema versionado permite evolução.
- Ordem determinística (sort por nome/file) → reproducibility.
- Sem dependências externas.

**Negativas:**

- Schema inline — quando passar v1, precisa migration.
- `Operations` usa `migrations.OperationType` — coupling entre
  pacotes. Aceitável (evidence é camada de agregação).
- Não há agregação de findings (caller computa summary).

## Alternativas consideradas

- **Schema dinâmico (map[string]any)**: flexibilidade total, perda
  de type safety. Rejeitado.
- **YAML**: melhor legibilidade, pior tooling que JSON. Rejeitado.
- **Protobuf**: mais complexo, schema idem. JSON é suficiente. Rejeitado.
- **Múltiplos arquivos (1 por bloco)**: complica auditoria. Rejeitado.
