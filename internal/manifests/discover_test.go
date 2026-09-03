package manifests

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// mkFile cria arquivos sem depender de helpers externos.
func mkFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func byPath(m []Manifest) []string {
	out := make([]string, 0, len(m))
	for _, x := range m {
		out = append(out, x.Path)
	}
	sort.Strings(out)
	return out
}

// Aceitação: walk-up encontra o package.json do changed path.
func TestDiscoverFromChangedPathAncestors(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "package.json", "{}")
	mkFile(t, root, "src/index.js", "x")
	mkFile(t, root, "src/lib/util.js", "y")

	got, err := Discover(root,
		[]string{"src/lib/util.js", "src/index.js"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "package.json" || got[0].Kind != KindNodePackageJSON {
		t.Fatalf("manifests = %+v, quero [package.json node-package.json]", got)
	}
	if got[0].Dir != "" {
		t.Errorf("Dir = %q, quero vazio (root)", got[0].Dir)
	}
}

// Aceitação: monorepo — manifests próximos de cada app são encontrados.
func TestDiscoverMonorepoMultipleApps(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "package.json", "{}")          // root workspace
	mkFile(t, root, "apps/web/package.json", "{}") // web app
	mkFile(t, root, "apps/api/go.mod", "module x") // api service
	mkFile(t, root, "apps/api/cmd/main.go", "package main")
	mkFile(t, root, "apps/api/internal/handler.go", "package h")
	mkFile(t, root, "packages/ui/package.json", "{}") // ui lib
	mkFile(t, root, "packages/ui/src/Button.tsx", "x")
	mkFile(t, root, "docs/README.md", "x")

	got, err := Discover(root, []string{
		"apps/api/cmd/main.go",
		"apps/api/internal/handler.go",
		"packages/ui/src/Button.tsx",
		"docs/README.md",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"apps/api/go.mod",          // walk-up de apps/api/cmd e apps/api/internal
		"package.json",             // raiz (walk-up até "")
		"packages/ui/package.json", // próprio dir da ui
	}
	if g, w := byPath(got), want; !equalStr(g, w) {
		t.Fatalf("paths = %v, quero %v", g, w)
	}
}

func equalStr(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Aceitação: changed path sem manifest no ancestry → entry vazia (sem erro).
func TestDiscoverNoManifestReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "foo/bar.go", "package bar")

	got, err := Discover(root, []string{"foo/bar.go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("sem manifest esperado, veio %+v", got)
	}
}

// Aceitação: ManifestRoots adiciona subtree independente dos changed paths.
func TestDiscoverManifestRootsSubtree(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "services/auth/go.mod", "module a")
	mkFile(t, root, "services/billing/go.mod", "module b")
	mkFile(t, root, "services/billing/cmd/x.go", "package x")
	mkFile(t, root, "services/auth/handler.go", "package a")

	got, err := Discover(root,
		[]string{"services/auth/handler.go"},
		[]string{"services/billing"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"services/auth/go.mod",
		"services/billing/go.mod",
	}
	if g, w := byPath(got), want; !equalStr(g, w) {
		t.Fatalf("paths = %v, quero %v", g, w)
	}
}

// Aceitação: ManifestRoots pula lixo (node_modules, vendor, etc).
func TestDiscoverManifestRootsSkipsJunkDirs(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "real/package.json", "{}")
	mkFile(t, root, "real/node_modules/lodash/package.json", "{}")
	mkFile(t, root, "real/vendor/x/package.json", "{}")

	got, err := Discover(root, nil, []string{"real"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "real/package.json" {
		t.Fatalf("deveria ignorar node_modules/vendor, veio %+v", byPath(got))
	}
}

// Aceitação: deduplicação — mesmo manifest encontrado por dois paths = 1 entry.
func TestDiscoverDedupesManifests(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "package.json", "{}")
	mkFile(t, root, "src/a.js", "")
	mkFile(t, root, "src/b.js", "")

	got, err := Discover(root,
		[]string{"src/a.js", "src/b.js", "src/a.js"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("dedup falhou: %+v", byPath(got))
	}
}

// Aceitação: bounded — changed paths determinam visitação; arquivos
// em diretórios não relacionados não afetam a descoberta.
func TestDiscoverDoesNotVisitUnrelatedDirs(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "foo/bar/package.json", "{}")
	mkFile(t, root, "foo/bar/util.js", "")
	// Diretórios não ancestrais: devem ser ignorados.
	mkFile(t, root, "baz/large-tree/package.json", "{}")
	mkFile(t, root, "unrelated/package.json", "{}")

	got, err := Discover(root, []string{"foo/bar/util.js"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "foo/bar/package.json" {
		t.Fatalf("manifests = %+v, quero só foo/bar/package.json", byPath(got))
	}
}

// Aceitação: kinds reconhecidos para cada stack.
func TestDiscoverRecognizesStacks(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "Cargo.toml", "")
	mkFile(t, root, "src/lib.rs", "")
	mkFile(t, root, "pyproject.toml", "")
	mkFile(t, root, "main.py", "")
	mkFile(t, root, "pom.xml", "")
	mkFile(t, root, "src/main/java/App.java", "")
	mkFile(t, root, "composer.json", "{}")
	mkFile(t, root, "index.php", "")
	mkFile(t, root, "Gemfile", "")
	mkFile(t, root, "config/routes.rb", "")
	mkFile(t, root, "pubspec.yaml", "")
	mkFile(t, root, "lib/main.dart", "")
	mkFile(t, root, "mix.exs", "")
	mkFile(t, root, "lib/app.ex", "")

	got, err := Discover(root, []string{
		"src/lib.rs",
		"main.py",
		"src/main/java/App.java",
		"index.php",
		"config/routes.rb",
		"lib/main.dart",
		"lib/app.ex",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	wantKinds := map[string]Kind{
		"Cargo.toml":     KindRustCargo,
		"pyproject.toml": KindPythonPyproject,
		"pom.xml":        KindJavaPom,
		"composer.json":  KindPHPComposer,
		"Gemfile":        KindRubyGemfile,
		"pubspec.yaml":   KindFlutterPubspec,
		"mix.exs":        KindElixirMix,
	}
	if len(got) != len(wantKinds) {
		t.Fatalf("manifests = %+v, quero %d", byPath(got), len(wantKinds))
	}
	for _, m := range got {
		w, ok := wantKinds[m.Path]
		if !ok {
			t.Errorf("manifest inesperado: %s", m.Path)
			continue
		}
		if m.Kind != w {
			t.Errorf("%s: kind = %q, quero %q", m.Path, m.Kind, w)
		}
		delete(wantKinds, m.Path)
	}
	for k := range wantKinds {
		t.Errorf("manifest esperado não encontrado: %s", k)
	}
}

// Aceitação: lookupManifests puro (sem walk-up).
func TestLookupManifests(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "apps/web/package.json", "{}")

	got := lookupManifests(root, "apps/web")
	if len(got) != 1 {
		t.Fatalf("manifests = %+v, quero 1", got)
	}
	m := got[0]
	if m.Kind != KindNodePackageJSON {
		t.Errorf("Kind = %q", m.Kind)
	}
	if m.Dir != "apps/web" {
		t.Errorf("Dir = %q", m.Dir)
	}
	if m.Path != "apps/web/package.json" {
		t.Errorf("Path = %q", m.Path)
	}

	// Polyglot: package.json + pyproject.toml + Cargo.toml no mesmo dir.
	mkFile(t, root, "polyglot/package.json", "{}")
	mkFile(t, root, "polyglot/pyproject.toml", "")
	mkFile(t, root, "polyglot/Cargo.toml", "")
	poly := lookupManifests(root, "polyglot")
	if len(poly) != 3 {
		t.Errorf("polyglot = %+v, quero 3 manifests", poly)
	}

	// Dir inexistente: vazio.
	if got := lookupManifests(root, "no/such/dir"); len(got) != 0 {
		t.Errorf("dir inexistente devolveu %+v", got)
	}
}

// Aceitação: scanDir respeita limite de profundidade (depth > 8 = stop).
func TestScanDirDepthLimit(t *testing.T) {
	root := t.TempDir()
	// Cria 10 níveis de profundidade (d0..d9), cada um com package.json.
	for i := 0; i < 10; i++ {
		rel := ""
		for j := 0; j <= i; j++ {
			rel = filepath.Join(rel, "d"+string(rune('0'+j)))
		}
		mkFile(t, root, filepath.Join(rel, "package.json"), "{}")
	}

	found := map[string]Manifest{}
	if err := scanDir(filepath.Join(root, "d0"), root, found, 0); err != nil {
		t.Fatal(err)
	}
	// depth=0..8 processam (9 níveis); depth=9 não.
	// d8 = 9º nível (depth=8 ao visitar) → encontrado.
	// d9 = 10º nível (depth=9, blocked) → NÃO encontrado.
	if _, blocked := found["d0/d1/d2/d3/d4/d5/d6/d7/d8/d9/package.json"]; blocked {
		t.Errorf("manifest além do limite (d9) foi encontrado: %+v", found)
	}
	// E o do nível 8 foi encontrado (sanidade).
	if _, ok := found["d0/d1/d2/d3/d4/d5/d6/d7/d8/package.json"]; !ok {
		t.Errorf("manifest no nível 8 deveria ter sido encontrado: %+v", found)
	}
}
