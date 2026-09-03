package ninesmoke

import "testing"

// Aceitação: Health com URL vazia falha.
func TestHealthEmptyURL(t *testing.T) {
	g := NewGateway("", "")
	if _, err := g.Health(); err == nil {
		t.Errorf("expected error")
	}
}

// Aceitação: Health OK.
func TestHealthOK(t *testing.T) {
	g := NewGateway("http://localhost:20128", "token")
	h, err := g.Health()
	if err != nil {
		t.Errorf("health: %v", err)
	}
	if !h.Online {
		t.Errorf("offline")
	}
}

// Aceitação: ResolveCombo OK.
func TestResolveComboOK(t *testing.T) {
	g := NewGateway("http://localhost", "x")
	g.RegisterCombo("test", []string{"m1", "m2"})
	m, err := g.ResolveCombo("test", nil)
	if err != nil {
		t.Errorf("resolve: %v", err)
	}
	if m != "m1" {
		t.Errorf("expected m1: %s", m)
	}
}

// Aceitação: ResolveCombo missing.
func TestResolveComboMissing(t *testing.T) {
	g := NewGateway("http://localhost", "x")
	if _, err := g.ResolveCombo("nope", nil); err == nil {
		t.Errorf("expected error")
	}
}

// Aceitação: ResolveCombo exhausted.
func TestResolveComboExhausted(t *testing.T) {
	g := NewGateway("http://localhost", "x")
	g.RegisterCombo("tiny", []string{"m1"})
	if _, err := g.ResolveCombo("tiny", []string{"m1"}); err == nil {
		t.Errorf("expected exhausted")
	}
}

// Aceitação: Fallback pula failed.
func TestFallback(t *testing.T) {
	g := NewGateway("http://localhost", "x")
	g.RegisterCombo("c", []string{"m1", "m2", "m3"})
	m, err := g.ResolveCombo("c", []string{"m1"})
	if err != nil {
		t.Errorf("fb: %v", err)
	}
	if m != "m2" {
		t.Errorf("expected m2: %s", m)
	}
}

// Aceitação: RunSmoke completo.
func TestRunSmokeOK(t *testing.T) {
	g := NewGateway("http://localhost", "x")
	g.RegisterCombo("c", []string{"m1", "m2"})
	r := g.RunSmoke("c")
	if !r.Passed {
		t.Errorf("smoke: %+v", r)
	}
}

// Aceitação: RunSmoke falha se combo missing.
func TestRunSmokeBadCombo(t *testing.T) {
	g := NewGateway("http://localhost", "x")
	r := g.RunSmoke("nope")
	if r.Passed {
		t.Errorf("expected fail")
	}
	if len(r.Reasons) == 0 {
		t.Errorf("reasons vazio")
	}
}

// Aceitação: contains helper.
func TestContains(t *testing.T) {
	if !contains([]string{"a", "b"}, "a") {
		t.Errorf("yes")
	}
	if contains([]string{"a", "b"}, "c") {
		t.Errorf("no")
	}
	if contains(nil, "x") {
		t.Errorf("nil")
	}
}
