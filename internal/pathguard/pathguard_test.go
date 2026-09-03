package pathguard

import (
	"errors"
	"testing"
)

// Aceitação: path vazio.
func TestEmpty(t *testing.T) {
	g := NewGuard("/repo", nil)
	if err := g.Validate(""); err != ErrEmpty {
		t.Errorf("empty: %v", err)
	}
}

// Aceitação: path traversal.
func TestTraversal(t *testing.T) {
	g := NewGuard("/repo", nil)
	for _, p := range []string{"../etc/passwd", "a/../../b", "x/../y"} {
		if err := g.Validate(p); err != ErrTraversal {
			t.Errorf("%s: %v", p, err)
		}
	}
}

// Aceitação: path absoluto.
func TestAbsolute(t *testing.T) {
	g := NewGuard("/repo", nil)
	for _, p := range []string{"/etc/passwd", "/repo/a"} {
		if err := g.Validate(p); err != ErrAbsolute {
			t.Errorf("%s: %v", p, err)
		}
	}
}

// Aceitação: path válido.
func TestValid(t *testing.T) {
	g := NewGuard("/repo", nil)
	for _, p := range []string{"src/main.go", "README.md", "a/b/c.txt"} {
		if err := g.Validate(p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

// Aceitação: outside repo via join+rel.
func TestOutsideRepo(t *testing.T) {
	g := NewGuard("/repo", nil)
	if err := g.Validate("../../../etc/passwd"); err != ErrTraversal && err != ErrOutsideRepo {
		t.Errorf("escape: %v", err)
	}
}

// Aceitação: allowed roots enforcement.
func TestNotAllowed(t *testing.T) {
	g := NewGuard("/repo", []string{"src", "docs"})
	if err := g.Validate("src/foo.go"); err != nil {
		t.Errorf("src: %v", err)
	}
	if err := g.Validate("bin/secret"); err != ErrNotAllowed {
		t.Errorf("bin not allowed: %v", err)
	}
}

// Aceitação: allowed roots sem restrição.
func TestNoAllowedRoots(t *testing.T) {
	g := NewGuard("/repo", nil)
	if err := g.Validate("anything/here"); err != nil {
		t.Errorf("no restrict: %v", err)
	}
}

// Aceitação: SafeJoin OK.
func TestSafeJoin(t *testing.T) {
	g := NewGuard("/repo", nil)
	full, err := g.SafeJoin("src/foo.go")
	if err != nil {
		t.Errorf("join: %v", err)
	}
	if full != "/repo/src/foo.go" {
		t.Errorf("join result: %s", full)
	}
}

// Aceitação: SafeJoin erro.
func TestSafeJoinErr(t *testing.T) {
	g := NewGuard("/repo", nil)
	if _, err := g.SafeJoin(""); !errors.Is(err, ErrEmpty) {
		t.Errorf("expected empty: %v", err)
	}
}

// Aceitação: IsSafe helper.
func TestIsSafe(t *testing.T) {
	if !IsSafe("src/main.go") {
		t.Errorf("safe")
	}
	if IsSafe("../etc") {
		t.Errorf("traversal")
	}
	if IsSafe("/abs") {
		t.Errorf("abs")
	}
	if IsSafe("") {
		t.Errorf("empty")
	}
}
