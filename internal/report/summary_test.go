package report

import (
	"strings"
	"testing"
)

// Aceitação: summary básico.
func TestGenerateSummary(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", QualityScore: 85, Grade: "A",
		RiskLevel: "LOW", Confidence: 0.85, ConfLevel: "high",
	}
	r, err := GenerateSummary(in)
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	if !strings.Contains(r.Headline, "PASS") {
		t.Errorf("headline")
	}
	if len(r.Bullets) == 0 {
		t.Errorf("bullets")
	}
	if r.Body == "" {
		t.Errorf("body")
	}
}

// Aceitação: RunID vazio.
func TestSummaryEmptyRunID(t *testing.T) {
	if _, err := GenerateSummary(SummaryInput{Profile: "release"}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Profile vazio.
func TestSummaryEmptyProfile(t *testing.T) {
	if _, err := GenerateSummary(SummaryInput{RunID: "r1"}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: bullets com risk factors.
func TestSummaryRiskFactors(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", QualityScore: 85, Grade: "A",
		RiskLevel: "HIGH", RiskFactors: []string{"migration", "breaking"},
		Confidence: 0.85, ConfLevel: "high",
	}
	r, _ := GenerateSummary(in)
	found := false
	for _, b := range r.Bullets {
		if strings.Contains(b, "HIGH") && strings.Contains(b, "migration") {
			found = true
		}
	}
	if !found {
		t.Errorf("risk bullet: %v", r.Bullets)
	}
}

// Aceitação: divergences.
func TestSummaryDivergences(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", QualityScore: 85,
		RiskLevel: "LOW", Confidence: 0.85, ConfLevel: "high",
		Divergences: 3, ArbiterOK: true,
	}
	r, _ := GenerateSummary(in)
	found := false
	for _, b := range r.Bullets {
		if strings.Contains(b, "3") && strings.Contains(b, "resolvidas") {
			found = true
		}
	}
	if !found {
		t.Errorf("div: %v", r.Bullets)
	}
}

// Aceitação: divergences pendentes.
func TestSummaryDivergencesPending(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "INCOMPLETE", QualityScore: 50,
		RiskLevel: "LOW", Confidence: 0.5, ConfLevel: "medium",
		Divergences: 2, ArbiterOK: false,
	}
	r, _ := GenerateSummary(in)
	found := false
	for _, b := range r.Bullets {
		if strings.Contains(b, "pendentes") {
			found = true
		}
	}
	if !found {
		t.Errorf("pendente: %v", r.Bullets)
	}
}

// Aceitação: breaking changes.
func TestSummaryBreaking(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", QualityScore: 85,
		RiskLevel: "LOW", Confidence: 0.85, ConfLevel: "high",
		BreakingChanges: 2,
	}
	r, _ := GenerateSummary(in)
	found := false
	for _, b := range r.Bullets {
		if strings.Contains(b, "Breaking") && strings.Contains(b, "2") {
			found = true
		}
	}
	if !found {
		t.Errorf("breaking: %v", r.Bullets)
	}
}

// Aceitação: SOLID regressões.
func TestSummarySOLIDNegative(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", QualityScore: 85,
		RiskLevel: "LOW", Confidence: 0.85, ConfLevel: "high",
		SOLIDDeltas: map[string]float64{"S": -5, "O": 2, "L": -3},
	}
	r, _ := GenerateSummary(in)
	found := false
	for _, b := range r.Bullets {
		if strings.Contains(b, "regressões") {
			found = true
		}
	}
	if !found {
		t.Errorf("solid neg: %v", r.Bullets)
	}
}

// Aceitação: limitations + components.
func TestSummaryMisc(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", QualityScore: 85,
		RiskLevel: "LOW", Confidence: 0.85, ConfLevel: "high",
		Limitations: []string{"a", "b"},
		Components:  []string{"api", "worker"},
	}
	r, _ := GenerateSummary(in)
	hasLim, hasComp := false, false
	for _, b := range r.Bullets {
		if strings.Contains(b, "Limitações") {
			hasLim = true
		}
		if strings.Contains(b, "Componentes") {
			hasComp = true
		}
	}
	if !hasLim || !hasComp {
		t.Errorf("misc: %v", r.Bullets)
	}
}

// Aceitação: body com breaking + divergences.
func TestSummaryBody(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", QualityScore: 85,
		RiskLevel: "LOW", Confidence: 0.85, ConfLevel: "high",
		BreakingChanges: 1, Divergences: 2, ArbiterOK: true,
	}
	r, _ := GenerateSummary(in)
	if !strings.Contains(r.Body, "breaking") {
		t.Errorf("body: %s", r.Body)
	}
	if !strings.Contains(r.Body, "resolveu") {
		t.Errorf("resolved: %s", r.Body)
	}
}

// Aceitação: IAExtendedSummary.
func TestIAExtendedSummary(t *testing.T) {
	ia := NewIAExtendedSummary("r1", "base", "gc", "gemini-3", []string{"x", "y"})
	if ia.RunID != "r1" {
		t.Errorf("runid")
	}
	if len(ia.IABullets) != 2 {
		t.Errorf("bullets")
	}
}

// Aceitação: sources inclui divergence_map.
func TestSummarySourcesDiv(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", Divergences: 2,
		RiskLevel: "LOW", Confidence: 0.85, ConfLevel: "high",
	}
	r, _ := GenerateSummary(in)
	found := false
	for _, s := range r.Sources {
		if s == "divergence_map" {
			found = true
		}
	}
	if !found {
		t.Errorf("sources: %v", r.Sources)
	}
}

// Aceitação: sources inclui release_notes c/ breaking.
func TestSummarySourcesBreaking(t *testing.T) {
	in := SummaryInput{
		RunID: "r1", Profile: "release",
		GateStatus: "PASS", BreakingChanges: 1,
		RiskLevel: "LOW", Confidence: 0.85, ConfLevel: "high",
	}
	r, _ := GenerateSummary(in)
	found := false
	for _, s := range r.Sources {
		if s == "release_notes" {
			found = true
		}
	}
	if !found {
		t.Errorf("sources: %v", r.Sources)
	}
}

// Aceitação: helpers numéricos.
func TestSummaryHelpers(t *testing.T) {
	if ftoa2(1.5) != "1.50" {
		t.Errorf("1.50")
	}
	if itoa2(0) != "0" {
		t.Errorf("0")
	}
	if padLeft2("5", 2) != "05" {
		t.Errorf("pad")
	}
}

// Aceitação: negativeSOLID helper.
func TestNegativeSOLID(t *testing.T) {
	d := map[string]float64{"S": -5, "O": 2, "L": -3, "I": 0, "D": 10}
	neg := negativeSOLID(d)
	if len(neg) != 2 {
		t.Errorf("count: %v", neg)
	}
}

// Aceitação: joinSorted helper.
func TestJoinSorted(t *testing.T) {
	if joinSorted([]string{"c", "a", "b"}, ",") != "a,b,c" {
		t.Errorf("sort")
	}
	if joinSorted(nil, ",") != "" {
		t.Errorf("vazio")
	}
}
