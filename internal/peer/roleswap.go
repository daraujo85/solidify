// Role-swap runner (SAI-093).
//
// Executa pipeline com Y como Peer B e X como Arbiter.
// Mantém isolamento de contexto entre runs: cada ator
// recebe seu próprio snapshot de evidence.
package peer

import (
	"errors"
	"fmt"
	"time"
)

// Role enum.
type Role string

const (
	RolePeerA   Role = "peer_a"
	RolePeerB   Role = "peer_b"
	RoleArbiter Role = "arbiter_c"
)

// ActorAssignment qual modelo/provider em qual role.
type ActorAssignment struct {
	Role     Role
	Provider string
	Model    string
}

// Assignment plan.
type Assignment struct {
	Original []ActorAssignment // baseline: X=PeerA, Y=PeerB, Z=Arbiter
	Swapped  []ActorAssignment // swap: Y=PeerB, X=Arbiter
}

// ErrInvalidAssignment assignment vazio.
var ErrInvalidAssignment = errors.New("roleswap: assignment vazio")

// ErrDuplicateRole duas actors na mesma role.
var ErrDuplicateRole = errors.New("roleswap: role duplicada")

// ErrMissingArbiter sem arbiter no assignment.
var ErrMissingArbiter = errors.New("roleswap: sem arbiter")

// ErrInsufficientPeers menos de 2 peers.
var ErrInsufficientPeers = errors.New("roleswap: < 2 peers")

// ValidateAssignment checa consistência.
func ValidateAssignment(a []ActorAssignment) error {
	if len(a) == 0 {
		return ErrInvalidAssignment
	}
	peers := 0
	roles := map[Role]bool{}
	for _, x := range a {
		if x.Role == "" || x.Provider == "" || x.Model == "" {
			return fmt.Errorf("roleswap: actor incompleto: %+v", x)
		}
		if roles[x.Role] {
			return fmt.Errorf("%w: %s", ErrDuplicateRole, x.Role)
		}
		roles[x.Role] = true
		if x.Role == RolePeerA || x.Role == RolePeerB {
			peers++
		}
	}
	if peers < 2 {
		return ErrInsufficientPeers
	}
	if !roles[RoleArbiter] {
		return ErrMissingArbiter
	}
	return nil
}

// IsolatedContext contexto isolado por actor — não vazou
// entre runs.
type IsolatedContext struct {
	RunID     string
	ActorRole Role
	Evidence  string // copy-on-read
	StartedAt time.Time
}

// NewIsolatedContext cria contexto isolado.
func NewIsolatedContext(runID string, role Role, evidence string) *IsolatedContext {
	cp := make([]byte, len(evidence))
	copy(cp, evidence)
	return &IsolatedContext{
		RunID:     runID,
		ActorRole: role,
		Evidence:  string(cp),
		StartedAt: time.Now().UTC(),
	}
}

// EvidenceAt retorna view do evidence no tempo t.
// Importante: t dentro do contexto (não-global).
func (c *IsolatedContext) EvidenceAt(t time.Time) string {
	// cópia rasa; caller não muta original.
	cp := make([]byte, len(c.Evidence))
	copy(cp, c.Evidence)
	_ = t // evidência não muda dentro do run; placeholder p/ futuro.
	return string(cp)
}

// RoleSwapResult saída.
type RoleSwapResult struct {
	RunID       string             `json:"run_id"`
	Original    []ActorAssignment  `json:"original"`
	Swapped     []ActorAssignment  `json:"swapped"`
	Contexts    []*IsolatedContext `json:"contexts"`
	GeneratedAt time.Time          `json:"generated_at"`
	Hash        string             `json:"hash"`
}

// BuildSwap cria assignment swapped a partir do original.
// Regra: PeerB vira Arbiter (e vice-versa), PeerA
// mantém. Y como Peer B (original) e X como Arbiter
// (swapped) é a forma canônica do projeto.
func BuildSwap(orig []ActorAssignment) ([]ActorAssignment, error) {
	if err := ValidateAssignment(orig); err != nil {
		return nil, err
	}
	out := make([]ActorAssignment, 0, len(orig))
	for _, a := range orig {
		switch a.Role {
		case RolePeerA:
			out = append(out, a)
		case RolePeerB:
			// PeerB vira Arbiter.
			out = append(out, ActorAssignment{
				Role:     RoleArbiter,
				Provider: a.Provider,
				Model:    a.Model,
			})
		case RoleArbiter:
			// Arbiter vira PeerB.
			out = append(out, ActorAssignment{
				Role:     RolePeerB,
				Provider: a.Provider,
				Model:    a.Model,
			})
		}
	}
	if err := ValidateAssignment(out); err != nil {
		return nil, fmt.Errorf("swap inválido: %w", err)
	}
	return out, nil
}

// ExecuteSwap roda pipeline com isolamento.
//
// `evidence` é o snapshot original (string). Cada actor
// recebe cópia independente via `NewIsolatedContext`.
func ExecuteSwap(runID string, original, swapped []ActorAssignment, evidence string) (*RoleSwapResult, error) {
	if runID == "" {
		return nil, errors.New("roleswap: runID vazio")
	}
	if err := ValidateAssignment(original); err != nil {
		return nil, fmt.Errorf("original: %w", err)
	}
	if err := ValidateAssignment(swapped); err != nil {
		return nil, fmt.Errorf("swapped: %w", err)
	}
	contexts := []*IsolatedContext{}
	for _, a := range swapped {
		contexts = append(contexts, NewIsolatedContext(runID, a.Role, evidence))
	}
	return &RoleSwapResult{
		RunID:       runID,
		Original:    original,
		Swapped:     swapped,
		Contexts:    contexts,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// ActorsByRole helper.
func ActorsByRole(a []ActorAssignment, r Role) (ActorAssignment, bool) {
	for _, x := range a {
		if x.Role == r {
			return x, true
		}
	}
	return ActorAssignment{}, false
}

// HashAssignment SHA-like determinístico (sem crypto).
func HashAssignment(a []ActorAssignment) string {
	out := ""
	for i, x := range a {
		if i > 0 {
			out += "|"
		}
		out += string(x.Role) + ":" + x.Provider + "/" + x.Model
	}
	return out
}
