// Package evidence — manifest de evidências por run.
//
// SAI-023: refs para Git, changes, components, migrations, envs,
// limitações. Serializa em JSON para gravar como artifact e prover
// para a LLM.
//
// Estrutura:
//
//	Evidence {
//	  RunID, GeneratedAt, Schema,
//	  Git        GitEvidence
//	  Changes    ChangesEvidence
//	  Components []ComponentEvidence
//	  Migrations []MigrationEvidence
//	  Env        EnvEvidence
//	  Limits     []string
//	}
package evidence

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/diegoaraujo/solidify/internal/migrations"
)

// SchemaVersion atual do Evidence.
const SchemaVersion = "1"

// Evidence é o manifest agregado de evidências.
type Evidence struct {
	RunID       string              `json:"run_id"`
	GeneratedAt time.Time           `json:"generated_at"`
	Schema      string              `json:"schema"`
	Git         GitEvidence         `json:"git"`
	Changes     ChangesEvidence     `json:"changes"`
	Components  []ComponentEvidence `json:"components"`
	Migrations  []MigrationEvidence `json:"migrations"`
	Env         EnvEvidence         `json:"env"`
	Limits      []string            `json:"limits"`
}

// GitEvidence referencia o estado git do run.
type GitEvidence struct {
	BaseSHA     string `json:"base_sha"`
	HeadSHA     string `json:"head_sha"`
	BaseRef     string `json:"base_ref"`
	HeadRef     string `json:"head_ref"`
	Remote      string `json:"remote"`
	Branch      string `json:"branch"`
	CommitCount int    `json:"commit_count"`
	IsClean     bool   `json:"is_clean"`
}

// ChangesEvidence sumariza o diff de base→head.
type ChangesEvidence struct {
	FilesChanged int            `json:"files_changed"`
	Insertions   int            `json:"insertions"`
	Deletions    int            `json:"deletions"`
	Paths        []string       `json:"paths"`
	ByLanguage   map[string]int `json:"by_language"`
}

// ComponentEvidence descreve um component detectado.
type ComponentEvidence struct {
	Name      string   `json:"name"`
	Framework string   `json:"framework"`
	Language  string   `json:"language"`
	Paths     []string `json:"paths"`
	Score     float64  `json:"score"`
	Class     string   `json:"class"`
}

// MigrationEvidence descreve um arquivo de migration.
type MigrationEvidence struct {
	File       string                     `json:"file"`
	Framework  string                     `json:"framework"`
	Operations []migrations.OperationType `json:"operations"`
	RiskLevel  string                     `json:"risk_level"`
	Findings   []string                   `json:"findings"`
}

// EnvEvidence sumariza env vars detectadas vs documentadas.
type EnvEvidence struct {
	Used       []string `json:"used"`
	Documented []string `json:"documented"`
	Added      []string `json:"added"`
	Removed    []string `json:"removed"`
	Secrets    []string `json:"secrets"`
}

// New cria Evidence vazio com cabeçalhos e timestamps.
func New(runID string) *Evidence {
	return &Evidence{
		RunID:       runID,
		GeneratedAt: time.Now().UTC(),
		Schema:      SchemaVersion,
		Limits:      []string{},
	}
}

// Marshal serializa com json.MarshalIndent.
func (e *Evidence) Marshal() ([]byte, error) {
	if e == nil {
		return nil, fmt.Errorf("evidence: nil")
	}
	return json.MarshalIndent(e, "", "  ")
}

// AddLimit registra uma limitação conhecida (escopo do analyzer, etc).
func (e *Evidence) AddLimit(msg string) {
	e.Limits = append(e.Limits, msg)
}

// AddComponent registra um component.
func (e *Evidence) AddComponent(c ComponentEvidence) {
	if c.Paths == nil {
		c.Paths = []string{}
	}
	e.Components = append(e.Components, c)
}

// AddMigration registra um migration evidence.
func (e *Evidence) AddMigration(m MigrationEvidence) {
	if m.Operations == nil {
		m.Operations = []migrations.OperationType{}
	}
	if m.Findings == nil {
		m.Findings = []string{}
	}
	e.Migrations = append(e.Migrations, m)
}

// SetEnv preenche o bloco env.
func (e *Evidence) SetEnv(used, documented, added, removed, secrets []string) {
	if used == nil {
		used = []string{}
	}
	if documented == nil {
		documented = []string{}
	}
	if added == nil {
		added = []string{}
	}
	if removed == nil {
		removed = []string{}
	}
	if secrets == nil {
		secrets = []string{}
	}
	// Dedupe + sort para reprodutibilidade.
	e.Env.Used = uniqSorted(used)
	e.Env.Documented = uniqSorted(documented)
	e.Env.Added = uniqSorted(added)
	e.Env.Removed = uniqSorted(removed)
	e.Env.Secrets = uniqSorted(secrets)
}

// SetChanges preenche o bloco changes.
func (e *Evidence) SetChanges(filesChanged, insertions, deletions int, paths []string, byLang map[string]int) {
	if paths == nil {
		paths = []string{}
	}
	if byLang == nil {
		byLang = map[string]int{}
	}
	sort.Strings(paths)
	e.Changes.FilesChanged = filesChanged
	e.Changes.Insertions = insertions
	e.Changes.Deletions = deletions
	e.Changes.Paths = paths
	e.Changes.ByLanguage = byLang
}

// SetGit preenche o bloco git.
func (e *Evidence) SetGit(g GitEvidence) {
	e.Git = g
}

// SortedComponents devolve components ordenados por nome.
func (e *Evidence) SortedComponents() []ComponentEvidence {
	out := make([]ComponentEvidence, len(e.Components))
	copy(out, e.Components)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// SortedMigrations devolve migrations ordenados por file.
func (e *Evidence) SortedMigrations() []MigrationEvidence {
	out := make([]MigrationEvidence, len(e.Migrations))
	copy(out, e.Migrations)
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out
}

// uniqSorted devolve slice com itens únicos ordenados.
func uniqSorted(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
