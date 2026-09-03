// Package sonar — discovery de config SonarQube/SonarCloud.
//
// SAI-033: detecta sonar-project.properties, .solidify/sonar.json, e
// env refs. Resolve precedence: override Solidify > sonar-project > env.
package sonar

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config — configuração Sonar resolvida.
type Config struct {
	HostURL    string            `json:"host_url"`
	Token      string            `json:"token,omitempty"`     // valor raw se vier de env resolvido
	TokenRef   string            `json:"token_ref,omitempty"` // ex: "env:SONAR_TOKEN"
	Login      string            `json:"login,omitempty"`
	ProjectKey string            `json:"project_key"`
	Org        string            `json:"organization,omitempty"`
	Sources    []string          `json:"sources,omitempty"`
	Exclusions []string          `json:"exclusions,omitempty"`
	Inclusions []string          `json:"inclusions,omitempty"`
	CovPaths   []string          `json:"coverage_report_paths,omitempty"`
	TestPaths  []string          `json:"test_report_paths,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
	Source     string            `json:"source"` // "override" | "properties" | "env"
}

// EnvRef referencia env var (não vaza valor).
type EnvRef struct {
	Key string // ex: "sonar.token"
	Env string // ex: "SONAR_TOKEN"
}

// Discoverer descobre config.
type Discoverer struct {
	Root string
}

// New cria discoverer.
func New(root string) *Discoverer {
	return &Discoverer{Root: root}
}

// Discover devolve config mergeada + bool (true se config achada).
func (d *Discoverer) Discover() (*Config, bool, error) {
	if d.Root == "" {
		return nil, false, fmt.Errorf("sonar: root vazio")
	}
	propsPath := filepath.Join(d.Root, "sonar-project.properties")
	overridePath := filepath.Join(d.Root, ".solidify", "sonar.json")

	var cfg Config
	var found bool

	// 1. Override Solidify (highest precedence).
	if data, err := os.ReadFile(overridePath); err == nil {
		var ov Override
		if err := json.Unmarshal(data, &ov); err != nil {
			return nil, false, fmt.Errorf("sonar: override parse: %w", err)
		}
		cfg = applyOverride(cfg, ov)
		cfg.Source = "override"
		found = true
	}

	// 2. sonar-project.properties.
	if data, err := os.ReadFile(propsPath); err == nil {
		props, err := parseProperties(string(data))
		if err != nil {
			return nil, found, fmt.Errorf("sonar: properties parse: %w", err)
		}
		cfg = applyProperties(cfg, props)
		if !found {
			cfg.Source = "properties"
		}
		found = true
	}

	// 3. Env vars (lowest).
	if cfg.HostURL == "" {
		if v := os.Getenv("SONAR_HOST_URL"); v != "" {
			cfg.HostURL = v
			if !found {
				cfg.Source = "env"
			}
			found = true
		}
	}
	if cfg.Token == "" && cfg.TokenRef == "" {
		if v := os.Getenv("SONAR_TOKEN"); v != "" {
			cfg.Token = v
			cfg.TokenRef = "env:SONAR_TOKEN"
		}
	}

	return &cfg, found, nil
}

// EnvRefs devolve lista de env vars referenciadas.
func (d *Discoverer) EnvRefs(cfg *Config) []EnvRef {
	var refs []EnvRef
	if cfg.TokenRef != "" {
		refs = append(refs, EnvRef{Key: "sonar.token", Env: strings.TrimPrefix(cfg.TokenRef, "env:")})
	}
	if cfg.Extra != nil {
		for k, v := range cfg.Extra {
			if strings.HasPrefix(v, "env:") {
				refs = append(refs, EnvRef{Key: k, Env: strings.TrimPrefix(v, "env:")})
			}
		}
	}
	return refs
}

// ResolveEnv preenche Token se vier de env. NÃO chamar antes de validar
// permissões (env var pode conter secret real).
func (d *Discoverer) ResolveEnv(cfg *Config) {
	if cfg.Token == "" && cfg.TokenRef != "" {
		envName := strings.TrimPrefix(cfg.TokenRef, "env:")
		cfg.Token = os.Getenv(envName)
	}
	if cfg.Extra != nil {
		for k, v := range cfg.Extra {
			if strings.HasPrefix(v, "env:") {
				envName := strings.TrimPrefix(v, "env:")
				cfg.Extra[k] = os.Getenv(envName)
			}
		}
	}
}

// Override carregado de .solidify/sonar.json.
type Override struct {
	HostURL    string            `json:"host_url"`
	TokenRef   string            `json:"token_ref"`
	Login      string            `json:"login"`
	ProjectKey string            `json:"project_key"`
	Org        string            `json:"organization"`
	Sources    []string          `json:"sources"`
	Exclusions []string          `json:"exclusions"`
	Inclusions []string          `json:"inclusions"`
	CovPaths   []string          `json:"coverage_report_paths"`
	TestPaths  []string          `json:"test_report_paths"`
	Extra      map[string]string `json:"extra"`
}

func applyOverride(cfg Config, ov Override) Config {
	if ov.HostURL != "" {
		cfg.HostURL = ov.HostURL
	}
	if ov.TokenRef != "" {
		cfg.TokenRef = ov.TokenRef
	}
	if ov.Login != "" {
		cfg.Login = ov.Login
	}
	if ov.ProjectKey != "" {
		cfg.ProjectKey = ov.ProjectKey
	}
	if ov.Org != "" {
		cfg.Org = ov.Org
	}
	if len(ov.Sources) > 0 {
		cfg.Sources = ov.Sources
	}
	if len(ov.Exclusions) > 0 {
		cfg.Exclusions = ov.Exclusions
	}
	if len(ov.Inclusions) > 0 {
		cfg.Inclusions = ov.Inclusions
	}
	if len(ov.CovPaths) > 0 {
		cfg.CovPaths = ov.CovPaths
	}
	if len(ov.TestPaths) > 0 {
		cfg.TestPaths = ov.TestPaths
	}
	if len(ov.Extra) > 0 {
		if cfg.Extra == nil {
			cfg.Extra = map[string]string{}
		}
		for k, v := range ov.Extra {
			cfg.Extra[k] = v
		}
	}
	return cfg
}

// parseProperties parseia formato Java properties.
func parseProperties(content string) (map[string]string, error) {
	out := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		eq := strings.IndexAny(line, "=:")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		out[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// applyProperties preenche cfg a partir de properties Sonar.
func applyProperties(cfg Config, props map[string]string) Config {
	if v, ok := props["sonar.host.url"]; ok && cfg.HostURL == "" {
		cfg.HostURL = v
	}
	if v, ok := props["sonar.token"]; ok && cfg.Token == "" && cfg.TokenRef == "" {
		if strings.HasPrefix(v, "env:") {
			cfg.TokenRef = v
		} else {
			cfg.Token = v
		}
	}
	if v, ok := props["sonar.login"]; ok && cfg.Login == "" {
		cfg.Login = v
	}
	if v, ok := props["sonar.projectKey"]; ok && cfg.ProjectKey == "" {
		cfg.ProjectKey = v
	}
	if v, ok := props["sonar.organization"]; ok && cfg.Org == "" {
		cfg.Org = v
	}
	if v, ok := props["sonar.sources"]; ok && len(cfg.Sources) == 0 {
		cfg.Sources = splitComma(v)
	}
	if v, ok := props["sonar.exclusions"]; ok && len(cfg.Exclusions) == 0 {
		cfg.Exclusions = splitComma(v)
	}
	if v, ok := props["sonar.inclusions"]; ok && len(cfg.Inclusions) == 0 {
		cfg.Inclusions = splitComma(v)
	}
	if v, ok := props["sonar.coverageReportPaths"]; ok && len(cfg.CovPaths) == 0 {
		cfg.CovPaths = splitComma(v)
	}
	if v, ok := props["sonar.test.reportPaths"]; ok && len(cfg.TestPaths) == 0 {
		cfg.TestPaths = splitComma(v)
	}
	// Extras: tudo que começa com "sonar." e não foi consumido.
	consumed := map[string]bool{
		"sonar.host.url": true, "sonar.token": true, "sonar.login": true,
		"sonar.projectKey": true, "sonar.organization": true,
		"sonar.sources": true, "sonar.exclusions": true, "sonar.inclusions": true,
		"sonar.coverageReportPaths": true, "sonar.test.reportPaths": true,
	}
	if cfg.Extra == nil {
		cfg.Extra = map[string]string{}
	}
	for k, v := range props {
		if !strings.HasPrefix(k, "sonar.") {
			continue
		}
		if consumed[k] {
			continue
		}
		if _, ok := cfg.Extra[k]; !ok {
			cfg.Extra[k] = v
		}
	}
	return cfg
}

// splitComma divide por vírgula e trim.
func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// HasMinimumConfig devolve true se tem host + token (ref) + projectKey.
func (c *Config) HasMinimumConfig() bool {
	return c.HostURL != "" && (c.Token != "" || c.TokenRef != "") && c.ProjectKey != ""
}

// IsCloud devolve true se for SonarCloud (host contém sonarcloud.io).
func (c *Config) IsCloud() bool {
	return strings.Contains(c.HostURL, "sonarcloud.io")
}
