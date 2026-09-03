// E2E migration/env risk — SAI-110 acceptance.
package e2e

import (
	"testing"

	"github.com/diegoaraujo/solidify/internal/migenv"
)

// Aceitação: migration destrutiva (DROP) eleva risco (30 pts), mas
// sozinha não bloqueia (threshold default 50).
func TestE2E_DestructiveMigrationElevatesRisk(t *testing.T) {
	changes := []migenv.Change{
		{Kind: migenv.KindMigration, Path: "db/002_drop_legacy.sql", Removed: 1, RiskLevel: "high"},
	}
	rs := migenv.CalculateRisk(changes, migenv.DefaultThreshold)
	if rs.Total != 30 {
		t.Fatalf("risk total esperado 30, veio %d", rs.Total)
	}
	if rs.Breaks {
		t.Errorf("30 sozinho não deve bloquear (threshold=%d)", migenv.DefaultThreshold)
	}
	if len(rs.Reasons) == 0 {
		t.Errorf("migration alto risco devia gerar reason")
	}
}

// Aceitação: migration destrutiva + env destrutiva somam acima do
// threshold e bloqueiam a release.
func TestE2E_DestructiveMigrationPlusEnvBlocksRelease(t *testing.T) {
	changes := []migenv.Change{
		{Kind: migenv.KindMigration, Path: "db/002_drop_legacy.sql", Removed: 1, RiskLevel: "high"},
		{Kind: migenv.KindEnv, Path: ".env.example", Removed: 2, RiskLevel: "high"},
	}
	rs := migenv.CalculateRisk(changes, migenv.DefaultThreshold)
	if !rs.Breaks {
		t.Fatalf("esperado bloqueio (total=%d, threshold=%d): %+v", rs.Total, migenv.DefaultThreshold, rs)
	}
}

// Aceitação: migration destrutiva sem down = falha estrutural obrigatória,
// independente do risk score — sem rollback plan não sai release.
func TestE2E_DestructiveMigrationWithoutRollbackFails(t *testing.T) {
	check := migenv.MigrationCheck{HasUp: true, HasDown: false, HasBaseline: true}
	if err := migenv.ValidateMigration(check); err == nil {
		t.Fatal("esperado erro: migration destrutiva sem down")
	}
}
