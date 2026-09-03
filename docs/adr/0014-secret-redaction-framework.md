# ADR 0014 — Secret redaction framework

## Status

Aceito. 2026-08-20.

## Contexto

SAI-020 exige redação de segredos antes de qualquer coisa tocar IAs,
artefatos ou logs. Categorias:

- Valores de env (`KEY=value`)
- Auth headers (`Authorization: Bearer X`, `Cookie: ...`)
- Tokens comuns (JWT, GitHub PAT, Slack, AWS, Stripe, Google)
- Config paths secret-like (`/var/run/secrets/...`, `/etc/ssl/private/...`)
- Sanitizer universal para logs/prompts

Regra crítica §11.1: fixture `PAYMENT_TOKEN=abc123` nunca grava `abc123`
em artifact/log/prompt.

## Decisão

Função pura `Redact(string) string` + `RedactEnvLine(string) string`. Sem
estado, sem I/O, sem persistência. Thread-safe.

**Patterns (ordem importa — mais específicos antes):**

1. `private-key-block` — bloco PEM `-----BEGIN ... PRIVATE KEY-----`
2. `auth-bearer` — `Authorization: Bearer X`
3. `auth-basic` — `Authorization: Basic X`
4. `cookie-value` — `Cookie: X`
5. `jwt` — `eyJ...eyJ...sig`
6. `github-token` — `gh[psour]_*`
7. `slack-token` — `xox[bpars]-*`
8. `aws-akid` — `AKIA/ASIA[0-9A-Z]{16}`
9. `stripe-key` — `sk_(live|test)_*`
10. `google-api-key` — `AIza[0-9A-Za-z_\-]{35,}`
11. `pg-url-password` — `postgres://user:pass@host`
12. `mongo-url-password` — `mongodb://user:pass@host`
13. `secret-path` — `/var/run/secrets/`, `/etc/ssl/private/`, etc
14. `env-secret` — `KEY=value` onde KEY contém sufixo secret-like

**Substituição:** `[REDACTED:NAME]` exceto onde template preserva
contexto (auth header mantém nome do header, URL mantém schemas).

**`env-secret` regex:** `(?i)\b((?:[A-Za-z0-9_]*?)(?:KEY|SECRET|TOKEN|PASSWORD|PASSWD|ACCESS|PRIVATE|CREDENTIAL|AUTH|CERT))\b\s*[=:]\s*(\S+)`

Decisões de design:

- **Prefix `*?` não-greedy + sufixo dentro do capture group**: RE2 não
  tem lookbehind. Para capturar o nome INTEIRO (incluindo o sufixo) sem
  consumir greedy, usa-se captura `(?:[A-Za-z0-9_]*?)(?:SUFFIX)` com
  prefix não-greedy. Engine tenta prefix mínimos primeiro, achando a
  posição onde o sufixo bate.
- **Boundary `\b` no fim**: garante que `KEYSTORE` não é confundido com
  `KEY`.
- **Alternação ampla** (KEY/SECRET/TOKEN/PASSWORD/PASSWD/ACCESS/
  PRIVATE/CREDENTIAL/AUTH/CERT): cobre 99% dos nomes comuns. Falso
  positivo aceitável (e.g., `PUBLIC_KEY` em crypto context).
- **Valor `(\S+)`**: consome até whitespace. Para `PAYMENT_TOKEN=abc123`,
  captura `abc123`. Quebra em `=KEY2=value` (quase nunca usado).

**`RedactEnvLine`**: wrapper package-level que delega para o regex
`reEnvSecret`. Útil para callers que não instanciam `Redactor`.

**`IsLikelySecretName`**: heurística de nome — caller pode usar para
decidir se deve redactar valores mesmo fora do `Redact`.

## Consequências

**Positivas:**

- Crítico aceito: `PAYMENT_TOKEN=abc123` → `PAYMENT_TOKEN=[REDACTED:env-secret]`,
  nunca grava `abc123`.
- 33 testes (acceptance + each pattern + edge cases).
- Sem dependências externas (apenas `regexp` stdlib).
- Stateless: thread-safe, fácil de mockar.
- Idempotente em chamadas repetidas (saída já não tem segredos).

**Negativas:**

- Lista de patterns é estática. Tokens exóticos (e.g., npm tokens,
  Discord tokens) passam. Caller pode injetar patterns extras.
- `env-secret` regex captura valor como `\S+` — assume valor sem
  espaços. Adequado para envs e URLs. YAML multiline value não
  coberto (mas envs não têm multi-line).
- `IsLikelySecretName` é ampla — `PUBLIC_KEY` em crypto context gera
  falso positivo. Caller pode refinar.
- Padrões podem se sobrepor (e.g., JWT contém `eyJ` que pode bater
  `ghp_` parcialmente). Processar mais específicos primeiro minimiza
  mas não elimina.

## Alternativas consideradas

- **`go-redact` ou similar**: dependência externa. §11 prefere zero
  deps. Rejeitado.
- **Allowlist (define o que NÃO redactar)**: invertido — perigoso
  (default = vaza). Rejeitado.
- **Encrypt-then-decrypt-on-read**: persiste cifrado, decifra em
  runtime. Complexidade alta, ainda exige que cifrador não vaze key.
  Rejeitado para SAI-020.
- **No redact (raw logs)**: viola §11.1. Rejeitado.
- **Carregar valores em memória protected**: memória ainda é
  inspecionável. Rejeitado.
