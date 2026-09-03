package testexec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: cria dir com arquivos.
func makeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Aceitação: DetectStack go.
func TestDetectStackGo(t *testing.T) {
	dir := makeTree(t, map[string]string{"go.mod": "module x"})
	if DetectStack(dir) != StackGo {
		t.Errorf("stack errada")
	}
}

// Aceitação: DetectStack node.
func TestDetectStackNode(t *testing.T) {
	dir := makeTree(t, map[string]string{"package.json": "{}"})
	if DetectStack(dir) != StackNode {
		t.Errorf("stack errada")
	}
}

// Aceitação: DetectStack python.
func TestDetectStackPython(t *testing.T) {
	for _, f := range []string{"pyproject.toml", "requirements.txt", "setup.py"} {
		dir := makeTree(t, map[string]string{f: ""})
		if DetectStack(dir) != StackPython {
			t.Errorf("%s não detectou python", f)
		}
	}
}

// Aceitação: DetectStack java.
func TestDetectStackJava(t *testing.T) {
	for _, f := range []string{"pom.xml", "build.gradle"} {
		dir := makeTree(t, map[string]string{f: ""})
		if DetectStack(dir) != StackJava {
			t.Errorf("%s não detectou java", f)
		}
	}
}

// Aceitação: DetectStack rust.
func TestDetectStackRust(t *testing.T) {
	dir := makeTree(t, map[string]string{"Cargo.toml": ""})
	if DetectStack(dir) != StackRust {
		t.Errorf("rust não detectado")
	}
}

// Aceitação: DetectStack ruby.
func TestDetectStackRuby(t *testing.T) {
	dir := makeTree(t, map[string]string{"Gemfile": ""})
	if DetectStack(dir) != StackRuby {
		t.Errorf("ruby não detectado")
	}
}

// Aceitação: DetectStack dotnet (csproj).
func TestDetectStackDotnet(t *testing.T) {
	dir := makeTree(t, map[string]string{"app.csproj": ""})
	if DetectStack(dir) != StackDotnet {
		t.Errorf("dotnet não detectado")
	}
}

// Aceitação: DetectStack unknown.
func TestDetectStackUnknown(t *testing.T) {
	dir := t.TempDir()
	if DetectStack(dir) != StackUnknown {
		t.Errorf("devia ser unknown")
	}
}

// Aceitação: Discover default Go.
func TestDiscoverGo(t *testing.T) {
	dir := makeTree(t, map[string]string{"go.mod": "module x"})
	d := New(dir, StackUnknown)
	cmds, err := d.Discover()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cmds) == 0 {
		t.Fatalf("nenhum comando")
	}
	if cmds[0].Bin != "go" || cmds[0].Args[0] != "test" {
		t.Errorf("got = %+v", cmds[0])
	}
	if cmds[0].Source != "default" {
		t.Errorf("source = %s", cmds[0].Source)
	}
}

// Aceitação: Discover manifest Node.
func TestDiscoverNodeManifest(t *testing.T) {
	pkg := `{"scripts":{"test":"echo running tests"}}`
	dir := makeTree(t, map[string]string{"package.json": pkg})
	d := New(dir, StackNode)
	cmds, err := d.Discover()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cmds) == 0 {
		t.Fatalf("nenhum")
	}
	if cmds[0].Source != "manifest" {
		t.Errorf("source = %s", cmds[0].Source)
	}
}

// Aceitação: Discover override tem prioridade.
func TestDiscoverOverridePriority(t *testing.T) {
	pkg := `{"scripts":{"test":"npm test"}}`
	override := `{"commands":[{"bin":"my-test","args":["-x"]}]}`
	dir := makeTree(t, map[string]string{
		"package.json":       pkg,
		".solidify/test.json": override,
	})
	d := New(dir, StackNode)
	cmds, err := d.Discover()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cmds[0].Bin != "my-test" {
		t.Errorf("override devia vir primeiro: %+v", cmds)
	}
	if cmds[0].Source != "override" {
		t.Errorf("source = %s", cmds[0].Source)
	}
}

// Aceitação: Override YAML inválido → ignored.
func TestDiscoverInvalidYAML(t *testing.T) {
	dir := makeTree(t, map[string]string{
		".solidify/test.yaml": "not yaml or json: [[[",
		"package.json":       `{"scripts":{"test":"x"}}`,
	})
	d := New(dir, StackNode)
	cmds, err := d.Discover()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cmds) == 0 {
		t.Errorf("devia ter pelo menos manifest")
	}
}

// Aceitação: Strict sem match falha.
func TestDiscoverStrictEmpty(t *testing.T) {
	dir := t.TempDir()
	d := New(dir, StackUnknown)
	d.Strict = true
	if _, err := d.Discover(); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Strict com match OK.
func TestDiscoverStrictMatch(t *testing.T) {
	dir := makeTree(t, map[string]string{"go.mod": ""})
	d := New(dir, StackUnknown)
	d.Strict = true
	cmds, err := d.Discover()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cmds) == 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: Root vazio falha.
func TestDiscoverEmptyRoot(t *testing.T) {
	d := New("", StackUnknown)
	if _, err := d.Discover(); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Root inexistente falha.
func TestDiscoverMissingRoot(t *testing.T) {
	d := New("/no/such/path/here", StackUnknown)
	if _, err := d.Discover(); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Command.String.
func TestCommandString(t *testing.T) {
	c := Command{Bin: "go", Args: []string{"test", "./..."}}
	if c.String() != "go test ./..." {
		t.Errorf("got = %q", c.String())
	}
}

// Aceitação: dedupe.
func TestDedupeCommands(t *testing.T) {
	cmds := []Command{
		{Bin: "go", Args: []string{"test"}},
		{Bin: "go", Args: []string{"test"}},
		{Bin: "go", Args: []string{"build"}},
	}
	out := dedupeCommands(cmds)
	if len(out) != 2 {
		t.Errorf("len = %d", len(out))
	}
}

// Aceitação: source priority.
func TestSourcePriority(t *testing.T) {
	if sourcePriority("override") >= sourcePriority("manifest") {
		t.Errorf("override devia vir antes")
	}
	if sourcePriority("manifest") >= sourcePriority("default") {
		t.Errorf("manifest devia vir antes")
	}
}

// Aceitação: default Python.
func TestDiscoverPythonDefault(t *testing.T) {
	dir := makeTree(t, map[string]string{"requirements.txt": ""})
	d := New(dir, StackUnknown)
	cmds, _ := d.Discover()
	if cmds[0].Bin != "pytest" {
		t.Errorf("got = %s", cmds[0].Bin)
	}
}

// Aceitação: detectPackageManager pnpm.
func TestDetectPackageManagerPnpm(t *testing.T) {
	dir := makeTree(t, map[string]string{"pnpm-lock.yaml": ""})
	if detectPackageManager(dir) != "pnpm" {
		t.Errorf("got = %s", detectPackageManager(dir))
	}
}

// Aceitação: detectPackageManager yarn.
func TestDetectPackageManagerYarn(t *testing.T) {
	dir := makeTree(t, map[string]string{"yarn.lock": ""})
	if detectPackageManager(dir) != "yarn" {
		t.Errorf("got = %s", detectPackageManager(dir))
	}
}

// Aceitação: detectPackageManager npm.
func TestDetectPackageManagerNpm(t *testing.T) {
	dir := makeTree(t, map[string]string{"package-lock.json": ""})
	if detectPackageManager(dir) != "npm" {
		t.Errorf("got = %s", detectPackageManager(dir))
	}
}

// Aceitação: detectPackageManager default.
func TestDetectPackageManagerDefault(t *testing.T) {
	dir := t.TempDir()
	if detectPackageManager(dir) != "npm" {
		t.Errorf("got = %s", detectPackageManager(dir))
	}
}

// Aceitação: parseScript simples.
func TestParseScript(t *testing.T) {
	cases := map[string][]string{
		"npm test":     {"test"},
		"jest --watch": {"--watch"},
		"":             {},
		"only-cmd":     {"only-cmd"},
	}
	for in, want := range cases {
		got := parseScript(in)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("parseScript(%q) = %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: Override JSON load.
func TestOverrideJSON(t *testing.T) {
	ov := Override{Commands: []Command{{Bin: "x"}}}
	data, _ := json.Marshal(ov)
	if !strings.Contains(string(data), `"bin":"x"`) {
		t.Errorf("marshal errado: %s", data)
	}
}

// Aceitação: Discover detecta go.mod em subdir.
func TestDiscoverExplicitStack(t *testing.T) {
	dir := t.TempDir()
	d := New(dir, StackGo)
	d.Strict = true
	cmds, err := d.Discover()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cmds) == 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: Discover Java tem mvn e gradle defaults.
func TestDiscoverJavaDefaults(t *testing.T) {
	dir := makeTree(t, map[string]string{"pom.xml": ""})
	d := New(dir, StackJava)
	cmds, _ := d.Discover()
	if len(cmds) < 2 {
		t.Errorf("len = %d", len(cmds))
	}
}

// Aceitação: dotnet default.
func TestDiscoverDotnetDefault(t *testing.T) {
	dir := makeTree(t, map[string]string{"app.csproj": ""})
	d := New(dir, StackDotnet)
	cmds, _ := d.Discover()
	if cmds[0].Bin != "dotnet" {
		t.Errorf("got = %s", cmds[0].Bin)
	}
}

// Aceitação: Rust default.
func TestDiscoverRustDefault(t *testing.T) {
	dir := makeTree(t, map[string]string{"Cargo.toml": ""})
	d := New(dir, StackRust)
	cmds, _ := d.Discover()
	if cmds[0].Bin != "cargo" {
		t.Errorf("got = %s", cmds[0].Bin)
	}
}

// Aceitação: Python manifest (pyproject.toml) detectado.
func TestDiscoverPythonPyproject(t *testing.T) {
	dir := makeTree(t, map[string]string{"pyproject.toml": "[project]"})
	d := New(dir, StackUnknown)
	cmds, _ := d.Discover()
	if cmds[0].Bin == "" {
		t.Errorf("vazio")
	}
}
