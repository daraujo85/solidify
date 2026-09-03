// Peer A ingestion scenarios (SAI-057).
//
// Fixtures representando violações dos princípios SOLID para
// alimentar o Peer Review. Cada caso tem: símbolo, evidência
// esperada, e rationale.
package mcpserver

// SolidPrinciple princípio SOLID.
type SolidPrinciple string

const (
	PrincipleSRP SolidPrinciple = "SRP" // Single Responsibility
	PrincipleOCP SolidPrinciple = "OCP" // Open/Closed
	PrincipleLSP SolidPrinciple = "LSP" // Liskov Substitution
	PrincipleISP SolidPrinciple = "ISP" // Interface Segregation
	PrincipleDIP SolidPrinciple = "DIP" // Dependency Inversion
)

// AllPrinciples lista completa.
func AllPrinciples() []SolidPrinciple {
	return []SolidPrinciple{PrincipleSRP, PrincipleOCP, PrincipleLSP, PrincipleISP, PrincipleDIP}
}

// PeerIngestScenario cenário de ingestão.
type PeerIngestScenario struct {
	Name        string         `json:"name"`
	Principle   SolidPrinciple `json:"principle"`
	Symbol      string         `json:"symbol"`
	Description string         `json:"description"`
	// Findings esperados (count, severity principal).
	ExpectFindings int    `json:"expect_findings"`
	ExpectSeverity string `json:"expect_severity"` // "low"|"medium"|"high"|"critical"
	// Evidence snippet que o peer deve olhar.
	Evidence string `json:"evidence"`
	// Reference anchor: file:line aproximado.
	Reference string `json:"reference"`
	// Applicable quando false, cenário é N/A (ex: LSP sem herança).
	Applicable bool `json:"applicable"`
}

// PeerIngestScenarios conjunto de fixtures canônicas.
func PeerIngestScenarios() []PeerIngestScenario {
	return []PeerIngestScenario{
		{
			Name:           "srp_class_too_many_duties",
			Principle:      PrincipleSRP,
			Symbol:         "UserService",
			Description:    "Classe com 4+ motivos para mudar: persiste, notifica, valida, exporta. SRP violada.",
			ExpectFindings: 1,
			ExpectSeverity: "high",
			Evidence:       "func (s *UserService) Save(...) ... func (s *UserService) SendEmail(...) ... func (s *UserService) Validate(...) ... func (s *UserService) ExportCSV(...)",
			Reference:      "internal/users/service.go:1-300",
			Applicable:     true,
		},
		{
			Name:           "srp_mixed_io_and_domain",
			Principle:      PrincipleSRP,
			Symbol:         "OrderProcessor",
			Description:    "Método Process carrega IO (http.Get) e regra de domínio misturados.",
			ExpectFindings: 1,
			ExpectSeverity: "medium",
			Evidence:       "func (p *OrderProcessor) Process(id string) { resp, _ := http.Get(...); p.applyRules(resp) }",
			Reference:      "internal/orders/processor.go:42",
			Applicable:     true,
		},
		{
			Name:           "ocp_modify_existing_for_extension",
			Principle:      PrincipleOCP,
			Symbol:         "PaymentRouter",
			Description:    "Adicionar novo provider requer editar switch/case central. OCP violada.",
			ExpectFindings: 1,
			ExpectSeverity: "medium",
			Evidence:       "func (r *PaymentRouter) Route(p Provider) { switch p.Type { case Stripe: ... case PayPal: ... } }",
			Reference:      "internal/payments/router.go:18",
			Applicable:     true,
		},
		{
			Name:           "ocp_flag_parameter",
			Principle:      PrincipleOCP,
			Symbol:         "Renderer",
			Description:    "Função recebe booleano `asHtml` que muda comportamento — extensão via flag, não via subtipo.",
			ExpectFindings: 1,
			ExpectSeverity: "low",
			Evidence:       "func Render(t Template, asHtml bool) string { if asHtml { ... } else { ... } }",
			Reference:      "internal/render/render.go:8",
			Applicable:     true,
		},
		{
			Name:           "lsp_subtype_breaks_contract",
			Principle:      PrincipleLSP,
			Symbol:         "Square : Rectangle",
			Description:    "Square herda Rectangle mas SetWidth/SetHeight violam invariante (área quadrado ≠ retângulo quando w≠h).",
			ExpectFindings: 1,
			ExpectSeverity: "high",
			Evidence:       "func (s *Square) SetWidth(w int) { s.width = w; s.height = w } // quebra Rectangle.SetHeight",
			Reference:      "internal/geom/square.go:12",
			Applicable:     true,
		},
		{
			Name:           "lsp_not_applicable_no_inheritance",
			Principle:      PrincipleLSP,
			Symbol:         "(none)",
			Description:    "Codebase sem hierarquia de tipos — LSP não aplicável neste run.",
			ExpectFindings: 0,
			ExpectSeverity: "low",
			Evidence:       "grep -r 'struct.*struct' internal/ | wc -l → 0",
			Reference:      "(repo-wide)",
			Applicable:     false,
		},
		{
			Name:           "isp_fat_interface",
			Principle:      PrincipleISP,
			Symbol:         "Repository",
			Description:    "Interface com 15 métodos forçando implementações a depender de métodos que não usam.",
			ExpectFindings: 1,
			ExpectSeverity: "medium",
			Evidence:       "type Repository interface { Find; Save; Delete; Update; List; Count; BulkInsert; Export; Import; Migrate; ... }",
			Reference:      "internal/store/repository.go:5",
			Applicable:     true,
		},
		{
			Name:           "isp_interface_with_io",
			Principle:      PrincipleISP,
			Symbol:         "UserReader",
			Description:    "UserReader inclui Write além de Read — força readers read-only a ter Write no-op.",
			ExpectFindings: 1,
			ExpectSeverity: "low",
			Evidence:       "type UserReader interface { Read(id) (*User, error); Write(*User) error }",
			Reference:      "internal/users/reader.go:4",
			Applicable:     true,
		},
		{
			Name:           "dip_concrete_dependency",
			Principle:      PrincipleDIP,
			Symbol:         "MySQLOrderRepo",
			Description:    "Service depende de MySQLOrderRepo concreto, não de interface OrderRepository. DIP violada.",
			ExpectFindings: 1,
			ExpectSeverity: "high",
			Evidence:       "type OrderService struct { repo *MySQLOrderRepo } // concreto no campo",
			Reference:      "internal/orders/service.go:14",
			Applicable:     true,
		},
		{
			Name:           "dip_static_factory_call",
			Principle:      PrincipleDIP,
			Symbol:         "Notifier",
			Description:    "Notifier chama `&SMTPClient{}` direto — new() acoplado. DIP violada em runtime.",
			ExpectFindings: 1,
			ExpectSeverity: "medium",
			Evidence:       "func NewNotifier() *Notifier { return &Notifier{client: &SMTPClient{}} }",
			Reference:      "internal/notify/notifier.go:22",
			Applicable:     true,
		},
	}
}

// ScenarioByPrinciple filtra cenários por princípio.
func ScenarioByPrinciple(scenarios []PeerIngestScenario, p SolidPrinciple) []PeerIngestScenario {
	out := make([]PeerIngestScenario, 0)
	for _, s := range scenarios {
		if s.Principle == p {
			out = append(out, s)
		}
	}
	return out
}

// ScenarioByName busca por nome único.
func ScenarioByName(scenarios []PeerIngestScenario, name string) (PeerIngestScenario, bool) {
	for _, s := range scenarios {
		if s.Name == name {
			return s, true
		}
	}
	return PeerIngestScenario{}, false
}

// ValidateScenario checa consistência de um cenário.
func ValidateScenario(s PeerIngestScenario) []string {
	var errs []string
	if s.Name == "" {
		errs = append(errs, "name vazio")
	}
	if s.Principle == "" {
		errs = append(errs, "principle vazio")
	}
	if s.Applicable && s.ExpectFindings == 0 {
		errs = append(errs, "applicable mas expect_findings=0")
	}
	if !s.Applicable && s.ExpectFindings > 0 {
		errs = append(errs, "not applicable mas expect_findings>0")
	}
	switch s.ExpectSeverity {
	case "low", "medium", "high", "critical":
	default:
		errs = append(errs, "severity inválida")
	}
	return errs
}

// ValidateScenarios valida lista inteira. Retorna map name→errs.
func ValidateScenarios(scenarios []PeerIngestScenario) map[string][]string {
	out := make(map[string][]string)
	for _, s := range scenarios {
		if errs := ValidateScenario(s); len(errs) > 0 {
			out[s.Name] = errs
		}
	}
	return out
}

// IngestionSummary sumariza cenários.
type IngestionSummary struct {
	Total         int            `json:"total"`
	Applicable    int            `json:"applicable"`
	NotApplicable int            `json:"not_applicable"`
	ByPrinciple   map[string]int `json:"by_principle"`
	BySeverity    map[string]int `json:"by_severity"`
}

// Summarize agrega estatísticas.
func Summarize(scenarios []PeerIngestScenario) IngestionSummary {
	sum := IngestionSummary{
		Total:       len(scenarios),
		ByPrinciple: make(map[string]int),
		BySeverity:  make(map[string]int),
	}
	for _, s := range scenarios {
		if s.Applicable {
			sum.Applicable++
		} else {
			sum.NotApplicable++
		}
		sum.ByPrinciple[string(s.Principle)]++
		if s.ExpectSeverity != "" {
			sum.BySeverity[s.ExpectSeverity]++
		}
	}
	return sum
}

// ExpectedCoverageAllPrinciples verifica se todos os 5 princípios
// estão representados.
func ExpectedCoverageAllPrinciples(scenarios []PeerIngestScenario) bool {
	covered := make(map[SolidPrinciple]bool)
	for _, s := range scenarios {
		if s.Applicable {
			covered[s.Principle] = true
		}
	}
	for _, p := range AllPrinciples() {
		if !covered[p] {
			return false
		}
	}
	return true
}

// IngestionFinding converte cenário em finding estruturado.
type IngestionFinding struct {
	Symbol    string         `json:"symbol"`
	Principle SolidPrinciple `json:"principle"`
	Severity  string         `json:"severity"`
	Note      string         `json:"note"`
}

// ScenarioToFinding produz finding para um cenário.
func ScenarioToFinding(s PeerIngestScenario) IngestionFinding {
	return IngestionFinding{
		Symbol:    s.Symbol,
		Principle: s.Principle,
		Severity:  s.ExpectSeverity,
		Note:      s.Description,
	}
}

// ScenariosToFindings converte lista.
func ScenariosToFindings(scenarios []PeerIngestScenario) []IngestionFinding {
	out := make([]IngestionFinding, 0)
	for _, s := range scenarios {
		if s.Applicable {
			out = append(out, ScenarioToFinding(s))
		}
	}
	return out
}
