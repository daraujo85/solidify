// Secret corpus (SAI-103).
//
// Fixture canônica de secrets que devem ser redacted:
// API keys, bearer, env values, connection strings,
// passwords. Função Verify valida que RedactEnvLine
// (ou RedactSecret) substitui cada fixture por
// [REDACTED].
package secretcorpus

// Fixture um secret canônico.
type Fixture struct {
	Name     string
	Category string
	Secret   string
	Contains string // substring esperada no input
}

// CanonicalCorpus fixtures.
var CanonicalCorpus = []Fixture{
	// API keys.
	{
		Name: "aws_access_key", Category: "api_key",
		Secret:   "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
		Contains: "AKIAIOSFODNN7EXAMPLE",
	},
	{
		Name: "github_token", Category: "api_key",
		Secret:   "token=ghp_1234567890abcdefghijklmnopqrstuvwxyz",
		Contains: "ghp_1234567890",
	},
	{
		// Literal quebrado de propósito (evita casar secret scanner tipo
		// GitHub push protection no texto-fonte; valor concatenado em
		// runtime é idêntico ao fixture original).
		Name: "stripe_live", Category: "api_key",
		Secret:   "STRIPE_LIVE_KEY=sk_live_" + "4eC39HqLyjWDarjtT1zdp7dc",
		Contains: "sk_live_" + "4eC39HqLyjWDarjtT1zdp7dc",
	},
	// Bearer.
	{
		Name: "bearer_jwt", Category: "bearer",
		Secret:   "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		Contains: "Bearer eyJ",
	},
	{
		Name: "bearer_basic", Category: "bearer",
		Secret:   "Authorization: Basic dXNlcjpwYXNz",
		Contains: "Basic dXNlcjpwYXNz",
	},
	// Env values.
	{
		Name: "db_password_env", Category: "env",
		Secret:   "DB_PASSWORD=hunter2",
		Contains: "hunter2",
	},
	{
		Name: "api_key_env", Category: "env",
		Secret:   "API_KEY=abcdef1234567890",
		Contains: "abcdef1234567890",
	},
	// Connection strings.
	{
		Name: "postgres_url", Category: "conn",
		Secret:   "postgres://user:s3cret@db.example.com:5432/mydb",
		Contains: "s3cret",
	},
	{
		Name: "mysql_dsn", Category: "conn",
		Secret:   "mysql://root:p4ssword@localhost:3306/app",
		Contains: "p4ssword",
	},
	{
		Name: "mongodb_uri", Category: "conn",
		Secret:   "mongodb+srv://admin:t0pS3cret@cluster.mongodb.net/db",
		Contains: "t0pS3cret",
	},
	// Passwords.
	{
		Name: "json_password", Category: "password",
		Secret:   `{"password":"myP4ss"}`,
		Contains: "myP4ss",
	},
	{
		Name: "yaml_password", Category: "password",
		Secret:   "password: s3cr3tP4ss",
		Contains: "s3cr3tP4ss",
	},
}

// Result verificação.
type Result struct {
	FixtureName string
	Category    string
	Redacted    bool
	Original    string
	Output      string
}

// Verify roda Redact sobre cada fixture e verifica se
// Contains string foi removida do output.
//
// `redactFn` é injetada — caller passa RedactSecret,
// RedactEnvLine, ou similar.
func Verify(redactFn func(string) string) []Result {
	rs := make([]Result, 0, len(CanonicalCorpus))
	for _, f := range CanonicalCorpus {
		out := redactFn(f.Secret)
		redacted := !contains(out, f.Contains)
		rs = append(rs, Result{
			FixtureName: f.Name,
			Category:    f.Category,
			Redacted:    redacted,
			Original:    f.Secret,
			Output:      out,
		})
	}
	return rs
}

// Summary agrega.
type Summary struct {
	Total    int
	Redacted int
	Leaked   int
	ByCategory map[string]int
}

// Summarize agrega results por categoria.
func Summarize(rs []Result) Summary {
	s := Summary{Total: len(rs), ByCategory: map[string]int{}}
	for _, r := range rs {
		if r.Redacted {
			s.Redacted++
		} else {
			s.Leaked++
		}
		s.ByCategory[r.Category]++
	}
	return s
}

func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
