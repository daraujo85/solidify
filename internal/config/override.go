package config

import (
	"strconv"
	"strings"

	"github.com/diegoaraujo/solidify/internal/errs"
)

// Overrides são ajustes de config vindos da linha de comando, aplicados
// depois do arquivo e antes da validação.
//
// Só campos operacionais são sobrescrevíveis por flag. Peso de score, política
// de perfil e requisitos de contractual ficam fora de propósito: relaxar
// contractual por flag de linha de comando anularia o "falha fechado".
type Overrides map[string]string

// OverridableKeys lista as chaves aceitas, para mensagem de erro e docs.
func OverridableKeys() []string {
	keys := make([]string, 0, len(setters))
	for key := range setters {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

// setters mapeia chave pontuada para a função que a aplica.
var setters = map[string]func(*Config, string) error{
	"project.name": func(c *Config, v string) error {
		c.Project.Name = v
		return nil
	},
	"project.default_base": func(c *Config, v string) error {
		c.Project.DefaultBase = v
		return nil
	},
	"project.artifact_dir": func(c *Config, v string) error {
		c.Project.ArtifactDir = v
		return nil
	},
	"analysis.default_profile": func(c *Config, v string) error {
		c.Analysis.DefaultProfile = v
		return nil
	},
	"analysis.use_merge_base": boolSetter(func(c *Config, v bool) { c.Analysis.UseMergeBase = v }),
	"analysis.scheduler.max_light_jobs": intSetter(func(c *Config, v int) {
		c.Analysis.Scheduler.MaxLightJobs = v
	}),
	"analysis.scheduler.max_heavy_jobs": intSetter(func(c *Config, v int) {
		c.Analysis.Scheduler.MaxHeavyJobs = v
	}),
	"analyzers.sonar.enabled":      boolSetter(func(c *Config, v bool) { c.Analyzers.Sonar.Enabled = v }),
	"analyzers.security.enabled":   boolSetter(func(c *Config, v bool) { c.Analyzers.Security.Enabled = v }),
	"analyzers.lighthouse.enabled": boolSetter(func(c *Config, v bool) { c.Analyzers.Lighthouse.Enabled = v }),
	"analyzers.load.enabled":       boolSetter(func(c *Config, v bool) { c.Analyzers.Load.Enabled = v }),
	"analyzers.tests.enabled":      boolSetter(func(c *Config, v bool) { c.Analyzers.Tests.Enabled = v }),
	"analyzers.coverage.enabled":   boolSetter(func(c *Config, v bool) { c.Analyzers.Coverage.Enabled = v }),
	"ai.external_provider.base_url_host": func(c *Config, v string) error {
		c.AI.ExternalProvider.BaseURLHost = v
		return nil
	},
	"ai.external_provider.base_url_docker": func(c *Config, v string) error {
		c.AI.ExternalProvider.BaseURLDocker = v
		return nil
	},
	// Aceita apenas o NOME da variável; validateEnvRef rejeita um valor colado aqui.
	"ai.external_provider.api_key_env": func(c *Config, v string) error {
		c.AI.ExternalProvider.APIKeyEnv = v
		return nil
	},
	"ai.external_provider.request_timeout_seconds": intSetter(func(c *Config, v int) {
		c.AI.ExternalProvider.RequestTimeoutSeconds = v
	}),
	"ai.external_provider.model_discovery": boolSetter(func(c *Config, v bool) {
		c.AI.ExternalProvider.ModelDiscovery = v
	}),
	"ai.selection.mode": func(c *Config, v string) error {
		c.AI.Selection.Mode = v
		return nil
	},
	"report.language": func(c *Config, v string) error {
		c.Report.Language = v
		return nil
	},
	"report.pdf.browser_mode": func(c *Config, v string) error {
		c.Report.PDF.BrowserMode = v
		return nil
	},
	"privacy.persist_full_diff": boolSetter(func(c *Config, v bool) { c.Privacy.PersistFullDiff = v }),
	"privacy.persist_model_requests": boolSetter(func(c *Config, v bool) {
		c.Privacy.PersistModelRequests = v
	}),
	"privacy.persist_model_responses": boolSetter(func(c *Config, v bool) {
		c.Privacy.PersistModelResponses = v
	}),
}

// ParseOverride divide "chave=valor" e valida a chave.
func ParseOverride(raw string) (key, value string, err error) {
	idx := strings.Index(raw, "=")
	if idx <= 0 {
		return "", "", errs.Newf(errs.CodeUsage, "override inválido: %s", quote(raw)).
			WithHint("use --set chave=valor")
	}
	key = strings.TrimSpace(raw[:idx])
	value = raw[idx+1:]
	if _, ok := setters[key]; !ok {
		return "", "", errs.Newf(errs.CodeUsage, "chave de override não suportada: %s", quote(key)).
			WithHint("chaves aceitas: " + strings.Join(OverridableKeys(), ", "))
	}
	return key, value, nil
}

// Set registra um override já validado.
func (o Overrides) Set(key, value string) { o[key] = value }

// apply aplica os overrides em ordem determinística de chave.
func (o Overrides) apply(cfg *Config) error {
	keys := make([]string, 0, len(o))
	for key := range o {
		keys = append(keys, key)
	}
	sortStrings(keys)

	for _, key := range keys {
		setter, ok := setters[key]
		if !ok {
			return errs.Newf(errs.CodeUsage, "chave de override não suportada: %s", quote(key)).
				WithHint("chaves aceitas: " + strings.Join(OverridableKeys(), ", "))
		}
		if err := setter(cfg, o[key]); err != nil {
			return err
		}
	}
	return nil
}

func boolSetter(assign func(*Config, bool)) func(*Config, string) error {
	return func(c *Config, raw string) error {
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return errs.Newf(errs.CodeUsage, "valor booleano inválido: %s", quote(raw)).
				WithHint("use true ou false")
		}
		assign(c, parsed)
		return nil
	}
}

func intSetter(assign func(*Config, int)) func(*Config, string) error {
	return func(c *Config, raw string) error {
		parsed, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return errs.Newf(errs.CodeUsage, "valor inteiro inválido: %s", quote(raw))
		}
		assign(c, parsed)
		return nil
	}
}
