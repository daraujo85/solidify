package redact

import (
	"strings"
	"testing"
)

// containsRedacted devolve true se output contém algum placeholder [REDACTED:...].
func containsRedacted(s string) bool {
	return strings.Contains(s, "[REDACTED:")
}

// Aceitação crítica do SAI-020: PAYMENT_TOKEN=abc123 nunca grava abc123.
func TestAcceptancePaymentTokenNeverPersisted(t *testing.T) {
	r := New()
	input := "PAYMENT_TOKEN=abc123"
	out := r.Redact(input)
	if strings.Contains(out, "abc123") {
		t.Fatalf("CRÍTICO: abc123 vazou em %q", out)
	}
	if !containsRedacted(out) {
		t.Errorf("esperava placeholder de redação em %q", out)
	}
	if !strings.Contains(out, "PAYMENT_TOKEN") {
		t.Errorf("KEY deveria ser preservado em %q", out)
	}
}

// Aceitação crítica: env value secret-like nunca vaza.
func TestAcceptanceMultipleEnvSecretsNeverLeak(t *testing.T) {
	r := New()
	// Literal quebrado em duas partes de propósito: evita casar o padrão
	// de secret scanner (ex. GitHub push protection) no texto-fonte, sem
	// mudar o valor concatenado em runtime que o teste realmente exercita.
	input := "DATABASE_PASSWORD=hunter2\n" +
		"STRIPE_API_KEY=sk_live_" + "aaaabbbbccccddddeeeeffff\n" +
		"GITHUB_TOKEN=ghp_abcdef0123456789abcdef0123456789abcd\n"
	out := r.Redact(input)
	for _, leak := range []string{"hunter2", "sk_live_aaaabbbb", "ghp_abcdef"} {
		if strings.Contains(out, leak) {
			t.Errorf("CRÍTICO: %q vazou em:\n%s", leak, out)
		}
	}
}

// Aceitação: Authorization Bearer é redactado, header preservado.
func TestRedactAuthorizationBearer(t *testing.T) {
	r := New()
	out := r.Redact(`Authorization: Bearer ghp_abcdef0123456789abcdef0123456789abcd`)
	if strings.Contains(out, "ghp_abcdef") {
		t.Errorf("token vazou: %q", out)
	}
	if !strings.Contains(out, "Authorization") {
		t.Errorf("header name sumiu: %q", out)
	}
	if !strings.Contains(out, "Bearer") {
		t.Errorf("Bearer sumiu: %q", out)
	}
}

// Aceitação: Authorization Basic é redactado.
func TestRedactAuthorizationBasic(t *testing.T) {
	r := New()
	out := r.Redact(`Authorization: Basic dXNlcjpwYXNz`)
	if strings.Contains(out, "dXNlcjpwYXNz") {
		t.Errorf("basic vazou: %q", out)
	}
}

// Aceitação: Cookie value é redactado.
func TestRedactCookie(t *testing.T) {
	r := New()
	out := r.Redact(`Cookie: session=abc123def456`)
	if strings.Contains(out, "abc123def456") {
		t.Errorf("cookie vazou: %q", out)
	}
	if !strings.Contains(out, "Cookie:") {
		t.Errorf("header sumiu: %q", out)
	}
}

// Aceitação: JWT é redactado.
func TestRedactJWT(t *testing.T) {
	r := New()
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	out := r.Redact("header=" + jwt)
	if strings.Contains(out, jwt) {
		t.Errorf("JWT vazou: %q", out)
	}
	if !strings.Contains(out, "[REDACTED:jwt]") {
		t.Errorf("esperava placeholder jwt em %q", out)
	}
}

// Aceitação: GitHub PAT (ghp_) é redactado.
func TestRedactGitHubPAT(t *testing.T) {
	r := New()
	pat := "ghp_abcdef0123456789abcdef0123456789abcd"
	out := r.Redact("token=" + pat)
	if strings.Contains(out, pat) {
		t.Errorf("PAT vazou: %q", out)
	}
}

// Aceitação: Slack token xoxb-... é redactado.
func TestRedactSlackToken(t *testing.T) {
	r := New()
	tok := "xoxb-" + "1234567890-abcdefghijklmnop"
	out := r.Redact("slack=" + tok)
	if strings.Contains(out, tok) {
		t.Errorf("Slack vazou: %q", out)
	}
}

// Aceitação: AWS Access Key ID é redactado.
func TestRedactAWSAKID(t *testing.T) {
	r := New()
	key := "AKIAIOSFODNN7EXAMPLE"
	out := r.Redact("aws_key=" + key)
	if strings.Contains(out, key) {
		t.Errorf("AWS vazou: %q", out)
	}
}

// Aceitação: chave privada PEM completa é redactada.
func TestRedactPrivateKeyBlock(t *testing.T) {
	r := New()
	pem := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAxxxx
-----END RSA PRIVATE KEY-----`
	out := r.Redact(pem)
	if strings.Contains(out, "MIIEpAIBAAKCAQEAxxxx") {
		t.Errorf("PEM vazou: %q", out)
	}
	if !strings.Contains(out, "[REDACTED:private-key-block]") {
		t.Errorf("esperava placeholder private-key-block: %q", out)
	}
}

// Aceitação: Postgres URL com password é redactado, user preservado.
func TestRedactPostgresURL(t *testing.T) {
	r := New()
	out := r.Redact("postgres://app:hunter2@db:5432/prod")
	if strings.Contains(out, "hunter2") {
		t.Errorf("pg password vazou: %q", out)
	}
	if !strings.Contains(out, "app") {
		t.Errorf("user sumiu: %q", out)
	}
	if !strings.Contains(out, "db:5432") {
		t.Errorf("host sumiu: %q", out)
	}
}

// Aceitação: MongoDB URL com password.
func TestRedactMongoURL(t *testing.T) {
	r := New()
	out := r.Redact("mongodb://app:hunter2@mongo:27017/db")
	if strings.Contains(out, "hunter2") {
		t.Errorf("mongo password vazou: %q", out)
	}
}

// Aceitação: secret paths são redactados.
func TestRedactSecretPath(t *testing.T) {
	r := New()
	cases := []string{
		"/var/run/secrets/kubernetes.io/serviceaccount/token",
		"/etc/ssl/private/server.key",
		"/run/secrets/myapp/credentials",
		".docker/secrets/db_password",
	}
	for _, p := range cases {
		out := r.Redact("mount=" + p)
		if strings.Contains(out, p) {
			t.Errorf("path vazou: %q em %q", p, out)
		}
	}
}

// Aceitação: Stripe live key é redactado.
func TestRedactStripeKey(t *testing.T) {
	r := New()
	key := "sk_live_" + "abcdefghijklmnopqrstuvwx"
	out := r.Redact("stripe=" + key)
	if strings.Contains(out, key) {
		t.Errorf("Stripe vazou: %q", out)
	}
}

// Aceitação: Google API key é redactado.
func TestRedactGoogleAPIKey(t *testing.T) {
	r := New()
	key := "AIzaSyA-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456"
	out := r.Redact("google=" + key)
	if strings.Contains(out, key) {
		t.Errorf("Google API key vazou: %q", out)
	}
}

// Aceitação: env-style KEY=value preserva KEY, redacta VALUE.
func TestRedactEnvStylePreservesKey(t *testing.T) {
	r := New()
	out := r.Redact("PAYMENT_TOKEN=abc123")
	if !strings.Contains(out, "PAYMENT_TOKEN=") {
		t.Errorf("KEY não preservada: %q", out)
	}
	if strings.Contains(out, "abc123") {
		t.Errorf("value vazou: %q", out)
	}
}

// Aceitação: env value de KEY não-secret-like NÃO é redactado.
func TestRedactNonSecretEnvValueUntouched(t *testing.T) {
	r := New()
	out := r.Redact("PORT=8080")
	if !strings.Contains(out, "PORT=8080") {
		t.Errorf("PORT não devia ser redactado: %q", out)
	}
	if containsRedacted(out) {
		t.Errorf("esperava sem redação em %q", out)
	}
}

// Aceitação: RedactEnv atalho para formato env.
func TestRedactEnvShortcut(t *testing.T) {
	out := RedactEnvLine("PAYMENT_TOKEN=abc123")
	if strings.Contains(out, "abc123") {
		t.Errorf("RedactEnv vazou: %q", out)
	}
	if !strings.Contains(out, "PAYMENT_TOKEN=") {
		t.Errorf("RedactEnv não preservou key: %q", out)
	}
}

// Aceitação: RedactEnv não toca em KEY não-segredo.
func TestRedactEnvShortcutNonSecret(t *testing.T) {
	out := RedactEnvLine("PORT=8080")
	if out != "PORT=8080" {
		t.Errorf("PORT alterado: %q", out)
	}
}

// Aceitação: case-insensitive para nomes secret-like.
func TestRedactEnvCaseInsensitive(t *testing.T) {
	cases := []string{
		"payment_token=abc",
		"Payment_Token=abc",
		"PAYMENT_token=abc",
	}
	for _, c := range cases {
		out := New().Redact(c)
		if strings.Contains(out, "abc") {
			t.Errorf("case-insensitive falhou: %q → %q", c, out)
		}
	}
}

// Aceitação: múltiplos segredos no mesmo texto, todos redactados.
func TestRedactMultipleInOneText(t *testing.T) {
	r := New()
	// Mesma quebra proposital de literal citada acima (secret scanner).
	input := "Config:\n" +
		"DATABASE_PASSWORD=hunter2\n" +
		"API_KEY=secret-api-key\n" +
		"GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n" +
		"Authorization: Bearer ghp_yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy\n" +
		"JWT=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig\n" +
		"Stripe: sk_live_" + "xxxxxxxxxxxxxxxxxxxxxxxxxxxx\n" +
		"postgres://u:p@host/db\n"
	out := r.Redact(input)
	for _, leak := range []string{
		"hunter2", "secret-api-key",
		"ghp_xxxxxxxxx", "ghp_yyyyyyy",
		"eyJhbGciOiJIUzI1NiJ9",
		"sk_live_xxxxx",
		":p@",
	} {
		if strings.Contains(out, leak) {
			t.Errorf("vazou %q em:\n%s", leak, out)
		}
	}
}

// Aceitação: texto sem segredos é preservado.
func TestRedactNoSecretsUntouched(t *testing.T) {
	r := New()
	input := `Hello world!
This is a normal log message.
No secrets here.`
	out := r.Redact(input)
	if out != input {
		t.Errorf("alterado: %q → %q", input, out)
	}
}

// Aceitação: vazio retorna vazio.
func TestRedactEmpty(t *testing.T) {
	if out := New().Redact(""); out != "" {
		t.Errorf("vazio: %q", out)
	}
}

// Aceitação: IsLikelySecretName reconhece sufixos comuns.
func TestIsLikelySecretName(t *testing.T) {
	cases := map[string]bool{
		"API_KEY":      true,
		"db_password":  true,
		"access_token": true,
		"secret_base":  true,
		"PORT":         false,
		"DEBUG":        false,
		"LOG_LEVEL":    false,
		"DATABASE_URL": false,
	}
	for name, want := range cases {
		if got := IsLikelySecretName(name); got != want {
			t.Errorf("%s: got %v, quero %v", name, got, want)
		}
	}
}

// Aceitação: Authorization Bearer mas com placeholder já redactado: idempotente?
// Não precisa ser idempotente — o redactor pode ser chamado várias vezes
// sem causar duplicação perigosa (vai continuar a redactar a string
// resultante que pode já ter [REDACTED:...]).
func TestRedactRepeatedCallsAreSafe(t *testing.T) {
	r := New()
	s := "PAYMENT_TOKEN=abc123"
	once := r.Redact(s)
	twice := r.Redact(once)
	if strings.Contains(twice, "abc123") {
		t.Errorf("segunda passada vazou: %q", twice)
	}
	// Segunda passada mantém o redacted, sem criar artefatos estranhos.
	if !strings.Contains(twice, "[REDACTED:") {
		t.Errorf("segunda passada perdeu placeholder: %q", twice)
	}
}

// Aceitação: Private key block — múltiplas chaves no mesmo texto.
func TestRedactMultiplePrivateKeys(t *testing.T) {
	r := New()
	pem1 := `-----BEGIN RSA PRIVATE KEY-----
MIIE1
-----END RSA PRIVATE KEY-----`
	pem2 := `-----BEGIN EC PRIVATE KEY-----
MHcCAQE
-----END EC PRIVATE KEY-----`
	out := r.Redact(pem1 + "\n" + pem2)
	if strings.Contains(out, "MIIE1") || strings.Contains(out, "MHcCAQE") {
		t.Errorf("chave vazou: %q", out)
	}
}

// Aceitação: Bearer com case misto (auTHorization).
func TestRedactAuthCaseInsensitive(t *testing.T) {
	r := New()
	out := r.Redact("auTHorization: beARer my-token-12345")
	if strings.Contains(out, "my-token-12345") {
		t.Errorf("case-insensitive falhou: %q", out)
	}
}

// Aceitação: env-style com export prefix (shell).
func TestRedactEnvWithExport(t *testing.T) {
	r := New()
	out := r.Redact(`export PAYMENT_TOKEN=abc123`)
	if strings.Contains(out, "abc123") {
		t.Errorf("export prefix: vazou: %q", out)
	}
}

// Aceitação: yaml-style KEY: value.
func TestRedactYamlStyle(t *testing.T) {
	r := New()
	out := r.Redact("password: hunter2")
	if strings.Contains(out, "hunter2") {
		t.Errorf("yaml vazou: %q", out)
	}
}

// Aceitação: Slack legacy xoxp- prefix.
func TestRedactSlackLegacy(t *testing.T) {
	r := New()
	tok := "xoxp-1234567890-abcdefghijklmnopqrstuvwx"
	out := r.Redact("token=" + tok)
	if strings.Contains(out, tok) {
		t.Errorf("Slack vazou: %q", out)
	}
}

// Aceitação: zero-config — New() não panica, retorna redactor funcional.
func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() = nil")
	}
	// sanity: funciona.
	_ = r.Redact("API_KEY=secret")
}

// Aceitação: redaction não quebra em conteúdo binário/aleatório.
func TestRedactBinarySafe(t *testing.T) {
	r := New()
	// Texto com bytes estranhos (mas não zero).
	input := "log line\x00with\x01binary\x02bytes"
	out := r.Redact(input)
	if strings.Contains(out, "API_KEY") || containsRedacted(out) {
		t.Errorf("binário alterado indevidamente: %q", out)
	}
}
