// Package envx — detector de uso de variáveis de ambiente multi-stack.
//
// Implementa ARCHITECTURE.md §11.2. Padrões suportados:
//
//	Node/Vite: process.env.X, import.meta.env.X
//	Go: os.Getenv, os.LookupEnv
//	.NET: Environment.GetEnvironmentVariable
//	Java/Spring: @Value("${X}"), ${X}, Environment.getProperty
//	Python: os.environ, os.getenv
//	PHP: getenv, $_ENV, $_SERVER, env()
//	Ruby: ENV[], ENV.fetch
//	Dart: String.fromEnvironment, Platform.environment
//	Elixir: System.get_env
//	Rust: std::env::var, env::var
//
// Regra de segurança §11.1: a ferramenta NÃO carrega valores; só
// captura o NOME da variável.
package envx

import (
	"regexp"
	"strings"
)

// Form é a forma canônica da referência (ex.: "process.env",
// "os.Getenv"). Útil pra apresentação.
type Form string

const (
	FormProcessEnv    Form = "process.env"
	FormImportMetaEnv Form = "import.meta.env"
	FormOSGetenv      Form = "os.Getenv"
	FormOSLookupEnv   Form = "os.LookupEnv"
	FormDotnetEnvVar  Form = "Environment.GetEnvironmentVariable"
	FormSpringValue   Form = "@Value"
	FormSpringBrace   Form = "${}"
	FormJavaEnvProp   Form = "Environment.getProperty"
	FormOSEnviron     Form = "os.environ"
	FormOSGetenvPy    Form = "os.getenv"
	FormPHPGetenv     Form = "getenv"
	FormPHPEnvArray   Form = "$_ENV"
	FormPHPLaravel    Form = "env()"
	FormRubyENV       Form = "ENV[]"
	FormRubyFetch     Form = "ENV.fetch"
	FormDartFromEnv   Form = "String.fromEnvironment"
	FormDartPlatform  Form = "Platform.environment"
	FormElixirSystem  Form = "System.get_env"
	FormRustEnvVar    Form = "env::var"
)

// Usage é uma referência detectada a uma variável de ambiente.
type Usage struct {
	Name       string // nome da env var
	Form       Form   // forma canônica
	Path       string // path do arquivo
	Line       int    // linha 1-based
	HasDefault bool   // true se a chamada traz default (ex.: os.environ.get(X, Y))
}

// Detect devolve as usages encontradas no conteúdo src de um arquivo
// identificado por path. Aceita o conteúdo bruto; não chama nada
// externo (não carrega valores).
func Detect(path string, src []byte) []Usage {
	var out []Usage
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		for _, p := range patterns {
			for _, m := range p.re.FindAllStringSubmatch(line, -1) {
				name := extractName(m, p.nameIdx)
				if !validEnvName(name, p.allowDots) {
					continue
				}
				out = append(out, Usage{
					Name:       name,
					Form:       p.form,
					Path:       path,
					Line:       i + 1,
					HasDefault: p.hasDefault && strings.Contains(m[0], ","),
				})
			}
		}
	}
	// Dedupe por (path, name, line) — @Value("${X}") também casa como
	// ${X} (SpringBrace); queremos só o mais específico (primeiro
	// match vence, e patterns é ordenado com @Value antes de ${}).
	seen := make(map[string]bool, len(out))
	dedup := out[:0]
	for _, u := range out {
		key := u.Path + "|" + u.Name + "|" + intToStr(u.Line)
		if seen[key] {
			continue
		}
		seen[key] = true
		dedup = append(dedup, u)
	}
	return dedup
}

// pattern é uma regra de detecção para uma forma.
type pattern struct {
	form       Form
	re         *regexp.Regexp
	nameIdx    int  // índice do primeiro grupo que captura o nome
	hasDefault bool // se a forma pode ter default (ex.: os.environ.get(X, Y))
	allowDots  bool // se o nome pode ter '.' ou '-'
}

// envName identifica nomes "compatíveis" — letras/dígitos/_ começando
// por letra/_ (padrão POSIX-ish).
var envNameSimple = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// envNameDotted aceita '.' e '-' extras (Spring, kebab-case).
var envNameDotted = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.\-]*$`)

// validEnvName devolve true se s parece um nome de env var válido.
func validEnvName(s string, dotted bool) bool {
	if s == "" {
		return false
	}
	if dotted {
		return envNameDotted.MatchString(s)
	}
	return envNameSimple.MatchString(s)
}

// Padrões compilados. Cada regex captura o NOME no grupo nameIdx
// (geralmente 1). Para formas que aceitam tanto identificador (.X)
// quanto string ("X" / 'X'), a regex cobre os dois.
//
// NOTA: \bENV é usado para RubyENV para que $_ENV (PHP) NÃO case
// como Ruby — `_` é word char e bloqueia \b.
var patterns = []pattern{
	// Node: process.env.X ou process.env["X"] / ['X'].
	{form: FormProcessEnv, nameIdx: 1,
		re: regexp.MustCompile(`process\.env(?:\.([A-Za-z_][A-Za-z0-9_]*)|\[\s*(?:"([^"]+)"|'([^']+)')\s*\])`)},
	// Vite: import.meta.env.X.
	{form: FormImportMetaEnv, nameIdx: 1,
		re: regexp.MustCompile(`import\.meta\.env(?:\.([A-Za-z_][A-Za-z0-9_]*)|\[\s*(?:"([^"]+)"|'([^']+)')\s*\])`)},
	// Go: os.Getenv("X"), os.LookupEnv("X").
	{form: FormOSGetenv, nameIdx: 1,
		re: regexp.MustCompile(`os\.Getenv\(\s*(?:"([^"]+)"|'([^']+)')\s*\)`)},
	{form: FormOSLookupEnv, nameIdx: 1,
		re: regexp.MustCompile(`os\.LookupEnv\(\s*(?:"([^"]+)"|'([^']+)')\s*\)`)},
	// .NET: Environment.GetEnvironmentVariable("X").
	{form: FormDotnetEnvVar, nameIdx: 1,
		re: regexp.MustCompile(`Environment\.GetEnvironmentVariable\(\s*(?:"([^"]+)"|'([^']+)')\s*\)`)},
	// Spring: @Value("${X}") — Spring permite dots/hífens.
	{form: FormSpringValue, nameIdx: 1, allowDots: true,
		re: regexp.MustCompile(`@Value\s*\(\s*(?:"|')\$\{([A-Za-z_][A-Za-z0-9_.\-]*)\}(?:"|')\s*\)`)},
	// Spring/Yaml: ${X} em arquivos .yml/.properties. allowDots para
	// Spring placeholders tipo ${spring.datasource.url}.
	{form: FormSpringBrace, nameIdx: 1, allowDots: true,
		re: regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_.\-]*)\}`)},
	// Java: System.getenv("X") e Environment.getProperty("X").
	// allowDots=true para Spring property placeholders tipo
	// app.datasource.url.
	{form: FormJavaEnvProp, nameIdx: 1, allowDots: true,
		re: regexp.MustCompile(`(?:System\.getenv|Environment\.getProperty)\(\s*(?:"([^"]+)"|'([^']+)')\s*\)`)},
	// Python: os.environ["X"] / ['X'].
	{form: FormOSEnviron, nameIdx: 1,
		re: regexp.MustCompile(`os\.environ\[\s*(?:"([^"]+)"|'([^']+)')\s*\]`)},
	// Python: os.environ.get("X", default).
	{form: FormOSEnviron, nameIdx: 1, hasDefault: true,
		re: regexp.MustCompile(`os\.environ\.get\(\s*(?:"([^"]+)"|'([^']+)')\s*[^)]*\)`)},
	// Python: os.getenv("X", default).
	{form: FormOSGetenvPy, nameIdx: 1, hasDefault: true,
		re: regexp.MustCompile(`os\.getenv\(\s*(?:"([^"]+)"|'([^']+)')\s*[^)]*\)`)},
	// PHP: getenv("X"). (?:^|[^.]) evita matchar System.getenv
	// (Java/Kotlin) que tem '.' antes. Go regex não tem lookbehind,
	// então consumimos 1 char antes.
	{form: FormPHPGetenv, nameIdx: 1,
		re: regexp.MustCompile(`(?:^|[^.])getenv\(\s*(?:"([^"]+)"|'([^']+)')\s*\)`)},
	// PHP: $_ENV["X"], $_SERVER["X"], $_ENV['X'].
	{form: FormPHPEnvArray, nameIdx: 1,
		re: regexp.MustCompile(`\$_(?:ENV|SERVER)\[\s*(?:"([^"]+)"|'([^']+)')\s*\]`)},
	// Laravel: env("X", default).
	{form: FormPHPLaravel, nameIdx: 1, hasDefault: true,
		re: regexp.MustCompile(`\benv\(\s*(?:"([^"]+)"|'([^']+)')\s*[^)]*\)`)},
	// Ruby: ENV["X"]. \bENV garante que $_ENV NÃO case.
	{form: FormRubyENV, nameIdx: 1,
		re: regexp.MustCompile(`\bENV\[\s*(?:"([^"]+)"|'([^']+)')\s*\]`)},
	// Ruby: ENV.fetch("X").
	{form: FormRubyFetch, nameIdx: 1,
		re: regexp.MustCompile(`\bENV\.fetch\(\s*(?:"([^"]+)"|'([^']+)')\s*\)`)},
	// Dart: String.fromEnvironment("X").
	{form: FormDartFromEnv, nameIdx: 1,
		re: regexp.MustCompile(`String\.fromEnvironment\(\s*(?:"([^"]+)"|'([^']+)')\s*[,)]`)},
	// Dart: Platform.environment["X"].
	{form: FormDartPlatform, nameIdx: 1,
		re: regexp.MustCompile(`Platform\.environment\[\s*(?:"([^"]+)"|'([^']+)')\s*\]`)},
	// Elixir: System.get_env("X").
	{form: FormElixirSystem, nameIdx: 1,
		re: regexp.MustCompile(`System\.get_env\(\s*(?:"([^"]+)"|'([^']+)')\s*\)`)},
	// Rust: std::env::var("X"), std::env::var_os("X"), env::var("X").
	{form: FormRustEnvVar, nameIdx: 1,
		re: regexp.MustCompile(`(?:std::)?env::var(?:_os)?\(\s*(?:"([^"]+)"|'([^']+)')\s*\)`)},
}

// extractName devolve o primeiro grupo não-vazio a partir de nameIdx.
func extractName(m []string, nameIdx int) string {
	for j := nameIdx; j < len(m); j++ {
		if m[j] != "" {
			return m[j]
		}
	}
	return ""
}

// intToStr converte int para string sem importar strconv só pra isso.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
