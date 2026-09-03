# ADR 0049 — Peer Review JSON Schema

Status: Aceito. 2026-08-20.

## Contexto

SAI-055: validator JSON Schema para Peer Review + fixtures
válidas/inválidas. Sem dependência externa — subset Draft 7
suficiente.

## Decisão

`PeerReviewJSONSchema` literal const em JSON Draft 7:
- type: object, additionalProperties: false
- required: run_id, actor, schema, evidence_hash, verdict
- properties: run_id(1..200), actor(1..200), schema(enum[1]),
  evidence_hash(pattern `[0-9a-f]{64}`), verdict(enum
  approve/request_changes/comment), findings(object),
  notes(maxLength 10000)

`SchemaValidator{schema}` + `Parse()` unmarshal em `SchemaNode`.

Subset implementado:
- type check (string/number/object/array/null/boolean)
- required
- additionalProperties (false→reject, true/default→allow)
- enum (qualquer tipo)
- string: minLength/maxLength/pattern

`SchemaError{Path, Message}` + `SchemaErrors[]` (implementa
`error`).

`Validate([]byte)` parse JSON → validateNode. `ValidateMap`
aceita map[string]any direto.

`ValidPeerReviewFixture` const + `InvalidPeerReviewFixtures`
map (8 casos: missing_run_id, unknown_schema, bad_hash,
bad_verdict, additional_field, actor_too_long, notes_too_long,
wrong_type).

`RunSchemaFixtures(v, valid, invalids) → map[name]bool`:
valida todos e retorna mapa de passes.

`jsonTypeOf` helper switch para type detection.

## Consequências

- 16 testes (schema parseable, validator empty/garbage, valid
  fixture, invalid fixtures, parse, validate garbage/map,
  SchemaErrors string, jsonTypeOf, fixtures runner, number
  mismatch, SchemaNode custom, missing required, additional
  false/true, IsValid).
- Sem deps externas — só stdlib `encoding/json` + `regexp`.
- `additionalProperties: false` força caller a evoluir schema
  antes de adicionar fields.

## Trade-offs

- Subset Draft 7 — não cobre `allOf`/`oneOf`/`anyOf`/`$ref`/
  `format`/`definitions`. Aceitável: Solidify usa minimal
  schema; expansão futura se preciso.
- Pattern regex compilado por validação (`regexp.MatchString`)
  — overhead aceitável; schema tem ~5 patterns.
- `findings` type=object sem spec — caller decide estrutura.
  Posterior SAI poderia definir nested schema.
- Composite literal `SchemaErrors{...}` Go exige type element
  explícito (`SchemaErrors{SchemaError{...}}`); tests usam
  `var ... SchemaErrors` para nil/empty.
