# ADR 0070 — Release report JSON Schema + validator

Status: Aceito. 2026-08-20.

## Contexto

SAI-076: validar report gerado vs schema canônico
`schemas/release-report.schema.json`. Schema já existia;
enum `quality_gate.status` tinha `PASS_WITH_WARNINGS`
inconsistente com gate real (`PASS`/`WARN`/`FAIL`/`BLOCKED`/`INCOMPLETE`).

## Decisão

Schema: trocou `PASS_WITH_WARNINGS` por `WARN` no enum
de `quality_gate.status`.

Validator interno `internal/report/schema.go`:

- Draft 2020-12 subset — só o que report usa.
- Suporta: `type` (object/array/string/number/integer/
  boolean/null), `required`, `enum`, `const`, `pattern`
  (regex), `minimum`/`maximum`, `minLength`,
  `additionalProperties:false`, `properties`, `items`,
  `$ref` intra-doc `#/$defs/X`.
- Sem dep externa — stdlib (`encoding/json`, `regexp`,
  `fmt`).
- `Validator{schema, root}` guarda root p/ resolver
  `$ref` (`$defs` vive na raiz).
- `Validate(data []byte)` retorna primeiro erro com path
  JSON (`$.foo.bar[0]`).
- `ResolveRef(root, ref)` exportado p/ testes/helpers.

`TestValidatorRealSchema`: monta report mínimo válido
(exige `finalPrinciple` completo) e roda validator contra
o JSON real.

## Consequências

- 19 testes: type, enum, pattern, range, required,
  additional, array items, integer strict, $ref,
  const, nested, real schema ok/fail, edge cases
  (nil/parse/schema).
- Schema enums alinhados com gate (`WARN` em vez de
  `PASS_WITH_WARNINGS`).
- Validator falha rápido com path — debug mais simples
  quando report não bate schema.
- Sem suporte p/ `$ref` externo, `oneOf`/`anyOf`,
  `format` (date-time validado só por type), `if/then`,
  `dependencies`. Trade-off documentado abaixo.

## Trade-offs

- Sem dep JSON-Schema completa (tipo `xeipuuv/gojsonschema`).
  Cobertura intencional: subset. Trade-off: zero deps +
  controle total > conformidade exata c/ Draft 2020-12.
  ADR futuro pode trocar por lib se necessário.
- `format` não validado (date-time, uri, etc.) — fica
  por conta do produtor. ADR seguinte pode adicionar
  hook opcional.
- `oneOf`/`anyOf` não cobertos — schema atual não usa;
  se passar a usar, ADR p/ estender validator.
- Erro único (primeiro falha) — não coleta todos. Trade-
  off: simples > exaustivo. Schema bem formado não tem
  múltiplos erros na prática.