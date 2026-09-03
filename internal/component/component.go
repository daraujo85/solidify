// Package component classifica um range de commits (conjunto de paths)
// em um componente canônico, conforme ARCHITECTURE.md §8.3.
//
// Ordem de prioridade (mais específico vence em empate):
//
//  1. database     (migrations, db/, schema/, sql/)
//  2. infra        (infra/, terraform/, k8s/, helm/, ansible/, deploy/)
//  3. mobile       (mobile/, ios/, android/, ou stack Flutter)
//  4. worker       (workers/, jobs/, tasks/, queues/, consumers/)
//  5. backend-api  (apps/api/, api/, server/, backend/, services/)
//  6. frontend-web (apps/web/, web/, frontend/, client/, ui/, pages/)
//  7. library      (lib/, libs/, packages/, modules/, sdk/, utils/)
//  8. unknown
package component

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/diegoaraujo/solidify/internal/manifests"
	"github.com/diegoaraujo/solidify/internal/stack"
)

// Component é o tipo de componente canônico.
type Component string

const (
	FrontendWeb Component = "frontend-web"
	BackendAPI  Component = "backend-api"
	Worker      Component = "worker"
	Library     Component = "library"
	Mobile      Component = "mobile"
	Infra       Component = "infra"
	Database    Component = "database"
	Unknown     Component = "unknown"
)

// patternHint associa path-prefix a um componente.
type patternHint struct {
	prefixes []string // nome exato de segmento (case-sensitive)
	contains []string // substring em qualquer profundidade
	comp     Component
}

// hints na ORDEM de prioridade. Cada path é testado contra cada hint em
// sequência; o primeiro match vence (database > infra > mobile > worker
// > backend-api > frontend-web > library).
var hints = []patternHint{
	// Database — muito específico, vence quase tudo.
	{prefixes: []string{"migrations", "db", "prisma", "schema", "sql", "alembic"},
		comp: Database},
	{prefixes: []string{"infra", "iac", "terraform", "k8s", "helm", "ansible",
		"cloudformation", "deploy", "pulumi"},
		comp: Infra},
	{prefixes: []string{"mobile", "ios", "android", "swift", "kotlin"},
		comp: Mobile},
	{prefixes: []string{"workers", "worker", "jobs", "job", "tasks", "task",
		"queues", "queue", "consumers", "consumer", "cron", "schedulers"},
		comp: Worker},
	{prefixes: []string{"api", "server", "backend", "services", "controllers",
		"endpoints", "handlers", "rest"},
		comp: BackendAPI},
	{prefixes: []string{"web", "frontend", "client", "ui", "pages",
		"components", "views", "screens", "public", "static", "assets"},
		contains: []string{"/web/", "/frontend/", "/client/"},
		comp:     FrontendWeb},
	{prefixes: []string{"lib", "libs", "packages", "modules", "sdk",
		"utils", "helpers", "shared", "common", "core"},
		comp: Library},
}

// Classify devolve o componente dominante para um conjunto de paths
// alterados. Usa também manifests e stacks para confirmar (ex.: Flutter
// → mobile mesmo sem prefixo /mobile/).
func Classify(changedPaths []string, ms []manifests.Manifest, ds []stack.Detection) Component {
	if len(changedPaths) == 0 && len(ms) == 0 {
		return Unknown
	}

	// 1. Flutter detectado + pelo menos um path dentro do escopo → mobile.
	for _, d := range ds {
		if d.Stack == stack.StackFlutter && d.Confidence >= 1.0 {
			for _, p := range changedPaths {
				if pathInFlutterScope(p, ms) {
					return Mobile
				}
			}
		}
	}

	// 2. Score por path-hint. Cada path pode dar 1 voto.
	scores := make(map[Component]int)
	for _, p := range changedPaths {
		comp := matchPath(p)
		if comp != Unknown {
			scores[comp]++
		}
	}
	// 3. Manifests reforçam o voto SÓ se nenhum path votou — manifest
	// sozinho indica "existe projeto X no repo", não "X foi alterado".
	if len(scores) == 0 {
		for _, m := range ms {
			if c := matchPath(m.Path); c != Unknown {
				scores[c]++
			}
		}
	}

	// 4. Empate: prioridade do mais específico (ordem de hints).
	if len(scores) == 0 {
		return Unknown
	}
	max := 0
	for _, s := range scores {
		if s > max {
			max = s
		}
	}
	for _, h := range hints {
		if scores[h.comp] == max {
			return h.comp
		}
	}
	return Unknown
}

// matchPath devolve o componente cujo pattern bate com o path. Itera
// hints em ordem de prioridade; testa cada segmento do path (não só o
// primeiro). apps/api/handlers/user.go → "api" no meio, retorna
// BackendAPI.
func matchPath(p string) Component {
	p = filepath.ToSlash(p)
	segments := strings.Split(p, "/")
	for _, h := range hints {
		for _, pref := range h.prefixes {
			for _, seg := range segments {
				if seg == pref {
					return h.comp
				}
			}
		}
		for _, sub := range h.contains {
			if strings.Contains(p, sub) {
				return h.comp
			}
		}
	}
	return Unknown
}

// pathInFlutterScope devolve true se p está dentro de algum manifest
// Flutter (pubspec.yaml).
func pathInFlutterScope(p string, ms []manifests.Manifest) bool {
	for _, m := range ms {
		if m.Kind != manifests.KindFlutterPubspec {
			continue
		}
		if m.Dir == "" {
			return true
		}
		if strings.HasPrefix(p, m.Dir+"/") || p == m.Dir {
			return true
		}
	}
	return false
}

// Distribution devolve a distribuição de paths por componente — útil em
// relatório quando há mais de um componente no range (monorepo).
type Distribution struct {
	Component Component
	Paths     []string
}

// ClassifyAll devolve a partição de changedPaths por componente. Paths
// não casados vão para um bucket Unknown. Útil para relatar monorepos
// com frontend + backend.
func ClassifyAll(changedPaths []string, ms []manifests.Manifest, ds []stack.Detection) []Distribution {
	buckets := make(map[Component][]string)
	for _, p := range changedPaths {
		c := matchPath(p)
		// Override Flutter: stack Flutter + path dentro do escopo → mobile.
		if c == Unknown {
			for _, d := range ds {
				if d.Stack == stack.StackFlutter && pathInFlutterScope(p, ms) {
					c = Mobile
					break
				}
			}
		}
		buckets[c] = append(buckets[c], p)
	}
	out := make([]Distribution, 0, len(buckets))
	for c, paths := range buckets {
		sort.Strings(paths)
		out = append(out, Distribution{Component: c, Paths: paths})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Component < out[j].Component })
	return out
}
