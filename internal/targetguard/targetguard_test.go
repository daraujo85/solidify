package targetguard

import "testing"

// Aceitação: mode não-perigoso sempre passa.
func TestSafeModePasses(t *testing.T) {
	g := NewGuard([]string{"localhost"})
	if err := g.Validate("anything.com", Mode("lint")); err != nil {
		t.Errorf("safe: %v", err)
	}
}

// Aceitação: localhost permitido.
func TestLocalhostAllowed(t *testing.T) {
	g := NewGuard([]string{"localhost", "127.0.0.1"})
	if err := g.Validate("localhost", ModeZAPActive); err != nil {
		t.Errorf("localhost: %v", err)
	}
	if err := g.Validate("127.0.0.1", ModeK6); err != nil {
		t.Errorf("127: %v", err)
	}
}

// Aceitação: host vazio.
func TestEmptyHost(t *testing.T) {
	g := NewGuard([]string{"localhost"})
	if err := g.Validate("", ModeZAPActive); err != ErrNotAllowed {
		t.Errorf("empty: %v", err)
	}
}

// Aceitação: host não permitido.
func TestNotAllowed(t *testing.T) {
	g := NewGuard([]string{"localhost"})
	if err := g.Validate("prod.example.com", ModeZAPActive); err != ErrNotAllowed {
		t.Errorf("not allowed: %v", err)
	}
}

// Aceitação: produção bloqueada via heurística.
func TestProductionHeuristic(t *testing.T) {
	g := NewGuard([]string{"*.amazonaws.com"})
	for _, host := range []string{"ec2.amazonaws.com", "myapp.azure.com", "api.prod.example.com"} {
		if err := g.Validate(host, ModeK6); err != ErrProduction {
			t.Errorf("%s: %v", host, err)
		}
	}
}

// Aceitação: k6 = dangerous.
func TestK6Dangerous(t *testing.T) {
	g := NewGuard([]string{"localhost"})
	if err := g.Validate("localhost", ModeK6); err != nil {
		t.Errorf("k6 localhost: %v", err)
	}
	if err := g.Validate("other.com", ModeK6); err != ErrNotAllowed {
		t.Errorf("k6 other: %v", err)
	}
}

// Aceitação: IsDangerous helper.
func TestIsDangerous(t *testing.T) {
	if !IsDangerous(ModeZAPActive) {
		t.Errorf("zap")
	}
	if !IsDangerous(ModeK6) {
		t.Errorf("k6")
	}
	if !IsDangerous(ModeDestruct) {
		t.Errorf("destruct")
	}
	if IsDangerous(Mode("lint")) {
		t.Errorf("lint dangerous")
	}
}

// Aceitação: CheckProductionOnly.
func TestCheckProductionOnly(t *testing.T) {
	if !CheckProductionOnly("api.amazonaws.com") {
		t.Errorf("aws")
	}
	if !CheckProductionOnly("myapp.azure.com") {
		t.Errorf("azure")
	}
	if CheckProductionOnly("localhost") {
		t.Errorf("localhost")
	}
}

// Aceitação: case-insensitive allowlist.
func TestCaseInsensitive(t *testing.T) {
	g := NewGuard([]string{"LocalHost"})
	if err := g.Validate("LOCALHOST", ModeZAPActive); err != nil {
		t.Errorf("case: %v", err)
	}
}
