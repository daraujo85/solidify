// E2E migration/env risk (SAI-110).
//
// Detecta mudanças em migrations e env vars; calcula
// risk score; bloqueia release se score > threshold.
package migenv

import "errors"

// ChangeKind tipo de mudança.
type ChangeKind string

const (
	KindMigration ChangeKind = "migration"
	KindEnv       ChangeKind = "env"
	KindConfig    ChangeKind = "config"
)

// Change uma mudança detectada.
type Change struct {
	Kind      ChangeKind
	Path      string
	Added     int
	Removed   int
	Modified  int
	RiskLevel string // low|medium|high
}

// RiskWeights peso por kind+level.
var RiskWeights = map[ChangeKind]map[string]int{
	KindMigration: {"low": 5, "medium": 15, "high": 30},
	KindEnv:       {"low": 3, "medium": 10, "high": 25},
	KindConfig:    {"low": 1, "medium": 5, "high": 10},
}

// RiskScore total.
type RiskScore struct {
	Total   int
	Breaks  bool
	Reasons []string
}

// DefaultThreshold bloqueio.
const DefaultThreshold = 50

// CalculateRisk soma scores.
func CalculateRisk(changes []Change, threshold int) RiskScore {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	rs := RiskScore{}
	for _, c := range changes {
		w, ok := RiskWeights[c.Kind]
		if !ok {
			continue
		}
		score, ok := w[c.RiskLevel]
		if !ok {
			continue
		}
		rs.Total += score
		if score >= 15 {
			rs.Reasons = append(rs.Reasons, string(c.Kind)+" alto risco")
		}
	}
	rs.Breaks = rs.Total >= threshold
	return rs
}

// MigrationCheck helpers.
type MigrationCheck struct {
	HasUp       bool
	HasDown     bool
	HasBaseline bool
	Valid       bool
	Reasons     []string
}

// ValidateMigration valida estrutura de migration.
func ValidateMigration(c MigrationCheck) error {
	if !c.HasUp {
		return errors.New("migenv: sem up")
	}
	if !c.HasDown {
		return errors.New("migenv: sem down")
	}
	if !c.HasBaseline {
		return errors.New("migenv: sem baseline")
	}
	return nil
}

// EnvCheck helpers.
type EnvCheck struct {
	Added       []string
	Removed     []string
	HasDefaults bool
	HasSecret   bool // secret exposto
}

// ValidateEnv checa mudanças de env.
func ValidateEnv(c EnvCheck) []string {
	warnings := []string{}
	if len(c.Removed) > 0 {
		warnings = append(warnings, "env vars removidas")
	}
	if len(c.Added) > 5 {
		warnings = append(warnings, "muitas env vars adicionadas")
	}
	if !c.HasDefaults {
		warnings = append(warnings, "sem defaults")
	}
	if c.HasSecret {
		warnings = append(warnings, "secret exposto")
	}
	return warnings
}
