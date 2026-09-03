// Package redact — framework de redação de segredos.
//
// Implementa SAI-020: redação de valores de env, auth headers, tokens
// comuns (JWT, GitHub PAT, Slack, AWS), config paths secret-like e
// sanitizer universal para logs/prompts.
//
// Regra de §11.1: nada com valor secreto chega a artifact/log/prompt.
//
// Uso:
//
//	r := redact.New()
//	out := r.Redact(input)
//	out := r.RedactEnv("PAYMENT_TOKEN=abc123")  →  "PAYMENT_TOKEN=[REDACTED:env-secret]"
package redact

import (
	"regexp"
)

// Redactor aplica um conjunto de padrões de redação a strings. Thread-safe
// (stateless após construção).
type Redactor struct {
	patterns []pattern
}

// New devolve um Redactor com os padrões default.
func New() *Redactor {
	return &Redactor{patterns: defaultPatterns()}
}

// pattern é uma regra de redação.
type pattern struct {
	name string // tipo para diagnóstico (vai no placeholder)
	re   *regexp.Regexp
	// template: se set, usa este template com $1, $2 etc. Senão usa
	// "[REDACTED:name]" simples.
	template string
}

// Redact devolve a string com todos os segredos detectados substituídos.
func (r *Redactor) Redact(s string) string {
	for _, p := range r.patterns {
		if p.template != "" {
			s = p.re.ReplaceAllString(s, p.template)
		} else {
			s = p.re.ReplaceAllString(s, "[REDACTED:"+p.name+"]")
		}
	}
	return s
}

// RedactEnv é um atalho: redacta uma string no formato "KEY=value".
// Preserva KEY, substitui VALUE por [REDACTED:env-secret] se KEY bate
// heurística de nome de segredo.
func (r *Redactor) RedactEnv(line string) string {
	return reEnvSecret.ReplaceAllString(line, `${1}=[REDACTED:env-secret]`)
}

// RedactEnvLine é o wrapper package-level de RedactEnv — útil quando
// não há Redactor instanciado.
func RedactEnvLine(line string) string {
	return reEnvSecret.ReplaceAllString(line, `${1}=[REDACTED:env-secret]`)
}

// IsLikelySecretName devolve true se nome bate heurística de KEY/SECRET/
// TOKEN/etc. Útil para o caller decidir se uma env var deve ter valor
// sempre redactado.
func IsLikelySecretName(name string) bool {
	return reSecretSuffix.MatchString(name)
}

// defaultPatterns lista os padrões default. Ordem importa: padrões
// mais específicos (private key block) antes dos mais genéricos.
func defaultPatterns() []pattern {
	return []pattern{
		// Bloco de chave privada (PEM). Deve vir antes de JWT porque
		// blocos PEM podem conter "BEGIN ... PRIVATE KEY" que poderia
		// confundir patterns genéricos.
		{name: "private-key-block",
			re: regexp.MustCompile(`-----BEGIN\s+(?:RSA\s+|DSA\s+|EC\s+|OPENSSH\s+|PGP\s+|ENCRYPTED\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(?:RSA\s+|DSA\s+|EC\s+|OPENSSH\s+|PGP\s+|ENCRYPTED\s+)?PRIVATE\s+KEY-----`)},
		// Header Authorization: Bearer X.
		{name: "auth-bearer",
			re:       regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)([A-Za-z0-9._\-+/=]+)`),
			template: "${1}[REDACTED:auth-bearer]"},
		// Authorization Basic Auth.
		{name: "auth-basic",
			re:       regexp.MustCompile(`(?i)(authorization\s*:\s*basic\s+)([A-Za-z0-9+/=]+)`),
			template: "${1}[REDACTED:auth-basic]"},
		// Cookie: name=value.
		{name: "cookie-value",
			re:       regexp.MustCompile(`(?i)(cookie\s*:\s*)([^;\r\n]+)`),
			template: "${1}[REDACTED:cookie-value]"},
		// JWT (eyJ...).
		{name: "jwt",
			re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b`)},
		// GitHub PAT: ghp_, gho_, ghu_, ghs_, ghr_.
		{name: "github-token",
			re: regexp.MustCompile(`\bgh[psour]_[A-Za-z0-9]{36,}\b`)},
		// Slack: xox[bpars]-...
		{name: "slack-token",
			re: regexp.MustCompile(`\bxox[bpars]-[A-Za-z0-9-]{10,}\b`)},
		// AWS Access Key ID: AKIA / ASIA prefix.
		{name: "aws-akid",
			re: regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
		// Stripe live key: sk_live_xxx.
		{name: "stripe-key",
			re: regexp.MustCompile(`\bsk_(?:live|test)_[A-Za-z0-9]{20,}\b`)},
		// Google API key: AIza...
		{name: "google-api-key",
			re: regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35,}\b`)},
		// Postgres connection string com password.
		{name: "pg-url-password",
			re:       regexp.MustCompile(`(postgres(?:ql)?://[^:\s]+:)([^@\s]+)(@)`),
			template: "${1}[REDACTED:pg-url-password]${3}"},
		// MongoDB connection string com password.
		{name: "mongo-url-password",
			re:       regexp.MustCompile(`(mongodb(?:\+srv)?://[^:\s]+:)([^@\s]+)(@)`),
			template: "${1}[REDACTED:mongo-url-password]${3}"},
		// Secret paths (filesystem).
		{name: "secret-path",
			re: regexp.MustCompile(`(?:/var/run/secrets/|/etc/ssl/private/|/run/secrets/|\.docker/secrets/)[^\s"'<>]+`)},
		// env-style: KEY=value ou KEY: value onde KEY contém
		// chave secret-like (KEY/SECRET/TOKEN/PASSWORD/etc) em
		// qualquer posição, com o nome terminando lá. Preserva
		// KEY, redacta VALUE. RE2 não suporta lookbehind, então
		// usamos prefix não-greedy + sufixo dentro do capture
		// group + boundary final.
		{name: "env-secret",
			re:       regexp.MustCompile(`(?i)\b((?:[A-Za-z0-9_]*?)(?:KEY|SECRET|TOKEN|PASSWORD|PASSWD|ACCESS|PRIVATE|CREDENTIAL|AUTH|CERT))\b\s*[=:]\s*(\S+)`),
			template: "${1}=[REDACTED:env-secret]"},
	}
}

// Regex compiladas expostas (apenas para tests).
var (
	// env-secret — exportada para tests.
	reEnvSecret = regexp.MustCompile(`(?i)\b((?:[A-Za-z0-9_]*?)(?:KEY|SECRET|TOKEN|PASSWORD|PASSWD|ACCESS|PRIVATE|CREDENTIAL|AUTH|CERT))\b\s*[=:]\s*(\S+)`)
	// Sufixos de nome de segredo.
	reSecretSuffix = regexp.MustCompile(`(?i)(KEY|SECRET|TOKEN|PASSWORD|PASSWD|ACCESS|PRIVATE|CREDENTIAL|AUTH|CERT)`)
)
