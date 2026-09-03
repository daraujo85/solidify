// Package envx — comparador base vs head para env vars.
//
// Implementa §11.3 (states) e §11.4 (metadata):
//
//	added / removed / changed-use
//	documentation-added / documentation-missing
//	required (sem default) / optional (com default)
//	likely_secret (heurística de nome)
//
// Não carrega valores (§11.1).
package envx

import (
	"regexp"
	"sort"
	"strings"
)

// DiffState é o estado da env var no range.
type DiffState string

const (
	DiffAdded      DiffState = "added"
	DiffRemoved    DiffState = "removed"
	DiffChangedUse DiffState = "changed-use"
	DiffDocAdded   DiffState = "documentation-added"
	DiffDocMissing DiffState = "documentation-missing"
)

// Diff é a comparação de uma env var entre base e head.
type Diff struct {
	Name         string
	States       []DiffState
	BaseUses     int      // nº de usos no base
	HeadUses     int      // nº de usos no head
	BaseFiles    []string // paths únicos onde aparece no base
	HeadFiles    []string // paths únicos onde aparece no head
	Documented   bool     // nome presente em .env.example/.sample/etc
	HasDefault   bool     // alguma chamada no head traz default
	Required     bool     // presente no head, sem default
	LikelySecret bool     // heurística de nome
}

// Compare devolve a lista ordenada de Diffs para o conjunto union
// de nomes em base e head. secretChecker pode ser nil (sem marcar
// likely_secret). documentedNames é a lista extraída de arquivos de
// documentação (.env.example, .env.sample, application.yml, etc).
func Compare(base, head []Usage, documentedNames []string, secretChecker func(string) bool) []Diff {
	baseByName := indexByName(base)
	headByName := indexByName(head)
	docSet := make(map[string]bool, len(documentedNames))
	for _, n := range documentedNames {
		docSet[n] = true
	}

	var names []string
	seen := make(map[string]bool)
	for n := range baseByName {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for n := range headByName {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)

	out := make([]Diff, 0, len(names))
	for _, name := range names {
		b := baseByName[name]
		h := headByName[name]
		d := Diff{
			Name:      name,
			BaseUses:  len(b),
			HeadUses:  len(h),
			BaseFiles: uniqueFiles(b),
			HeadFiles: uniqueFiles(h),
		}
		inBase := len(b) > 0
		inHead := len(h) > 0
		d.Documented = docSet[name]
		d.HasDefault = anyHasDefault(h)
		d.Required = inHead && !d.HasDefault
		if secretChecker != nil {
			d.LikelySecret = secretChecker(name)
		}
		switch {
		case !inBase && inHead:
			d.States = append(d.States, DiffAdded)
		case inBase && !inHead:
			d.States = append(d.States, DiffRemoved)
		case inBase && inHead:
			if !sameUsage(b, h) {
				d.States = append(d.States, DiffChangedUse)
			}
		}
		if d.Documented {
			d.States = append(d.States, DiffDocAdded)
		} else {
			d.States = append(d.States, DiffDocMissing)
		}
		out = append(out, d)
	}
	return out
}

// DocumentedNames extrai nomes declarados em arquivos de documentação
// (.env.example, .env.sample, application.yml, etc). Aceita formato:
//
//	NAME=value           (shell, .env*)
//	export NAME=value    (shell)
//	NAME: value          (yaml/properties) — exige VALOR após ':'
//	# NAME=value         (comentado — ainda conta como documentado)
//
// Linhas que começam com '#' (comentário puro) também contam se tiverem
// a forma NAME=.
//
// Para yaml, só captura chaves que têm valor explícito — "app:" como
// namespace pai (sem valor) é ignorado.
func DocumentedNames(src []byte) []string {
	seen := make(map[string]bool)
	var out []string
	for _, line := range strings.Split(string(src), "\n") {
		trim := strings.TrimSpace(line)
		// Remove '#' inicial se for comentário.
		trim = strings.TrimPrefix(trim, "#")
		trim = strings.TrimSpace(trim)
		if trim == "" {
			continue
		}
		m := reEnvDoc.FindStringSubmatch(trim)
		if m == nil {
			continue
		}
		// Exige conteúdo após '=' ou ':' — exclui "app:" (yaml parent).
		after := trim[len(m[0]):]
		if strings.TrimSpace(after) == "" {
			continue
		}
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// IsLikelySecret devolve true se name bate heurística de nome de
// segredo (§11.4 likely_secret). Heurística: contains KEY/SECRET/TOKEN/
// PASSWORD/PASS/CREDENTIAL/PRIVATE/AUTH/CERT (case-insensitive).
func IsLikelySecret(name string) bool {
	return reSecretSuffix.MatchString(name)
}

var (
	// Aceita "export" opcional, identifier, depois '=' ou ':'.
	reEnvDoc = regexp.MustCompile(`^(?:export\s+)?([A-Za-z_][A-Za-z0-9_.\-]*)\s*[=:]`)
	// Sufixos comuns de segredo.
	reSecretSuffix = regexp.MustCompile(`(?i)(KEY|SECRET|TOKEN|PASSWORD|PASS|ACCESS|PRIVATE|CREDENTIAL|AUTH|CERT)`)
)

// indexByName agrupa Usages por Name.
func indexByName(us []Usage) map[string][]Usage {
	out := make(map[string][]Usage)
	for _, u := range us {
		out[u.Name] = append(out[u.Name], u)
	}
	return out
}

// uniqueFiles devolve os paths únicos onde Name aparece, ordenado.
func uniqueFiles(us []Usage) []string {
	seen := make(map[string]bool)
	var out []string
	for _, u := range us {
		if !seen[u.Path] {
			seen[u.Path] = true
			out = append(out, u.Path)
		}
	}
	sort.Strings(out)
	return out
}

// anyHasDefault devolve true se algum Usage traz HasDefault.
func anyHasDefault(us []Usage) bool {
	for _, u := range us {
		if u.HasDefault {
			return true
		}
	}
	return false
}

// sameUsage compara duas listas de Usage da mesma env var — verifica
// se a forma/hasDefault são as mesmas. Paths podem mudar (mudança
// trivial não é "changed-use").
func sameUsage(a, b []Usage) bool {
	if len(a) != len(b) {
		return false
	}
	// Indexa por (Form, HasDefault).
	aSet := make(map[string]bool)
	for _, u := range a {
		aSet[string(u.Form)+"|"+boolStr(u.HasDefault)] = true
	}
	for _, u := range b {
		key := string(u.Form) + "|" + boolStr(u.HasDefault)
		if !aSet[key] {
			return false
		}
	}
	return true
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
