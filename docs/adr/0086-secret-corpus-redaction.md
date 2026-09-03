# ADR 0086 — Secret corpus redaction tests (SAI-103)

Status: Aceito. 2026-08-20.

## Contexto

SAI-103: API keys, bearer tokens, env values,
connection strings, passwords — todos devem ser
redacted antes de log/relatório. Corpus canônico p/
validação.

## Decisão

`internal/secretcorpus/`:

- `Fixture{Name, Category, Secret, Contains}`.
- `CanonicalCorpus`: 12 fixtures cobrindo:
  - api_key: AWS, GitHub PAT, Stripe.
  - bearer: JWT, Basic.
  - env: DB_PASSWORD, API_KEY.
  - conn: postgres/mysql/mongodb URLs.
  - password: JSON, YAML.
- `Verify(redactFn func(string) string) []Result` —
  aplica redactor em cada fixture, verifica se
  `Contains` substring foi removida.
- `Result{FixtureName, Category, Redacted, Original,
  Output}`.
- `Summary{Total, Redacted, Leaked, ByCategory}` +
  `Summarize(rs)`.

## Consequências

- 5 testes: verify count, summarize, leak detection,
  categories cobertas, contains helper, integration
  com RedactEnvLine-style.
- `Verify` aceita redactor injetado — testa
  qualquer implementação (RedactSecret,
  RedactEnvLine, custom).
- 5 categorias cobertas: api_key, bearer, env, conn,
  password.

## Trade-offs

- Corpus é estático — novos formatos de secret
  precisam ser adicionados manualmente. Trade-off:
  determinismo > coverage automática.
- `Verify` só checa se `Contains` sumiu — não valida
  formato do redacted output. Trade-off:
  simplicidade > formato exato.
- Sem integração com regex — fixtures são literais.
  Trade-off: literal > pattern matching.
