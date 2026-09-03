// Package migrations detecta migrations em paths alterados,
// identificando o framework/ORM usado e extraindo metadados básicos.
//
// Frameworks reconhecidos (ARCHITECTURE.md §10.2):
//
//	Flyway, Liquibase, EF Core, Prisma, TypeORM, Sequelize,
//	Knex, Django, Rails, Laravel + generic SQL em diretórios
//	configuráveis.
//
// A detecção opera em duas camadas:
//  1. Pattern match exato por framework (path glob/regex).
//  2. Fallback para generic SQL se o path estiver num diretório
//     de migrations configurado.
//
// Quando o path casa múltiplos frameworks, o mais específico vence.
package migrations

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Framework identifica o motor de migrations.
type Framework string

const (
	Flyway     Framework = "flyway"
	Liquibase  Framework = "liquibase"
	EFCore     Framework = "efcore"
	Prisma     Framework = "prisma"
	TypeORM    Framework = "typeorm"
	Sequelize  Framework = "sequelize"
	Knex       Framework = "knex"
	Django     Framework = "django"
	Rails      Framework = "rails"
	Laravel    Framework = "laravel"
	GenericSQL Framework = "generic-sql"
	Unknown    Framework = "unknown"
)

// ChangeStatus descreve o status da migration no range.
type ChangeStatus string

const (
	Added    ChangeStatus = "added"
	Modified ChangeStatus = "modified"
	Deleted  ChangeStatus = "deleted"
	Renamed  ChangeStatus = "renamed"
)

// Migration é o objeto normalizado retornado pelo detector.
type Migration struct {
	Framework   Framework    `json:"framework"`
	Path        string       `json:"path"`
	Version     string       `json:"version,omitempty"`
	Description string       `json:"description,omitempty"`
	Status      ChangeStatus `json:"status"`
	HasRollback bool         `json:"has_rollback"` // down/rollback inferido por path vizinho
	RollbackOf  string       `json:"rollback_of,omitempty"`
}

// DetectedMigration é o resultado de Detect com path original e status.
type DetectedMigration struct {
	Path     string
	Status   ChangeStatus
	Rollback string // path da migration reversa (down/rollback) quando achada
}

// frameworkRule é uma regra de detecção por framework.
type frameworkRule struct {
	framework Framework
	// match devolve (version, description, matched) dado o path do repo.
	match func(path string) (version, description string, ok bool)
	// priority: maior = mais específico (vence empate).
	priority int
}

// rules em ordem de prioridade (maior primeiro). Cada rule tem match()
// próprio — path glob, regex, ou combinação.
var rules []frameworkRule

func init() {
	rules = []frameworkRule{
		// Flyway: V<version>__<desc>.sql, R__<desc>.sql, U<version>.sql
		{framework: Flyway, priority: 90,
			match: flywayMatch},
		// Liquibase: changelog XML/YAML/JSON/SQL com prefixos comuns.
		{framework: Liquibase, priority: 80,
			match: liquibaseMatch},
		// EF Core: <Name>Migration.cs em Migrations/.
		{framework: EFCore, priority: 85,
			match: efcoreMatch},
		// Prisma: prisma/migrations/<timestamp>_<name>/migration.sql
		{framework: Prisma, priority: 85,
			match: prismaMatch},
		// TypeORM: src/migrations/<Name>.ts ou migrations/<Name>.ts
		{framework: TypeORM, priority: 70,
			match: typeormMatch},
		// Sequelize: migrations/<timestamp>-<name>.js
		{framework: Sequelize, priority: 70,
			match: sequelizeMatch},
		// Knex: knex migrations: <timestamp>_<name>.js ou <name>.js
		{framework: Knex, priority: 70,
			match: knexMatch},
		// Django: */migrations/<NNNN>_<name>.py
		{framework: Django, priority: 85,
			match: djangoMatch},
		// Rails: db/migrate/<timestamp>_<name>.rb
		{framework: Rails, priority: 85,
			match: railsMatch},
		// Laravel: database/migrations/<timestamp>_<name>.php
		{framework: Laravel, priority: 85,
			match: laravelMatch},
	}
}

// flywayMatch: V<version>__<desc>.sql, R__<desc>.sql, U<version>.sql.
func flywayMatch(p string) (string, string, bool) {
	base := pathBase(p)
	// V<version>__<desc>.sql — version é tudo até "__"
	if m := regexp.MustCompile(`^V(.+?)__(.+)\.sql$`).FindStringSubmatch(base); m != nil {
		return m[1], descNormalize(m[2]), true
	}
	// R__<desc>.sql — repeatable, version = "R"
	if m := regexp.MustCompile(`^R__(.+)\.sql$`).FindStringSubmatch(base); m != nil {
		return "R", descNormalize(m[1]), true
	}
	// U<version>.sql — undo
	if m := regexp.MustCompile(`^U(.+)\.sql$`).FindStringSubmatch(base); m != nil {
		return "U" + m[1], "undo", true
	}
	return "", "", false
}

// liquibaseMatch: changelog/dbchangelog com extensões xml/yaml/yml/json/sql.
func liquibaseMatch(p string) (string, string, bool) {
	base := strings.ToLower(pathBase(p))
	dir := strings.ToLower(filepath.ToSlash(filepath.Dir(p)))
	if !strings.Contains(dir, "liquibase") && !strings.Contains(base, "changelog") &&
		!strings.Contains(base, "databasechangelog") {
		return "", "", false
	}
	switch ext(base) {
	case ".xml", ".yaml", ".yml", ".json", ".sql":
		name := strings.TrimSuffix(base, ext(base))
		return name, name, true
	}
	return "", "", false
}

// efcoreMatch: termina em "Migration.cs" ou ".Designer.cs" dentro de Migrations/.
func efcoreMatch(p string) (string, string, bool) {
	dir := filepath.ToSlash(filepath.Dir(p))
	if !strings.Contains(dir, "/Migrations") && !strings.HasSuffix(dir, "Migrations") {
		return "", "", false
	}
	base := pathBase(p)
	if strings.HasSuffix(base, "Migration.cs") {
		name := strings.TrimSuffix(base, "Migration.cs")
		return name, name, true
	}
	return "", "", false
}

// prismaMatch: prisma/migrations/<timestamp>_<name>/migration.sql
func prismaMatch(p string) (string, string, bool) {
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) < 4 {
		return "", "", false
	}
	if parts[0] != "prisma" || parts[1] != "migrations" {
		return "", "", false
	}
	if pathBase(p) != "migration.sql" {
		return "", "", false
	}
	dir := parts[2]
	// dir = "<timestamp>_<name>"
	idx := strings.IndexByte(dir, '_')
	if idx <= 0 {
		return dir, dir, true
	}
	return dir[:idx], descNormalize(dir[idx+1:]), true
}

// typeormMatch: "<Name>.ts" em migrations/ (em qualquer profundidade).
func typeormMatch(p string) (string, string, bool) {
	if ext(pathBase(p)) != ".ts" {
		return "", "", false
	}
	dir := filepath.ToSlash(filepath.Dir(p))
	parts := strings.Split(dir, "/")
	for _, part := range parts {
		if part == "migrations" {
			base := strings.TrimSuffix(pathBase(p), ".ts")
			return base, base, true
		}
	}
	return "", "", false
}

// sequelizeMatch: migrations/<timestamp>-<name>.js|.ts.
func sequelizeMatch(p string) (string, string, bool) {
	dir := filepath.ToSlash(filepath.Dir(p))
	if firstSeg(dir) != "migrations" {
		return "", "", false
	}
	e := ext(pathBase(p))
	if e != ".js" && e != ".ts" {
		return "", "", false
	}
	base := strings.TrimSuffix(pathBase(p), e)
	// timestamp-name
	parts := strings.SplitN(base, "-", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], descNormalize(parts[1]), true
}

// knexMatch: migrations/<timestamp>_<name>.js (sem "-" no knex).
func knexMatch(p string) (string, string, bool) {
	dir := filepath.ToSlash(filepath.Dir(p))
	if firstSeg(dir) != "migrations" {
		return "", "", false
	}
	e := ext(pathBase(p))
	if e != ".js" && e != ".ts" {
		return "", "", false
	}
	base := strings.TrimSuffix(pathBase(p), e)
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return "", "", false
	}
	return base[:idx], descNormalize(base[idx+1:]), true
}

// djangoMatch: */migrations/<NNNN>_<name>.py.
func djangoMatch(p string) (string, string, bool) {
	e := ext(pathBase(p))
	if e != ".py" {
		return "", "", false
	}
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) < 3 {
		return "", "", false
	}
	if parts[len(parts)-2] != "migrations" {
		return "", "", false
	}
	base := strings.TrimSuffix(pathBase(p), ".py")
	idx := strings.IndexByte(base, '_')
	if idx < 4 { // NNNN_
		return "", "", false
	}
	return base[:idx], descNormalize(base[idx+1:]), true
}

// railsMatch: db/migrate/<timestamp>_<name>.rb.
func railsMatch(p string) (string, string, bool) {
	e := ext(pathBase(p))
	if e != ".rb" {
		return "", "", false
	}
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) < 3 {
		return "", "", false
	}
	if parts[len(parts)-2] != "migrate" {
		return "", "", false
	}
	if parts[len(parts)-3] != "db" {
		return "", "", false
	}
	base := strings.TrimSuffix(pathBase(p), ".rb")
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return "", "", false
	}
	return base[:idx], descNormalize(base[idx+1:]), true
}

// laravelMatch: database/migrations/<timestamp>_<name>.php.
func laravelMatch(p string) (string, string, bool) {
	e := ext(pathBase(p))
	if e != ".php" {
		return "", "", false
	}
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) < 3 {
		return "", "", false
	}
	if parts[len(parts)-2] != "migrations" {
		return "", "", false
	}
	if parts[len(parts)-3] != "database" {
		return "", "", false
	}
	base := strings.TrimSuffix(pathBase(p), ".php")
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return "", "", false
	}
	return base[:idx], descNormalize(base[idx+1:]), true
}

// Detect devolve migrations normalizadas a partir dos paths detectados.
// genericDirs são diretórios de migration SQL genérica (ex.: "migrations",
// "db/migrate", "db/migrations"). Paths em outros lugares mas com .sql são
// ignorados — só conta se estão num generic dir OU casam framework
// específico.
func Detect(dms []DetectedMigration, genericDirs []string) []Migration {
	out := make([]Migration, 0, len(dms))
	for _, dm := range dms {
		m, ok := detectOne(dm.Path, genericDirs)
		if !ok {
			continue
		}
		m.Status = dm.Status
		m.RollbackOf = dm.Rollback
		m.HasRollback = dm.Rollback != ""
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Framework < out[j].Framework
	})
	return out
}

// detectOne tenta todos os frameworks em ordem de prioridade; se nenhum
// casa, tenta generic SQL se path está num genericDir.
func detectOne(path string, genericDirs []string) (Migration, bool) {
	// 1. Regras específicas.
	bestPri := -1
	var best Migration
	var bestOK bool
	for _, r := range rules {
		v, desc, ok := r.match(path)
		if !ok {
			continue
		}
		if r.priority > bestPri {
			bestPri = r.priority
			best = Migration{
				Framework:   r.framework,
				Path:        filepath.ToSlash(path),
				Version:     v,
				Description: desc,
			}
			bestOK = true
		}
	}
	if bestOK {
		return best, true
	}
	// 2. Generic SQL em diretório configurado.
	if ext(pathBase(path)) == ".sql" && inAnyDir(path, genericDirs) {
		return Migration{
			Framework:   GenericSQL,
			Path:        filepath.ToSlash(path),
			Description: strings.TrimSuffix(pathBase(path), ".sql"),
		}, true
	}
	return Migration{}, false
}

// inAnyDir devolve true se path está em algum dos dirs (relativo ao repo).
func inAnyDir(path string, dirs []string) bool {
	p := filepath.ToSlash(path)
	for _, d := range dirs {
		d = filepath.ToSlash(d)
		if d == "" || d == "." {
			// qualquer path sem "/" no primeiro segmento.
			if !strings.Contains(p, "/") {
				return true
			}
			continue
		}
		if strings.HasPrefix(p, d+"/") || p == d {
			return true
		}
	}
	return false
}

// descNormalize troca "_" e "-" por espaço — usado nas descrições de
// migrations para que "add_email" / "add-email" vire "add email".
func descNormalize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	return s
}

func pathBase(p string) string {
	return filepath.Base(filepath.ToSlash(p))
}

func firstSeg(p string) string {
	p = strings.TrimPrefix(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}

func ext(name string) string {
	e := filepath.Ext(name)
	return strings.ToLower(e)
}
