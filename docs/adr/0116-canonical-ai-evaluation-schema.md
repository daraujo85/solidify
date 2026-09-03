# ADR 0116 — Canonical AI Evaluation Schema (SAI-116)

Status: Proposto. 2026-08-20.

## Contexto

SAI-116: hoje o peer reviewer devolve JSON em formato
livre. O orquestrador (`internal/app/run.go`) tem
3 caminhos de extração (`scores.quality_score`,
`solid.{S,O,L,I,D}.after_score.value`, `gate_assessment`)
e cai silenciosamente em `globalScore = 70` quando
nenhum bate.

Esse `70` é falsamente legítmo: parece uma nota
real no report, mas é só "parser falhou em todos
os formatos conhecidos". `grade=C`, `confidence=0.85`
reforçam a ilusão.

Consequência observada no teste dos 3 perfis (2026-08-20):
quick=60 / release=75 / contractual=85 produziram
todos score=70 e gate "abaixo do threshold" —
comparação entre perfis contaminada, nenhum sinal
real de qualidade.

## Decisão

### 1. Schema canônico obrigatório

`internal/peer/schema.go` declara `CanonicalSchema`
(JSON Schema Draft 7):

```json
{
  "solid": {
    "S": { "after_score": { "value": 82 } },
    "O": { "after_score": { "value": 76 } },
    "L": { "after_score": { "value": 91 } },
    "I": { "after_score": { "value": 85 } },
    "D": { "after_score": { "value": 79 } }
  },
  "quality_score": 82,
  "confidence": 0.87
}
```

`internal/peer/executor.go` passa `JSONSchema:
CanonicalSchema` por default (caller pode
sobrescrever). Quando o provider suporta
`response_format: {type:"json_schema", schema:{...}}`,
vai nativo (OpenAI, OpenRouter, Gemini direto).
Quando não suporta (9Router combos), parser
estrito pós-resposta + repair único.

### 2. Eliminar fallback silencioso

Novo campo `ScoreStatus`:
- `available` — schema bateu, score real
- `unavailable` — schema falhou mesmo após repair
- `error` — chamada ao provider falhou

Quando `unavailable`, `globalScore = 0` e o gate
vira `INCOMPLETE` (não `FAIL` com score artificial).
Report mostra `score_status` explicitamente —
"parece válido" deixa de ser opção.

### 3. Repair loop em vez de fallback

Se a 1ª resposta não casa com `CanonicalSchema`:
1. Repair único com prompt de correção apontando
   exatamente quais campos faltam (`solid.L` ausente,
   `after_score` deve ser objeto com `value:number`).
2. Se repair também falha → `ScoreStatus=unavailable`,
   `globalScore=0`, gate=INCOMPLETE.

### 4. Wire no orchestrator

`internal/app/run.go` usa o `JSONSchema` option
(já existe, só não era passado). Score extraction
vira um único caminho: `quality_score` direto,
média de `solid.*.after_score.value` como
redundância (se `quality_score` ausente).

## Consequências

- Report mais honesto: `score_status` visível.
- Gate distingue "FAIL por qualidade" de
  "INCOMPLETE por schema falhou".
- Repair consome ~1 round-trip extra em caso raro
  (~5% baseado nos testes).
- Provider que ignora `response_format` continua
  funcionando via validação client-side.

## Trade-offs

- 1 round-trip extra no caminho unhappy (repair).
  Aceitável: raro e necessário pra auditabilidade.
- Schema fixo não captura nuances que modelos
  poderiam expressar. Trade-off: auditabilidade
  > flexibilidade.
- `response_format: json_schema` exige provider que
  honra o campo. Combos do 9Router podem não honrar
  → cai na validação client-side. Documentado.

## Não-objetivos

- Múltiplos schemas por perfil. Um schema canônico
  serve `quick`/`release`/`contractual`.
- Suporte a JSON Schema Draft 2020-12. Draft 7 cobre
  todos os providers testados.
