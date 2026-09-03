// Package secrets — secret scanner adapter.
//
// SAI-037: prefer Gitleaks; roda no range/repo conforme capacidade;
// parser JSON. Fallback: regex pattern (genérico).
package secrets

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Severity classificação de finding.
type Severity string

const (
	SevHigh   Severity = "high"   // secret confirmado (private key, API key valid)
	SevMedium Severity = "medium" // secret provável (token-like, JWT)
	SevLow    Severity = "low"    // heurística fraca (password=, secret=)
	SevInfo   Severity = "info"
)

// Finding normalizado.
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	FilePath string   `json:"file_path"`
	Line     int      `json:"line"`
	Match    string   `json:"match"`  // texto casado (redacted)
	Commit   string   `json:"commit"` // SHA se aplicável
	Author   string   `json:"author"`
	Secret   string   `json:"secret"` // valor detectado (parcial)
	Entropy  float64  `json:"entropy"`
	Source   string   `json:"source"` // "gitleaks" | "regex" | "trufflehog"
}

// Report resultado.
type Report struct {
	Tool     string    `json:"tool"`
	Findings []Finding `json:"findings"`
	Counts   Counts    `json:"counts"`
}

// Counts agregado.
type Counts struct {
	Total      int              `json:"total"`
	BySeverity map[Severity]int `json:"by_severity"`
}

// HasHighSeverity devolve true se tem ≥1 high.
func (r *Report) HasHighSeverity() bool {
	return r.Counts.BySeverity[SevHigh] > 0
}

// SortFindings ordena por severity (high primeiro), arquivo, linha.
func (r *Report) SortFindings() {
	// insertion sort simples.
	for i := 1; i < len(r.Findings); i++ {
		for j := i; j > 0 && findingRank(r.Findings[j]) < findingRank(r.Findings[j-1]); j-- {
			r.Findings[j], r.Findings[j-1] = r.Findings[j-1], r.Findings[j]
		}
	}
}

func findingRank(f Finding) int {
	switch f.Severity {
	case SevHigh:
		return 0
	case SevMedium:
		return 1
	case SevLow:
		return 2
	default:
		return 3
	}
}

// AddFinding adiciona + atualiza counts.
func (r *Report) AddFinding(f Finding) {
	r.Findings = append(r.Findings, f)
	if r.Counts.BySeverity == nil {
		r.Counts.BySeverity = make(map[Severity]int)
	}
	r.Counts.BySeverity[f.Severity]++
	r.Counts.Total++
}

// ParseGitleaksJSON parseia output do Gitleaks (formato array JSON).
func ParseGitleaksJSON(data []byte) (*Report, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return &Report{Tool: "gitleaks"}, nil
	}
	// Gitleaks v8+ emite JSON array; v7 emitia JSON lines (NDJSON).
	if data[0] == '[' {
		return parseGitleaksArray(data)
	}
	return parseGitleaksNDJSON(data)
}

func parseGitleaksArray(data []byte) (*Report, error) {
	var raw []gitleaksFinding
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("secrets: gitleaks array: %w", err)
	}
	rep := &Report{Tool: "gitleaks"}
	for _, g := range raw {
		rep.AddFinding(g.normalize())
	}
	return rep, nil
}

func parseGitleaksNDJSON(data []byte) (*Report, error) {
	rep := &Report{Tool: "gitleaks"}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var g gitleaksFinding
		if err := json.Unmarshal([]byte(line), &g); err != nil {
			continue // skip linhas inválidas
		}
		rep.AddFinding(g.normalize())
	}
	return rep, nil
}

// gitleaksFinding — schema atual Gitleaks v8+.
type gitleaksFinding struct {
	Description string   `json:"Description"`
	RuleID      string   `json:"RuleID"`
	Match       string   `json:"Match"`
	Secret      string   `json:"Secret"`
	File        string   `json:"File"`
	Line        int      `json:"Line"`
	Commit      string   `json:"Commit"`
	Author      string   `json:"Author"`
	Date        string   `json:"Date"`
	Tags        []string `json:"Tags"`
	Entropy     float64  `json:"Entropy"`
}

func (g gitleaksFinding) normalize() Finding {
	sev := SevMedium // default
	switch {
	case strings.Contains(strings.ToLower(g.Description), "high") ||
		strings.Contains(strings.ToLower(g.RuleID), "aws") ||
		strings.Contains(strings.ToLower(g.RuleID), "private-key"):
		sev = SevHigh
	case strings.Contains(strings.ToLower(g.Description), "low"):
		sev = SevLow
	}
	return Finding{
		Rule:     g.RuleID,
		Severity: sev,
		FilePath: g.File,
		Line:     g.Line,
		Match:    g.Match,
		Secret:   g.Secret,
		Commit:   g.Commit,
		Author:   g.Author,
		Entropy:  g.Entropy,
		Source:   "gitleaks",
	}
}

// RedactSecret devolve string segura pra log.
// Mantém primeiros 4 e últimos 2 chars; resto vira "***".
func RedactSecret(s string) string {
	const max = 64
	if len(s) <= 6 {
		return "***"
	}
	if len(s) > max {
		s = s[:max]
	}
	prefix := s[:4]
	suffix := s[len(s)-2:]
	return prefix + strings.Repeat("*", len(s)-6) + suffix
}

// IsLikelySecret heurística rápida pra fallback regex.
// Retorna true se string tem entropy alta + tamanho típico.
func IsLikelySecret(s string) bool {
	if len(s) < 16 || len(s) > 256 {
		return false
	}
	e := shannonEntropy(s)
	return e >= 4.0
}

// shannonEntropy calcula entropy em bits/char.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	counts := make(map[rune]int)
	for _, r := range s {
		counts[r]++
	}
	var e float64
	total := float64(len(s))
	for _, c := range counts {
		p := float64(c) / total
		if p > 0 {
			e -= p * math.Log2(p)
		}
	}
	return e
}
