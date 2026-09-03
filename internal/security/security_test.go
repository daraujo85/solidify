package security

import "testing"

// Aceitação: CalcScore sem findings.
func TestCalcScoreEmpty(t *testing.T) {
	s := CalcScore(nil)
	if s.Value != 100 {
		t.Errorf("valor = %d, quero 100", s.Value)
	}
	if s.Total != 0 {
		t.Errorf("total = %d", s.Total)
	}
}

// Aceitação: CalcScore deductions.
func TestCalcScoreDeductions(t *testing.T) {
	cases := []struct {
		findings []Finding
		want     int
	}{
		{[]Finding{{Severity: SevCritical}}, 75},
		{[]Finding{{Severity: SevHigh}}, 90},
		{[]Finding{{Severity: SevMedium}}, 97},
		{[]Finding{{Severity: SevLow}}, 99},
		{[]Finding{{Severity: SevInfo}}, 100},
		{[]Finding{{Severity: SevError}}, 90},   // Error = High
		{[]Finding{{Severity: SevWarning}}, 97}, // Warning = Medium
		{[]Finding{
			{Severity: SevCritical},
			{Severity: SevHigh},
			{Severity: SevMedium},
		}, 62}, // 100 - 25 - 10 - 3
		{[]Finding{
			{Severity: SevCritical},
			{Severity: SevCritical},
			{Severity: SevCritical},
			{Severity: SevCritical},
			{Severity: SevCritical},
		}, 0}, // floor
	}
	for _, c := range cases {
		got := CalcScore(c.findings).Value
		if got != c.want {
			t.Errorf("got %d, quero %d", got, c.want)
		}
	}
}

// Aceitação: CalcScore counts.
func TestCalcScoreCounts(t *testing.T) {
	s := CalcScore([]Finding{
		{Severity: SevHigh, Source: SourceSonar},
		{Severity: SevHigh, Source: SourceSonar},
		{Severity: SevMedium, Source: SourceSecrets},
	})
	if s.Total != 3 {
		t.Errorf("total = %d", s.Total)
	}
	if s.BySeverity[SevHigh] != 2 {
		t.Errorf("high = %d", s.BySeverity[SevHigh])
	}
	if s.BySource[SourceSonar] != 2 {
		t.Errorf("sonar = %d", s.BySource[SourceSonar])
	}
}

// Aceitação: EvaluateGate passes when under threshold.
func TestEvaluateGatePass(t *testing.T) {
	findings := []Finding{
		{Severity: SevLow, Source: SourceSonar},
		{Severity: SevMedium, Source: SourceSonar},
	}
	g := EvaluateGate(findings, DefaultThreshold())
	if !g.Allow {
		t.Errorf("devia passar: %s", g.Reason)
	}
}

// Aceitação: EvaluateGate blocks on critical.
func TestEvaluateGateCritical(t *testing.T) {
	findings := []Finding{
		{Severity: SevCritical, Source: SourceSonar},
	}
	g := EvaluateGate(findings, DefaultThreshold())
	if g.Allow {
		t.Errorf("devia bloquear")
	}
	if len(g.Blocked) != 1 {
		t.Errorf("blocked = %d", len(g.Blocked))
	}
}

// Aceitação: EvaluateGate blocks on high.
func TestEvaluateGateHigh(t *testing.T) {
	findings := []Finding{
		{Severity: SevHigh, Source: SourceSonar},
		{Severity: SevHigh, Source: SourceSonar},
		{Severity: SevHigh, Source: SourceSonar},
	}
	g := EvaluateGate(findings, DefaultThreshold())
	if g.Allow {
		t.Errorf("devia bloquear (3 high > 0)")
	}
}

// Aceitação: EvaluateGate blocks on medium.
func TestEvaluateGateMedium(t *testing.T) {
	findings := make([]Finding, 6)
	for i := range findings {
		findings[i] = Finding{Severity: SevMedium, Source: SourceSonar}
	}
	g := EvaluateGate(findings, DefaultThreshold())
	if g.Allow {
		t.Errorf("devia bloquear (6 medium > 5)")
	}
}

// Aceitação: EvaluateGate blocks on secrets.
func TestEvaluateGateSecrets(t *testing.T) {
	findings := []Finding{
		{Severity: SevHigh, Source: SourceSecrets},
	}
	g := EvaluateGate(findings, DefaultThreshold())
	if g.Allow {
		t.Errorf("devia bloquear por secret")
	}
}

// Aceitação: EvaluateGate blocks on critical vuln.
func TestEvaluateGateCriticalVuln(t *testing.T) {
	findings := []Finding{
		{Severity: SevCritical, Source: SourceOSV},
	}
	g := EvaluateGate(findings, DefaultThreshold())
	if g.Allow {
		t.Errorf("devia bloquear por vuln critical")
	}
}

// Aceitação: EvaluateGate secret LOW não bloqueia (BlockOnSecrets só high/critical).
func TestEvaluateGateSecretLow(t *testing.T) {
	findings := []Finding{
		{Severity: SevLow, Source: SourceSecrets},
	}
	g := EvaluateGate(findings, DefaultThreshold())
	if !g.Allow {
		t.Errorf("secret low não devia bloquear")
	}
}

// Aceitação: DefaultThreshold.
func TestDefaultThreshold(t *testing.T) {
	th := DefaultThreshold()
	if th.MaxCritical != 0 || th.MaxHigh != 0 {
		t.Errorf("err")
	}
	if !th.BlockOnSecrets || !th.BlockOnCriticalVuln {
		t.Errorf("err")
	}
}

// Aceitação: Threshold custom permite high.
func TestEvaluateGateCustomThreshold(t *testing.T) {
	th := Threshold{MaxCritical: 0, MaxHigh: 5, MaxMedium: 100}
	findings := []Finding{
		{Severity: SevHigh, Source: SourceSonar},
		{Severity: SevHigh, Source: SourceSonar},
	}
	g := EvaluateGate(findings, th)
	if !g.Allow {
		t.Errorf("custom threshold devia permitir")
	}
}

// Aceitação: SortFindings.
func TestSortFindings(t *testing.T) {
	findings := []Finding{
		{Severity: SevLow, Source: SourceSonar, FilePath: "z"},
		{Severity: SevCritical, Source: SourceOSV, FilePath: "a"},
		{Severity: SevHigh, Source: SourceSonar, FilePath: "a"},
	}
	SortFindings(findings)
	if findings[0].Severity != SevCritical {
		t.Errorf("sort err: %+v", findings)
	}
}

// Aceitação: severity rank.
func TestSeverityRank(t *testing.T) {
	if severityRank(SevCritical) >= severityRank(SevLow) {
		t.Errorf("rank ordem errada")
	}
	if severityRank(SevError) != severityRank(SevHigh) {
		t.Errorf("Error = High")
	}
	if severityRank(SevWarning) != severityRank(SevMedium) {
		t.Errorf("Warning = Medium")
	}
}

// Aceitação: MergeFindings.
func TestMergeFindings(t *testing.T) {
	a := []Finding{{Rule: "a"}}
	b := []Finding{{Rule: "b"}, {Rule: "c"}}
	merged := MergeFindings(a, b)
	if len(merged) != 3 {
		t.Errorf("len = %d", len(merged))
	}
}

// Aceitação: MergeFindings empty.
func TestMergeFindingsEmpty(t *testing.T) {
	merged := MergeFindings()
	if len(merged) != 0 {
		t.Errorf("err")
	}
}

// Aceitação: Gate vazio.
func TestEvaluateGateEmpty(t *testing.T) {
	g := EvaluateGate(nil, DefaultThreshold())
	if !g.Allow {
		t.Errorf("empty devia passar")
	}
	if g.Score.Value != 100 {
		t.Errorf("score = %d", g.Score.Value)
	}
}

// Aceitação: BlockOnSecrets false permite secret high (com threshold permissivo).
func TestEvaluateGateBlockOnSecretsFalse(t *testing.T) {
	th := Threshold{MaxHigh: 100, MaxMedium: 100, MaxLow: 100}
	th.BlockOnSecrets = false
	findings := []Finding{
		{Severity: SevHigh, Source: SourceSecrets},
	}
	g := EvaluateGate(findings, th)
	if !g.Allow {
		t.Errorf("BlockOnSecrets=false + threshold permissivo devia permitir: %s", g.Reason)
	}
}

// Aceitação: BlockOnCriticalVuln false permite OSV critical (com threshold permissivo).
func TestEvaluateGateBlockOnCriticalVulnFalse(t *testing.T) {
	th := Threshold{MaxCritical: 100, MaxHigh: 100, MaxMedium: 100, MaxLow: 100}
	th.BlockOnCriticalVuln = false
	findings := []Finding{
		{Severity: SevCritical, Source: SourceOSV},
	}
	g := EvaluateGate(findings, th)
	if !g.Allow {
		t.Errorf("BlockOnCriticalVuln=false + threshold permissivo devia permitir: %s", g.Reason)
	}
}

// Aceitação: Low threshold.
func TestEvaluateGateLow(t *testing.T) {
	findings := make([]Finding, 51)
	for i := range findings {
		findings[i] = Finding{Severity: SevLow, Source: SourceSonar}
	}
	g := EvaluateGate(findings, DefaultThreshold())
	if g.Allow {
		t.Errorf("devia bloquear (51 low > 50)")
	}
}

// Aceitação: Score floor 0.
func TestCalcScoreFloor(t *testing.T) {
	findings := []Finding{
		{Severity: SevCritical},
		{Severity: SevCritical},
		{Severity: SevCritical},
		{Severity: SevCritical},
		{Severity: SevCritical},
	}
	if CalcScore(findings).Value != 0 {
		t.Errorf("floor não bateu")
	}
}

// Aceitação: Sources constants.
func TestSourceConstants(t *testing.T) {
	if SourceSonar == "" || SourceSecrets == "" {
		t.Errorf("constants vazias")
	}
}

// Aceitação: Finding Extras.
func TestFindingExtras(t *testing.T) {
	f := Finding{
		Extras: map[string]string{"key": "value"},
	}
	if f.Extras["key"] != "value" {
		t.Errorf("err")
	}
}
