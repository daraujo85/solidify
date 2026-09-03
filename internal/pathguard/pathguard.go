// Path guard (SAI-101).
//
// Valida que paths não fogem do repo root ou dos
// allowed roots. Defesa contra path traversal (../../)
// e leitura fora de escopo.
package pathguard

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrOutsideRepo path fora do repo root.
var ErrOutsideRepo = errors.New("pathguard: fora do repo")

// ErrTraversal tentativa de traversal.
var ErrTraversal = errors.New("pathguard: path traversal detectado")

// ErrAbsolute path absoluto (não permitido).
var ErrAbsolute = errors.New("pathguard: path absoluto")

// ErrEmpty path vazio.
var ErrEmpty = errors.New("pathguard: path vazio")

// ErrNotAllowed root não está na allowlist.
var ErrNotAllowed = errors.New("pathguard: root não permitido")

// Guard validador.
type Guard struct {
	RepoRoot     string
	AllowedRoots []string
}

// NewGuard cria guard.
func NewGuard(repoRoot string, allowed []string) *Guard {
	return &Guard{RepoRoot: repoRoot, AllowedRoots: allowed}
}

// Validate checa path.
func (g *Guard) Validate(path string) error {
	if path == "" {
		return ErrEmpty
	}
	// Rejeita traversal explícito.
	if strings.Contains(path, "..") {
		return ErrTraversal
	}
	// Rejeita absoluto (deve ser relativo ao repo).
	if filepath.IsAbs(path) {
		return ErrAbsolute
	}
	// Resolve dentro do repo.
	clean := filepath.Clean(path)
	if g.RepoRoot != "" {
		full := filepath.Join(g.RepoRoot, clean)
		rel, err := filepath.Rel(g.RepoRoot, full)
		if err != nil || strings.HasPrefix(rel, "..") {
			return ErrOutsideRepo
		}
	}
	// Verifica allowed roots (se houver).
	if len(g.AllowedRoots) > 0 {
		ok := false
		for _, r := range g.AllowedRoots {
			if strings.HasPrefix(clean, r) {
				ok = true
				break
			}
		}
		if !ok {
			return ErrNotAllowed
		}
	}
	return nil
}

// SafeJoin une path ao repo root após validar.
func (g *Guard) SafeJoin(path string) (string, error) {
	if err := g.Validate(path); err != nil {
		return "", err
	}
	return filepath.Join(g.RepoRoot, path), nil
}

// IsSafe helper estático — checa apenas traversal/absolute.
func IsSafe(path string) bool {
	if path == "" {
		return false
	}
	if filepath.IsAbs(path) {
		return false
	}
	if strings.Contains(path, "..") {
		return false
	}
	return true
}
