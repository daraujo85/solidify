package config

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/diegoaraujo/solidify/internal/errs"
)

// envNamePattern é o formato aceito para nome de variável de ambiente.
var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Validate verifica a config efetiva. O primeiro erro encontrado é devolvido
// com o caminho do campo, para que a mensagem seja acionável.
func Validate(c Config) error { return c.Validate() }

// Validate verifica a config efetiva.
func (c Config) Validate() error {
	checks := []func() error{
		c.validateSchemaVersion,
		c.validateProject,
		c.validateAnalysis,
		c.validateProfiles,
		c.validateScoring,
		c.validateGit,
		c.validateAnalyzers,
		c.validateAI,
		c.validateReport,
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func invalid(field, msg string) *errs.Error {
	return errs.New(errs.CodeConfig, msg).WithField(field)
}

func (c Config) validateSchemaVersion() error {
	if c.SchemaVersion != SchemaVersion {
		return invalid("schema_version",
			"versão de schema não suportada: "+quote(c.SchemaVersion)).
			WithHint("este binário aceita apenas " + quote(SchemaVersion))
	}
	return nil
}

func (c Config) validateProject() error {
	if strings.TrimSpace(c.Project.Name) == "" {
		return invalid("project.name", "nome do projeto é obrigatório")
	}
	if strings.TrimSpace(c.Project.DefaultBase) == "" {
		return invalid("project.default_base", "base default é obrigatória")
	}
	if err := validateRelativeDir("project.artifact_dir", c.Project.ArtifactDir); err != nil {
		return err
	}
	for i, pattern := range c.Project.IssueKeyPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return errs.Wrap(errs.CodeConfig, "regex de issue key inválida", err).
				WithField(indexed("project.issue_key_patterns", i))
		}
	}
	return nil
}

// validateRelativeDir barra caminho absoluto e escape do repo: o artifact dir
// vive dentro do projeto (regra de segurança de path traversal, SAI-101).
func validateRelativeDir(field, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return invalid(field, "diretório é obrigatório")
	}
	if filepath.IsAbs(dir) {
		return invalid(field, "diretório deve ser relativo ao repositório")
	}
	clean := filepath.ToSlash(filepath.Clean(dir))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return invalid(field, "diretório não pode escapar do repositório")
	}
	return nil
}

func (c Config) validateAnalysis() error {
	if _, ok := c.Profiles[c.Analysis.DefaultProfile]; !ok {
		return invalid("analysis.default_profile",
			"perfil default inexistente: "+quote(c.Analysis.DefaultProfile)).
			WithHint("perfis disponíveis: " + strings.Join(c.profileNames(), ", "))
	}

	positives := []struct {
		field string
		value int
	}{
		{"analysis.context.summary_max_chars", c.Analysis.Context.SummaryMaxChars},
		{"analysis.context.shard_max_chars", c.Analysis.Context.ShardMaxChars},
		{"analysis.context.file_max_chars", c.Analysis.Context.FileMaxChars},
		{"analysis.context.max_context_expansions_per_changed_file", c.Analysis.Context.MaxContextExpansionsPerChangedFile},
		{"analysis.scheduler.max_light_jobs", c.Analysis.Scheduler.MaxLightJobs},
		{"analysis.scheduler.max_heavy_jobs", c.Analysis.Scheduler.MaxHeavyJobs},
		{"analysis.scheduler.max_browser_jobs", c.Analysis.Scheduler.MaxBrowserJobs},
		{"analysis.scheduler.max_active_network_jobs", c.Analysis.Scheduler.MaxActiveNetworkJobs},
	}
	for _, p := range positives {
		if p.value < 1 {
			return invalid(p.field, "valor deve ser >= 1")
		}
	}

	// Um shard não pode ser menor que um arquivo individual, senão o budget de
	// evidência é insatisfazível por construção (SAI-024).
	if c.Analysis.Context.ShardMaxChars < c.Analysis.Context.FileMaxChars {
		return invalid("analysis.context.shard_max_chars",
			"shard_max_chars deve ser >= file_max_chars")
	}
	return nil
}

func (c Config) validateProfiles() error {
	if len(c.Profiles) == 0 {
		return invalid("profiles", "ao menos um perfil é obrigatório")
	}
	for name := range c.Profiles {
		if strings.TrimSpace(name) == "" {
			return invalid("profiles", "nome de perfil vazio")
		}
		// Resolve para detectar extends inexistente e ciclo.
		if _, err := c.ResolveProfile(name); err != nil {
			return err
		}
	}

	// Regra da spec: contractual falha fechado.
	if _, ok := c.Profiles["contractual"]; ok {
		resolved, err := c.ResolveProfile("contractual")
		if err != nil {
			return err
		}
		if !resolved.RequirePeerA || !resolved.RequirePeerB || !resolved.RequireArbiter {
			return invalid("profiles.contractual",
				"contractual exige peer A, peer B e árbitro").
				WithHint("o perfil contractual precisa falhar fechado; não relaxe estes requisitos")
		}
		if !resolved.FailOnRequiredAnalyzerSkip {
			return invalid("profiles.contractual.fail_on_required_analyzer_skip",
				"contractual exige falhar quando analyzer obrigatório é pulado")
		}
	}
	return nil
}

func (c Config) validateScoring() error {
	weights := []struct {
		field string
		value int
	}{
		{"scoring.weights.solid", c.Scoring.Weights.Solid},
		{"scoring.weights.static_quality", c.Scoring.Weights.StaticQuality},
		{"scoring.weights.security", c.Scoring.Weights.Security},
		{"scoring.weights.tests", c.Scoring.Weights.Tests},
		{"scoring.weights.performance", c.Scoring.Weights.Performance},
		{"scoring.weights.frontend", c.Scoring.Weights.Frontend},
	}
	for _, w := range weights {
		if w.value < 0 {
			return invalid(w.field, "peso não pode ser negativo")
		}
	}
	if sum := c.Scoring.Weights.Sum(); sum != 100 {
		return invalid("scoring.weights", "pesos devem somar 100, somam "+itoa(sum))
	}

	for _, s := range []struct {
		field string
		value float64
	}{
		{"scoring.gate.minimum_quality_score", c.Scoring.Gate.MinimumQualityScore},
		{"scoring.gate.minimum_solid_score", c.Scoring.Gate.MinimumSolidScore},
	} {
		if s.value < 0 || s.value > 100 {
			return invalid(s.field, "valor deve estar entre 0 e 100")
		}
	}

	for i, severity := range c.Scoring.Gate.FailOnNewSecuritySeverities {
		if !validSeverity(severity) {
			return invalid(indexed("scoring.gate.fail_on_new_security_severities", i),
				"severidade inválida: "+quote(severity)).
				WithHint("use critical, high, medium, low ou info")
		}
	}
	return nil
}

func validSeverity(s string) bool {
	switch strings.ToLower(s) {
	case "critical", "high", "medium", "low", "info":
		return true
	}
	return false
}

func (c Config) validateGit() error {
	if c.Git.DiffContextLines < 0 {
		return invalid("git.diff_context_lines", "valor não pode ser negativo")
	}
	if c.Git.DiffContextLines > 100 {
		return invalid("git.diff_context_lines", "valor acima de 100 estoura o budget de contexto")
	}
	return nil
}

func (c Config) validateAnalyzers() error {
	sonar := c.Analyzers.Sonar
	if sonar.TokenEnv != "" {
		if err := validateEnvRef("analyzers.sonar.token_env", sonar.TokenEnv); err != nil {
			return err
		}
	}
	if sonar.BaseURL != "" {
		if err := validateURL("analyzers.sonar.base_url", sonar.BaseURL); err != nil {
			return err
		}
	}

	security := c.Analyzers.Security
	if security.ZAPActive && len(security.ActiveTargetAllowlist) == 0 {
		return invalid("analyzers.security.active_target_allowlist",
			"active scan exige allowlist de targets não vazia").
			WithHint("scan ativo nunca roda contra host descoberto automaticamente")
	}
	for i, target := range security.ActiveTargetAllowlist {
		if strings.TrimSpace(target) == "" {
			return invalid(indexed("analyzers.security.active_target_allowlist", i), "target vazio")
		}
	}

	if sum := c.Analyzers.Lighthouse.Categories.Sum(); sum != 100 {
		return invalid("analyzers.lighthouse.categories",
			"pesos das categorias devem somar 100, somam "+itoa(sum))
	}

	switch c.Analyzers.Load.DefaultMode {
	case "smoke", "load", "stress", "soak":
	default:
		return invalid("analyzers.load.default_mode",
			"modo inválido: "+quote(c.Analyzers.Load.DefaultMode)).
			WithHint("use smoke, load, stress ou soak")
	}

	for i, cmd := range c.Analyzers.Tests.Commands {
		if len(cmd) == 0 {
			return invalid(indexed("analyzers.tests.commands", i),
				"comando vazio").
				WithHint("comandos são argv (lista), não string de shell")
		}
	}
	return nil
}

func (c Config) validateAI() error {
	provider := c.AI.ExternalProvider
	if provider.Type != "openai-compatible" {
		return invalid("ai.external_provider.type",
			"tipo de provider não suportado: "+quote(provider.Type)).
			WithHint("o MVP suporta apenas openai-compatible")
	}
	if provider.BaseURLHost == "" && provider.BaseURLDocker == "" {
		return invalid("ai.external_provider.base_url_host",
			"informe base_url_host ou base_url_docker")
	}
	for field, raw := range map[string]string{
		"ai.external_provider.base_url_host":   provider.BaseURLHost,
		"ai.external_provider.base_url_docker": provider.BaseURLDocker,
	} {
		if raw == "" {
			continue
		}
		if err := validateURL(field, raw); err != nil {
			return err
		}
	}
	if provider.APIKeyEnv != "" {
		if err := validateEnvRef("ai.external_provider.api_key_env", provider.APIKeyEnv); err != nil {
			return err
		}
	}
	if provider.RequestTimeoutSeconds < 1 {
		return invalid("ai.external_provider.request_timeout_seconds", "timeout deve ser >= 1")
	}

	switch c.AI.Selection.Mode {
	case "pinned", "discover", "hybrid":
	default:
		return invalid("ai.selection.mode", "modo inválido: "+quote(c.AI.Selection.Mode)).
			WithHint("use pinned, discover ou hybrid")
	}
	if c.AI.Selection.Mode == "pinned" && len(c.AI.Selection.PeerB.Preferred) == 0 {
		return invalid("ai.selection.peer_b.preferred",
			"modo pinned exige ao menos um modelo preferido para o peer B")
	}
	if c.AI.Selection.Probe.CacheHours < 0 {
		return invalid("ai.selection.probe.cache_hours", "valor não pode ser negativo")
	}

	// SAI-118: "auto" e "http-combo" adicionados. "external" preservado
	// como alias legacy de "http-combo".
	src := c.AI.PeerA.Source
	if src != "terminal-mcp" && src != "external" && src != "http-combo" && src != "auto" {
		return invalid("ai.peer_a.source", "origem inválida: "+quote(src)).
			WithHint("use terminal-mcp, http-combo (ou external/alias) ou auto")
	}

	review := c.AI.Review
	if review.Temperature < 0 || review.Temperature > 2 {
		return invalid("ai.review.temperature", "temperatura deve estar entre 0 e 2")
	}
	if review.MaxRepairAttempts < 0 || review.MaxRepairAttempts > 3 {
		return invalid("ai.review.max_repair_attempts", "valor deve estar entre 0 e 3")
	}
	if strings.TrimSpace(review.PeerPromptVersion) == "" {
		return invalid("ai.review.peer_prompt_version", "versão do prompt é obrigatória")
	}
	if strings.TrimSpace(review.ArbiterPromptVersion) == "" {
		return invalid("ai.review.arbiter_prompt_version", "versão do prompt é obrigatória")
	}
	return nil
}

func (c Config) validateReport() error {
	if strings.TrimSpace(c.Report.Language) == "" {
		return invalid("report.language", "idioma é obrigatório")
	}
	switch c.Report.PDF.Paper {
	case "A4", "Letter":
	default:
		return invalid("report.pdf.paper", "papel inválido: "+quote(c.Report.PDF.Paper)).
			WithHint("use A4 ou Letter")
	}
	switch c.Report.PDF.BrowserMode {
	case "auto", "host", "docker", "off":
	default:
		return invalid("report.pdf.browser_mode", "modo inválido: "+quote(c.Report.PDF.BrowserMode)).
			WithHint("use auto, host, docker ou off")
	}
	if c.Report.LogoPath != "" {
		if err := validateRelativeDir("report.logo_path", c.Report.LogoPath); err != nil {
			return err
		}
	}
	return nil
}

// validateEnvRef garante que o campo carrega um *nome* de variável, não um valor.
// Um valor de secret aqui vazaria para artifacts e logs.
func validateEnvRef(field, value string) error {
	if !envNamePattern.MatchString(value) {
		return invalid(field, "esperava um nome de variável de ambiente").
			WithHint("campos *_env guardam o nome (ex.: SONAR_TOKEN), nunca o valor")
	}
	return nil
}

func validateURL(field, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return errs.Wrap(errs.CodeConfig, "URL inválida", err).WithField(field)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return invalid(field, "URL deve usar http ou https")
	}
	if parsed.Host == "" {
		return invalid(field, "URL sem host")
	}
	if parsed.User != nil {
		return invalid(field, "URL não pode conter credenciais embutidas").
			WithHint("use um campo *_env para a credencial")
	}
	return nil
}

func indexed(field string, i int) string { return field + "[" + strconv.Itoa(i) + "]" }

func quote(s string) string { return `"` + s + `"` }

func itoa(i int) string { return strconv.Itoa(i) }
