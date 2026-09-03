// Package stack detecta stacks (linguagens/runtimes) a partir dos
// manifests descobertos. Determinístico, sem IA.
//
// Cada kind de manifest vota num stack. Empate vai pro stack com
// mais votos; desempate final usa a ordem canônica
// (Node > Go > Python > Java > Rust > PHP > Ruby > Flutter > .NET).
//
// Output: stack + confidence (0..1) + evidence (paths que justificam).
package stack

import (
	"sort"

	"github.com/diegoaraujo/solidify/internal/manifests"
)

// Stack é o runtime/linguagem detectado.
type Stack string

const (
	StackNode    Stack = "node"
	StackGo      Stack = "go"
	StackPython  Stack = "python"
	StackJava    Stack = "java"
	StackRust    Stack = "rust"
	StackPHP     Stack = "php"
	StackRuby    Stack = "ruby"
	StackFlutter Stack = "flutter"
	StackDotnet  Stack = "dotnet"
)

// Detection é uma detecção de stack com confiança e evidência.
type Detection struct {
	Stack      Stack
	Confidence float64 // 0.0 a 1.0
	Evidence   []string
}

// voteStrength é o peso de cada Kind na votação. Lockfiles (yarn.lock,
// pnpm-lock.yaml) sozinhos não provam o stack — valem 0.5. Um único
// manifest "forte" (package.json, go.mod) já vale 1.0. Glob matches
// (.csproj) também valem 1.0.
var kindToStack = map[manifests.Kind]struct {
	stack  Stack
	weight float64
}{
	manifests.KindNodePackageJSON:    {StackNode, 1.0},
	manifests.KindNodeYarnLock:       {StackNode, 0.5},
	manifests.KindNodePnpmLock:       {StackNode, 0.5},
	manifests.KindGoMod:              {StackGo, 1.0},
	manifests.KindPythonPyproject:    {StackPython, 1.0},
	manifests.KindPythonRequirements: {StackPython, 0.7},
	manifests.KindRustCargo:          {StackRust, 1.0},
	manifests.KindJavaPom:            {StackJava, 1.0},
	manifests.KindJavaGradle:         {StackJava, 1.0},
	manifests.KindPHPComposer:        {StackPHP, 1.0},
	manifests.KindRubyGemfile:        {StackRuby, 1.0},
	manifests.KindFlutterPubspec:     {StackFlutter, 1.0},
	manifests.KindElixirMix:          {StackRuby, 0.3}, // Elixir é runtime próprio, mas "vive" no ecossistema Ruby-like
	manifests.KindDotnetCsproj:       {StackDotnet, 1.0},
	manifests.KindDotnetFsproj:       {StackDotnet, 1.0},
	manifests.KindDotnetSln:          {StackDotnet, 0.5},
	manifests.KindDotnetGlobalJson:   {StackDotnet, 0.3},
}

// Detect devolve uma detecção por stack presente nos manifests.
// Ordenado por confiança decrescente, depois por tiebreak canônico.
func Detect(ms []manifests.Manifest) []Detection {
	scores := make(map[Stack]float64)
	evidence := make(map[Stack][]string)
	for _, m := range ms {
		v, ok := kindToStack[m.Kind]
		if !ok {
			continue
		}
		scores[v.stack] += v.weight
		evidence[v.stack] = append(evidence[v.stack], m.Path)
	}

	out := make([]Detection, 0, len(scores))
	for st, sc := range scores {
		conf := sc
		if conf > 1.0 {
			conf = 1.0
		}
		ev := evidence[st]
		sort.Strings(ev)
		out = append(out, Detection{
			Stack:      st,
			Confidence: round2(conf),
			Evidence:   ev,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return stackOrder(out[i].Stack) < stackOrder(out[j].Stack)
	})
	return out
}

// round2 trunca para 2 casas decimais (sem arredondar).
func round2(f float64) float64 {
	return float64(int(f*100)) / 100
}

// stackOrder define a ordem canônica para tiebreak.
func stackOrder(s Stack) int {
	switch s {
	case StackNode:
		return 0
	case StackGo:
		return 1
	case StackPython:
		return 2
	case StackJava:
		return 3
	case StackRust:
		return 4
	case StackPHP:
		return 5
	case StackRuby:
		return 6
	case StackFlutter:
		return 7
	case StackDotnet:
		return 8
	default:
		return 99
	}
}

// Primary devolve a stack com maior confiança, ou vazio se nada detectado.
func Primary(d []Detection) (Stack, bool) {
	if len(d) == 0 {
		return "", false
	}
	return d[0].Stack, true
}
