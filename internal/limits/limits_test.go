package limits

import "testing"

// Aceitação: EffectiveConcurrency heavy → 1.
func TestEffectiveConcurrencyHeavy(t *testing.T) {
	p := StandardProfiles["heavy"]
	if EffectiveConcurrency(p) != 1 {
		t.Errorf("heavy: %d", EffectiveConcurrency(p))
	}
}

// Aceitação: EffectiveConcurrency light → 4.
func TestEffectiveConcurrencyLight(t *testing.T) {
	p := StandardProfiles["light"]
	if EffectiveConcurrency(p) != 4 {
		t.Errorf("light: %d", EffectiveConcurrency(p))
	}
}

// Aceitação: EffectiveConcurrency zero → 1.
func TestEffectiveConcurrencyZero(t *testing.T) {
	p := Profile{Concurrency: 0}
	if EffectiveConcurrency(p) != 1 {
		t.Errorf("zero")
	}
}

// Aceitação: Resolve tool conhecido.
func TestResolveK6(t *testing.T) {
	p, ok := Resolve("k6")
	if !ok {
		t.Fatal("not found")
	}
	if p.Name != "k6" || !p.Heavy {
		t.Errorf("k6: %+v", p)
	}
}

// Aceitação: Resolve tool desconhecido.
func TestResolveUnknown(t *testing.T) {
	if _, ok := Resolve("nope"); ok {
		t.Errorf("found")
	}
}

// Aceitação: ComposeFragment contém campos esperados.
func TestComposeFragment(t *testing.T) {
	p := StandardProfiles["heavy"]
	frag := ComposeFragment(p)
	for _, s := range []string{"cpus:", "mem_limit:", "concurrency:", "heavy:"} {
		if !contains(frag, s) {
			t.Errorf("missing %s in: %s", s, frag)
		}
	}
}

func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// Aceitação: ValidateProfile OK.
func TestValidateOK(t *testing.T) {
	p := StandardProfiles["medium"]
	if err := ValidateProfile(p); err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: ValidateProfile CPU vazio.
func TestValidateNoCPU(t *testing.T) {
	p := StandardProfiles["light"]
	p.CPU = ""
	if err := ValidateProfile(p); err == nil {
		t.Errorf("no cpu")
	}
}

// Aceitação: ValidateProfile Memory vazio.
func TestValidateNoMem(t *testing.T) {
	p := StandardProfiles["light"]
	p.Memory = ""
	if err := ValidateProfile(p); err == nil {
		t.Errorf("no mem")
	}
}

// Aceitação: ValidateProfile heavy com concurrency > 1.
func TestValidateHeavyBad(t *testing.T) {
	p := Profile{CPU: "1", Memory: "1g", Heavy: true, Concurrency: 2}
	if err := ValidateProfile(p); err == nil {
		t.Errorf("heavy bad")
	}
}

// Aceitação: itoaL helper.
func TestItoaL(t *testing.T) {
	if itoaL(0) != "0" {
		t.Errorf("0")
	}
	if itoaL(42) != "42" {
		t.Errorf("42")
	}
	if itoaL(-7) != "-7" {
		t.Errorf("-7")
	}
}
