package mcpserver

import (
	"strings"
	"testing"
)

// Aceitação: AllPrinciples cobre os 5 SOLID.
func TestAllPrinciples(t *testing.T) {
	ps := AllPrinciples()
	if len(ps) != 5 {
		t.Errorf("esperado 5 princípios, tem %d", len(ps))
	}
	want := map[SolidPrinciple]bool{
		PrincipleSRP: true, PrincipleOCP: true, PrincipleLSP: true,
		PrincipleISP: true, PrincipleDIP: true,
	}
	for _, p := range ps {
		if !want[p] {
			t.Errorf("princípio inesperado: %s", p)
		}
	}
}

// Aceitação: cenários cobrem SRP/OCP/LSP/ISP/DIP.
func TestPeerIngestScenariosCoverAll(t *testing.T) {
	scenarios := PeerIngestScenarios()
	if len(scenarios) == 0 {
		t.Fatal("vazio")
	}
	if !ExpectedCoverageAllPrinciples(scenarios) {
		t.Errorf("princípios não cobertos")
	}
}

// Aceitação: SRP cenários existem.
func TestScenarioSRP(t *testing.T) {
	scenarios := ScenarioByPrinciple(PeerIngestScenarios(), PrincipleSRP)
	if len(scenarios) < 1 {
		t.Errorf("sem SRP")
	}
	for _, s := range scenarios {
		if !s.Applicable {
			t.Errorf("SRP não-applicable: %s", s.Name)
		}
	}
}

// Aceitação: OCP cenários.
func TestScenarioOCP(t *testing.T) {
	scenarios := ScenarioByPrinciple(PeerIngestScenarios(), PrincipleOCP)
	if len(scenarios) < 1 {
		t.Errorf("sem OCP")
	}
}

// Aceitação: LSP tem applicable + not-applicable.
func TestScenarioLSP(t *testing.T) {
	scenarios := ScenarioByPrinciple(PeerIngestScenarios(), PrincipleLSP)
	hasApp, hasNot := false, false
	for _, s := range scenarios {
		if s.Applicable {
			hasApp = true
		} else {
			hasNot = true
		}
	}
	if !hasApp {
		t.Errorf("LSP sem applicable")
	}
	if !hasNot {
		t.Errorf("LSP sem not-applicable (N/A)")
	}
}

// Aceitação: ISP cenários.
func TestScenarioISP(t *testing.T) {
	scenarios := ScenarioByPrinciple(PeerIngestScenarios(), PrincipleISP)
	if len(scenarios) < 1 {
		t.Errorf("sem ISP")
	}
}

// Aceitação: DIP cenários.
func TestScenarioDIP(t *testing.T) {
	scenarios := ScenarioByPrinciple(PeerIngestScenarios(), PrincipleDIP)
	if len(scenarios) < 1 {
		t.Errorf("sem DIP")
	}
}

// Aceitação: ScenarioByName busca.
func TestScenarioByName(t *testing.T) {
	scenarios := PeerIngestScenarios()
	s, ok := ScenarioByName(scenarios, "srp_class_too_many_duties")
	if !ok {
		t.Fatal("não achou")
	}
	if s.Principle != PrincipleSRP {
		t.Errorf("principle")
	}
	_, ok = ScenarioByName(scenarios, "does-not-exist")
	if ok {
		t.Errorf("achou fantasma")
	}
}

// Aceitação: ValidateScenario OK.
func TestValidateScenarioOK(t *testing.T) {
	s := PeerIngestScenario{
		Name: "x", Principle: PrincipleSRP,
		ExpectFindings: 1, ExpectSeverity: "high", Applicable: true,
	}
	if errs := ValidateScenario(s); len(errs) > 0 {
		t.Errorf("esperado válido: %v", errs)
	}
}

// Aceitação: ValidateScenario errors.
func TestValidateScenarioErrors(t *testing.T) {
	s := PeerIngestScenario{
		Name: "", Principle: "",
		ExpectFindings: 0, ExpectSeverity: "invalid", Applicable: true,
	}
	errs := ValidateScenario(s)
	if len(errs) < 3 {
		t.Errorf("esperado ≥3 erros: %v", errs)
	}
}

// Aceitação: ValidateScenarios sem erros.
func TestValidateScenariosClean(t *testing.T) {
	errs := ValidateScenarios(PeerIngestScenarios())
	if len(errs) > 0 {
		t.Errorf("cenários com erro: %v", errs)
	}
}

// Aceitação: Summarize stats.
func TestSummarize(t *testing.T) {
	scenarios := PeerIngestScenarios()
	sum := Summarize(scenarios)
	if sum.Total != len(scenarios) {
		t.Errorf("total")
	}
	if sum.Applicable+sum.NotApplicable != sum.Total {
		t.Errorf("applicable+notapplicable != total")
	}
	if len(sum.ByPrinciple) != 5 {
		t.Errorf("princípios: %d", len(sum.ByPrinciple))
	}
	if sum.ByPrinciple["SRP"] < 1 {
		t.Errorf("SRP")
	}
	if sum.BySeverity["high"] < 1 {
		t.Errorf("high")
	}
}

// Aceitação: ScenariosToFindings filtra not-applicable.
func TestScenariosToFindingsFiltersNA(t *testing.T) {
	scenarios := PeerIngestScenarios()
	findings := ScenariosToFindings(scenarios)
	for _, f := range findings {
		if f.Principle == PrincipleLSP && f.Symbol == "(none)" {
			t.Errorf("N/A não devia virar finding")
		}
	}
	if len(findings) >= len(scenarios) {
		t.Errorf("devia filtrar N/A")
	}
}

// Aceitação: ScenarioToFinding copy.
func TestScenarioToFinding(t *testing.T) {
	s := PeerIngestScenario{
		Symbol: "X", Principle: PrincipleDIP,
		ExpectSeverity: "high", Description: "test",
		Applicable: true,
	}
	f := ScenarioToFinding(s)
	if f.Symbol != "X" || f.Principle != PrincipleDIP {
		t.Errorf("copy")
	}
	if !strings.Contains(f.Note, "test") {
		t.Errorf("note")
	}
}

// Aceitação: principle invariants (todas applicable têm findings>0).
func TestScenarioInvariants(t *testing.T) {
	for _, s := range PeerIngestScenarios() {
		if s.Applicable && s.ExpectFindings == 0 {
			t.Errorf("%s applicable sem findings", s.Name)
		}
		if !s.Applicable && s.ExpectFindings > 0 {
			t.Errorf("%s N/A com findings", s.Name)
		}
	}
}

// Aceitação: severity válida para todos.
func TestScenarioSeverities(t *testing.T) {
	for _, s := range PeerIngestScenarios() {
		switch s.ExpectSeverity {
		case "low", "medium", "high", "critical":
		default:
			t.Errorf("%s severity inválida: %s", s.Name, s.ExpectSeverity)
		}
	}
}

// Aceitação: evidence não-vazia para applicable.
func TestScenarioEvidence(t *testing.T) {
	for _, s := range PeerIngestScenarios() {
		if s.Applicable && s.Evidence == "" {
			t.Errorf("%s applicable sem evidence", s.Name)
		}
		if s.Applicable && s.Reference == "" {
			t.Errorf("%s applicable sem reference", s.Name)
		}
	}
}

// Aceitação: names únicos.
func TestScenarioUniqueNames(t *testing.T) {
	scenarios := PeerIngestScenarios()
	seen := make(map[string]bool)
	for _, s := range scenarios {
		if seen[s.Name] {
			t.Errorf("duplicado: %s", s.Name)
		}
		seen[s.Name] = true
	}
}

// Aceitação: ExpectedCoverageAllPrinciples edge cases.
func TestExpectedCoverageMissing(t *testing.T) {
	scenarios := []PeerIngestScenario{
		{Name: "x", Principle: PrincipleSRP, Applicable: true, ExpectFindings: 1, ExpectSeverity: "low"},
	}
	if ExpectedCoverageAllPrinciples(scenarios) {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ExpectedCoverageAllPrinciples all covered.
func TestExpectedCoverageAllCovered(t *testing.T) {
	scenarios := []PeerIngestScenario{
		{Name: "s", Principle: PrincipleSRP, Applicable: true, ExpectFindings: 1, ExpectSeverity: "low"},
		{Name: "o", Principle: PrincipleOCP, Applicable: true, ExpectFindings: 1, ExpectSeverity: "low"},
		{Name: "l", Principle: PrincipleLSP, Applicable: true, ExpectFindings: 1, ExpectSeverity: "low"},
		{Name: "i", Principle: PrincipleISP, Applicable: true, ExpectFindings: 1, ExpectSeverity: "low"},
		{Name: "d", Principle: PrincipleDIP, Applicable: true, ExpectFindings: 1, ExpectSeverity: "low"},
	}
	if !ExpectedCoverageAllPrinciples(scenarios) {
		t.Errorf("devia cobrir")
	}
}

// Aceitação: empty list.
func TestEmptyListCoverage(t *testing.T) {
	if ExpectedCoverageAllPrinciples(nil) {
		t.Errorf("vazio = não-coberto")
	}
}
