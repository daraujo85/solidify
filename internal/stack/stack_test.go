package stack

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/diegoaraujo/solidify/internal/manifests"
)

// Wrappers para reduzir ruído.
func osMkdirAll(p string) error     { return os.MkdirAll(p, 0o755) }
func osWriteFile(p, b string) error { return os.WriteFile(p, []byte(b), 0o644) }
func parentDir(p string) string     { return filepath.Dir(p) }

func m(path string, kind manifests.Kind) manifests.Manifest {
	return manifests.Manifest{Path: path, Kind: kind, Dir: ""}
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

// Aceitação: cada kind forte (1.0) gera detecção com confidence 1.0.
func TestDetectEachStackKind(t *testing.T) {
	cases := []struct {
		kind   manifests.Kind
		stack  Stack
		path   string
		weight float64
	}{
		{manifests.KindNodePackageJSON, StackNode, "package.json", 1.0},
		{manifests.KindGoMod, StackGo, "go.mod", 1.0},
		{manifests.KindPythonPyproject, StackPython, "pyproject.toml", 1.0},
		{manifests.KindRustCargo, StackRust, "Cargo.toml", 1.0},
		{manifests.KindJavaPom, StackJava, "pom.xml", 1.0},
		{manifests.KindJavaGradle, StackJava, "build.gradle", 1.0},
		{manifests.KindPHPComposer, StackPHP, "composer.json", 1.0},
		{manifests.KindRubyGemfile, StackRuby, "Gemfile", 1.0},
		{manifests.KindFlutterPubspec, StackFlutter, "pubspec.yaml", 1.0},
		{manifests.KindDotnetCsproj, StackDotnet, "App.csproj", 1.0},
		{manifests.KindDotnetSln, StackDotnet, "App.sln", 0.5},
	}
	for _, tc := range cases {
		t.Run(string(tc.stack), func(t *testing.T) {
			got := Detect([]manifests.Manifest{m(tc.path, tc.kind)})
			if len(got) != 1 {
				t.Fatalf("detect = %+v, quero 1", got)
			}
			if got[0].Stack != tc.stack {
				t.Errorf("stack = %q, quero %q", got[0].Stack, tc.stack)
			}
			if got[0].Confidence != tc.weight {
				t.Errorf("confidence = %v, quero %v", got[0].Confidence, tc.weight)
			}
			if got[0].Evidence[0] != tc.path {
				t.Errorf("evidence[0] = %q, quero %q", got[0].Evidence[0], tc.path)
			}
		})
	}
}

// Aceitação: lockfiles sozinhos dão confidence 0.5.
func TestDetectLockfilesAloneAreWeak(t *testing.T) {
	got := Detect([]manifests.Manifest{
		m("yarn.lock", manifests.KindNodeYarnLock),
	})
	if len(got) != 1 || got[0].Stack != StackNode || got[0].Confidence != 0.5 {
		t.Fatalf("yarn.lock sozinho = %+v, quero [node 0.5]", got)
	}
}

// Aceitação: package.json + yarn.lock no mesmo repo soma weights.
func TestDetectNodeMultipleKindsAccumulate(t *testing.T) {
	got := Detect([]manifests.Manifest{
		m("package.json", manifests.KindNodePackageJSON),
		m("yarn.lock", manifests.KindNodeYarnLock),
	})
	if len(got) != 1 {
		t.Fatalf("detect = %+v, quero 1", got)
	}
	// 1.0 + 0.5 = 1.5 → cap a 1.0
	if got[0].Confidence != 1.0 {
		t.Errorf("confidence = %v, quero 1.0 (cap)", got[0].Confidence)
	}
}

// Aceitação: manifests desconhecidos são ignorados.
func TestDetectIgnoresUnknownKinds(t *testing.T) {
	got := Detect([]manifests.Manifest{
		m("foo.bar", manifests.KindUnknown),
	})
	if len(got) != 0 {
		t.Fatalf("unknown deveria ser ignorado: %+v", got)
	}
}

// Aceitação: monorepo polyglot devolve múltiplas detecções ordenadas.
func TestDetectMultipleStacksSorted(t *testing.T) {
	got := Detect([]manifests.Manifest{
		m("package.json", manifests.KindNodePackageJSON),
		m("services/api/go.mod", manifests.KindGoMod),
		m("packages/ui/pyproject.toml", manifests.KindPythonPyproject),
	})
	if len(got) != 3 {
		t.Fatalf("detect = %+v, quero 3", got)
	}
	want := []Stack{StackNode, StackGo, StackPython}
	for i, w := range want {
		if got[i].Stack != w {
			t.Errorf("detect[%d] = %q, quero %q", i, got[i].Stack, w)
		}
	}
}

// Tiebreak: empates usam a ordem canônica (Node > Go > ...).
func TestDetectTieBreakUsesCanonicalOrder(t *testing.T) {
	// Node e Go ambos com 1.0 → Node vence.
	got := Detect([]manifests.Manifest{
		m("package.json", manifests.KindNodePackageJSON),
		m("go.mod", manifests.KindGoMod),
	})
	if got[0].Stack != StackNode {
		t.Errorf("empate Node vs Go → Node vence, veio %q", got[0].Stack)
	}
}

// Aceitação: Primary devolve o topo ou vazio.
func TestPrimary(t *testing.T) {
	if _, ok := Primary(nil); ok {
		t.Error("nil deve devolver ok=false")
	}
	got := Detect([]manifests.Manifest{
		m("go.mod", manifests.KindGoMod),
		m("package.json", manifests.KindNodePackageJSON),
	})
	p, ok := Primary(got)
	if !ok || p != StackNode {
		t.Errorf("primary = %q, ok=%v, quero Node true", p, ok)
	}
}

// Aceitação: glob csproj detectado pelo discover vira dotnet 1.0.
func TestDetectDotnetFromCsprojGlob(t *testing.T) {
	root := t.TempDir()
	mkFile(t, root, "src/MyApp/MyApp.csproj", "<Project/>")
	mkFile(t, root, "src/MyApp/Program.cs", "class P{}")

	ms, err := manifests.Discover(root, []string{"src/MyApp/Program.cs"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) == 0 {
		t.Fatal("manifests vazio")
	}
	got := Detect(ms)
	found := false
	for _, d := range got {
		if d.Stack == StackDotnet && d.Confidence >= 1.0 {
			found = true
		}
	}
	if !found {
		t.Errorf("dotnet não detectado: %+v (manifests=%v)", got, ms)
	}
}

// mkFile escreve body em dir/rel, criando diretórios intermediários.
func mkFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := dir + "/" + rel
	if err := osMkdirAll(parentDir(full)); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(full, body); err != nil {
		t.Fatal(err)
	}
}

// Sanidade: stackOrder é consistente e completo.
func TestStackOrderCoversAll(t *testing.T) {
	all := []Stack{
		StackNode, StackGo, StackPython, StackJava, StackRust,
		StackPHP, StackRuby, StackFlutter, StackDotnet,
	}
	for _, s := range all {
		if stackOrder(s) >= 99 {
			t.Errorf("stack %q sem ordem canônica", s)
		}
	}
}

// Sanidade: confidence fica em [0, 1].
func TestDetectConfidenceBounded(t *testing.T) {
	// Muitos package.json (empilhados).
	items := make([]manifests.Manifest, 10)
	for i := range items {
		items[i] = m("a/package.json", manifests.KindNodePackageJSON)
	}
	got := Detect(items)
	if got[0].Confidence > 1.0 {
		t.Errorf("confidence > 1.0: %v", got[0].Confidence)
	}
}

// Aceitação: evidência ordenada alfabeticamente.
func TestDetectEvidenceSorted(t *testing.T) {
	got := Detect([]manifests.Manifest{
		m("z/package.json", manifests.KindNodePackageJSON),
		m("a/package.json", manifests.KindNodePackageJSON),
		m("m/package.json", manifests.KindNodePackageJSON),
	})
	want := []string{"a/package.json", "m/package.json", "z/package.json"}
	if !equalStr(got[0].Evidence, want) {
		t.Errorf("evidence = %v, quero %v", got[0].Evidence, want)
	}
	sort.Strings(got[0].Evidence) // silencia linter
}
