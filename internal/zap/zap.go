// Package zap — OWASP ZAP adapters.
//
// SAI-041/042: ZAP Baseline (passive scan) e ZAP API/active scan.
// Allowlist obrigatória — produção NUNCA pode ser alvo por
// descoberta automática. Só roda em class local/test.
package zap

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// TargetClass identifica contexto do target.
type TargetClass string

const (
	ClassLocal   TargetClass = "local"   // localhost, 127.0.0.1
	ClassTest    TargetClass = "test"    // *.test.example.com, staging
	ClassStaging TargetClass = "staging" // *.staging.example.com
	ClassProd    TargetClass = "prod"    // *.example.com (BLOQUEADO por default)
	ClassUnknown TargetClass = "unknown"
)

// ClassifyTarget classifica host por heurística.
func ClassifyTarget(rawURL string) (TargetClass, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ClassUnknown, fmt.Errorf("zap: parse url: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ClassUnknown, fmt.Errorf("zap: host vazio")
	}

	// local (.local/.localhost vence tudo)
	if strings.HasSuffix(host, ".local") || strings.Contains(host, "localhost") {
		return ClassLocal, nil
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return ClassLocal, nil
	}
	if strings.HasPrefix(host, "127.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "10.") {
		return ClassLocal, nil
	}
	// test (DOMINIO .test vence .example)
	if strings.HasSuffix(host, ".test.local") {
		return ClassTest, nil
	}
	if strings.HasSuffix(host, ".test") {
		return ClassTest, nil
	}
	if strings.HasPrefix(host, "test.") || strings.HasPrefix(host, "test") || strings.Contains(host, ".test.") {
		return ClassTest, nil
	}
	// staging
	if strings.HasSuffix(host, ".staging.local") {
		return ClassStaging, nil
	}
	if strings.Contains(host, "staging") || strings.HasSuffix(host, ".stg") {
		return ClassStaging, nil
	}
	// default: prod (assumir prod se nada bate — conservative)
	return ClassProd, nil
}

// AllowlistEntry — host permitido.
type AllowlistEntry struct {
	Host  string      `json:"host"`
	Class TargetClass `json:"class"`
}

// Allowlist gerencia targets permitidos.
type Allowlist struct {
	entries []AllowlistEntry
}

// NewAllowlist cria allowlist a partir de entries.
func NewAllowlist(entries []AllowlistEntry) *Allowlist {
	return &Allowlist{entries: entries}
}

// DefaultAllowlist — só local/test permitidos.
func DefaultAllowlist() *Allowlist {
	return &Allowlist{
		entries: []AllowlistEntry{
			{Host: "localhost", Class: ClassLocal},
			{Host: "127.0.0.1", Class: ClassLocal},
			{Host: "::1", Class: ClassLocal},
			{Host: "*.test", Class: ClassTest},
			{Host: "*.test.local", Class: ClassTest},
			{Host: "*.staging.local", Class: ClassStaging},
		},
	}
}

// Allows devolve true se host + class estão permitidos.
// PROD só passa se a entry explicitamente tiver class=ClassProd.
func (a *Allowlist) Allows(host string, class TargetClass) bool {
	host = strings.ToLower(host)
	for _, e := range a.entries {
		if matchHost(host, e.Host) {
			if e.Class == class {
				return true
			}
			// host pattern matches mas classe não — OK se for upgrade (local → test)
			if classUpgrade(e.Class, class) {
				return true
			}
		}
	}
	return false
}

func classUpgrade(want, got TargetClass) bool {
	ranks := map[TargetClass]int{
		ClassLocal: 0, ClassTest: 1, ClassStaging: 2,
	}
	return ranks[got] >= ranks[want]
}

// matchHost match simples com wildcards.
func matchHost(host, pattern string) bool {
	pattern = strings.ToLower(pattern)
	if pattern == host {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".test"
		return strings.HasSuffix(host, suffix)
	}
	return false
}

// Alert severity ZAP.
type Alert struct {
	Name       string `json:"name"`
	Risk       string `json:"risk"`       // informational|low|medium|high
	Confidence string `json:"confidence"` // low|medium|high|confirmed
	URL        string `json:"url"`
	Method     string `json:"method,omitempty"`
	Param      string `json:"param,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
	CWE        string `json:"cwe,omitempty"`
	WASCID     string `json:"wascid,omitempty"`
	Solution   string `json:"solution,omitempty"`
	Desc       string `json:"desc,omitempty"`
	AlertRef   string `json:"alert_ref,omitempty"`
}

// Severity classifica Alert.
type Severity string

const (
	SevHigh   Severity = "high"
	SevMedium Severity = "medium"
	SevLow    Severity = "low"
	SevInfo   Severity = "info"
)

// NormalizeAlert converte Alert → Finding normalizado.
type Finding struct {
	Name       string   `json:"name"`
	Severity   Severity `json:"severity"`
	Risk       string   `json:"risk"`
	Confidence string   `json:"confidence"`
	URL        string   `json:"url"`
	Method     string   `json:"method"`
	Param      string   `json:"param"`
	CWE        string   `json:"cwe"`
	Solution   string   `json:"solution"`
	Source     string   `json:"source"` // "zap-baseline" | "zap-api"
}

func NormalizeAlert(a Alert, source string) Finding {
	return Finding{
		Name:       a.Name,
		Severity:   riskToSeverity(a.Risk),
		Risk:       a.Risk,
		Confidence: a.Confidence,
		URL:        a.URL,
		Method:     a.Method,
		Param:      a.Param,
		CWE:        a.CWE,
		Solution:   a.Solution,
		Source:     source,
	}
}

func riskToSeverity(risk string) Severity {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "high":
		return SevHigh
	case "medium":
		return SevMedium
	case "low":
		return SevLow
	case "informational":
		return SevInfo
	}
	return SevInfo
}

// Report resultado normalizado.
type Report struct {
	Mode     string      `json:"mode"` // "baseline" | "api"
	Target   string      `json:"target"`
	Class    TargetClass `json:"class"`
	Findings []Finding   `json:"findings"`
	Counts   Counts      `json:"counts"`
}

// Counts agregado.
type Counts struct {
	Total      int              `json:"total"`
	BySeverity map[Severity]int `json:"by_severity"`
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

// HasHigh devolve true se tem high.
func (r *Report) HasHigh() bool {
	return r.Counts.BySeverity[SevHigh] > 0
}

// ZAPReportJSON schema subset.
type zapReport struct {
	Site []struct {
		Alerts []Alert `json:"alerts"`
	} `json:"site"`
}

// ParseZAPReportJSON parseia output ZAP report JSON.
func ParseZAPReportJSON(data []byte, mode, target string, class TargetClass) (*Report, error) {
	var raw zapReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("zap: parse: %w", err)
	}
	rep := &Report{Mode: mode, Target: target, Class: class}
	for _, s := range raw.Site {
		for _, a := range s.Alerts {
			rep.AddFinding(NormalizeAlert(a, "zap-"+mode))
		}
	}
	return rep, nil
}

// TargetGuard verifica se target é permitido antes de scan.
type TargetGuard struct {
	Allowlist *Allowlist
	AllowProd bool // override (NÃO usar em produção normal)
}

// Validate checa class + allowlist. AllowProd bypassa PROD default-bloco,
// mas entry com class=ClassProd na allowlist ainda é exigida.
func (g *TargetGuard) Validate(rawURL string) error {
	class, err := ClassifyTarget(rawURL)
	if err != nil {
		return err
	}
	if class == ClassProd && !g.AllowProd {
		return fmt.Errorf("zap: target é PROD (bloqueado por segurança): %s", rawURL)
	}
	if !g.Allowlist.Allows(strings.ToLower(urlHost(rawURL)), class) {
		return fmt.Errorf("zap: target %q (class=%s) não está na allowlist", rawURL, class)
	}
	return nil
}

// urlHost helper.
func urlHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
