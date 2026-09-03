// Package lighthouse — Lighthouse adapter.
//
// SAI-044/045: importar scores oficiais Lighthouse (perf/a11y/
// best-practices/SEO) e agregar pillar frontend.
//
// Backend-only fixture (frontend.Mode=Disabled ou Mode=Unset)
// NÃO invoca scanner — função ShouldRun devolve false.
package lighthouse

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Mode do scan.
type Mode string

const (
	ModeContainer Mode = "container" // chrome via docker
	ModeLocal     Mode = "local"     // chrome local
	ModeDisabled  Mode = "disabled"  // skip
)

// Category scores (0-1).
type Score float64

// IsZero devolve true se score == 0.
func (s Score) IsZero() bool { return s == 0 }

// ToPercent converte 0-1 em 0-100.
func (s Score) ToPercent() int { return int(s * 100) }

// CategoryKey identifica pilar.
type CategoryKey string

const (
	CatPerformance   CategoryKey = "performance"
	CatAccessibility CategoryKey = "accessibility"
	CatBestPractices CategoryKey = "best-practices"
	CatSEO           CategoryKey = "seo"
	CatPWA           CategoryKey = "pwa"
)

// Pillar pesos para agregação.
type Pillar struct {
	Performance   Score `json:"performance"`
	Accessibility Score `json:"accessibility"`
	BestPractices Score `json:"best-practices"`
	SEO           Score `json:"seo"`
	PWA           Score `json:"pwa"`
}

// DefaultPillarWeights peso default.
type DefaultPillarWeights struct {
	Performance   float64 `json:"performance"`
	Accessibility float64 `json:"accessibility"`
	BestPractices float64 `json:"best-practices"`
	SEO           float64 `json:"seo"` // informativo default
}

// DefaultWeights devolve pesos default (SEO=0 = informativo).
func DefaultWeights() DefaultPillarWeights {
	return DefaultPillarWeights{
		Performance:   0.4,
		Accessibility: 0.3,
		BestPractices: 0.3,
		SEO:           0.0,
	}
}

// AggregatePillar compõe score agregado via pesos (SEO=0 → não conta).
func AggregatePillar(p Pillar, w DefaultPillarWeights) Score {
	total := 0.0
	sumW := 0.0
	if w.Performance > 0 {
		total += float64(p.Performance) * w.Performance
		sumW += w.Performance
	}
	if w.Accessibility > 0 {
		total += float64(p.Accessibility) * w.Accessibility
		sumW += w.Accessibility
	}
	if w.BestPractices > 0 {
		total += float64(p.BestPractices) * w.BestPractices
		sumW += w.BestPractices
	}
	if w.SEO > 0 {
		total += float64(p.SEO) * w.SEO
		sumW += w.SEO
	}
	if sumW == 0 {
		return 0
	}
	return Score(total / sumW)
}

// Audit individual.
type Audit struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Score        Score  `json:"score,omitempty"` // 0-1, omit se null
	DisplayValue string `json:"display_value,omitempty"`
	Details      string `json:"details,omitempty"` // JSON string (sanitizado)
}

// Category audit group.
type Category struct {
	Key    CategoryKey `json:"key"`
	Title  string      `json:"title"`
	Score  Score       `json:"score"`
	Audits []Audit     `json:"audits"`
}

// Report resultado.
type Report struct {
	URL            string     `json:"url"`
	FinalURL       string     `json:"final_url"`
	FetchTime      string     `json:"fetch_time"`
	UserAgent      string     `json:"user_agent"`
	Mode           Mode       `json:"mode"`
	Categories     []Category `json:"categories"`
	Pillar         Pillar     `json:"pillar"`
	AggregateScore Score      `json:"aggregate_score"`
	Error          string     `json:"error,omitempty"`
}

// lhJSON subset.
type lhJSON struct {
	FinalURL   string `json:"finalUrl"`
	FetchTime  string `json:"fetchTime"`
	UserAgent  string `json:"userAgent"`
	Categories map[string]struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Score Score  `json:"score"`
	} `json:"categories"`
	Audits map[string]struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Score        Score  `json:"score"`
		DisplayValue string `json:"displayValue"`
	} `json:"audits"`
}

// ParseLighthouseJSON parseia JSON do Lighthouse report.
func ParseLighthouseJSON(data []byte, url, modeStr string) (*Report, error) {
	var raw lhJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("lighthouse: parse: %w", err)
	}
	if len(raw.Categories) == 0 {
		return nil, errors.New("lighthouse: sem categories")
	}
	mode := Mode(modeStr)
	if mode == "" {
		mode = ModeContainer
	}
	rep := &Report{
		URL:       url,
		FinalURL:  raw.FinalURL,
		FetchTime: raw.FetchTime,
		UserAgent: raw.UserAgent,
		Mode:      mode,
	}
	for key, c := range raw.Categories {
		ck := CategoryKey(key)
		rep.Categories = append(rep.Categories, Category{
			Key:   ck,
			Title: c.Title,
			Score: c.Score,
		})
		switch ck {
		case CatPerformance:
			rep.Pillar.Performance = c.Score
		case CatAccessibility:
			rep.Pillar.Accessibility = c.Score
		case CatBestPractices:
			rep.Pillar.BestPractices = c.Score
		case CatSEO:
			rep.Pillar.SEO = c.Score
		case CatPWA:
			rep.Pillar.PWA = c.Score
		}
	}
	rep.AggregateScore = AggregatePillar(rep.Pillar, DefaultWeights())
	return rep, nil
}

// ShouldRun devolve true se scan deve ser executado.
// Backend-only fixture (Mode=Disabled/Unset) → false.
func ShouldRun(modeStr string) bool {
	m := Mode(modeStr)
	switch m {
	case ModeDisabled, "":
		return false
	}
	return true
}

// Config scanner.
type Config struct {
	Mode     Mode   `json:"mode,omitempty"`
	Browser  string `json:"browser,omitempty"`  // chrome binary path
	Headless bool   `json:"headless,omitempty"` // default true
}

// DefaultConfig devolve defaults seguros.
func DefaultConfig() Config {
	return Config{Mode: ModeContainer, Headless: true}
}

// SanitizeUserAgent remove path/leaks.
func SanitizeUserAgent(ua string) string {
	if ua == "" {
		return ""
	}
	// Remove (http://...) URLs.
	out := ""
	for _, ch := range ua {
		if ch == '(' {
			break
		}
		out += string(ch)
	}
	return out
}

// CategoryScoreByKey helper.
func (r *Report) CategoryScoreByKey(k CategoryKey) Score {
	for _, c := range r.Categories {
		if c.Key == k {
			return c.Score
		}
	}
	return 0
}

// HasPerformance devolve true se category existe.
func (r *Report) HasPerformance() bool {
	return r.CategoryScoreByKey(CatPerformance) > 0 || r.Pillar.Performance > 0
}

// FailedAudits lista audits com score < 0.5 (fail-ish).
func (r *Report) FailedAudits(threshold Score) []string {
	var out []string
	for _, c := range r.Categories {
		for _, a := range c.Audits {
			if a.Score > 0 && a.Score < threshold {
				out = append(out, a.ID)
			}
		}
	}
	return out
}

// IsInformativeOnly (SEO só) devolve true se só categoria informativa existe.
func (r *Report) IsInformativeOnly() bool {
	if r.Pillar.Performance > 0 || r.Pillar.Accessibility > 0 || r.Pillar.BestPractices > 0 {
		return false
	}
	return r.Pillar.SEO > 0
}

// IsPassing devolve true se aggregate >= threshold.
func (r *Report) IsPassing(threshold Score) bool {
	return r.AggregateScore >= threshold
}

// EmptyReport devolve report sentinela para skip.
func EmptyReport(url string, reason string) *Report {
	return &Report{
		URL:   url,
		Mode:  ModeDisabled,
		Error: reason,
	}
}
