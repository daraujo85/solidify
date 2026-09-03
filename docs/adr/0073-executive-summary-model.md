# ADR 0073 — Executive summary input model

Status: Aceito. 2026-08-20.

## Contexto

SAI-079: summary executivo a partir de fatos/arbiter
verdict. Se usar IA, etapa separada. Regra de ouro:
summary NUNCA altera scores — só descreve estado.

## Decisão

`internal/report/summary.go`:

- `SummaryInput` agregador de fatos puros (gate, scores,
  risk, confidence, divergences, breaking changes,
  SOLID deltas, limitations, components).
- `GenerateSummary(in)` produz `SummaryResult` com
  headline, body, bullets, sources. Determinístico —
  mesma entrada → mesma saída.
- `Headline` = `"{profile} — grade {grade}, gate {status},
  risk {level}"`.
- `Bullets` sempre inclui: gate (status+score), risk
  (level+fatores ordenados), confidence (level+%),
  divergences (count+resolved/pending). Opcional:
  breaking changes, SOLID regressões, limitações,
  componentes.
- `Body` parágrafo contínuo com perfil, gate, breaking,
  divergences, risk, confidence.
- `Sources` lista de blocos que contribuíram —
  `divergence_map` aparece se há divergences,
  `release_notes` se há breaking.
- `IAExtendedSummary` separado: `BaseSummary + IABullets
  + Provider + ModelID` — IA é camada adicional sobre
  summary determinístico. Nunca substitui.

Determinismo: funções puras (sem clock, sem rand, sem
I/O). `GeneratedAt` é a única coisa que muda entre
chamadas — caller controla se inclui ou não.

## Consequências

- 16 testes: basic, empty runid/profile, risk factors,
  divergences resolved/pending, breaking, SOLID
  regressões, limitations/components, body com breaking
  + resolved, IAExtended, sources div/breaking,
  helpers, negativeSOLID, joinSorted.
- Score é read-only aqui — gerador só consome
  `QualityScore`, `Confidence`, `SOLIDDeltas` para
  formatar. Sem write-back.
- Sort estável em risk factors e components — output
  reproduzível byte-a-byte (exceto `GeneratedAt`).
- IAExtended documenta proveniência — provider/model
  explícitos no JSON p/ auditoria.

## Trade-offs

- Sem integração com `ai.Provider` aqui — caller
  decide se chama IA ou não. Trade-off: package
  foca em lógica pura; IA fica em outro lugar.
- `GeneratedAt` no output quebra byte-stability entre
  runs. Trade-off: audit trail > imutabilidade
  temporal do summary. Caller pode remover se quiser.
- Templates hardcoded em PT-BR — internacionalização
  fica p/ ADR futuro.