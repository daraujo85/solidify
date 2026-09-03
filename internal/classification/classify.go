// Package classification atribui commits a categorias canônicas
// (CONTRACTS.md / ARCHITECTURE.md §9.2) de forma determinística.
//
// Ordem de prioridade:
//  1. breaking-change explícito (Conventional "!" ou footer "BREAKING CHANGE:")
//  2. Conventional Commit type (feat→feature, fix→bugfix, …)
//  3. path hints (migrations/, *.sql, Dockerfile, docs/, *_test.*, etc.)
//  4. unknown
//
// A IA é apenas para ambiguidades (Fase 4 do spec) — não utilizada aqui.
package classification

import (
	"path/filepath"
	"strings"

	"github.com/diegoaraujo/solidify/internal/gitx"
)

// Category é uma categoria canônica de mudança.
type Category string

const (
	CategoryFeature        Category = "feature"
	CategoryBugfix         Category = "bugfix"
	CategoryRefactor       Category = "refactor"
	CategoryPerformance    Category = "performance"
	CategorySecurity       Category = "security"
	CategoryTest           Category = "test"
	CategoryDocs           Category = "docs"
	CategoryBuild          Category = "build"
	CategoryCI             Category = "ci"
	CategoryChore          Category = "chore"
	CategoryMigration      Category = "migration"
	CategoryConfiguration  Category = "configuration"
	CategoryBreakingChange Category = "breaking-change"
	CategoryUnknown        Category = "unknown"
)

// ClassifyCommit devolve a categoria de um commit dado os paths por ele
// tocados. paths pode ser vazio (só Conventional + breaking contam).
func ClassifyCommit(c gitx.Commit, paths []string) Category {
	if c.Conventional.Breaking {
		return CategoryBreakingChange
	}
	if cat := fromConventionalType(c.Conventional.Type); cat != CategoryUnknown {
		return cat
	}
	return fromPathHints(paths)
}

// fromConventionalType mapeia Conventional Type → Category.
// Retorna CategoryUnknown se o type não é canônico.
func fromConventionalType(t string) Category {
	switch t {
	case "feat", "feature":
		return CategoryFeature
	case "fix", "bugfix":
		return CategoryBugfix
	case "refactor":
		return CategoryRefactor
	case "perf", "performance":
		return CategoryPerformance
	case "security":
		return CategorySecurity
	case "test", "tests":
		return CategoryTest
	case "docs", "doc":
		return CategoryDocs
	case "build":
		return CategoryBuild
	case "ci":
		return CategoryCI
	case "chore":
		return CategoryChore
	default:
		return CategoryUnknown
	}
}

// fromPathHints agrega pistas do path. Empate: migration > configuration >
// docs > test > ci (mais específico primeiro).
func fromPathHints(paths []string) Category {
	if len(paths) == 0 {
		return CategoryUnknown
	}
	var (
		migration, config, docs, test, ci int
	)
	for _, p := range paths {
		p = filepath.ToSlash(p)
		switch {
		case isMigrationPath(p):
			migration++
		case isCIConfigPath(p):
			ci++
		case isConfigPath(p):
			config++
		case isTestPath(p):
			test++
		case isDocsPath(p):
			docs++
		}
	}
	switch {
	case migration > 0:
		return CategoryMigration
	case ci > 0:
		return CategoryCI
	case config > 0:
		return CategoryConfiguration
	case test > 0:
		return CategoryTest
	case docs > 0:
		return CategoryDocs
	default:
		return CategoryUnknown
	}
}

// isMigrationPath detecta migrations/ e arquivos SQL fora de testes.
func isMigrationPath(p string) bool {
	if isTestPath(p) {
		return false
	}
	// Primeiro componente migrations/ ou db/migrate/ ou prisma/migrations/
	first := firstSegment(p)
	if first == "migrations" || first == "db" || first == "prisma" || first == "alembic" {
		// db/migrate/X, db/migrations/X, prisma/migrations/X
		if strings.HasPrefix(p, "db/migrate/") || strings.HasPrefix(p, "db/migrations/") ||
			strings.HasPrefix(p, "prisma/migrations/") || strings.HasPrefix(p, "alembic/") ||
			first == "migrations" {
			return true
		}
		// migrations/ no root
		if strings.HasPrefix(p, "migrations/") {
			return true
		}
	}
	// *.sql fora de test/
	if strings.HasSuffix(p, ".sql") {
		return true
	}
	// *.migration.ts / *.migration.js / *.up.sql / *.down.sql
	base := filepath.Base(p)
	if strings.HasSuffix(base, ".migration.ts") || strings.HasSuffix(base, ".migration.js") ||
		strings.HasSuffix(base, ".migration.go") || strings.HasSuffix(base, ".migration.py") {
		return true
	}
	return false
}

// isConfigPath detecta arquivos de configuração: env, json/toml/yaml/ini
// nas raízes, Dockerfile, docker-compose, manifests de dependência.
func isConfigPath(p string) bool {
	if isTestPath(p) {
		return false
	}
	base := filepath.Base(p)
	// Manifests e Dockerfile no root.
	depth := strings.Count(p, "/")
	if depth == 0 {
		switch base {
		case "Dockerfile", "Makefile", "package.json", "go.mod", "go.sum",
			"Cargo.toml", "Cargo.lock", "pyproject.toml", "requirements.txt",
			"Pipfile", "Pipfile.lock", "pom.xml", "build.gradle", "build.gradle.kts",
			"composer.json", "Gemfile", "Gemfile.lock", "pubspec.yaml",
			"mix.exs", "setup.py", "setup.cfg":
			return true
		}
	}
	// Arquivos .env e afins.
	if strings.HasPrefix(base, ".env") {
		return true
	}
	// Configuração YAML/JSON/TOML/INI em qualquer profundidade de pastas
	// "config/" ou "configs/" ou ".config/".
	first := firstSegment(p)
	if first == "config" || first == "configs" || first == ".config" {
		return isConfigFile(base)
	}
	// *.config.ts / *.config.js / *.config.mjs (webpack, vite, etc).
	if strings.HasSuffix(base, ".config.ts") || strings.HasSuffix(base, ".config.js") ||
		strings.HasSuffix(base, ".config.mjs") || strings.HasSuffix(base, ".config.cjs") {
		return true
	}
	return false
}

// isConfigFile detecta extensão de arquivo de configuração.
func isConfigFile(name string) bool {
	switch filepath.Ext(name) {
	case ".yaml", ".yml", ".json", ".toml", ".ini", ".conf", ".properties":
		return true
	}
	return false
}

// isDocsPath detecta arquivos de documentação.
func isDocsPath(p string) bool {
	base := filepath.Base(p)
	if strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".rst") ||
		strings.HasSuffix(base, ".adoc") || strings.HasSuffix(base, ".txt") {
		// README/CHANGELOG/LICENSE em qualquer lugar contam como docs.
		if strings.HasPrefix(base, "README") || strings.HasPrefix(base, "CHANGELOG") ||
			strings.HasPrefix(base, "LICENSE") || strings.HasPrefix(base, "CONTRIBUTING") ||
			strings.HasPrefix(base, "AUTHORS") {
			return true
		}
		// *.md em pastas docs/ ou doc/ ou .github/
		first := firstSegment(p)
		if first == "docs" || first == "doc" || first == "documentation" {
			return true
		}
		// *.md especificamente fora de código (heurística: profundidade 1 e
		// nome começa com maiúscula) — conservador, só compte com test paths.
		return false
	}
	first := firstSegment(p)
	if first == "docs" || first == "doc" || first == "documentation" {
		// Qualquer arquivo em pasta docs/ conta.
		return true
	}
	return false
}

// isTestPath detecta arquivos/pastas de teste.
func isTestPath(p string) bool {
	base := filepath.Base(p)
	// Sufixos comuns.
	if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, "_test.rb") || strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.js") || strings.HasSuffix(base, ".spec.tsx") ||
		strings.HasSuffix(base, ".spec.jsx") || strings.HasSuffix(base, "Test.java") ||
		strings.HasSuffix(base, "Tests.java") || strings.HasSuffix(base, "Test.kt") {
		return true
	}
	// Pastas.
	first := firstSegment(p)
	switch first {
	case "test", "tests", "__tests__", "spec", "specs", "testdata":
		return true
	}
	return false
}

// isCIConfigPath detecta arquivos de CI: GitHub Actions, GitLab CI,
// Jenkins, CircleCI, Travis.
func isCIConfigPath(p string) bool {
	first := firstSegment(p)
	switch first {
	case ".github", ".circleci":
		// Qualquer arquivo dentro.
		return true
	}
	if strings.HasPrefix(p, ".github/workflows/") || strings.HasPrefix(p, ".github/actions/") {
		return true
	}
	base := filepath.Base(p)
	switch base {
	case ".gitlab-ci.yml", ".travis.yml", "Jenkinsfile", "appveyor.yml",
		"azure-pipelines.yml", "bitbucket-pipelines.yml":
		return true
	}
	return false
}

// firstSegment devolve o primeiro segmento do path (sem leading slash).
func firstSegment(p string) string {
	p = strings.TrimPrefix(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}
