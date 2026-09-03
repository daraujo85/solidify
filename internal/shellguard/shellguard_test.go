package shellguard

import "testing"

// Aceitação: arg vazio.
func TestArgEmpty(t *testing.T) {
	if err := ValidateArg(""); err != ErrEmptyArg {
		t.Errorf("empty: %v", err)
	}
}

// Aceitação: arg válido.
func TestArgValid(t *testing.T) {
	for _, a := range []string{"hello", "feat(api): add /users", "fix-bug", "abc123"} {
		if err := ValidateArg(a); err != nil {
			t.Errorf("%s: %v", a, err)
		}
	}
}

// Aceitação: injeção via dangerous chars.
func TestArgDangerousChars(t *testing.T) {
	for _, a := range []string{
		"foo;rm -rf /",
		"foo&&bar",
		"foo|grep",
		"foo`whoami`",
		"foo$(id)",
		"foo>bar",
		"foo<bar",
		"foo\nrm",
		"foo\\bar",
	} {
		if err := ValidateArg(a); err != ErrInjection {
			t.Errorf("%s: %v", a, err)
		}
	}
}

// Aceitação: shell metachars.
func TestArgShellMetachars(t *testing.T) {
	for _, a := range []string{
		"foo&&bar",
		"foo||bar",
		"foo>>bar",
		"foo<<bar",
		"foo$(id)",
		"foo${HOME}",
	} {
		if err := ValidateArg(a); err != ErrInjection {
			t.Errorf("%s: %v", a, err)
		}
	}
}

// Aceitação: ValidateArgs múltiplos.
func TestArgsMulti(t *testing.T) {
	args := []string{"git", "log", "--oneline"}
	if err := ValidateArgs(args); err != nil {
		t.Errorf("multi: %v", err)
	}
	args = []string{"git", "log; rm -rf /"}
	if err := ValidateArgs(args); err != ErrInjection {
		t.Errorf("multi bad: %v", err)
	}
}

// Aceitação: SanitizeSubject válido.
func TestSubjectOK(t *testing.T) {
	if err := SanitizeSubject("feat(api): add user endpoint"); err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: SanitizeSubject vazio.
func TestSubjectEmpty(t *testing.T) {
	if err := SanitizeSubject(""); err != ErrEmptyArg {
		t.Errorf("empty: %v", err)
	}
}

// Aceitação: SanitizeSubject injection.
func TestSubjectInj(t *testing.T) {
	if err := SanitizeSubject("feat; rm -rf /"); err != ErrInjection {
		t.Errorf("inj: %v", err)
	}
	if err := SanitizeSubject("feat\nrm -rf /"); err != ErrInjection {
		t.Errorf("newline: %v", err)
	}
}

// Aceitação: SanitizeSubject > 200.
func TestSubjectLong(t *testing.T) {
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	if err := SanitizeSubject(string(long)); err == nil {
		t.Errorf("long allowed")
	}
}

// Aceitação: SanitizePath válido.
func TestPathOK(t *testing.T) {
	if err := SanitizePath("src/main.go"); err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: SanitizePath traversal.
func TestPathTraversal(t *testing.T) {
	if err := SanitizePath("../etc/passwd"); err != ErrInjection {
		t.Errorf("traversal: %v", err)
	}
}

// Aceitação: SanitizePath dangerous chars.
func TestPathDangerous(t *testing.T) {
	for _, p := range []string{"foo;rm", "foo|grep", "foo`id`"} {
		if err := SanitizePath(p); err != ErrInjection {
			t.Errorf("%s: %v", p, err)
		}
	}
}

// Aceitação: HasMetachar helper.
func TestHasMetachar(t *testing.T) {
	if !HasMetachar("foo;bar") {
		t.Errorf("detected")
	}
	if HasMetachar("foo") {
		t.Errorf("false positive")
	}
}
