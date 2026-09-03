package migenv

import "testing"

// Aceitação: CalculateRisk vazio.
func TestCalculateRiskEmpty(t *testing.T) {
	rs := CalculateRisk(nil, 50)
	if rs.Total != 0 || rs.Breaks {
		t.Errorf("vazio: %+v", rs)
	}
}

// Aceitação: CalculateRisk threshold default.
func TestCalculateRiskDefault(t *testing.T) {
	changes := []Change{
		{Kind: KindMigration, RiskLevel: "high"}, // 30
		{Kind: KindEnv, RiskLevel: "high"},       // 25
	}
	rs := CalculateRisk(changes, 0) // default 50
	if !rs.Breaks {
		t.Errorf("expected break: %+v", rs)
	}
	if rs.Total != 55 {
		t.Errorf("total: %d", rs.Total)
	}
}

// Aceitação: CalculateRisk abaixo do threshold.
func TestCalculateRiskBelow(t *testing.T) {
	changes := []Change{
		{Kind: KindEnv, RiskLevel: "low"}, // 3
		{Kind: KindConfig, RiskLevel: "medium"}, // 5
	}
	rs := CalculateRisk(changes, 50)
	if rs.Breaks {
		t.Errorf("expected no break: %+v", rs)
	}
}

// Aceitação: ValidateMigration OK.
func TestValidateMigrationOK(t *testing.T) {
	c := MigrationCheck{HasUp: true, HasDown: true, HasBaseline: true}
	if err := ValidateMigration(c); err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: ValidateMigration sem up.
func TestValidateMigrationNoUp(t *testing.T) {
	c := MigrationCheck{HasDown: true, HasBaseline: true}
	if err := ValidateMigration(c); err == nil {
		t.Errorf("expected up error")
	}
}

// Aceitação: ValidateMigration sem down.
func TestValidateMigrationNoDown(t *testing.T) {
	c := MigrationCheck{HasUp: true, HasBaseline: true}
	if err := ValidateMigration(c); err == nil {
		t.Errorf("expected down error")
	}
}

// Aceitação: ValidateMigration sem baseline.
func TestValidateMigrationNoBaseline(t *testing.T) {
	c := MigrationCheck{HasUp: true, HasDown: true}
	if err := ValidateMigration(c); err == nil {
		t.Errorf("expected baseline error")
	}
}

// Aceitação: ValidateEnv sem warnings.
func TestValidateEnvOK(t *testing.T) {
	w := ValidateEnv(EnvCheck{HasDefaults: true})
	if len(w) != 0 {
		t.Errorf("warnings: %v", w)
	}
}

// Aceitação: ValidateEnv warnings.
func TestValidateEnvWarnings(t *testing.T) {
	c := EnvCheck{
		Removed:     []string{"OLD"},
		Added:       []string{"A", "B", "C", "D", "E", "F", "G"},
		HasDefaults: false,
		HasSecret:   true,
	}
	w := ValidateEnv(c)
	if len(w) < 3 {
		t.Errorf("esperado ≥3 warnings: %v", w)
	}
}
