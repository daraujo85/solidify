// Package applicability decide se cada gate é APPLICABLE,
// NOT_APPLICABLE ou CONDITIONAL para um dado release.
//
// Regras baseadas em ARCHITECTURE.md §8.4:
//   - Lighthouse: APPLICABLE só se há frontend-web E target configurado.
//     CONDITIONAL se há frontend-web sem target. NOT_APPLICABLE se não
//     há frontend-web.
//   - Sonar: APPLICABLE se há código. CONDITIONAL se cobertura/quality
//     não configurados.
//   - Security: APPLICABLE para backend-api, infra, library, worker.
//   - Tests: APPLICABLE se há código (não só docs/config/migration).
//   - ZAP: APPLICABLE para frontend-web OU backend-api.
//   - k6: APPLICABLE para backend-api OU worker.
//   - Migration: APPLICABLE se há paths de migration.
//   - Env: APPLICABLE se há env changes.
package applicability

import (
	"path/filepath"
	"strings"

	"github.com/diegoaraujo/solidify/internal/component"
	"github.com/diegoaraujo/solidify/internal/stack"
)

// Verdict é a decisão de applicability para um gate.
type Verdict string

const (
	Applicable    Verdict = "APPLICABLE"
	NotApplicable Verdict = "NOT_APPLICABLE"
	Conditional   Verdict = "CONDITIONAL"
)

// Gate é o gate (regra) sob decisão.
type Gate string

const (
	GateSonar      Gate = "sonar"
	GateTests      Gate = "tests"
	GateSecurity   Gate = "security"
	GateLighthouse Gate = "lighthouse"
	GateZAP        Gate = "zap"
	GateK6         Gate = "k6"
	GateMigration  Gate = "migration"
	GateEnv        Gate = "env"
)

// Decision é o veredito para um gate.
type Decision struct {
	Gate    Gate
	Verdict Verdict
	Reason  string
}

// Profile é o resumo do release sob análise.
type Profile struct {
	// Component principal (Classify) e componentes detectados
	// (ClassifyAll) — um release pode ter mais de um.
	Components []component.Component
	// Stacks detectadas.
	Stacks []stack.Detection
	// ChangedPaths do range. Caminhos relativos ao repoRoot.
	ChangedPaths []string
	// HasMigrations indica se alguma migration foi tocada.
	HasMigrations bool
	// HasEnvChanges indica se .env* ou variáveis de ambiente foram tocadas.
	HasEnvChanges bool
	// Configurações do projeto:
	LighthouseTarget string // URL configurada; vazio = não configurado
	SonarConfigured  bool
	ZAPConfigured    bool
	K6Configured     bool
}

// Decide devolve as decisões de applicability para todos os gates.
func Decide(p Profile) []Decision {
	compSet := make(map[component.Component]bool)
	for _, c := range p.Components {
		compSet[c] = true
	}
	hasCode := hasCodeChanges(p.ChangedPaths)

	return []Decision{
		decideSonar(p, hasCode),
		decideTests(p, hasCode),
		decideSecurity(compSet),
		decideLighthouse(p, compSet),
		decideZAP(p, compSet),
		decideK6(p, compSet),
		decideMigration(p),
		decideEnv(p),
	}
}

// decideSonar: roda quando há código. Sem config de qualidade vira
// CONDITIONAL.
func decideSonar(p Profile, hasCode bool) Decision {
	if !hasCode {
		return Decision{Gate: GateSonar, Verdict: NotApplicable,
			Reason: "sem paths de código no range"}
	}
	if !p.SonarConfigured {
		return Decision{Gate: GateSonar, Verdict: Conditional,
			Reason: "há código, mas qualidade/cobertura não configuradas"}
	}
	return Decision{Gate: GateSonar, Verdict: Applicable,
		Reason: "código presente e Sonar configurado"}
}

// decideTests: roda quando há código (não-docs/config).
func decideTests(p Profile, hasCode bool) Decision {
	if !hasCode {
		return Decision{Gate: GateTests, Verdict: NotApplicable,
			Reason: "sem paths de código no range"}
	}
	return Decision{Gate: GateTests, Verdict: Applicable,
		Reason: "código presente"}
}

// decideSecurity: roda para código de servidor / infra / lib.
func decideSecurity(compSet map[component.Component]bool) Decision {
	for _, c := range []component.Component{
		component.BackendAPI, component.Infra, component.Library, component.Worker,
	} {
		if compSet[c] {
			return Decision{Gate: GateSecurity, Verdict: Applicable,
				Reason: "componente " + string(c) + " requer análise de segurança"}
		}
	}
	return Decision{Gate: GateSecurity, Verdict: Conditional,
		Reason: "nenhum componente sensível; análise reduzida"}
}

// decideLighthouse: SÓ roda com frontend-web + target.
// Sem frontend-web ⇒ NOT_APPLICABLE (aceite crítico).
func decideLighthouse(p Profile, compSet map[component.Component]bool) Decision {
	if !compSet[component.FrontendWeb] {
		return Decision{Gate: GateLighthouse, Verdict: NotApplicable,
			Reason: "componente frontend-web ausente"}
	}
	if p.LighthouseTarget == "" {
		return Decision{Gate: GateLighthouse, Verdict: Conditional,
			Reason: "frontend-web presente, mas target Lighthouse não configurado"}
	}
	return Decision{Gate: GateLighthouse, Verdict: Applicable,
		Reason: "frontend-web + target " + p.LighthouseTarget}
}

// decideZAP: DAST em web endpoints ou API HTTP.
func decideZAP(p Profile, compSet map[component.Component]bool) Decision {
	if compSet[component.FrontendWeb] || compSet[component.BackendAPI] {
		if !p.ZAPConfigured {
			return Decision{Gate: GateZAP, Verdict: Conditional,
				Reason: "web/api presente, mas target ZAP não configurado"}
		}
		return Decision{Gate: GateZAP, Verdict: Applicable,
			Reason: "web/api presente e ZAP configurado"}
	}
	return Decision{Gate: GateZAP, Verdict: NotApplicable,
		Reason: "nem frontend-web nem backend-api"}
}

// decideK6: load test em serviços de backend ou workers.
func decideK6(p Profile, compSet map[component.Component]bool) Decision {
	if compSet[component.BackendAPI] || compSet[component.Worker] {
		if !p.K6Configured {
			return Decision{Gate: GateK6, Verdict: Conditional,
				Reason: "backend/worker presente, mas script k6 não configurado"}
		}
		return Decision{Gate: GateK6, Verdict: Applicable,
			Reason: "backend/worker presente e k6 configurado"}
	}
	return Decision{Gate: GateK6, Verdict: NotApplicable,
		Reason: "nem backend-api nem worker"}
}

// decideMigration: depende de HasMigrations.
func decideMigration(p Profile) Decision {
	if p.HasMigrations {
		return Decision{Gate: GateMigration, Verdict: Applicable,
			Reason: "migrations no range"}
	}
	return Decision{Gate: GateMigration, Verdict: NotApplicable,
		Reason: "sem migrations no range"}
}

// decideEnv: depende de HasEnvChanges.
func decideEnv(p Profile) Decision {
	if p.HasEnvChanges {
		return Decision{Gate: GateEnv, Verdict: Applicable,
			Reason: "mudanças em .env* / variáveis"}
	}
	return Decision{Gate: GateEnv, Verdict: NotApplicable,
		Reason: "sem mudanças de env no range"}
}

// hasCodeChanges devolve true se há ao menos um path que parece código
// (extensão ou pasta típica de código). Não inclui .md, .lock, .env,
// .yaml (config puro), migrations SQL.
func hasCodeChanges(paths []string) bool {
	for _, p := range paths {
		if isCodePath(p) {
			return true
		}
	}
	return false
}

// isCodePath detecta se p parece código-fonte.
func isCodePath(p string) bool {
	p = filepath.ToSlash(p)
	base := filepath.Base(p)
	// Pastas de código.
	first := firstSegment(p)
	if first == "test" || first == "tests" || first == "__tests__" ||
		first == "spec" || first == "specs" || first == "testdata" {
		return false
	}
	if first == "migrations" || first == "db" || first == "prisma" {
		return false
	}
	// Docs.
	if strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".rst") ||
		strings.HasSuffix(base, ".txt") || strings.HasSuffix(base, ".adoc") {
		return false
	}
	// Configurações.
	if strings.HasPrefix(base, ".env") {
		return false
	}
	switch filepath.Ext(base) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".py", ".rb", ".rs", ".java", ".kt", ".swift", ".dart",
		".php", ".cs", ".fs", ".cpp", ".c", ".h", ".hpp",
		".scala", ".groovy", ".clj", ".ex", ".exs", ".lua",
		".vue", ".svelte", ".elm":
		return true
	}
	// Lockfiles / config / build artifacts: não contam.
	switch base {
	case "package.json", "go.mod", "Cargo.toml", "Gemfile", "pom.xml",
		"build.gradle", "build.gradle.kts", "composer.json",
		"pyproject.toml", "requirements.txt", "pubspec.yaml",
		"Dockerfile", "Makefile":
		return true // manifest/build files indicam código
	}
	return false
}

// firstSegment devolve o primeiro segmento do path.
func firstSegment(p string) string {
	p = strings.TrimPrefix(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}
