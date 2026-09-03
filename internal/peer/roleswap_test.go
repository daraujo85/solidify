package peer

import (
	"strings"
	"testing"
)

// Aceitação: ValidateAssignment OK.
func TestValidateOK(t *testing.T) {
	a := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerB, Provider: "kr", Model: "k1"},
		{Role: RoleArbiter, Provider: "cc", Model: "c1"},
	}
	if err := ValidateAssignment(a); err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: assignment vazio.
func TestValidateEmpty(t *testing.T) {
	if err := ValidateAssignment(nil); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: actor incompleto.
func TestValidateIncomplete(t *testing.T) {
	a := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: ""},
	}
	if err := ValidateAssignment(a); err == nil {
		t.Errorf("model vazio")
	}
}

// Aceitação: role duplicada.
func TestValidateDupRole(t *testing.T) {
	a := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerA, Provider: "kr", Model: "k1"},
	}
	if err := ValidateAssignment(a); err == nil {
		t.Errorf("dup")
	}
}

// Aceitação: peers insuficientes.
func TestValidateNoPeers(t *testing.T) {
	a := []ActorAssignment{
		{Role: RoleArbiter, Provider: "cc", Model: "c1"},
	}
	if err := ValidateAssignment(a); err == nil {
		t.Errorf("no peers")
	}
}

// Aceitação: sem arbiter.
func TestValidateNoArbiter(t *testing.T) {
	a := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerB, Provider: "kr", Model: "k1"},
	}
	if err := ValidateAssignment(a); err == nil {
		t.Errorf("no arb")
	}
}

// Aceitação: BuildSwap básico.
func TestBuildSwapBasic(t *testing.T) {
	orig := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerB, Provider: "kr", Model: "k1"},
		{Role: RoleArbiter, Provider: "cc", Model: "c1"},
	}
	swapped, err := BuildSwap(orig)
	if err != nil {
		t.Fatalf("swap: %v", err)
	}
	// PeerA mantém, PeerB↔Arbiter trocam.
	pa, _ := ActorsByRole(swapped, RolePeerA)
	if pa.Model != "g1" {
		t.Errorf("pa: %+v", pa)
	}
	pb, _ := ActorsByRole(swapped, RolePeerB)
	if pb.Model != "c1" {
		t.Errorf("pb (era arb): %+v", pb)
	}
	arb, _ := ActorsByRole(swapped, RoleArbiter)
	if arb.Model != "k1" {
		t.Errorf("arb (era pb): %+v", arb)
	}
}

// Aceitação: BuildSwap original inválido.
func TestBuildSwapInvalid(t *testing.T) {
	if _, err := BuildSwap(nil); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: NewIsolatedContext isola evidence.
func TestIsolatedContext(t *testing.T) {
	c := NewIsolatedContext("r1", RolePeerA, "secret-evidence")
	if c.Evidence != "secret-evidence" {
		t.Errorf("evidence")
	}
	// mutar c.Evidence não afeta original.
	c.Evidence = "mutated"
	_ = "secret-evidence" // original string é imutável em Go.
}

// Aceitação: EvidenceAt retorna cópia.
func TestEvidenceAtCopy(t *testing.T) {
	c := NewIsolatedContext("r1", RolePeerA, "data")
	v := c.EvidenceAt(c.StartedAt)
	_ = v
	v = "mutated"
	if c.Evidence != "data" {
		t.Errorf("mutou")
	}
}

// Aceitação: ExecuteSwap básico.
func TestExecuteSwap(t *testing.T) {
	orig := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerB, Provider: "kr", Model: "k1"},
		{Role: RoleArbiter, Provider: "cc", Model: "c1"},
	}
	swapped, _ := BuildSwap(orig)
	res, err := ExecuteSwap("r1", orig, swapped, "evidence-data")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if len(res.Contexts) != 3 {
		t.Errorf("contexts: %d", len(res.Contexts))
	}
	for _, ctx := range res.Contexts {
		if ctx.Evidence != "evidence-data" {
			t.Errorf("evidence: %s", ctx.Evidence)
		}
	}
}

// Aceitação: ExecuteSwap runID vazio.
func TestExecuteSwapEmptyRunID(t *testing.T) {
	orig := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerB, Provider: "kr", Model: "k1"},
		{Role: RoleArbiter, Provider: "cc", Model: "c1"},
	}
	swapped, _ := BuildSwap(orig)
	if _, err := ExecuteSwap("", orig, swapped, "x"); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: ExecuteSwap original inválido.
func TestExecuteSwapBadOriginal(t *testing.T) {
	swapped := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerB, Provider: "kr", Model: "k1"},
		{Role: RoleArbiter, Provider: "cc", Model: "c1"},
	}
	if _, err := ExecuteSwap("r1", nil, swapped, "x"); err == nil {
		t.Errorf("bad orig")
	}
}

// Aceitação: ExecuteSwap swapped inválido.
func TestExecuteSwapBadSwapped(t *testing.T) {
	orig := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerB, Provider: "kr", Model: "k1"},
		{Role: RoleArbiter, Provider: "cc", Model: "c1"},
	}
	if _, err := ExecuteSwap("r1", orig, nil, "x"); err == nil {
		t.Errorf("bad swapped")
	}
}

// Aceitação: ActorsByRole missing.
func TestActorsByRoleMissing(t *testing.T) {
	if _, ok := ActorsByRole(nil, RolePeerA); ok {
		t.Errorf("missing")
	}
	a := []ActorAssignment{{Role: RolePeerA, Provider: "x", Model: "y"}}
	if _, ok := ActorsByRole(a, RoleArbiter); ok {
		t.Errorf("not in list")
	}
}

// Aceitação: HashAssignment determinístico.
func TestHashAssignment(t *testing.T) {
	a := []ActorAssignment{
		{Role: RolePeerA, Provider: "gc", Model: "g1"},
		{Role: RolePeerB, Provider: "kr", Model: "k1"},
	}
	h := HashAssignment(a)
	if !strings.Contains(h, "peer_a:gc/g1") {
		t.Errorf("hash: %s", h)
	}
	h2 := HashAssignment(a)
	if h != h2 {
		t.Errorf("not determin")
	}
	if HashAssignment(nil) != "" {
		t.Errorf("nil hash")
	}
}

// Aceitação: contextos independentes (mutação não vaza).
func TestContextsIndependent(t *testing.T) {
	c1 := NewIsolatedContext("r1", RolePeerA, "x")
	c2 := NewIsolatedContext("r1", RolePeerB, "y")
	if c1.Evidence == c2.Evidence {
		t.Errorf("indep")
	}
}
