package lighthouse

import (
	"strings"
	"testing"
)

// Aceitação: Score ToPercent.
func TestScoreToPercent(t *testing.T) {
	if Score(0.5).ToPercent() != 50 {
		t.Errorf("50")
	}
	if Score(1.0).ToPercent() != 100 {
		t.Errorf("100")
	}
	if Score(0).ToPercent() != 0 {
		t.Errorf("0")
	}
}

// Aceitação: Score IsZero.
func TestScoreIsZero(t *testing.T) {
	if !Score(0).IsZero() {
		t.Errorf("0")
	}
	if Score(0.01).IsZero() {
		t.Errorf("0.01 não-zero")
	}
}

// Aceitação: DefaultWeights soma = 1.0.
func TestDefaultWeights(t *testing.T) {
	w := DefaultWeights()
	sum := w.Performance + w.Accessibility + w.BestPractices + w.SEO
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("sum = %f, quero 1.0", sum)
	}
	if w.SEO != 0 {
		t.Errorf("SEO default = informativo (0)")
	}
}

// Aceitação: AggregatePillar perf=1.0, a11y=1.0, bp=1.0 = 1.0.
func TestAggregatePillarPerfect(t *testing.T) {
	p := Pillar{Performance: 1, Accessibility: 1, BestPractices: 1, SEO: 1}
	got := AggregatePillar(p, DefaultWeights())
	if got != 1.0 {
		t.Errorf("got %f, quero 1.0", got)
	}
}

// Aceitação: AggregatePillar weighted.
func TestAggregatePillarWeighted(t *testing.T) {
	p := Pillar{Performance: 1, Accessibility: 0, BestPractices: 0}
	// 0.4*1 + 0.3*0 + 0.3*0 = 0.4
	got := AggregatePillar(p, DefaultWeights())
	if got < 0.39 || got > 0.41 {
		t.Errorf("got %f, quero ~0.4", got)
	}
}

// Aceitação: AggregatePillar zero weights = 0.
func TestAggregatePillarZero(t *testing.T) {
	w := DefaultPillarWeights{}
	if AggregatePillar(Pillar{Performance: 1}, w) != 0 {
		t.Errorf("zero weights = 0")
	}
}

// Aceitação: AggregatePillar SEO informativo não conta.
func TestAggregatePillarSEOInformative(t *testing.T) {
	p := Pillar{Performance: 0.5, Accessibility: 0.5, BestPractices: 0.5, SEO: 1.0}
	// SEO=0 peso → perf=0.5 + a11y=0.5 + bp=0.5 = 0.5 (sem diluição)
	got := AggregatePillar(p, DefaultWeights())
	if got < 0.49 || got > 0.51 {
		t.Errorf("got %f, quero 0.5", got)
	}
}

// Aceitação: ParseLighthouseJSON básico.
func TestParseLighthouseJSONBasic(t *testing.T) {
	data := `{
		"finalUrl":"https://x.com/",
		"fetchTime":"2026-01-01T00:00:00.000Z",
		"userAgent":"Mozilla/5.0 Chrome",
		"categories":{
			"performance":{"id":"performance","title":"Performance","score":0.9},
			"accessibility":{"id":"accessibility","title":"A11y","score":0.8},
			"best-practices":{"id":"best-practices","title":"BP","score":1.0},
			"seo":{"id":"seo","title":"SEO","score":0.7}
		},
		"audits":{}
	}`
	rep, err := ParseLighthouseJSON([]byte(data), "https://x.com/", "container")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rep.Pillar.Performance != 0.9 {
		t.Errorf("perf = %f", rep.Pillar.Performance)
	}
	if rep.Pillar.SEO != 0.7 {
		t.Errorf("seo = %f", rep.Pillar.SEO)
	}
	if rep.AggregateScore == 0 {
		t.Errorf("agg vazio")
	}
	if len(rep.Categories) != 4 {
		t.Errorf("cats = %d", len(rep.Categories))
	}
}

// Aceitação: ParseLighthouseJSON inválido.
func TestParseLighthouseJSONInvalid(t *testing.T) {
	if _, err := ParseLighthouseJSON([]byte("garbage"), "x", "container"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ParseLighthouseJSON sem categories.
func TestParseLighthouseJSONNoCats(t *testing.T) {
	if _, err := ParseLighthouseJSON([]byte(`{}`), "x", "container"); err == nil {
		t.Errorf("sem cats devia falhar")
	}
}

// Aceitação: ParseLighthouseJSON só SEO (informativo).
func TestParseLighthouseJSONSEOOnly(t *testing.T) {
	data := `{
		"finalUrl":"x",
		"categories":{"seo":{"id":"seo","title":"SEO","score":0.5}},
		"audits":{}
	}`
	rep, err := ParseLighthouseJSON([]byte(data), "x", "container")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !rep.IsInformativeOnly() {
		t.Errorf("só SEO devia ser informative-only")
	}
}

// Aceitação: ParseLighthouseJSON perf + a11y + bp = não informativo.
func TestParseLighthouseJSONNotInformative(t *testing.T) {
	data := `{
		"finalUrl":"x",
		"categories":{
			"performance":{"id":"performance","title":"P","score":0.5},
			"accessibility":{"id":"accessibility","title":"A","score":0.5},
			"best-practices":{"id":"best-practices","title":"B","score":0.5}
		},
		"audits":{}
	}`
	rep, _ := ParseLighthouseJSON([]byte(data), "x", "container")
	if rep.IsInformativeOnly() {
		t.Errorf("não devia ser informative-only")
	}
}

// Aceitação: ShouldRun.
func TestShouldRun(t *testing.T) {
	if ShouldRun("") {
		t.Errorf("vazio não roda")
	}
	if ShouldRun(string(ModeDisabled)) {
		t.Errorf("disabled não roda")
	}
	if !ShouldRun(string(ModeContainer)) {
		t.Errorf("container roda")
	}
	if !ShouldRun(string(ModeLocal)) {
		t.Errorf("local roda")
	}
}

// Aceitação: CategoryScoreByKey.
func TestCategoryScoreByKey(t *testing.T) {
	rep := &Report{
		Categories: []Category{
			{Key: CatPerformance, Score: 0.9},
			{Key: CatSEO, Score: 0.7},
		},
	}
	if rep.CategoryScoreByKey(CatPerformance) != 0.9 {
		t.Errorf("perf")
	}
	if rep.CategoryScoreByKey(CatSEO) != 0.7 {
		t.Errorf("seo")
	}
	if rep.CategoryScoreByKey(CatPWA) != 0 {
		t.Errorf("missing = 0")
	}
}

// Aceitação: HasPerformance.
func TestHasPerformance(t *testing.T) {
	if !(&Report{Pillar: Pillar{Performance: 0.9}}).HasPerformance() {
		t.Errorf("devia ter")
	}
	if (&Report{}).HasPerformance() {
		t.Errorf("vazio")
	}
}

// Aceitação: FailedAudits.
func TestFailedAudits(t *testing.T) {
	rep := &Report{
		Categories: []Category{
			{Key: CatPerformance, Audits: []Audit{
				{ID: "a", Score: 0.9}, // pass
				{ID: "b", Score: 0.3}, // fail
			}},
		},
	}
	failed := rep.FailedAudits(0.5)
	if len(failed) != 1 || failed[0] != "b" {
		t.Errorf("got %v", failed)
	}
}

// Aceitação: FailedAudits empty.
func TestFailedAuditsEmpty(t *testing.T) {
	failed := (&Report{}).FailedAudits(0.5)
	if len(failed) != 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: IsPassing.
func TestIsPassing(t *testing.T) {
	rep := &Report{AggregateScore: 0.85}
	if !rep.IsPassing(0.8) {
		t.Errorf("0.85 >= 0.8 devia passar")
	}
	if rep.IsPassing(0.9) {
		t.Errorf("0.85 < 0.9 não devia passar")
	}
}

// Aceitação: EmptyReport.
func TestEmptyReport(t *testing.T) {
	r := EmptyReport("x", "skip")
	if r.Mode != ModeDisabled {
		t.Errorf("mode")
	}
	if !strings.Contains(r.Error, "skip") {
		t.Errorf("err = %q", r.Error)
	}
}

// Aceitação: SanitizeUserAgent.
func TestSanitizeUserAgent(t *testing.T) {
	if SanitizeUserAgent("") != "" {
		t.Errorf("empty")
	}
	if SanitizeUserAgent("Mozilla/5.0 (X11)") != "Mozilla/5.0 " {
		t.Errorf("got %q", SanitizeUserAgent("Mozilla/5.0 (X11)"))
	}
	if SanitizeUserAgent("Chrome") != "Chrome" {
		t.Errorf("no parens")
	}
}

// Aceitação: DefaultConfig.
func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Mode != ModeContainer {
		t.Errorf("mode")
	}
	if !c.Headless {
		t.Errorf("headless")
	}
}

// Aceitação: Mode constants.
func TestModeConstants(t *testing.T) {
	if ModeContainer == "" || ModeDisabled == "" || ModeLocal == "" {
		t.Errorf("constants")
	}
}

// Aceitação: CategoryKey constants.
func TestCategoryKeyConstants(t *testing.T) {
	for _, c := range []CategoryKey{CatPerformance, CatAccessibility, CatBestPractices, CatSEO, CatPWA} {
		if c == "" {
			t.Errorf("constants")
		}
	}
}

// Aceitação: ParseLighthouseJSON mode default.
func TestParseLighthouseJSONModeDefault(t *testing.T) {
	data := `{"finalUrl":"x","categories":{"performance":{"id":"p","title":"P","score":1}},"audits":{}}`
	rep, _ := ParseLighthouseJSON([]byte(data), "x", "")
	if rep.Mode != ModeContainer {
		t.Errorf("default = container, got %v", rep.Mode)
	}
}

// Aceitação: PWA category.
func TestParseLighthouseJSONPWA(t *testing.T) {
	data := `{"finalUrl":"x","categories":{"pwa":{"id":"pwa","title":"PWA","score":0.5}},"audits":{}}`
	rep, _ := ParseLighthouseJSON([]byte(data), "x", "container")
	if rep.Pillar.PWA != 0.5 {
		t.Errorf("pwa = %f", rep.Pillar.PWA)
	}
}
