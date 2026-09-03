package envx

import (
	"sort"
	"strings"
	"testing"
)

// collect devolve os nomes detectados, ordenados, em uma única linha.
func collect(usages []Usage) []string {
	out := make([]string, 0, len(usages))
	for _, u := range usages {
		out = append(out, u.Name+":"+string(u.Form))
	}
	sort.Strings(out)
	return out
}

func findByName(usages []Usage, name string) *Usage {
	for i := range usages {
		if usages[i].Name == name {
			return &usages[i]
		}
	}
	return nil
}

// Aceitação: Node process.env.NOME.
func TestDetectNodeProcessEnv(t *testing.T) {
	src := []byte(`const x = process.env.API_KEY;`)
	us := Detect("app.js", src)
	if len(us) != 1 || us[0].Name != "API_KEY" || us[0].Form != FormProcessEnv {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Node process.env["NOME"].
func TestDetectNodeProcessEnvString(t *testing.T) {
	src := []byte(`const x = process.env["API_KEY"];`)
	us := Detect("app.js", src)
	if len(us) != 1 || us[0].Name != "API_KEY" {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Vite import.meta.env.
func TestDetectViteImportMetaEnv(t *testing.T) {
	src := []byte(`const url = import.meta.env.VITE_API_URL;`)
	us := Detect("app.js", src)
	if len(us) != 1 || us[0].Name != "VITE_API_URL" || us[0].Form != FormImportMetaEnv {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Go os.Getenv.
func TestDetectGoOSGetenv(t *testing.T) {
	src := []byte(`v := os.Getenv("DATABASE_URL")`)
	us := Detect("main.go", src)
	if len(us) != 1 || us[0].Name != "DATABASE_URL" || us[0].Form != FormOSGetenv {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Go os.LookupEnv.
func TestDetectGoOSLookupEnv(t *testing.T) {
	src := []byte(`v, ok := os.LookupEnv("DEBUG")`)
	us := Detect("main.go", src)
	if len(us) != 1 || us[0].Name != "DEBUG" || us[0].Form != FormOSLookupEnv {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: .NET Environment.GetEnvironmentVariable.
func TestDetectDotnetEnv(t *testing.T) {
	src := []byte(`var x = Environment.GetEnvironmentVariable("ASPNETCORE_ENVIRONMENT");`)
	us := Detect("Program.cs", src)
	if len(us) != 1 || us[0].Name != "ASPNETCORE_ENVIRONMENT" || us[0].Form != FormDotnetEnvVar {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Spring @Value("${X}").
func TestDetectSpringValue(t *testing.T) {
	src := []byte(`@Value("${spring.datasource.url}")`)
	us := Detect("App.java", src)
	if len(us) != 1 || us[0].Name != "spring.datasource.url" || us[0].Form != FormSpringValue {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Spring ${X} em YAML (forma brace).
func TestDetectSpringBraceInYaml(t *testing.T) {
	src := []byte(`url: jdbc:postgresql://${DB_HOST}:5432/${DB_NAME}`)
	us := Detect("application.yml", src)
	names := collect(us)
	want := []string{"DB_HOST:" + string(FormSpringBrace), "DB_NAME:" + string(FormSpringBrace)}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, quero %v", names, want)
	}
}

// Aceitação: Java Environment.getProperty / System.getenv.
func TestDetectJavaSystem(t *testing.T) {
	src := []byte(`
String a = System.getenv("JAVA_HOME");
String b = Environment.getProperty("app.region");
`)
	us := Detect("App.java", src)
	names := []string{}
	for _, u := range us {
		names = append(names, u.Name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "JAVA_HOME,app.region" {
		t.Errorf("got %v", names)
	}
}

// Aceitação: Python os.environ["X"].
func TestDetectPythonOsEnviron(t *testing.T) {
	src := []byte(`db = os.environ["DATABASE_URL"]`)
	us := Detect("app.py", src)
	if len(us) != 1 || us[0].Name != "DATABASE_URL" || us[0].Form != FormOSEnviron {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Python os.environ.get tem HasDefault.
func TestDetectPythonOsEnvironGetHasDefault(t *testing.T) {
	src := []byte(`port = os.environ.get("PORT", "8000")`)
	us := Detect("app.py", src)
	if len(us) != 1 || !us[0].HasDefault {
		t.Errorf("devia ter HasDefault: %+v", us)
	}
}

// Aceitação: Python os.environ sem default não tem HasDefault.
func TestDetectPythonOsEnvironNoDefault(t *testing.T) {
	src := []byte(`port = os.environ["PORT"]`)
	us := Detect("app.py", src)
	if len(us) != 1 || us[0].HasDefault {
		t.Errorf("não devia ter HasDefault: %+v", us)
	}
}

// Aceitação: Python os.getenv.
func TestDetectPythonGetenv(t *testing.T) {
	src := []byte(`key = os.getenv("API_KEY", "default")`)
	us := Detect("app.py", src)
	if len(us) != 1 || us[0].Form != FormOSGetenvPy {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: PHP getenv.
func TestDetectPHPGetenv(t *testing.T) {
	src := []byte(`$secret = getenv("APP_SECRET");`)
	us := Detect("app.php", src)
	if len(us) != 1 || us[0].Name != "APP_SECRET" || us[0].Form != FormPHPGetenv {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: PHP $_ENV e $_SERVER.
func TestDetectPHPEnvArray(t *testing.T) {
	src := []byte(`
$v1 = $_ENV["APP_ENV"];
$v2 = $_SERVER["HTTP_HOST"];
`)
	us := Detect("app.php", src)
	if len(us) != 2 {
		t.Fatalf("got %+v", us)
	}
	for _, u := range us {
		if u.Form != FormPHPEnvArray {
			t.Errorf("Form = %s, quero PHPEnvArray", u.Form)
		}
	}
}

// Aceitação: Laravel env() com default.
func TestDetectPHPLaravelEnv(t *testing.T) {
	src := []byte(`$debug = env("APP_DEBUG", false);`)
	us := Detect("app.php", src)
	if len(us) != 1 || !us[0].HasDefault || us[0].Form != FormPHPLaravel {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Ruby ENV[] e ENV.fetch.
func TestDetectRubyENV(t *testing.T) {
	src := []byte(`
Rails.env = ENV["RAILS_ENV"] || "development"
db = ENV.fetch("DATABASE_URL")
`)
	us := Detect("app.rb", src)
	if len(us) != 2 {
		t.Fatalf("got %+v", us)
	}
	names := map[string]Form{}
	for _, u := range us {
		names[u.Name] = u.Form
	}
	if names["RAILS_ENV"] != FormRubyENV {
		t.Errorf("RAILS_ENV form = %s, quero %s", names["RAILS_ENV"], FormRubyENV)
	}
	if names["DATABASE_URL"] != FormRubyFetch {
		t.Errorf("DATABASE_URL form = %s, quero %s", names["DATABASE_URL"], FormRubyFetch)
	}
}

// Aceitação: Dart String.fromEnvironment.
func TestDetectDartFromEnv(t *testing.T) {
	src := []byte(`const apiUrl = String.fromEnvironment("API_URL");`)
	us := Detect("app.dart", src)
	if len(us) != 1 || us[0].Name != "API_URL" || us[0].Form != FormDartFromEnv {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Dart Platform.environment.
func TestDetectDartPlatformEnv(t *testing.T) {
	src := []byte(`final path = Platform.environment["PATH"];`)
	us := Detect("app.dart", src)
	if len(us) != 1 || us[0].Name != "PATH" || us[0].Form != FormDartPlatform {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Elixir System.get_env.
func TestDetectElixirSystemGetEnv(t *testing.T) {
	src := []byte(`secret = System.get_env("SECRET_KEY_BASE")`)
	us := Detect("app.ex", src)
	if len(us) != 1 || us[0].Name != "SECRET_KEY_BASE" || us[0].Form != FormElixirSystem {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Rust env::var.
func TestDetectRustEnvVar(t *testing.T) {
	src := []byte(`
let key = std::env::var("API_KEY")?;
let path = env::var("PATH")?;
`)
	us := Detect("main.rs", src)
	if len(us) != 2 {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: não chama nada externo (não carrega valores).
// Verifica que o nome detectado é literalmente o que estava no source,
// não valor real.
func TestDetectNeverLoadsValues(t *testing.T) {
	// Mesmo se a env var existisse no processo, o detector não a leria.
	t.Setenv("DEFINITELY_NOT_A_SECRET_FROM_OS", "should-not-appear")
	src := []byte(`const x = process.env["DEFINITELY_A_SECRET_FROM_OS"];`)
	us := Detect("app.js", src)
	if len(us) != 1 || us[0].Name != "DEFINITELY_A_SECRET_FROM_OS" {
		t.Errorf("não devia carregar valor: %+v", us)
	}
}

// Aceitação: múltiplas referências na mesma linha.
func TestDetectMultipleInSameLine(t *testing.T) {
	src := []byte(`a := os.Getenv("A"); b := os.Getenv("B");`)
	us := Detect("main.go", src)
	if len(us) != 2 {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: várias linhas, path e line corretos.
func TestDetectLineNumbers(t *testing.T) {
	src := []byte(`line1
line2 with os.Getenv("FOO")
line3
line4 with os.Getenv("BAR")
`)
	us := Detect("main.go", src)
	if len(us) != 2 {
		t.Fatalf("got %+v", us)
	}
	if us[0].Line != 2 || us[1].Line != 4 {
		t.Errorf("lines = %d, %d", us[0].Line, us[1].Line)
	}
}

// Aceitação: source sem env refs devolve vazio.
func TestDetectNoMatches(t *testing.T) {
	src := []byte(`print("hello world")`)
	us := Detect("app.py", src)
	if len(us) != 0 {
		t.Errorf("esperava vazio, got %+v", us)
	}
}

// Aceitação: aspas simples em JS/Python.
func TestDetectSingleQuoteString(t *testing.T) {
	src := []byte(`x = os.environ['PORT']`)
	us := Detect("app.py", src)
	if len(us) != 1 || us[0].Name != "PORT" {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: env var com dots (Spring style).
func TestDetectSpringDottedName(t *testing.T) {
	src := []byte(`@Value("${spring.datasource.url}")`)
	us := Detect("App.java", src)
	if len(us) != 1 || us[0].Name != "spring.datasource.url" {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: env var com underscore em qualquer stack.
func TestDetectUnderscoreName(t *testing.T) {
	src := []byte(`x = os.environ["MY_LONG_VAR_NAME_123"]`)
	us := Detect("app.py", src)
	if len(us) != 1 || us[0].Name != "MY_LONG_VAR_NAME_123" {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: caminho do arquivo é preservado.
func TestDetectPathIsSet(t *testing.T) {
	src := []byte(`x = process.env.NODE_ENV`)
	us := Detect("apps/web/src/config.ts", src)
	if len(us) != 1 || us[0].Path != "apps/web/src/config.ts" {
		t.Errorf("Path = %q", us[0].Path)
	}
}

// Aceitação: aspas duplas Ruby ENV[].
func TestDetectRubyDoubleQuote(t *testing.T) {
	src := []byte(`key = ENV["STRIPE_KEY"]`)
	us := Detect("app.rb", src)
	if len(us) != 1 || us[0].Name != "STRIPE_KEY" {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: Go Setenv detecta também.
func TestDetectGoSetenv(t *testing.T) {
	src := []byte(`os.Setenv("FOO", "bar")`)
	us := Detect("main.go", src)
	// Setenv é uma escrita, não leitura — não temos pattern pra ele
	// (não precisamos; o audit é sobre leitura). Confirma que não vaza.
	if len(us) != 0 {
		t.Errorf("Setenv não devia ser capturado: %+v", us)
	}
}

// Aceitação: path duplicado para o mesmo nome em arquivos diferentes
// gera entries separadas (sem dedup global).
func TestDetectMultipleFilesIndependent(t *testing.T) {
	src := []byte(`x = os.environ["SHARED"]`)
	u1 := Detect("a.py", src)
	u2 := Detect("b.py", src)
	if len(u1) != 1 || len(u2) != 1 {
		t.Fatalf("got %+v / %+v", u1, u2)
	}
	if u1[0].Path != "a.py" || u2[0].Path != "b.py" {
		t.Errorf("paths misturados")
	}
}

// Aceitação: padrão desconhecido não casa.
func TestDetectUnknownPatternIgnored(t *testing.T) {
	src := []byte(`x = "this is a random string with no env reference"`)
	us := Detect("app.py", src)
	if len(us) != 0 {
		t.Errorf("string puro: %+v", us)
	}
}

// Aceitação: ${X} em código JS (template literal) é detectado como brace.
// Útil para build configs (webpack DefinePlugin).
func TestDetectBraceInJS(t *testing.T) {
	src := []byte(`const url = "${API_URL}/v1/users";`)
	us := Detect("webpack.config.js", src)
	if len(us) != 1 || us[0].Name != "API_URL" {
		t.Fatalf("got %+v", us)
	}
}

// Aceitação: contagem total em arquivo multi-stack.
func TestDetectMixedStacks(t *testing.T) {
	src := []byte(`
const a = process.env.A;
v := os.Getenv("B")
$x = $_ENV["C"]
key = ENV["D"]
`)
	us := Detect("mixed.txt", src)
	if len(us) != 4 {
		t.Errorf("got %d: %+v", len(us), us)
	}
}

// Aceitação: pattern com nome não-env não captura.
func TestDetectRejectsInvalidName(t *testing.T) {
	src := []byte(`const x = os.environ["1INVALID"]`) // começa com dígito
	us := Detect("app.py", src)
	if len(us) != 0 {
		t.Errorf("nome inválido não devia casar: %+v", us)
	}
}

// Aceitação: detect com file vazio.
func TestDetectEmptyFile(t *testing.T) {
	us := Detect("empty.js", []byte{})
	if len(us) != 0 {
		t.Errorf("vazio: %+v", us)
	}
}

// Aceitação: nome kebab-case via Spring brace.
func TestDetectKebabCaseInBrace(t *testing.T) {
	src := []byte(`key: ${my-api-key}`)
	us := Detect("app.yml", src)
	if findByName(us, "my-api-key") == nil {
		t.Errorf("kebab-case não detectado: %+v", us)
	}
}
