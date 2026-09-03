# ADR 0126 — Schema v3 deferred (SAI-126)

Status: Deferido. 2026-08-21.

## Contexto

5 ADRs consecutivas (0120, 0121, 0122, 0123, 0124,
0125) registraram como próximo passo: "schema "3"
com `detailed_findings` por pilar SOLID — bump
**após field usage confirmar demanda**."

A exploração do repo mostra:

1. **`report.Principle.Findings []map[string]any`**
   já existe untyped desde SAI-077, mas nunca é
   populado pelo fluxo de run (run.go:300 não passa
   `SOLID:` pro `BuilderInput`).
2. **`schemas/peer-review.schema.json`$defs.finding`**
   já define o shape typed completo (`id, principle
   (enum S/O/L/I/D), severity, title, file,
   line_start, line_end, evidence_refs, rationale,
   recommendation, introduced_by_release`). Nenhum
   código Go valida contra esse schema hoje.
3. **`internal/peer/schema.go::CanonicalSchema`** —
   o JSON-schema enviado ao LLM como resposta
   esperada — **não pede** findings por pilar.
   Findings só aparecem como `issues[]` flat no
   topo.
4. **Sem demanda concreta**: `spec/TASKS.md` termina
   em SAI-115 (Fase 24). Backlog v3 vive só nas
   ADRs; nenhum consumer real pediu o bump.

## Decisão

**Deferir SAI-126.**

Sem field usage real:

- Tipagem forte de `Finding` em código Go seria
  especulação (YAGNI do projeto).
- Estender `CanonicalSchema` enviado ao LLM sem
  demanda real = prompt mais longo, respostas piores.
- Bump de schema "2" → "3" exige migração,
  atualização de canary/trend/migrate/validators —
  trabalho que se justica apenas se houver consumer
  pedindo.

Quando reabrir: SAI-126 só abre se aparecer um
consumer concreto (dashboard widget, export CSV,
integração externa) que precise de `findings` por
pilar tipados. **Não antecipar generalização.**

## Não-objetivos (reforço)

- Adicionar `Finding` typed em Go agora — espera
  demanda.
- Estender schema do LLM — espera demanda.
- Wire `report.Principle.Findings` no fluxo de run —
  mudança grande, sem destino claro.
- Bump schema "2" → "3" — não há v2 em produção
  ainda com usuários suficientes pra justificar.

## Reabertura (gatilhos)

SAI-126 reabre quando **pelo menos um** dos
seguintes aparecer:

1. Consumer real pede findings tipados (dashboard
   widget, CSV export, integração com Jira/Linear).
2. Field usage confirmar: mais de 5% dos records
   v2 incluem findings não-triviais (verificar via
   `peer-reviews stats --since ...`).
3. ADR nova propor schema "3" com justificativa
   concreta (não "achamos que seria útil").

## Consequências

- **Nada muda no código**: schema continua "2",
  sem typed Finding.
- **Backlog honesto**: ADRs 0120–0125 mantêm o
  item riscado, mas com nota "deferido por ADR-0126
  até demanda concreta".
- **Operator sem overhead**: rollout v2 (canary,
  trend, migrate) seguem normalmente.
- **Sem migração fantasma**: nenhum record v3
  fantasma no store.

## Verificação

- ADRs 0120–0125 mantêm referência a ADR-0126
  como destino da decisão de deferir.
- Nenhum código modificado.
- Spec/TASKS.md não atualizado (SAI-126 continua
  fora do scope oficial).

## Próximo passo

1. Fechar SAI-126 como deferido.
2. Operador monitora `peer-reviews stats`; se
   findings começarem a aparecer em volume,
   reabrir.
3. SAI-127+ (se houver): definidos pelo backlog
   externo ao schema, não pela evolução v2→v3.