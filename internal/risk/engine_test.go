package risk

import (
	"strings"
	"testing"
)

// Aceitação: Compute basic.
func TestComputeBasic(t *testing.T) {
	in := RiskInput{
		RunID: "r1",
		Factors: []RiskFactor{
			{Kind: KindSecurity, Severity: SeverityHigh, Description: "XSS"},
		},
	}
	r, err := Compute(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// security=20, high=3 → 60
	if r.RawScore != 60 {
		t.Errorf("raw: %d", r.RawScore)
	}
	if r.Normalized != 60 {
		t.Errorf("normalized: %f", r.Normalized)
	}
}

// Aceitação: RunID vazio.
func TestComputeEmptyRunID(t *testing.T) {
	if _, err := Compute(RiskInput{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: saturação no MaxRawScore.
func TestComputeSaturate(t *testing.T) {
	in := RiskInput{
		RunID: "r1",
		Factors: []RiskFactor{
			{Kind: KindSecurity, Severity: SeverityCritical},
			{Kind: KindBreaking, Severity: SeverityCritical},
			{Kind: KindMigration, Severity: SeverityCritical},
			{Kind: KindBlast, Severity: SeverityCritical},
			{Kind: KindEnv, Severity: SeverityCritical},
		},
	}
	r, _ := Compute(in)
	if r.RawScore > MaxRawScore {
		t.Errorf("saturado: %d", r.RawScore)
	}
	if r.Level != SeverityCritical {
		t.Errorf("level: %s", r.Level)
	}
}

// Aceitação: level low.
func TestLevelLow(t *testing.T) {
	if got := computeLevel(10); got != SeverityLow {
		t.Errorf("low: %s", got)
	}
}

// Aceitação: level medium.
func TestLevelMedium(t *testing.T) {
	r := &RiskScore{}
	r.Level = computeLevel(30)
	if r.Level != SeverityMedium {
		t.Errorf("medium")
	}
}

// Aceitação: level high.
func TestLevelHigh(t *testing.T) {
	r := &RiskScore{}
	r.Level = computeLevel(60)
	if r.Level != SeverityHigh {
		t.Errorf("high: %s", r.Level)
	}
}

// Aceitação: level critical.
func TestLevelCritical(t *testing.T) {
	r := &RiskScore{}
	r.Level = computeLevel(80)
	if r.Level != SeverityCritical {
		t.Errorf("critical")
	}
}

// Aceitação: breakdown por kind.
func TestBreakdownByKind(t *testing.T) {
	in := RiskInput{
		RunID: "r1",
		Factors: []RiskFactor{
			{Kind: KindSecurity, Severity: SeverityHigh},   // 20*3=60
			{Kind: KindSecurity, Severity: SeverityMedium}, // 20*2=40
		},
	}
	r, _ := Compute(in)
	if got := r.BreakdownByKind(KindSecurity); got != 60+40 {
		t.Errorf("breakdown: %d", got)
	}
	if got := r.BreakdownByKind(KindMigration); got != 0 {
		t.Errorf("vazio: %d", got)
	}
}

// Aceitação: topFactors ordenação.
func TestTopFactors(t *testing.T) {
	factors := []RiskFactor{
		{Kind: KindMigration, Severity: SeverityLow},   // 10*1=10
		{Kind: KindSecurity, Severity: SeverityHigh},   // 20*3=60
		{Kind: KindBreaking, Severity: SeverityMedium}, // 15*2=30
	}
	top := topFactors(factors, 5)
	if len(top) != 3 {
		t.Errorf("count: %d", len(top))
	}
	if top[0].Kind != KindSecurity {
		t.Errorf("top0: %s", top[0].Kind)
	}
}

// Aceitação: topFactors limit.
func TestTopFactorsLimit(t *testing.T) {
	factors := []RiskFactor{
		{Kind: KindMigration, Severity: SeverityHigh},
		{Kind: KindEnv, Severity: SeverityHigh},
		{Kind: KindBreaking, Severity: SeverityHigh},
	}
	top := topFactors(factors, 2)
	if len(top) != 2 {
		t.Errorf("limit: %d", len(top))
	}
}

// Aceitação: ValidateKind.
func TestValidateKind(t *testing.T) {
	for _, k := range []string{KindMigration, KindEnv, KindBreaking, KindSecurity, KindBlast, KindDisagree} {
		if !ValidateKind(k) {
			t.Errorf("%s inválido", k)
		}
	}
	if ValidateKind("random") {
		t.Errorf("random")
	}
}

// Aceitação: unknown kind tratado.
func TestUnknownKind(t *testing.T) {
	in := RiskInput{
		RunID: "r1",
		Factors: []RiskFactor{
			{Kind: "unknown", Severity: SeverityHigh},
		},
	}
	r, _ := Compute(in)
	if r.RawScore == 0 {
		t.Errorf("unknown deveria ter peso default")
	}
	if !ValidateKind(in.Factors[0].Kind) {
		// confirma
	}
}

// Aceitação: severity desconhecida → low.
func TestUnknownSeverity(t *testing.T) {
	in := RiskInput{
		RunID: "r1",
		Factors: []RiskFactor{
			{Kind: KindMigration, Severity: "weird"},
		},
	}
	r, _ := Compute(in)
	if r.RawScore != 10*1 {
		t.Errorf("weird → 1: %d", r.RawScore)
	}
}

// Aceitação: RenderScore.
func TestRenderScore(t *testing.T) {
	r := &RiskScore{
		RunID: "r1", RawScore: 60, Normalized: 60, Level: SeverityHigh,
		Breakdown:  map[string]int{KindSecurity: 60},
		TopFactors: []RiskFactor{{Kind: KindSecurity, Severity: SeverityHigh, Description: "XSS"}},
	}
	out := RenderScore(r)
	if !strings.Contains(out, "r1") {
		t.Errorf("runid")
	}
	if !strings.Contains(out, KindSecurity) {
		t.Errorf("security")
	}
	if !strings.Contains(out, "XSS") {
		t.Errorf("desc")
	}
	if RenderScore(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: IsShippable.
func TestIsShippable(t *testing.T) {
	var r *RiskScore
	if r.IsShippable() {
		t.Errorf("nil")
	}
	r = &RiskScore{Level: SeverityLow}
	if !r.IsShippable() {
		t.Errorf("low")
	}
	r = &RiskScore{Level: SeverityMedium}
	if !r.IsShippable() {
		t.Errorf("medium")
	}
	r = &RiskScore{Level: SeverityHigh}
	if r.IsShippable() {
		t.Errorf("high não shippable")
	}
	r = &RiskScore{Level: SeverityCritical}
	if r.IsShippable() {
		t.Errorf("critical não shippable")
	}
}

// Aceitação: Weights.
func TestWeightsDefined(t *testing.T) {
	for _, k := range []string{KindMigration, KindEnv, KindBreaking, KindSecurity, KindBlast, KindDisagree} {
		if Weights[k] == 0 {
			t.Errorf("%s sem peso", k)
		}
	}
}

// Aceitação: helpers.
func TestHelpers(t *testing.T) {
	if itoa3(0) != "0" {
		t.Errorf("0")
	}
	if itoa3(42) != "42" {
		t.Errorf("42")
	}
	if itoa3(-5) != "-5" {
		t.Errorf("neg")
	}
	if ftoa3(1.5) != "1.50" {
		t.Errorf("1.50")
	}
	if padLeft("5", 2) != "05" {
		t.Errorf("pad")
	}
	if padLeft("55", 2) != "55" {
		t.Errorf("nopad")
	}
}

// Aceitação: severity rank.
func TestSeverityRank(t *testing.T) {
	if SeverityRank[SeverityCritical] <= SeverityRank[SeverityHigh] {
		t.Errorf("rank")
	}
	if SeverityRank[SeverityHigh] <= SeverityRank[SeverityMedium] {
		t.Errorf("rank high/med")
	}
}

// Aceitação: empty factors → zero score.
func TestEmptyFactors(t *testing.T) {
	in := RiskInput{RunID: "r1"}
	r, _ := Compute(in)
	if r.RawScore != 0 {
		t.Errorf("vazio")
	}
	if r.Level != SeverityLow {
		t.Errorf("level: %s", r.Level)
	}
}
