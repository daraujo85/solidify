# ADR 0012 — Env usage detector multi-stack

## Status

Aceito. 2026-08-20.

## Contexto

SAI-018 pede detector de uso de env vars para todos os stacks listados
em ARCHITECTURE.md §11.2. **Regra crítica §11.1:** a ferramenta NÃO
carrega valores — só captura o NOME da variável.

Stacks cobertos:

- Node/Vite, Go, .NET, Java/Spring, Python, PHP/Laravel, Ruby,
  Elixir, Rust, Dart.

## Decisão

Função pura `Detect(path string, src []byte) []Usage`.

- **Regex por forma** — uma regex por pattern canônico. Tabela de
  patterns (Form, regex, nameIdx, hasDefault, allowDots).
- **Strip nada** — varre o source linha por linha. Não lê variáveis
  de ambiente, não faz eval, não importa nada do host.
- **Validação de nome** — depois de extrair via regex, valida que
  casa `^[A-Za-z_][A-Za-z0-9_]*$` (ou `... .-]` para Spring/kebab).
  Rejeita nomes com chars inválidos (`"1INVALID"` não casa).
- **HasDefault** — se a regex captura uma chamada com vírgula no
  meio (`os.environ.get("PORT", "8000")`), marca `HasDefault=true`.
- **Dedupe** — `(path, name, line)` único. `@Value("${X}")` casa
  tanto `FormSpringValue` quanto `FormSpringBrace`; patterns é
  ordenado (SpringValue antes de SpringBrace), dedup mantém o
  primeiro (mais específico).

**Formas suportadas:**

| Stack          | Form                   | hasDefault |
|----------------|------------------------|------------|
| Node           | `process.env.X`        | — |
| Vite           | `import.meta.env.X`    | — |
| Go             | `os.Getenv`, `os.LookupEnv` | — |
| .NET           | `Environment.GetEnvironmentVariable` | — |
| Java           | `Environment.getProperty`, `System.getenv` | — |
| Spring         | `@Value("${X}")`, `${X}` | — |
| Python         | `os.environ`, `os.getenv`, `os.environ.get` | ✓ p/ `get` |
| PHP            | `getenv`, `$_ENV`, `$_SERVER`, `env()` (Laravel) | ✓ p/ Laravel |
| Ruby           | `ENV[]`, `ENV.fetch`   | — |
| Dart           | `String.fromEnvironment`, `Platform.environment` | — |
| Elixir         | `System.get_env`       | — |
| Rust           | `std::env::var`        | — |

**Truques de regex Go (RE2 não tem lookbehind/lookahead):**

- **PHP `getenv` vs Java `System.getenv`**: PHP `getenv` é global
  function (sem prefixo). Java `System.getenv` tem `.` antes.
  Padrão `(?:^|[^.])getenv\(` consome 1 char não-ponto antes — assim
  `System.getenv` não casa mas `getenv` e `= getenv` casam.
- **Ruby `ENV[]` vs PHP `$_ENV`**: ambos têm `ENV` mas em `$_ENV` o
  `_` precede `ENV`. `\bENV` (word boundary antes) garante que
  `_ENV` não casse — `_` é word char, sem boundary.
- **Spring `${X}` em `@Value`**: ambas as regexes casam. Dedupe por
  (path, name, line) com patterns ordenados — SpringValue vence.

## Consequências

**Positivas:**

- Não carrega valores (segurança §11.1).
- Cobre 12 stacks + variantes Spring.
- Dedup evita ruído (`@Value` + `${}` na mesma linha = 1 finding).
- 33 testes cobrindo cada forma, edge cases (kebab-case, dotted
  Spring names, single vs double quotes, false positives entre
  stacks).

**Negativas:**

- Regex não cobre EXEC de script com env vars dinâmicas (`export
  $X`, `eval`), chamadas via helper local, geração via
  concatenação. Aceitável — SAI-018 cobre os casos óbvios; o resto
  exige análise dinâmica.
- Spring brace `${X}` casa TUDO com `${X}` mesmo fora de contexto
  Spring (template literals JS, etc). Caller deve filtrar por
  extensão do arquivo se quiser precisão.
- HasDefault é heurística (vírgula no match) — pode dar falso
  positivo para `os.environ["PORT",]` ou `env("X",)`. Aceitável.

## Alternativas consideradas

- **AST parsing real por stack** (tree-sitter, antlr): cobriria 100%
  mas adiciona 5+ MB de binário e latência. Para um detector de
  strings, overkill. Rejeitado.
- **Execução parcial do código** (carregar env de fato): viola §11.1
  e abre superfície de ataque. Rejeitado por princípio.
- **Whitelist manual de stacks**: pede config do projeto. Vai contra
  a detecção automática do §11.2. Rejeitado.