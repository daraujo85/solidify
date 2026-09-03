# ADR 0078 — Robustness report integration (SAI-095)

Status: Aceito. 2026-08-20.

## Contexto

SAI-095: bloco de robustness (SAI-094) precisa
aparecer no JSON/dashboard/PDF quando executado
role-swap. Sem mudificar schema do Report — só
anexar via `Extra`.

## Decisão

`internal/report/robustness.go`:

- `RobustnessBlock{Status, QualityDelta, LetterDeltas,
  Overlap, GateStable, OriginalRunID, SwappedRunID,
  OnlyOriginal, OnlySwapped, Both, Notes}`.
- `AttachRobustness(rep, orig, swapped)` — chama
  `peer.Compare` e anexa bloco em `rep.Extra["robustness"]`.
  Erros: `ErrNoRobustness` (snapshot nil), erro de
  compare propaga.
- `MarkRobustnessNotRun(rep)` — placeholder p/ pipeline
  que rodou sem role-swap. Status="not_run".
- `RobustnessFromReport(rep)` — extrai bloco (nil se
  ausente).
- `RenderRobustness()` textual p/ PDF/dashboard
  (reuso de padrão `Robustness[A vs B]\n  status=...`).
- Erros sentinel: `ErrNoRobustness`.
- `Extra` field já existe no Report (json:"-"),
  permite anexar sem alterar schema_version.

## Consequências

- 8 testes: attach ok, attach nil, attach nil rep,
  mark not_run, mark not_run nil, from nil/empty,
  render, helpers numéricos.
- Bloco fora do schema canônico — consumidores
  existentes (que ignoram campos desconhecidos)
  continuam funcionando.
- `MarkRobustnessNotRun` explícito — caller decide
  se é erro ou skip. UX: dashboard mostra "não
  executado" sem erro vermelho.

## Trade-offs

- `Extra` é `map[string]any` — type assertion na
  leitura. Trade-off: zero schema change > type
  safety.
- Render textual duplica padrão de `peer.RenderResult`
  — duas funções similares. Refator p/ shared helper
  fica p/ ADR futuro.
- `ftoaRob`/`itoaRob`/`padLeftRob` reimplementados —
  mesmo padrão de outros packages (peer, builder).
  Helper global seria melhor mas custa import
  cycle.
