# ADR 0129 — Evidence-Backed SOLID Applicability & Scoring (SAI-129)

Status: Proposto. 2026-09-01.

## Contexto

Investigação do golden case (projeto real, diff de rename de
1 linha, `HEAD~1..HEAD`) expôs dois defeitos mecânicos independentes
no pipeline atual de score, ambos confirmados por leitura de código:

1. **Schema não força decomposição.** `peer.CanonicalSchema`/
   `principleSchema` (`internal/peer/schema.go`) exige
   `after_score.value` numérico obrigatório nos 5 princípios; `applicable`
   é opcional e nunca lido por `validateCanonical()`
   (`internal/peer/executor.go:218-261`). O modelo é forçado a inventar
   nota mesmo quando o princípio não deveria ser avaliado.
2. **`quality_score` global é autorreportado, não calculado.**
   `extractQualityScore()` prefere o campo `quality_score` top-level;
   a média sobre `solid.*.after_score.value` é só fallback nunca
   exercido na prática. Isso vale igualmente para peer_a/peer_b (mesmo
   `peer.Executor`).
3. **Correção de um erro anterior**: o salto de score observado no
   golden case (40→72 ao adicionar peer_b/arbiter) é troca de fonte
   no switch simples de `internal/app/run.go` (peer_b > peer_a quando
   ambos rodam) — **não** arbitragem. O `Verdict`/`Resolutions` do
   Arbiter hoje só viram texto em `ai_review.divergences`; nunca
   realimentam o score.
4. **Prompt real não tem rubrica.** `peerBPromptTemplate`
   (`run.go:23-28`, usado por peer_a e peer_b igualmente) menciona
   "applicable" uma única vez, como anotação de tipo — zero critério.
   Existe um prompt bom com rubrica real (`prompts/peer-review.md`,
   52 linhas, regra explícita de "marcar não aplicável quando
   apropriado" + critério por princípio) mas é dead code — só
   carregado por `internal/mcpserver/prompt.go`, sem caller de
   produção. Mesmo padrão do bug do SAI-128 no arbiter (motor de
   prompt existente, nunca ligado).

Plano técnico completo investigado e aprovado pelo usuário em
`/Users/diegoaraujo/.claude/plans/gentle-crunching-boole.md` (16
seções: causas raiz, fluxo atual ponta a ponta, 3 opções de
arquitetura, algoritmo de agregação, algoritmo de applicability,
mudanças no Arbiter e no report, compat, testes, riscos).

### Relação com ADR-0120 (CanonicalPayload v2) e ADR-0126 (schema v3 deferred)

Análise explícita pedida antes de abrir este ADR:

- **ADR-0120** versiona o schema **externo** de submissão MCP —
  `PeerReviewSchemaVersion` (`internal/mcpserver/peer.go`, hoje `"2"`),
  o campo `schema` em `PeerReviewRecord` que distingue payload v1
  (JSON dentro de `Notes`) de v2 (`CanonicalPayload map[string]any`
  nativo). Esse wrapper é **untyped** — não versiona o shape interno
  do que vai dentro de `canonical_payload`.
- **ADR-0126** deferiu bump desse wrapper pra `"3"`, mas
  especificamente para `detailed_findings[]` tipados por pilar — sem
  demanda real confirmada (nenhum consumer pediu). Não é o mesmo
  problema que SAI-129 resolve.
- **SAI-129 mexe em outro schema**: `peer.CanonicalSchema`/
  `principleSchema` (ADR-0116, schema LLM-facing que popula o
  conteúdo de `canonical_payload`, não o wrapper de ADR-0120). Adiciona
  `applicability` (enum) e torna `score`/`after_score` condicional.
  Isso muda o *conteúdo* de `canonical_payload`, não o campo `schema`
  que ADR-0120 versiona.

**Decisão sobre versionamento**: SAI-129 **não bumpa**
`PeerReviewSchemaVersion` ("2" permanece) — o wrapper MCP não muda,
só o conteúdo dentro do map untyped. SAI-129 **não reabre** o gatilho
de ADR-0126 (motivo diferente: applicability tri-state, não findings
tipados; se algum dia findings[] forem pedidos por um consumer real,
isso continua sendo decisão separada, sob os critérios do próprio
ADR-0126). O que SAI-129 versiona é o **schema do report**
(`report.SchemaVersion`, hoje `"1.0.0"`) — sobe pra `"1.1.0"` (§11 do
plano), porque `report.Principle.Applicable bool` ganha companhia
(`Applicability string`) e isso é visível a consumers externos do
`release-report.json`, ao contrário do payload interno untyped do
MCP. Records v1/v2 antigos (sem `applicability`) continuam legíveis:
`Applicable` derivado de `Applicability=="APPLICABLE"` por 1-2
versões, documentado como deprecated — mesmo padrão de compat que
ADR-0120 usou para v1→v2.

## Decisão

Arquitetura **híbrida** (Opção C do plano, entre LLM-only,
heurística-only e híbrida): heurística determinística decide os
casos óbvios de applicability sem custo de LLM; LLM decide só os
casos ambíguos, com evidência mínima obrigatória; score final é
sempre agregado deterministicamente fora do LLM.

### 1. Contrato por princípio

```json
{
  "principle": "S",
  "applicability": "APPLICABLE | NOT_APPLICABLE | INSUFFICIENT_EVIDENCE",
  "applicability_source": "heuristic | llm",
  "score": 78,
  "confidence": 0.82,
  "reason": "texto curto",
  "evidence_refs": ["file.go:42-58", "hunk#3"]
}
```

Invariantes: `NOT_APPLICABLE` → `score=null`, peso 0 na agregação.
`INSUFFICIENT_EVIDENCE` → `score=null`, afeta `confidence`/
`score_status`, não a nota. `APPLICABLE` → exige `evidence_refs`
não-vazio.

### 2. Agregação determinística (`AggregateScore`, fora do LLM)

Só princípios `APPLICABLE` com `score != null` entram na média; se
nenhum, `globalScore=null` e `score_status` distingue
`NOT_APPLICABLE` (todos N/A) de `INSUFFICIENT_EVIDENCE` (LLM não
decidiu nenhum com confiança). Substitui a leitura de `quality_score`
top-level como fonte de verdade — campo vira telemetria.

### 3. Heurística de applicability (estágio 1, sem LLM)

Roda sobre `gitx.DiffFile` bruto. Sinais fortes (rename/format/
comentário puro → `CLEARLY_NOT_APPLICABLE` pros 5; novo import/
interface/branch/hunk multi-domínio → sinal `CLEARLY_APPLICABLE`,
LLM só pontua, não decide applicability). Resto → `AMBIGUOUS`, vai
pro LLM pedir applicability + `evidence_refs` antes de score.
Reaproveita `prompts/peer-review.md` (dead code hoje) como base
textual do estágio ambíguo.

### 4. Arbiter: applicability-first + score real

Quando `peer.Compute()`/`DivergenceMap` reporta `ApplicableMismatch`,
o arbiter resolve `applicability_verdicts[]` (com evidência mínima)
**antes** de `resolutions[]` de score. Diferente de hoje: o resultado
passa a alimentar `AggregateScore()` de verdade — corrige o achado
de que o Arbiter é hoje decorativo pro score final.

### 5. Report

`SOLIDBlock.Principles` finalmente populado (hoje sempre `{}` —
confirmado em `internal/report/builder.go:408-410`, `run.go` nunca
seta `bInput.SOLID`). `score_status` ganha estados novos
(`AVAILABLE/PARTIAL/NOT_APPLICABLE/INSUFFICIENT_EVIDENCE/UNAVAILABLE`),
mantendo `available/unavailable/error` como aliases de compat.
`report.SchemaVersion` bump `1.0.0`→`1.1.0` (ver seção de
versionamento acima).

## Consequências

- Score final passa a refletir decomposição verificável, não mais
  autorreport livre de um LLM.
- Princípio N/A não infla nem reduz o score global — peso 0 explícito.
- Arbiter deixa de ser decorativo: sua arbitragem realmente move o
  score quando há divergência real.
- Golden case (rename 1 linha) passa a produzir
  `score_status=NOT_APPLICABLE` em vez de um número de 40 ou 72
  fabricado por troca de fonte.
- Report schema sobe pra `1.1.0`; consumers que só checam
  `Applicable bool` continuam funcionando via campo derivado.

## Trade-offs

- Heurística mal calibrada pode gerar falso-negativo silencioso
  (violação real classificada N/A). Mitigado: heurística só decide
  `CLEARLY_NOT_APPLICABLE` com sinais fortes; ambíguo sempre vai pro
  LLM, nunca o inverso.
- Escopo maior que SAI-127/128: toca `peer/schema.go`,
  `peer/executor.go`, `arbiter/executor.go`, `arbiter/prompt.go`,
  `report/builder.go`, `app/run.go`, + pacote novo
  `internal/applicability`. Dividido em tasks (ver Próximo passo).
- `gate.Evaluate` precisa aprender a tratar `globalScore==null`
  (`NOT_APPLICABLE`) sem comparar contra threshold numérico — toque
  mínimo, não é o escopo de SAI-134 (thresholds hardcoded lendo
  `config.Scoring.Gate.MinimumQualityScore`, que fica **fora** deste
  round).

## Não-objetivos

- `detailed_findings[]` tipados por pilar — continua sob ADR-0126,
  não reaberto por este ADR.
- Bump de `PeerReviewSchemaVersion` (wrapper MCP) — permanece `"2"`.
- SAI-134 (thresholds do gate lendo config em vez de hardcoded) —
  candidato separado, não implementado nesta rodada.
- `ai.Selector`/`DistinctnessPolicy` real — pendência separada
  (ADR-0117, já sinalizada).
- Detectar collusion entre modelos via embedding similarity — fora
  de escopo.

## Próximo passo

1. **SAI-129A** — Schema (`applicability` obrigatório, `score`
   condicional) + `AggregateScore()` determinístico
   (`peer/schema.go`, `peer/executor.go`).
2. **SAI-129B** — Motor heurístico (`internal/applicability`, novo
   pacote) + integração no pipeline peer_a/peer_b + religar
   `prompts/peer-review.md` no lugar do `peerBPromptTemplate`.
3. **SAI-129C** — Arbiter: `applicability_verdicts` separado de
   score + resultado realimentando `globalScore`.
4. **SAI-129D** — Report: `SOLIDBlock.Principles` populado,
   `score_status` novos estados, `report.SchemaVersion` → `1.1.0`.

(SAI-134 fica de fora desta rodada, por instrução explícita.)
