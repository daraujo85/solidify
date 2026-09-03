// Package testexec — discovery de comandos de teste por stack.
//
// SAI-030: dado componente/stack, devolve lista ordenada de comandos
// de teste. Suporta override via .solidify/test.{yaml,json}.
package testexec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Stack identifica o ecossistema.
type Stack string

const (
	StackUnknown Stack = ""
	StackNode    Stack = "node"
	StackPython  Stack = "python"
	StackGo      Stack = "go"
	StackJava    Stack = "java"
	StackRust    Stack = "rust"
	StackRuby    Stack = "ruby"
	StackDotnet  Stack = "dotnet"
)

// Command é um comando a executar para rodar testes.
type Command struct {
	Stack  string   `json:"stack"`
	Bin    string   `json:"bin"`
	Args   []string `json:"args"`
	Cwd    string   `json:"cwd,omitempty"`
	Env    []string `json:"env,omitempty"`
	Source string   `json:"source"` // "override" | "manifest" | "default"
	Reason string   `json:"reason,omitempty"`
}

// String devolve linha de comando.
func (c Command) String() string {
	parts := []string{c.Bin}
	parts = append(parts, c.Args...)
	return strings.Join(parts, " ")
}

// Override carregado de .solidify/test.{yaml,json}.
type Override struct {
	Commands []Command `json:"commands"`
}

// Discoverer descobre comandos de teste.
type Discoverer struct {
	Root   string // dir raiz do projeto
	Stack  Stack  // stack primário (auto-detect se vazio)
	Comp   string // nome do componente
	Strict bool   // se true, falha quando não acha nada
}

// New cria discoverer.
func New(root string, stack Stack) *Discoverer {
	return &Discoverer{Root: root, Stack: stack}
}

// Discover devolve comandos ordenados (override > manifest > default).
func (d *Discoverer) Discover() ([]Command, error) {
	if d.Root == "" {
		return nil, fmt.Errorf("testexec: root vazio")
	}
	if _, err := os.Stat(d.Root); err != nil {
		return nil, fmt.Errorf("testexec: stat root: %w", err)
	}
	stack := d.Stack
	if stack == StackUnknown {
		stack = DetectStack(d.Root)
	}
	var cmds []Command
	// 1. Override.
	override, err := d.loadOverride()
	if err != nil {
		return nil, err
	}
	for _, c := range override.Commands {
		c.Source = "override"
		c.Stack = string(stack)
		if c.Cwd == "" {
			c.Cwd = d.Root
		}
		cmds = append(cmds, c)
	}
	// 2. Manifest detection (package.json scripts.test, etc).
	if manifest := d.detectManifest(stack); manifest != nil {
		manifest.Source = "manifest"
		manifest.Stack = string(stack)
		if manifest.Cwd == "" {
			manifest.Cwd = d.Root
		}
		cmds = append(cmds, *manifest)
	}
	// 3. Default por stack.
	for _, def := range defaultCommands(stack, d.Root) {
		cmds = append(cmds, def)
	}
	// Dedupe por bin+args.
	cmds = dedupeCommands(cmds)
	// Ordena por source priority.
	sort.SliceStable(cmds, func(i, j int) bool {
		return sourcePriority(cmds[i].Source) < sourcePriority(cmds[j].Source)
	})
	if d.Strict && len(cmds) == 0 {
		return nil, fmt.Errorf("testexec: nenhum comando descoberto para stack %s em %s", stack, d.Root)
	}
	return cmds, nil
}

func sourcePriority(s string) int {
	switch s {
	case "override":
		return 0
	case "manifest":
		return 1
	default:
		return 2
	}
}

// loadOverride lê .solidify/test.{yaml,json} no root.
func (d *Discoverer) loadOverride() (*Override, error) {
	dir := filepath.Join(d.Root, ".solidify")
	candidates := []string{"test.json", "test.yaml", "test.yml"}
	for _, name := range candidates {
		p := filepath.Join(dir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var ov Override
		// Tenta JSON primeiro.
		if err := json.Unmarshal(data, &ov); err == nil && len(ov.Commands) > 0 {
			return &ov, nil
		}
		// YAML não tem parser nativo — fallback: parse mínimo "bin: args"
		// ou suporta só JSON. Mantém simples.
	}
	return &Override{}, nil
}

// detectManifest lê package.json scripts.test etc.
func (d *Discoverer) detectManifest(stack Stack) *Command {
	switch stack {
	case StackNode:
		return d.detectNode()
	case StackPython:
		return d.detectPython()
	case StackRuby:
		return d.detectRuby()
	}
	return nil
}

func (d *Discoverer) detectNode() *Command {
	pkg := filepath.Join(d.Root, "package.json")
	data, err := os.ReadFile(pkg)
	if err != nil {
		return nil
	}
	var pkgData struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkgData); err != nil {
		return nil
	}
	testScript, ok := pkgData.Scripts["test"]
	if !ok || testScript == "" {
		return nil
	}
	// Tenta detectar package manager.
	pm := detectPackageManager(d.Root)
	args := parseScript(testScript)
	return &Command{
		Bin:    pm,
		Args:   args,
		Reason: "package.json scripts.test",
	}
}

func (d *Discoverer) detectPython() *Command {
	for _, f := range []string{"pyproject.toml", "tox.ini", "pytest.ini", "setup.py"} {
		if _, err := os.Stat(filepath.Join(d.Root, f)); err == nil {
			bin := "pytest"
			if _, err := lookupBin("pytest"); err != nil {
				bin = "python"
				args := []string{"-m", "pytest"}
				return &Command{Bin: bin, Args: args, Reason: f}
			}
			return &Command{Bin: bin, Reason: f}
		}
	}
	return nil
}

func (d *Discoverer) detectRuby() *Command {
	for _, f := range []string{"Rakefile", "Gemfile"} {
		if _, err := os.Stat(filepath.Join(d.Root, f)); err == nil {
			if _, err := os.Stat(filepath.Join(d.Root, "spec")); err == nil {
				return &Command{Bin: "rspec", Reason: "spec/"}
			}
			return &Command{Bin: "rake", Args: []string{"test"}, Reason: f}
		}
	}
	return nil
}

// detectPackageManager: pnpm-lock.yaml > yarn.lock > package-lock.json > npm.
func detectPackageManager(root string) string {
	switch {
	case fileExists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(root, "package-lock.json")):
		return "npm"
	}
	return "npm"
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// parseScript divide "npm test" em ["test"]. Apenas pra override
// mínimo. Não é shell parser.
func parseScript(s string) []string {
	parts := strings.Fields(s)
	if len(parts) <= 1 {
		return parts
	}
	// Remove primeiro (geralmente "npm"/"yarn"/"pnpm").
	return parts[1:]
}

// defaultCommands por stack.
func defaultCommands(stack Stack, root string) []Command {
	mk := func(bin string, args []string, reason string) Command {
		return Command{Bin: bin, Args: args, Source: "default", Reason: reason, Cwd: root, Stack: string(stack)}
	}
	switch stack {
	case StackNode:
		return []Command{mk("npm", []string{"test"}, "default-node")}
	case StackPython:
		return []Command{mk("pytest", nil, "default-python")}
	case StackGo:
		return []Command{mk("go", []string{"test", "./..."}, "default-go")}
	case StackJava:
		return []Command{
			mk("mvn", []string{"test"}, "default-mvn"),
			mk("gradle", []string{"test"}, "default-gradle"),
		}
	case StackRust:
		return []Command{mk("cargo", []string{"test"}, "default-rust")}
	case StackRuby:
		return []Command{mk("bundle", []string{"exec", "rake", "test"}, "default-ruby")}
	case StackDotnet:
		return []Command{mk("dotnet", []string{"test"}, "default-dotnet")}
	}
	return nil
}

// DetectStack detecta stack via arquivos no root.
func DetectStack(root string) Stack {
	markers := []struct {
		file  string
		stack Stack
	}{
		{"go.mod", StackGo},
		{"Cargo.toml", StackRust},
		{"package.json", StackNode},
		{"pyproject.toml", StackPython},
		{"requirements.txt", StackPython},
		{"setup.py", StackPython},
		{"pom.xml", StackJava},
		{"build.gradle", StackJava},
		{"Gemfile", StackRuby},
		{"*.csproj", StackDotnet},
		{"*.sln", StackDotnet},
	}
	for _, m := range markers {
		if strings.Contains(m.file, "*") {
			matches, _ := filepath.Glob(filepath.Join(root, m.file))
			if len(matches) > 0 {
				return m.stack
			}
		} else {
			if _, err := os.Stat(filepath.Join(root, m.file)); err == nil {
				return m.stack
			}
		}
	}
	return StackUnknown
}

// dedupeCommands remove duplicatas por bin+args.
func dedupeCommands(cmds []Command) []Command {
	seen := make(map[string]bool)
	out := make([]Command, 0, len(cmds))
	for _, c := range cmds {
		key := strings.Join(append([]string{c.Bin}, c.Args...), " ")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// lookupBin tenta achar binário (não fatal).
func lookupBin(name string) (string, error) {
	paths := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, dir := range paths {
		full := filepath.Join(dir, name)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full, nil
		}
	}
	return "", fmt.Errorf("not found: %s", name)
}
