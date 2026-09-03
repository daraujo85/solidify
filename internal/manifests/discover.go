// Package manifests descobre manifests relevantes para um range de
// commits sem fazer walk integral do repositório.
//
// Algoritmo:
//  1. Para cada path alterado, sobe a árvore (path → parent → root) até
//     achar um manifest ou atingir o repoRoot.
//  2. Para cada ManifestRoot configurado, desce só dentro daquele
//     subtree (não passeia o repo inteiro).
//
// Os manifests descobertos são deduplicados por path.
package manifests

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Kind identifica o tipo de manifest.
type Kind string

const (
	KindUnknown            Kind = ""
	KindNodePackageJSON    Kind = "node-package.json"
	KindNodeYarnLock       Kind = "node-yarn.lock"
	KindNodePnpmLock       Kind = "node-pnpm.lock"
	KindGoMod              Kind = "go-mod"
	KindPythonRequirements Kind = "python-requirements"
	KindPythonPyproject    Kind = "python-pyproject"
	KindRustCargo          Kind = "rust-cargo"
	KindJavaPom            Kind = "java-pom"
	KindJavaGradle         Kind = "java-gradle"
	KindPHPComposer        Kind = "php-composer"
	KindRubyGemfile        Kind = "ruby-gemfile"
	KindFlutterPubspec     Kind = "flutter-pubspec"
	KindElixirMix          Kind = "elixir-mix"
	KindDotnetCsproj       Kind = "dotnet-csproj"
	KindDotnetFsproj       Kind = "dotnet-fsproj"
	KindDotnetSln          Kind = "dotnet-sln"
	KindDotnetGlobalJson   Kind = "dotnet-global.json"
)

// Manifest é uma instância de manifest encontrada.
type Manifest struct {
	// Path é o path relativo ao repoRoot (com separador /).
	Path string
	// Kind identifica o tipo.
	Kind Kind
	// Dir é o diretório relativo (sem o nome do arquivo).
	Dir string
}

// fileToKind mapeia nome de arquivo (ou glob) → Kind. Vários podem
// coexistir num diretório (monorepo polyglot).
var fileToKind = []struct {
	name string
	kind Kind
}{
	{"package.json", KindNodePackageJSON},
	{"yarn.lock", KindNodeYarnLock},
	{"pnpm-lock.yaml", KindNodePnpmLock},
	{"go.mod", KindGoMod},
	{"requirements.txt", KindPythonRequirements},
	{"pyproject.toml", KindPythonPyproject},
	{"Cargo.toml", KindRustCargo},
	{"pom.xml", KindJavaPom},
	{"build.gradle", KindJavaGradle},
	{"build.gradle.kts", KindJavaGradle},
	{"composer.json", KindPHPComposer},
	{"Gemfile", KindRubyGemfile},
	{"pubspec.yaml", KindFlutterPubspec},
	{"mix.exs", KindElixirMix},
	{"*.csproj", KindDotnetCsproj},
	{"*.fsproj", KindDotnetFsproj},
	{"*.sln", KindDotnetSln},
	{"global.json", KindDotnetGlobalJson},
}

// Discover encontra manifests relevantes para os changed paths, subindo
// apenas nas cadeias de diretórios desses paths e nos subtrees
// configurados em manifestRoots. repoRoot é o diretório do repo git.
func Discover(repoRoot string, changedPaths []string, manifestRoots []string) ([]Manifest, error) {
	found := make(map[string]Manifest)

	// 1. Walk up de cada changed path.
	for _, p := range changedPaths {
		dir := path.Dir(p)
		if dir == "." {
			dir = ""
		}
		for {
			cur := dir
			if cur == "." {
				cur = ""
			}
			for _, m := range lookupManifests(repoRoot, cur) {
				found[m.Path] = m
			}
			if cur == "" {
				break
			}
			parent := path.Dir(cur)
			if parent == cur {
				break
			}
			dir = parent
		}
	}

	// 2. Scan dentro de cada ManifestRoot.
	for _, root := range manifestRoots {
		abs := filepath.Join(repoRoot, root)
		if err := scanDir(abs, repoRoot, found, 0); err != nil {
			return nil, err
		}
	}

	out := make([]Manifest, 0, len(found))
	for _, m := range found {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// lookupManifests devolve TODOS os manifests presentes no diretório dir
// (relativo). Pode haver vários num monorepo polyglot.
func lookupManifests(repoRoot, dir string) []Manifest {
	abs := filepath.Join(repoRoot, dir)
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil
	}
	var out []Manifest
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, e := range fileToKind {
			if matchPattern(entry.Name(), e.name) {
				full := filepath.Join(abs, entry.Name())
				rel, _ := filepath.Rel(repoRoot, full)
				out = append(out, Manifest{
					Path: filepath.ToSlash(rel),
					Kind: e.kind,
					Dir:  filepath.ToSlash(dir),
				})
			}
		}
	}
	return out
}

// matchPattern retorna true se name bate com pattern (suporta globs como
// "*.csproj"). name é o nome puro do arquivo.
func matchPattern(name, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return name == pattern
	}
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false
	}
	return matched
}

// scanDir desce a árvore a partir de root, achando manifests. depth limita
// a recursão para monorepos muito aninhados.
func scanDir(absDir, repoRoot string, found map[string]Manifest, depth int) error {
	if depth > 8 {
		return nil
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// Manifests no diretório atual.
	dirRel, _ := filepath.Rel(repoRoot, absDir)
	dirRelSlash := filepath.ToSlash(dirRel)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, e := range fileToKind {
			if matchPattern(entry.Name(), e.name) {
				rel, _ := filepath.Rel(repoRoot, filepath.Join(absDir, entry.Name()))
				found[filepath.ToSlash(rel)] = Manifest{
					Path: filepath.ToSlash(rel),
					Kind: e.kind,
					Dir:  dirRelSlash,
				}
			}
		}
	}
	// Recursa nos subdiretórios (exceto os óbvios de lixo).
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch name {
		case "node_modules", ".git", "vendor", "target", "build", "dist",
			".next", ".nuxt", ".venv", "venv", "__pycache__":
			continue
		}
		if err := scanDir(filepath.Join(absDir, name), repoRoot, found, depth+1); err != nil {
			return err
		}
	}
	return nil
}
