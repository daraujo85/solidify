package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diegoaraujo/solidify/internal/errs"
)

// write grava um solidify.json num diretório temporário e devolve o dir.
func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("escrever config: %v", err)
	}
	return dir
}

const minimalConfig = `{
  "schema_version": "1.0.0",
  "project": {"name": "acme-api"}
}`

func TestDefaultsAreValid(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("defaults inválidos: %v", err)
	}
}

// Um arquivo mínimo herda todos os defaults sem declará-los.
func TestLoadMergesOverDefaults(t *testing.T) {
	loaded, err := Load(write(t, minimalConfig), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.FromFile {
		t.Error("FromFile deveria ser true")
	}
	if loaded.Config.Project.Name != "acme-api" {
		t.Errorf("name = %q", loaded.Config.Project.Name)
	}
	if got := loaded.Config.Project.DefaultBase; got != "origin/main" {
		t.Errorf("default_base = %q, quero o default origin/main", got)
	}
	if got := loaded.Config.Analysis.Context.ShardMaxChars; got != 60000 {
		t.Errorf("shard_max_chars = %d, quero o default 60000", got)
	}
	if got := loaded.Config.Scoring.Weights.Solid; got != 50 {
		t.Errorf("peso solid = %d, quero 50 (SOLID é 50%% do score)", got)
	}
}

// Ausência de arquivo não é erro: os defaults são config válida.
func TestLoadWithoutFileUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	loaded, err := Load(dir, Overrides{"project.name": "sem-arquivo"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.FromFile || loaded.Path != "" {
		t.Errorf("FromFile=%v Path=%q, quero false/\"\"", loaded.FromFile, loaded.Path)
	}
}

// Caminho explícito inexistente é erro, com código not_found.
func TestLoadExplicitMissingPathFails(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nao-existe.json"), nil)
	if got := errs.CodeOf(err); got != errs.CodeNotFound {
		t.Fatalf("código = %q, quero not_found (err: %v)", got, err)
	}
}

// additionalProperties:false do schema, aplicado no decoder.
func TestUnknownFieldIsRejected(t *testing.T) {
	dir := write(t, `{"schema_version":"1.0.0","project":{"name":"x"},"turbo_mode":true}`)
	_, err := Load(dir, nil)
	if errs.CodeOf(err) != errs.CodeConfig {
		t.Fatalf("código = %q, quero config_invalid", errs.CodeOf(err))
	}
	if !strings.Contains(err.Error(), "turbo_mode") {
		t.Errorf("erro deveria nomear o campo: %v", err)
	}
}

// A mensagem precisa apontar arquivo e posição.
func TestSyntaxErrorReportsLineAndColumn(t *testing.T) {
	dir := write(t, "{\n  \"schema_version\": \"1.0.0\",\n  \"project\": {\"name\": \"x\",}\n}")
	_, err := Load(dir, nil)
	var typed *errs.Error
	if !asError(err, &typed) {
		t.Fatalf("erro não tipado: %v", err)
	}
	if !strings.Contains(typed.Field, FileName+":3:") {
		t.Errorf("field = %q, quero apontar %s linha 3", typed.Field, FileName)
	}
	if !strings.Contains(typed.Hint, "YAML") {
		t.Errorf("hint deveria dizer que só JSON é aceito: %q", typed.Hint)
	}
}

func TestTypeErrorReportsField(t *testing.T) {
	dir := write(t, `{"schema_version":"1.0.0","project":{"name":"x"},"git":{"diff_context_lines":"cinco"}}`)
	_, err := Load(dir, nil)
	var typed *errs.Error
	if !asError(err, &typed) {
		t.Fatalf("erro não tipado: %v", err)
	}
	if !strings.Contains(typed.Field, "diff_context_lines") {
		t.Errorf("field = %q, quero conter diff_context_lines", typed.Field)
	}
}

// ACEITE CRÍTICO: nem valor de env nem secret entram na config ou no hash.
func TestSecretValueNeverEntersConfigOrHash(t *testing.T) {
	const secretValue = "abc123"
	t.Setenv("PAYMENT_TOKEN", secretValue)
	t.Setenv("SONAR_TOKEN", secretValue)

	dir := write(t, `{
	  "schema_version": "1.0.0",
	  "project": {"name": "x"},
	  "analyzers": {"sonar": {"enabled": true, "token_env": "PAYMENT_TOKEN", "base_url": "https://sonar.local"}}
	}`)
	loaded, err := Load(dir, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	serialized, err := json.Marshal(loaded.Config)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(serialized), secretValue) {
		t.Fatalf("valor de secret vazou para a config serializada: %s", serialized)
	}
	if strings.Contains(loaded.Hash, secretValue) {
		t.Fatalf("valor de secret vazou para o hash: %s", loaded.Hash)
	}
	if got := loaded.Config.Analyzers.Sonar.TokenEnv; got != "PAYMENT_TOKEN" {
		t.Errorf("token_env = %q, quero o NOME da variável", got)
	}
	if refs := loaded.Config.EnvRefs(); len(refs) == 0 {
		t.Error("EnvRefs vazio; doctor não teria o que reportar")
	}
}

// Um valor colado num campo *_env é rejeitado: é a defesa contra
// "api_key_env": "sk-proj-abc123".
func TestEnvFieldRejectsValueLikeContent(t *testing.T) {
	for _, bad := range []string{"sk-proj-abc123", "SONAR TOKEN", "TOKEN=abc", "abc/def", ""} {
		if bad == "" {
			continue // vazio significa "não configurado", é permitido
		}
		cfg := Default()
		cfg.Project.Name = "x"
		cfg.AI.ExternalProvider.APIKeyEnv = bad
		err := cfg.Validate()
		if errs.CodeOf(err) != errs.CodeConfig {
			t.Errorf("api_key_env=%q deveria falhar, err=%v", bad, err)
		}
	}
}

func TestHashIsStableAndSensitive(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"

	first, err := cfg.Hash()
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	second, err := cfg.Hash()
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if first != second {
		t.Errorf("hash instável: %s != %s", first, second)
	}
	if !strings.HasPrefix(first, "sha256:") {
		t.Errorf("hash sem prefixo de algoritmo: %s", first)
	}

	cfg.Git.DiffContextLines++
	changed, err := cfg.Hash()
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if changed == first {
		t.Error("hash não mudou após alterar config; cache não seria invalidado")
	}
}

// $schema é decoração de editor e não deve afetar o cache.
func TestHashIgnoresSchemaField(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"
	without, _ := cfg.Hash()
	cfg.Schema = "./schemas/solidify-config.schema.json"
	with, _ := cfg.Hash()
	if without != with {
		t.Errorf("$schema alterou o hash: %s != %s", without, with)
	}
}

func TestOverridesApplyAfterFile(t *testing.T) {
	dir := write(t, minimalConfig)
	loaded, err := Load(dir, Overrides{
		"analysis.default_profile":          "release",
		"analyzers.lighthouse.enabled":      "false",
		"analysis.scheduler.max_light_jobs": "8",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.Config.Analysis.DefaultProfile; got != "release" {
		t.Errorf("default_profile = %q", got)
	}
	if loaded.Config.Analyzers.Lighthouse.Enabled {
		t.Error("lighthouse deveria estar desabilitado por override")
	}
	if got := loaded.Config.Analysis.Scheduler.MaxLightJobs; got != 8 {
		t.Errorf("max_light_jobs = %d, quero 8", got)
	}
}

func TestOverrideChangesHash(t *testing.T) {
	dir := write(t, minimalConfig)
	plain, err := Load(dir, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	overridden, err := Load(dir, Overrides{"analyzers.sonar.enabled": "false"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if plain.Hash == overridden.Hash {
		t.Error("override não mudou o hash; cache reusaria resultado errado")
	}
}

func TestParseOverride(t *testing.T) {
	key, value, err := ParseOverride("project.name=acme")
	if err != nil || key != "project.name" || value != "acme" {
		t.Fatalf("ParseOverride = %q,%q,%v", key, value, err)
	}
	// Valor com "=" dentro é preservado (URLs com query string).
	_, value, err = ParseOverride("ai.external_provider.base_url_host=http://h/v1?a=b")
	if err != nil || value != "http://h/v1?a=b" {
		t.Fatalf("valor = %q, err = %v", value, err)
	}
	for _, bad := range []string{"semigual", "=vazio", "scoring.weights.solid=90"} {
		if _, _, err := ParseOverride(bad); err == nil {
			t.Errorf("ParseOverride(%q) deveria falhar", bad)
		}
	}
}

// Pesos de score não são sobrescrevíveis por flag: relaxar o gate na linha de
// comando anularia o "falha fechado".
func TestScoringIsNotOverridable(t *testing.T) {
	for _, key := range []string{
		"scoring.weights.solid",
		"scoring.gate.minimum_quality_score",
		"profiles.contractual.require_arbiter",
	} {
		if _, ok := setters[key]; ok {
			t.Errorf("chave %q não deveria ser sobrescrevível por flag", key)
		}
	}
}

func TestInvalidBoolAndIntOverrides(t *testing.T) {
	dir := write(t, minimalConfig)
	if _, err := Load(dir, Overrides{"analyzers.sonar.enabled": "sim"}); errs.CodeOf(err) != errs.CodeUsage {
		t.Errorf("bool inválido: código = %q", errs.CodeOf(err))
	}
	if _, err := Load(dir, Overrides{"analysis.scheduler.max_heavy_jobs": "muitos"}); errs.CodeOf(err) != errs.CodeUsage {
		t.Errorf("int inválido: código = %q", errs.CodeOf(err))
	}
}

func TestValidationErrorsNameTheField(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		field  string
	}{
		{"schema version", func(c *Config) { c.SchemaVersion = "2.0.0" }, "schema_version"},
		{"nome vazio", func(c *Config) { c.Project.Name = "" }, "project.name"},
		{"artifact dir absoluto", func(c *Config) { c.Project.ArtifactDir = "/tmp/x" }, "project.artifact_dir"},
		{"artifact dir escapa", func(c *Config) { c.Project.ArtifactDir = "../fora" }, "project.artifact_dir"},
		{"regex inválida", func(c *Config) { c.Project.IssueKeyPatterns = []string{"["} }, "project.issue_key_patterns[0]"},
		{"perfil inexistente", func(c *Config) { c.Analysis.DefaultProfile = "turbo" }, "analysis.default_profile"},
		{"budget zero", func(c *Config) { c.Analysis.Context.FileMaxChars = 0 }, "analysis.context.file_max_chars"},
		{"shard menor que file", func(c *Config) { c.Analysis.Context.ShardMaxChars = 10 }, "analysis.context.shard_max_chars"},
		{"pesos não somam 100", func(c *Config) { c.Scoring.Weights.Solid = 40 }, "scoring.weights"},
		{"peso negativo", func(c *Config) { c.Scoring.Weights.Solid = -50; c.Scoring.Weights.Tests = 110 }, "scoring.weights.solid"},
		{"severidade inválida", func(c *Config) {
			c.Scoring.Gate.FailOnNewSecuritySeverities = []string{"catastrófico"}
		}, "scoring.gate.fail_on_new_security_severities[0]"},
		{"gate fora de faixa", func(c *Config) { c.Scoring.Gate.MinimumQualityScore = 120 }, "scoring.gate.minimum_quality_score"},
		{"contexto git negativo", func(c *Config) { c.Git.DiffContextLines = -1 }, "git.diff_context_lines"},
		{"lighthouse não soma 100", func(c *Config) { c.Analyzers.Lighthouse.Categories.SEO = 10 }, "analyzers.lighthouse.categories"},
		{"modo de carga inválido", func(c *Config) { c.Analyzers.Load.DefaultMode = "turbo" }, "analyzers.load.default_mode"},
		{"comando de teste vazio", func(c *Config) { c.Analyzers.Tests.Commands = [][]string{{}} }, "analyzers.tests.commands[0]"},
		{"provider não suportado", func(c *Config) { c.AI.ExternalProvider.Type = "anthropic" }, "ai.external_provider.type"},
		{"url sem esquema", func(c *Config) { c.AI.ExternalProvider.BaseURLHost = "127.0.0.1:20128" }, "ai.external_provider.base_url_host"},
		{"url com credencial", func(c *Config) {
			c.AI.ExternalProvider.BaseURLHost = "http://user:pass@127.0.0.1:20128/v1"
		}, "ai.external_provider.base_url_host"},
		{"timeout zero", func(c *Config) { c.AI.ExternalProvider.RequestTimeoutSeconds = 0 }, "ai.external_provider.request_timeout_seconds"},
		{"modo de seleção inválido", func(c *Config) { c.AI.Selection.Mode = "aleatório" }, "ai.selection.mode"},
		{"pinned sem modelo", func(c *Config) { c.AI.Selection.Mode = "pinned" }, "ai.selection.peer_b.preferred"},
		{"peer A inválido", func(c *Config) { c.AI.PeerA.Source = "telepatia" }, "ai.peer_a.source"},
		{"temperatura fora de faixa", func(c *Config) { c.AI.Review.Temperature = 5 }, "ai.review.temperature"},
		{"papel inválido", func(c *Config) { c.Report.PDF.Paper = "A3" }, "report.pdf.paper"},
		{"browser mode inválido", func(c *Config) { c.Report.PDF.BrowserMode = "firefox" }, "report.pdf.browser_mode"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			cfg.Project.Name = "x"
			tc.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("esperava erro de validação")
			}
			if errs.CodeOf(err) != errs.CodeConfig {
				t.Fatalf("código = %q, quero config_invalid", errs.CodeOf(err))
			}
			var typed *errs.Error
			if !asError(err, &typed) {
				t.Fatalf("erro não tipado: %v", err)
			}
			if typed.Field != tc.field {
				t.Errorf("field = %q, quero %q", typed.Field, tc.field)
			}
		})
	}
}

// Scan ativo sem allowlist é bloqueado: nunca atacar host descoberto.
func TestActiveScanRequiresAllowlist(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"
	cfg.Analyzers.Security.ZAPActive = true
	cfg.Analyzers.Security.ActiveTargetAllowlist = nil

	err := cfg.Validate()
	var typed *errs.Error
	if !asError(err, &typed) || typed.Field != "analyzers.security.active_target_allowlist" {
		t.Fatalf("erro = %v", err)
	}
}

// Contractual precisa falhar fechado; relaxá-lo é erro de config.
func TestContractualCannotBeRelaxed(t *testing.T) {
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Profiles["contractual"] = Profile{RequirePeerA: ptr(true)} },
		func(c *Config) {
			p := c.Profiles["contractual"]
			p.RequireArbiter = ptr(false)
			c.Profiles["contractual"] = p
		},
		func(c *Config) {
			p := c.Profiles["contractual"]
			p.FailOnRequiredAnalyzerSkip = ptr(false)
			c.Profiles["contractual"] = p
		},
	} {
		cfg := Default()
		cfg.Project.Name = "x"
		mutate(&cfg)
		if err := cfg.Validate(); errs.CodeOf(err) != errs.CodeConfig {
			t.Errorf("contractual relaxado passou na validação: %v", err)
		}
	}
}

func TestResolveProfileFollowsExtends(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"

	resolved, err := cfg.ResolveProfile("strict-plus")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if !resolved.RunRoleSwap {
		t.Error("run_role_swap deveria vir do próprio perfil")
	}
	// Herdados de contractual:
	if !resolved.RequireArbiter || !resolved.RequireDistinctExternalModels || !resolved.GeneratePDF {
		t.Errorf("herança de contractual falhou: %+v", resolved)
	}
}

// O perfil mais específico ganha do ancestral.
func TestResolveProfileChildOverridesParent(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"
	cfg.Profiles["custom"] = Profile{Extends: "quick", GeneratePDF: ptr(true)}

	resolved, err := cfg.ResolveProfile("custom")
	if err != nil {
		t.Fatalf("ResolveProfile: %v", err)
	}
	if !resolved.GeneratePDF {
		t.Error("filho deveria sobrescrever generate_pdf do pai")
	}
	if !resolved.RequirePeerA {
		t.Error("require_peer_a deveria ser herdado de quick")
	}
	if resolved.RequirePeerB {
		t.Error("require_peer_b deveria permanecer false, herdado de quick")
	}
}

func TestResolveProfileDetectsCycle(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"
	cfg.Profiles["a"] = Profile{Extends: "b"}
	cfg.Profiles["b"] = Profile{Extends: "a"}

	_, err := cfg.ResolveProfile("a")
	if err == nil || !strings.Contains(err.Error(), "ciclo") {
		t.Fatalf("esperava erro de ciclo, veio %v", err)
	}
}

func TestResolveUnknownProfile(t *testing.T) {
	cfg := Default()
	if _, err := cfg.ResolveProfile("inexistente"); errs.CodeOf(err) != errs.CodeConfig {
		t.Fatalf("código = %q", errs.CodeOf(err))
	}
}

// O exemplo da spec precisa carregar sem erro: ele é a documentação executável.
func TestSpecExampleLoads(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "spec", "solidify.example.json"))
	if err != nil {
		t.Skipf("exemplo da spec indisponível: %v", err)
	}
	dir := t.TempDir()
	if werr := os.WriteFile(filepath.Join(dir, FileName), data, 0o644); werr != nil {
		t.Fatalf("escrever: %v", werr)
	}
	if _, lerr := Load(dir, nil); lerr != nil {
		t.Fatalf("solidify.example.json não carrega: %v", lerr)
	}
}

// SAI-135: profile ativo exige peer_b/arbiter mas preferred[] vazio deve
// falhar cedo (antes de gastar diff/tokens), não só em modo pinned.
func TestValidateActiveProfileRequiresPreferredModel(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"
	// release exige peer_b e arbiter; defaults deixam ambos preferred vazio.

	err := cfg.ValidateActiveProfile("release")
	var typed *errs.Error
	if !asError(err, &typed) {
		t.Fatalf("esperava *errs.Error, veio %v", err)
	}
	if typed.Code != errs.CodeConfig {
		t.Fatalf("código = %q", typed.Code)
	}
	if typed.Field != "ai.selection.peer_b.preferred" {
		t.Fatalf("field = %q", typed.Field)
	}
}

func TestValidateActiveProfileOKWhenPreferredSet(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"
	cfg.AI.Selection.PeerB.Preferred = []string{"some-model"}
	cfg.AI.Selection.Arbiter.Preferred = []string{"other-model"}

	if err := cfg.ValidateActiveProfile("release"); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
}

func TestValidateActiveProfileSkipsUnrequiredActors(t *testing.T) {
	cfg := Default()
	cfg.Project.Name = "x"
	// quick só exige peer_a, que já tem preferred default.
	if err := cfg.ValidateActiveProfile("quick"); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
}

// asError é errors.As sem importar errors no corpo dos testes.
func asError(err error, target **errs.Error) bool {
	for err != nil {
		if typed, ok := err.(*errs.Error); ok {
			*target = typed
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}
