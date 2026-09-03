# ADR 0071 — Canonical report builder

Status: Aceito. 2026-08-20.

## Contexto

SAI-077: montar report canônico sem UI. Estrutura JSON
que alimenta dashboard (Fase 18), CI artifacts e
integração downstream.

## Decisão

`internal/report/builder.go`:

- `SchemaVersion = "1.0.0"` — bate com schema atual.
- `Report{...}` cobre 16 blocos: run, git, components,
  release_notes, migrations, env_changes, analyzers,
  solid, ai_review, scores, risk, quality_gate,
  recommendations, limitations, artifacts.
- Campos JSON aninhados (snake_case) — match com schema.
- Ponteiros (`*float64`, `*string`, `*int`, `*bool`)
  para opcionais — `omitempty` no JSON.
- `BuilderInput` agregador plano; `Builder.Build()`
  normaliza slices nil → `[]T{}`.
- `HashContent()` SHA-256 do JSON canônico (chaves
  ordenadas recursivamente).
- `MarshalJSON()` força `SchemaVersion` constante
  antes de serializar.
- `marshalCanonical` recursivo: ordena keys de
  `map[string]any`, mantém ordem de slices.

`marshalCanonical` é genérico (trabalha em `any`) —
não depende do tipo `Report`. Permite reuso em
qualquer agregado determinístico.

## Consequências

- 12 testes: build basic, empty runid/profile, default
  slices, hash determinístico, hash nil, marshal
  schema version, roundtrip, build com conteúdo,
  canonical keys, array, nested.
- Validador (SAI-076) + builder (SAI-077) cobrem
  ciclo produzir → validar.
- Hash determinístico viabiliza immutability
  check (SAI-078).
- Tipos ponteiro-opcional modelam "ausente" sem
  `null` explícito vs `0` — clareza na leitura.

## Trade-offs

- `BuilderInput` plano (não builders fluentes) — fácil
  de testar mas verbose pra construção real. CLI/UI
  podem usar helpers.
- `Extra map[string]any` marcado `json:"-"` —
  reservado p/ extensões futuras sem quebrar schema.
- Canonical marshal não escapa unicode igual
  `encoding/json` — diferença ignorável p/ SHA-256.
  Trade-off: simplicidade > byte-identical com JSON
  padrão.
- Sem validação cruzada (ex: hash != schema) aqui —
  fica p/ pipeline SAI-078 (immutability/hash).
- Schema version hardcoded — bump manual quando
  schema mudar. ADR pode adicionar versionamento
  semântico.