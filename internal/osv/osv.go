// Package osv — OSV-Scanner adapter.
//
// SAI-038: detecta lockfiles/SBOM, chama scanner e normaliza
// vulnerabilidades (CVE/GHSA ecosystem, severity CVSS).
package osv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Severity classificação interna.
type Severity string

const (
	SevCritical Severity = "critical" // CVSS >= 9.0
	SevHigh     Severity = "high"     // CVSS >= 7.0
	SevMedium   Severity = "medium"   // CVSS >= 4.0
	SevLow      Severity = "low"      // CVSS > 0
	SevInfo     Severity = "info"
)

// Vuln vulnerabilidade normalizada.
type Vuln struct {
	ID        string   `json:"id"`      // CVE-2024-XXXX ou GHSA-xxxx
	Aliases   []string `json:"aliases"` // GHSA-xxx, etc
	Package   string   `json:"package"`
	Version   string   `json:"version"`
	Ecosystem string   `json:"ecosystem"` // npm, pypi, go, maven, etc
	Severity  Severity `json:"severity"`
	CVSS      float64  `json:"cvss"`
	Summary   string   `json:"summary"`
	FixedIn   string   `json:"fixed_in,omitempty"`
	IsFixable bool     `json:"is_fixable"`
	Source    string   `json:"source"` // lockfile path
}

// Report resultado normalizado.
type Report struct {
	Tool      string   `json:"tool"`
	Vulns     []Vuln   `json:"vulns"`
	Counts    Counts   `json:"counts"`
	Lockfiles []string `json:"lockfiles"`
}

// Counts agregado.
type Counts struct {
	Total      int              `json:"total"`
	BySeverity map[Severity]int `json:"by_severity"`
	Fixable    int              `json:"fixable"`
}

// HasCriticalOrHigh devolve true se tem critical/high.
func (r *Report) HasCriticalOrHigh() bool {
	return r.Counts.BySeverity[SevCritical] > 0 || r.Counts.BySeverity[SevHigh] > 0
}

// AddVuln adiciona + atualiza counts.
func (r *Report) AddVuln(v Vuln) {
	r.Vulns = append(r.Vulns, v)
	if r.Counts.BySeverity == nil {
		r.Counts.BySeverity = make(map[Severity]int)
	}
	r.Counts.BySeverity[v.Severity]++
	r.Counts.Total++
	if v.IsFixable {
		r.Counts.Fixable++
	}
}

// SortVulns ordena por severity (critical primeiro), package, id.
func (r *Report) SortVulns() {
	sort.SliceStable(r.Vulns, func(i, j int) bool {
		if r.Vulns[i].Severity != r.Vulns[j].Severity {
			return severityRank(r.Vulns[i].Severity) < severityRank(r.Vulns[j].Severity)
		}
		if r.Vulns[i].Package != r.Vulns[j].Package {
			return r.Vulns[i].Package < r.Vulns[j].Package
		}
		return r.Vulns[i].ID < r.Vulns[j].ID
	})
}

func severityRank(s Severity) int {
	switch s {
	case SevCritical:
		return 0
	case SevHigh:
		return 1
	case SevMedium:
		return 2
	case SevLow:
		return 3
	default:
		return 4
	}
}

// CVSStoSeverity converte score CVSS pra Severity.
func CVSStoSeverity(cvss float64) Severity {
	switch {
	case cvss >= 9.0:
		return SevCritical
	case cvss >= 7.0:
		return SevHigh
	case cvss >= 4.0:
		return SevMedium
	case cvss > 0:
		return SevLow
	}
	return SevInfo
}

// DetectLockfiles varre dir recursivamente (1 nível) por lockfiles conhecidos.
func DetectLockfiles(root string) ([]string, error) {
	if root == "" {
		return nil, fmt.Errorf("osv: root vazio")
	}
	fi, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("osv: stat: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("osv: %s não é diretório", root)
	}

	known := []string{
		"package-lock.json",  // npm
		"yarn.lock",          // yarn
		"pnpm-lock.yaml",     // pnpm
		"Gemfile.lock",       // ruby
		"composer.lock",      // php
		"Cargo.lock",         // rust
		"go.sum",             // go
		"poetry.lock",        // python poetry
		"Pipfile.lock",       // python pipenv
		"requirements.txt",   // python pip
		"pom.xml",            // maven
		"build.gradle",       // gradle
		"build.gradle.kts",   // gradle kotlin
		"packages.lock.json", // .NET
	}

	found := []string{}
	for _, name := range known {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}

	// 1 nível de subdiretório pra monorepos (packages/*/lock).
	entries, err := os.ReadDir(root)
	if err != nil {
		return found, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(root, e.Name())
		for _, name := range known {
			p := filepath.Join(sub, name)
			if _, err := os.Stat(p); err == nil {
				found = append(found, p)
			}
		}
	}
	return found, nil
}

// osvVuln — schema interno OSV (subset relevante).
type osvVuln struct {
	ID               string   `json:"id"`
	Aliases          []string `json:"aliases"`
	Summary          string   `json:"summary"`
	Details          string   `json:"details"`
	Modified         string   `json:"modified"`
	Published        string   `json:"published"`
	DatabaseSpecific struct {
		// OSV-Scanner extension: severity list.
		Severity []struct {
			Type  string `json:"type"`
			Score string `json:"score"`
		} `json:"severity"`
	} `json:"database_specific"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
}

// osvPackage — package afetado pelo vuln.
type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
	Version   string `json:"version"`
}

// osvAffected — afetado (schema OSV completo).
type osvAffected struct {
	Package osvPackage `json:"package"`
	Ranges  []struct {
		Type   string `json:"type"`
		Events []struct {
			Introduced string `json:"introduced,omitempty"`
			Fixed      string `json:"fixed,omitempty"`
		} `json:"events"`
	} `json:"ranges"`
}

// osvResult — uma linha/result do output do osv-scanner.
type osvResult struct {
	Source struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"source"`
	Package osvPackage   `json:"package"`
	Vulns   []osvVulnRef `json:"vulns"`
}

// osvVulnRef — referência leve no result.
type osvVulnRef struct {
	ID       string   `json:"id"`
	Aliases  []string `json:"aliases"`
	Summary  string   `json:"summary"`
	Details  string   `json:"details"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	Affected []osvAffected `json:"affected"`
}

// ParseOSVScannerJSON parseia output do osv-scanner (formato: {results: [...]})
func ParseOSVScannerJSON(data []byte) (*Report, error) {
	var wrapper struct {
		Results []osvResult `json:"results"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("osv: parse: %w", err)
	}
	rep := &Report{Tool: "osv-scanner"}
	for _, r := range wrapper.Results {
		if r.Source.Path != "" {
			rep.Lockfiles = append(rep.Lockfiles, r.Source.Path)
		}
		for _, v := range r.Vulns {
			vuln := normalizeVuln(v, r.Package, r.Source.Path)
			rep.AddVuln(vuln)
		}
	}
	return rep, nil
}

func normalizeVuln(v osvVulnRef, pkg osvPackage, source string) Vuln {
	cvss := extractCVSS(v.Severity)
	fixable := false
	fixedIn := ""
	for _, a := range v.Affected {
		if a.Package.Name == pkg.Name && a.Package.Ecosystem == pkg.Ecosystem {
			for _, r := range a.Ranges {
				for _, ev := range r.Events {
					if ev.Fixed != "" {
						fixable = true
						fixedIn = ev.Fixed
					}
				}
			}
		}
	}
	return Vuln{
		ID:        v.ID,
		Aliases:   v.Aliases,
		Package:   pkg.Name,
		Version:   pkg.Version,
		Ecosystem: pkg.Ecosystem,
		Severity:  CVSStoSeverity(cvss),
		CVSS:      cvss,
		Summary:   v.Summary,
		FixedIn:   fixedIn,
		IsFixable: fixable,
		Source:    source,
	}
}

// extractCVSS extrai score CVSS numérico do array de severities.
func extractCVSS(severities []struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}) float64 {
	for _, s := range severities {
		t := strings.ToLower(s.Type)
		if strings.Contains(t, "cvss") || strings.Contains(t, "ubuntu") {
			if v, err := parseCVSSString(s.Score); err == nil {
				return v
			}
		}
	}
	return 0
}

// parseCVSSString parseia "CVSS:3.1/AV:N/AC:L/...:9.8" ou "9.8".
func parseCVSSString(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// se contém ":" extrai último token numérico.
	if idx := strings.LastIndex(s, ":"); idx >= 0 && idx < len(s)-1 {
		last := s[idx+1:]
		// pode ter "/" depois (vector CVSS).
		if slash := strings.Index(last, "/"); slash >= 0 {
			last = last[:slash]
		}
		return parseFloat(last)
	}
	return parseFloat(s)
}

// parseFloat minimalista.
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("vazio")
	}
	var sign float64 = 1
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = s[1:]
	}
	var whole, frac float64
	fracDiv := 1.0
	inFrac := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			d := float64(r - '0')
			if inFrac {
				frac = frac*10 + d
				fracDiv *= 10
			} else {
				whole = whole*10 + d
			}
		case r == '.':
			inFrac = true
		default:
			return 0, fmt.Errorf("invalid char %q", r)
		}
	}
	return sign * (whole + frac/fracDiv), nil
}

// DetectEcosystemByLockfile devolve ecosystem inferido pelo filename.
func DetectEcosystemByLockfile(path string) string {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(name, "package-lock") || strings.Contains(name, "yarn.lock") || strings.Contains(name, "pnpm-lock"):
		return "npm"
	case strings.Contains(name, "go.sum"):
		return "Go"
	case strings.Contains(name, "cargo.lock"):
		return "crates.io"
	case strings.Contains(name, "gemfile.lock"):
		return "RubyGems"
	case strings.Contains(name, "composer.lock"):
		return "Packagist"
	case strings.Contains(name, "poetry.lock") || strings.Contains(name, "pipfile.lock") || strings.Contains(name, "requirements.txt"):
		return "PyPI"
	case strings.Contains(name, "pom.xml") || strings.Contains(name, "build.gradle"):
		return "Maven"
	case strings.Contains(name, "packages.lock.json"):
		return "NuGet"
	}
	return "unknown"
}
