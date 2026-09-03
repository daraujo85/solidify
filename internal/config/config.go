// Package config carrega e valida o solidify.json.
//
// Invariantes:
//   - somente JSON (sem YAML/TOML), campo desconhecido é erro;
//   - secrets nunca aparecem aqui: a config guarda apenas o *nome* da variável
//     de ambiente (campos com sufixo `_env`), jamais o valor;
//   - a config efetiva (defaults + arquivo + overrides) tem hash estável, usado
//     como parte da chave de cache dos analyzers.
package config

// SchemaVersion é a única versão de config aceita por este binário.
const SchemaVersion = "1.0.0"

// Config é a config efetiva do Solidify.
type Config struct {
	Schema        string    `json:"$schema,omitempty"`
	SchemaVersion string    `json:"schema_version"`
	Project       Project   `json:"project"`
	Analysis      Analysis  `json:"analysis"`
	Profiles      Profiles  `json:"profiles"`
	Scoring       Scoring   `json:"scoring"`
	Git           Git       `json:"git"`
	Detectors     Detectors `json:"detectors"`
	Runtime       Runtime   `json:"runtime"`
	Targets       Targets   `json:"targets"`
	Analyzers     Analyzers `json:"analyzers"`
	AI            AI        `json:"ai"`
	Report        Report    `json:"report"`
	Privacy       Privacy   `json:"privacy"`
}

// Project descreve o repositório analisado.
type Project struct {
	Name             string   `json:"name"`
	DefaultBase      string   `json:"default_base"`
	ArtifactDir      string   `json:"artifact_dir"`
	IssueKeyPatterns []string `json:"issue_key_patterns"`
}

// Analysis controla escopo e recursos da análise.
type Analysis struct {
	DefaultProfile string    `json:"default_profile"`
	UseMergeBase   bool      `json:"use_merge_base"`
	Context        Context   `json:"context"`
	Scheduler      Scheduler `json:"scheduler"`
}

// Context são os budgets de evidência enviada ao LLM (SAI-024).
type Context struct {
	SummaryMaxChars                    int `json:"summary_max_chars"`
	ShardMaxChars                      int `json:"shard_max_chars"`
	FileMaxChars                       int `json:"file_max_chars"`
	MaxContextExpansionsPerChangedFile int `json:"max_context_expansions_per_changed_file"`
}

// Scheduler limita concorrência por classe de recurso (SAI-027).
type Scheduler struct {
	MaxLightJobs         int `json:"max_light_jobs"`
	MaxHeavyJobs         int `json:"max_heavy_jobs"`
	MaxBrowserJobs       int `json:"max_browser_jobs"`
	MaxActiveNetworkJobs int `json:"max_active_network_jobs"`
}

// Profiles mapeia nome do perfil para seus requisitos.
type Profiles map[string]Profile

// Profile é um conjunto de requisitos de execução. Ponteiros permitem
// distinguir "não declarado" (herda de Extends) de "declarado como false".
type Profile struct {
	Extends                       string `json:"extends,omitempty"`
	RequirePeerA                  *bool  `json:"require_peer_a,omitempty"`
	RequirePeerB                  *bool  `json:"require_peer_b,omitempty"`
	RequireArbiter                *bool  `json:"require_arbiter,omitempty"`
	RequireDistinctExternalModels *bool  `json:"require_distinct_external_models,omitempty"`
	GeneratePDF                   *bool  `json:"generate_pdf,omitempty"`
	ActiveSecurity                *bool  `json:"active_security,omitempty"`
	FailOnRequiredAnalyzerSkip    *bool  `json:"fail_on_required_analyzer_skip,omitempty"`
	RunRoleSwap                   *bool  `json:"run_role_swap,omitempty"`
}

// Scoring define pesos e limites do gate.
type Scoring struct {
	Weights Weights `json:"weights"`
	Gate    Gate    `json:"gate"`
}

// Weights são os pesos dos pilares; devem somar 100.
type Weights struct {
	Solid         int `json:"solid"`
	StaticQuality int `json:"static_quality"`
	Security      int `json:"security"`
	Tests         int `json:"tests"`
	Performance   int `json:"performance"`
	Frontend      int `json:"frontend"`
}

// Sum devolve a soma dos pesos.
func (w Weights) Sum() int {
	return w.Solid + w.StaticQuality + w.Security + w.Tests + w.Performance + w.Frontend
}

// Gate são os limites de aprovação (SAI-075).
type Gate struct {
	MinimumQualityScore                   float64  `json:"minimum_quality_score"`
	MinimumSolidScore                     float64  `json:"minimum_solid_score"`
	FailOnNewSecuritySeverities           []string `json:"fail_on_new_security_severities"`
	FailOnTestFailure                     bool     `json:"fail_on_test_failure"`
	FailOnUnacknowledgedCriticalMigration bool     `json:"fail_on_unacknowledged_critical_migration"`
}

// Git controla a coleta de diff.
type Git struct {
	RenameDetection  bool `json:"rename_detection"`
	CopyDetection    bool `json:"copy_detection"`
	DiffContextLines int  `json:"diff_context_lines"`
}

// Detectors configura os detectores determinísticos.
type Detectors struct {
	MigrationPaths        []string `json:"migration_paths"`
	EnvDocumentationFiles []string `json:"env_documentation_files"`
}

// Runtime são hooks opcionais para subir o app analisado (SAI-043).
type Runtime struct {
	PrepareCommand       []string `json:"prepare_command"`
	StartCommand         []string `json:"start_command"`
	Healthchecks         []string `json:"healthchecks"`
	StopCommand          []string `json:"stop_command"`
	DatabaseResetCommand []string `json:"database_reset_command"`
	AllowDatabaseReset   bool     `json:"allow_database_reset"`
}

// Targets são URLs conhecidas do app.
type Targets struct {
	Frontend []string `json:"frontend"`
	API      []string `json:"api"`
}

// Analyzers agrupa a config de cada analyzer.
type Analyzers struct {
	Tests      TestsAnalyzer      `json:"tests"`
	Sonar      SonarAnalyzer      `json:"sonar"`
	Security   SecurityAnalyzer   `json:"security"`
	Lighthouse LighthouseAnalyzer `json:"lighthouse"`
	Load       LoadAnalyzer       `json:"load"`
}

// TestsAnalyzer configura o test runner (SAI-030).
type TestsAnalyzer struct {
	Enabled               bool       `json:"enabled"`
	RequiredInContractual bool       `json:"required_in_contractual"`
	Commands              [][]string `json:"commands"`
}

// SonarAnalyzer configura a integração Sonar (SAI-033). TokenEnv guarda o
// *nome* da variável de ambiente, nunca o token.
type SonarAnalyzer struct {
	Enabled               bool   `json:"enabled"`
	RequiredInContractual bool   `json:"required_in_contractual"`
	BaseURL               string `json:"base_url"`
	TokenEnv              string `json:"token_env"`
	ProjectKey            string `json:"project_key"`
}

// SecurityAnalyzer configura as ferramentas de segurança (Fase 8).
type SecurityAnalyzer struct {
	Enabled               bool     `json:"enabled"`
	Gitleaks              bool     `json:"gitleaks"`
	OSVScanner            bool     `json:"osv_scanner"`
	Semgrep               bool     `json:"semgrep"`
	ZAPBaseline           bool     `json:"zap_baseline"`
	ZAPActive             bool     `json:"zap_active"`
	ActiveTargetAllowlist []string `json:"active_target_allowlist"`
}

// LighthouseAnalyzer configura o Lighthouse (SAI-044).
type LighthouseAnalyzer struct {
	Enabled    bool                 `json:"enabled"`
	Categories LighthouseCategories `json:"categories"`
}

// LighthouseCategories são os pesos das categorias; devem somar 100.
type LighthouseCategories struct {
	Performance   int `json:"performance"`
	Accessibility int `json:"accessibility"`
	BestPractices int `json:"best_practices"`
	SEO           int `json:"seo"`
}

// Sum devolve a soma dos pesos das categorias.
func (c LighthouseCategories) Sum() int {
	return c.Performance + c.Accessibility + c.BestPractices + c.SEO
}

// LoadAnalyzer configura o k6 (SAI-047).
type LoadAnalyzer struct {
	Enabled                   bool   `json:"enabled"`
	DefaultMode               string `json:"default_mode"`
	RequireThresholdsForScore bool   `json:"require_thresholds_for_score"`
}

// AI configura peers, provider e prompts.
type AI struct {
	PeerA            PeerA            `json:"peer_a"`
	ExternalProvider ExternalProvider `json:"external_provider"`
	Selection        Selection        `json:"selection"`
	Review           Review           `json:"review"`
	// PeerReview (SAI-121) — telemetria + cutoff.
	PeerReview PeerReview `json:"peer_review"`
}

// PeerReview configura telemetria e cutoff do schema v1 (SAI-121).
//
// MetricsPath: arquivo JSONL append-only de submissions. Default
// ~/.solidify/metrics/peer_reviews.jsonl.
//
// V1Cutoff: data (RFC3339) após a qual submissions schema v1 viram
// hard error (não warning). Vazio = sem cutoff, sempre warning.
type PeerReview struct {
	MetricsPath string `json:"metrics_path,omitempty"`
	V1Cutoff    string `json:"v1_cutoff,omitempty"`
}

// PeerA descreve a origem do Peer A.
// StoreDir é o diretório compartilhado entre `solidify mcp serve`
// (escreve via solidify_submit_peer_review) e `solidify run`
// (lê via polling do PeerReviewStore). Default: ~/.solidify/peer_reviews.
type PeerA struct {
	Source   string `json:"source"`
	StoreDir string `json:"store_dir,omitempty"`
}

// ExternalProvider é o provider OpenAI-compatible. APIKeyEnv guarda o *nome*
// da variável de ambiente, nunca a chave.
type ExternalProvider struct {
	Type                  string `json:"type"`
	Preset                string `json:"preset"`
	BaseURLHost           string `json:"base_url_host"`
	BaseURLDocker         string `json:"base_url_docker"`
	APIKeyEnv             string `json:"api_key_env"`
	ModelDiscovery        bool   `json:"model_discovery"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
}

// Selection controla a escolha de modelos (SAI-063).
// PeerA adicionado em SAI-118 — peer_a via http-combo usa o
// mesmo padrão de PeerB (preferred[] + exclude[]).
type Selection struct {
	Mode    string        `json:"mode"`
	PeerA   ModelChoice   `json:"peer_a,omitempty"` // SAI-118
	PeerB   ModelChoice   `json:"peer_b"`
	Arbiter ArbiterChoice `json:"arbiter"`
	Probe   Probe         `json:"probe"`
}

// ModelChoice lista preferências e exclusões de modelo.
type ModelChoice struct {
	Preferred []string `json:"preferred"`
	Exclude   []string `json:"exclude"`
}

// ArbiterChoice acrescenta a política de distinção do árbitro.
type ArbiterChoice struct {
	Preferred           []string `json:"preferred"`
	Exclude             []string `json:"exclude"`
	MustDifferFromPeerB bool     `json:"must_differ_from_peer_b"`
}

// Probe configura o capability probe (SAI-062).
type Probe struct {
	Enabled               bool `json:"enabled"`
	CacheHours            int  `json:"cache_hours"`
	RequireJSONCompliance bool `json:"require_json_compliance"`
}

// Review configura prompts e determinismo do peer review.
type Review struct {
	PeerPromptVersion    string  `json:"peer_prompt_version"`
	ArbiterPromptVersion string  `json:"arbiter_prompt_version"`
	Temperature          float64 `json:"temperature"`
	MaxRepairAttempts    int     `json:"max_repair_attempts"`
}

// Report configura o laudo.
type Report struct {
	Language    string `json:"language"`
	CompanyName string `json:"company_name"`
	LogoPath    string `json:"logo_path"`
	PDF         PDF    `json:"pdf"`
}

// PDF configura a renderização (Fase 19).
type PDF struct {
	Paper       string `json:"paper"`
	BrowserMode string `json:"browser_mode"`
}

// Privacy controla o que é persistido.
type Privacy struct {
	PersistFullDiff       bool `json:"persist_full_diff"`
	PersistModelRequests  bool `json:"persist_model_requests"`
	PersistModelResponses bool `json:"persist_model_responses"`
	RedactSecrets         bool `json:"redact_secrets"`
}

func ptr(b bool) *bool { return &b }

// Default devolve a config completa com os defaults do Solidify.
//
// Load decodifica o arquivo do usuário *sobre* este valor, então campos
// ausentes no arquivo mantêm o default automaticamente.
func Default() Config {
	return Config{
		SchemaVersion: SchemaVersion,
		Project: Project{
			DefaultBase:      "origin/main",
			ArtifactDir:      ".solidify",
			IssueKeyPatterns: []string{`[A-Z][A-Z0-9]+-[0-9]+`},
		},
		Analysis: Analysis{
			DefaultProfile: "quick",
			UseMergeBase:   true,
			Context: Context{
				SummaryMaxChars:                    20000,
				ShardMaxChars:                      60000,
				FileMaxChars:                       12000,
				MaxContextExpansionsPerChangedFile: 8,
			},
			Scheduler: Scheduler{
				MaxLightJobs:         4,
				MaxHeavyJobs:         1,
				MaxBrowserJobs:       1,
				MaxActiveNetworkJobs: 1,
			},
		},
		Profiles: Profiles{
			"quick": {
				RequirePeerA:   ptr(true),
				RequirePeerB:   ptr(false),
				RequireArbiter: ptr(false),
				GeneratePDF:    ptr(false),
				ActiveSecurity: ptr(false),
			},
			"release": {
				RequirePeerA:   ptr(true),
				RequirePeerB:   ptr(true),
				RequireArbiter: ptr(true),
				GeneratePDF:    ptr(true),
				ActiveSecurity: ptr(false),
			},
			"contractual": {
				RequirePeerA:                  ptr(true),
				RequirePeerB:                  ptr(true),
				RequireArbiter:                ptr(true),
				RequireDistinctExternalModels: ptr(true),
				GeneratePDF:                   ptr(true),
				ActiveSecurity:                ptr(false),
				FailOnRequiredAnalyzerSkip:    ptr(true),
			},
			"strict-plus": {
				Extends:     "contractual",
				RunRoleSwap: ptr(true),
			},
		},
		Scoring: Scoring{
			Weights: Weights{Solid: 50, StaticQuality: 15, Security: 15, Tests: 10, Performance: 5, Frontend: 5},
			Gate: Gate{
				MinimumQualityScore:                   70,
				MinimumSolidScore:                     65,
				FailOnNewSecuritySeverities:           []string{"critical", "high"},
				FailOnTestFailure:                     true,
				FailOnUnacknowledgedCriticalMigration: true,
			},
		},
		Git: Git{RenameDetection: true, CopyDetection: true, DiffContextLines: 5},
		Detectors: Detectors{
			MigrationPaths:        []string{"db/migration", "db/migrations", "migrations", "prisma/migrations"},
			EnvDocumentationFiles: []string{".env.example", ".env.sample", ".env.template"},
		},
		Runtime: Runtime{
			PrepareCommand:       []string{},
			StartCommand:         []string{},
			Healthchecks:         []string{},
			StopCommand:          []string{},
			DatabaseResetCommand: []string{},
		},
		Targets: Targets{Frontend: []string{}, API: []string{}},
		Analyzers: Analyzers{
			Tests: TestsAnalyzer{Enabled: true, RequiredInContractual: true, Commands: [][]string{}},
			Sonar: SonarAnalyzer{Enabled: true, TokenEnv: "SONAR_TOKEN"},
			Security: SecurityAnalyzer{
				Enabled: true, Gitleaks: true, OSVScanner: true, Semgrep: true,
				ZAPBaseline: true, ZAPActive: false,
				ActiveTargetAllowlist: []string{"127.0.0.1", "localhost", "host.docker.internal"},
			},
			Lighthouse: LighthouseAnalyzer{
				Enabled:    true,
				Categories: LighthouseCategories{Performance: 50, Accessibility: 25, BestPractices: 25, SEO: 0},
			},
			Load: LoadAnalyzer{Enabled: true, DefaultMode: "smoke", RequireThresholdsForScore: true},
		},
		AI: AI{
			PeerA: PeerA{Source: "terminal-mcp"},
			ExternalProvider: ExternalProvider{
				Type:                  "openai-compatible",
				Preset:                "9router",
				BaseURLHost:           "http://127.0.0.1:20128/v1",
				BaseURLDocker:         "http://host.docker.internal:20128/v1",
				APIKeyEnv:             "ANTHROPIC_AUTH_TOKEN",
				ModelDiscovery:        true,
				RequestTimeoutSeconds: 180,
			},
			Selection: Selection{
				Mode:    "hybrid",
				PeerA:   ModelChoice{Preferred: []string{"solidai-peer-a"}, Exclude: []string{}}, // SAI-118
				PeerB:   ModelChoice{Preferred: []string{}, Exclude: []string{}},
				Arbiter: ArbiterChoice{Preferred: []string{}, Exclude: []string{}, MustDifferFromPeerB: true},
				Probe:   Probe{Enabled: true, CacheHours: 168, RequireJSONCompliance: true},
			},
			Review: Review{
				PeerPromptVersion:    "peer-review-v1",
				ArbiterPromptVersion: "arbiter-v1",
				Temperature:          0,
				MaxRepairAttempts:    1,
			},
		},
		Report: Report{Language: "pt-BR", PDF: PDF{Paper: "A4", BrowserMode: "auto"}},
		Privacy: Privacy{
			PersistFullDiff:       true,
			PersistModelRequests:  false,
			PersistModelResponses: true,
			RedactSecrets:         true,
		},
	}
}

// EnvRefs devolve os nomes de variáveis de ambiente referenciadas pela config.
// Serve para `solidify doctor` reportar o que falta sem ler nenhum valor.
func (c Config) EnvRefs() []string {
	var refs []string
	if c.Analyzers.Sonar.TokenEnv != "" {
		refs = append(refs, c.Analyzers.Sonar.TokenEnv)
	}
	if c.AI.ExternalProvider.APIKeyEnv != "" {
		refs = append(refs, c.AI.ExternalProvider.APIKeyEnv)
	}
	return refs
}
