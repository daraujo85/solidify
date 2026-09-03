# ADR 0026 — Coverage importer

Status: Aceito. 2026-08-20.

## Contexto

SAI-032: coverage reports vêm em formatos diferentes por stack (LCOV
de lcov/gocover; Cobertura XML de JaCoCo/Cobertura/python-coverage).
Score/Gate precisa de %unificado pra comparar threshold.

## Decisão

`coverage.Report` normalizado:

- `LinePct`, `BranchPct` (0-100)
- `LinesTotal`, `LinesCovered`, `BranchesTotal`, `BranchesCovered`
- `Files []FileCoverage` com path + counts + pct por arquivo
- `Format` (lcov | cobertura) — útil pra audit

`Parse(format, r)` despacha. `ParseAuto(data)` detecta por prefixo
(`TN:` → LCOV, `<?xml` → Cobertura) ou heurística (`SF:` → LCOV).

LCOV parsing: scanner linha-a-linha. `SF:` inicia file, `LF/LH/BRF/BRH`
preenchem counts, `end_of_record` fecha. Path canônico em
`FileCoverage.Path`.

Cobertura XML: usa `encoding/xml`. Extrai `line-rate`, `branch-rate`
do root, mais `<class filename>` entries com `<lines><line hits>`.

`Merge` soma counts e recalcula pct. `IsGood(threshold)` é `LinePct >= threshold`.
`WorstFiles(n)` selection sort pra top-N piores.

## Consequências

- 18 testes (LCOV básico, branches, file pct, Cobertura XML,
  ParseAuto, Merge, IsGood, WorstFiles, edge cases).
- Coverage SAI-031 + coverage SAI-032 → score/gate (SAI-040+) tem
  dados consistentes.
- Sem dependência externa — só stdlib.

## Trade-offs

- Cobertura parser ignora `<conditions>` (variant Cobertura 2.x);
  suficiente pra maioria dos reporters.
- LCOV parsing suporta subset (SF/LF/LH/BRF/BRH/FNF/FNH); DA/DAC
  (line data) ignorado — não usado pra %.
