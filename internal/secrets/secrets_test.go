package secrets

import (
	"strings"
	"testing"
)

// Aceitação: gitleaks array.
func TestParseGitleaksArray(t *testing.T) {
	data := `[
		{
			"Description":"AWS Access Key",
			"RuleID":"aws-access-token",
			"Match":"AKIA****EX",
			"Secret":"AKIA****EX",
			"File":"config/aws.go",
			"Line":42,
			"Commit":"abc123",
			"Author":"dev@example.com",
			"Entropy":4.5,
			"Tags":["aws","key"]
		}
	]`
	rep, err := ParseGitleaksJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rep.Findings) != 1 {
		t.Fatalf("len = %d", len(rep.Findings))
	}
	if rep.Findings[0].Severity != SevHigh {
		t.Errorf("severity = %v", rep.Findings[0].Severity)
	}
	if rep.Findings[0].FilePath != "config/aws.go" {
		t.Errorf("file = %s", rep.Findings[0].FilePath)
	}
	if rep.Counts.Total != 1 {
		t.Errorf("total = %d", rep.Counts.Total)
	}
}

// Aceitação: gitleaks NDJSON.
func TestParseGitleaksNDJSON(t *testing.T) {
	data := `{"Description":"Generic API Key","RuleID":"generic-api-key","File":"x.go","Line":1,"Entropy":3.5}
{"Description":"Low entropy","RuleID":"low-entropy-string","File":"y.go","Line":2,"Entropy":2.0}
`
	rep, err := ParseGitleaksJSON([]byte(data))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rep.Findings) != 2 {
		t.Errorf("len = %d", len(rep.Findings))
	}
}

// Aceitação: gitleaks NDJSON inválido pula linha.
func TestParseGitleaksNDJSONInvalid(t *testing.T) {
	data := `not json
{"Description":"OK","RuleID":"r","File":"x","Line":1,"Entropy":3.0}
also garbage`
	rep, err := ParseGitleaksJSON([]byte(data))
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if len(rep.Findings) != 1 {
		t.Errorf("devia pular inválidos: got %d", len(rep.Findings))
	}
}

// Aceitação: gitleaks array inválido.
func TestParseGitleaksArrayInvalid(t *testing.T) {
	if _, err := ParseGitleaksJSON([]byte("[invalid")); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: input vazio.
func TestParseGitleaksEmpty(t *testing.T) {
	rep, err := ParseGitleaksJSON([]byte(""))
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if rep.Counts.Total != 0 {
		t.Errorf("err")
	}
}

// Aceitação: RedactSecret.
func TestRedactSecret(t *testing.T) {
	if got := RedactSecret("short"); got != "***" {
		t.Errorf("curto: got %q", got)
	}
	got := RedactSecret("AKIA1234567890ABCDEF")
	if !strings.Contains(got, "***") {
		t.Errorf("err: %q", got)
	}
	if len(got) != len("AKIA1234567890ABCDEF") {
		t.Errorf("err: %q (len=%d)", got, len(got))
	}
}

// Aceitação: RedactSecret truncado.
func TestRedactSecretLong(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := RedactSecret(long)
	if len(got) > 64 {
		t.Errorf("trunc devia limitar len: got %d", len(got))
	}
}

// Aceitação: RedactSecret muito curto.
func TestRedactSecretVeryShort(t *testing.T) {
	for _, s := range []string{"", "x", "ab", "abc", "abcd", "abcde", "abcdef"} {
		got := RedactSecret(s)
		if len(s) <= 6 && got != "***" {
			t.Errorf("input %q (%d) devia dar ***: got %q", s, len(s), got)
		}
	}
}

// Aceitação: IsLikelySecret.
func TestIsLikelySecret(t *testing.T) {
	// curto: não.
	if IsLikelySecret("short") {
		t.Errorf("curto não devia")
	}
	// longo + entropy alta.
	if !IsLikelySecret("aB3$kL9!mN2#pQ7@rS5&tU8*") {
		t.Errorf("devia detectar (entropy alta)")
	}
	// longo + entropy baixa (repetição).
	if IsLikelySecret(strings.Repeat("a", 30)) {
		t.Errorf("repetição não devia")
	}
}

// Aceitação: shannonEntropy.
func TestShannonEntropy(t *testing.T) {
	e := shannonEntropy("abcabc")
	if e <= 0 {
		t.Errorf("err")
	}
	if shannonEntropy("") != 0 {
		t.Errorf("vazio")
	}
	if shannonEntropy("aaaa") != 0 {
		t.Errorf("constante devia ser 0")
	}
}

// Aceitação: Severity mapping via gitleaksFinding.normalize.
func TestGitleaksFindingNormalize(t *testing.T) {
	cases := []struct {
		desc, rule string
		want       Severity
	}{
		{"AWS access key detected", "aws-access-token", SevHigh},
		{"RSA Private Key found", "private-key", SevHigh},
		{"Generic API Key", "generic-api-key", SevMedium},
		{"Low entropy string", "low-entropy", SevLow},
		{"unknown rule", "x-custom", SevMedium},
	}
	for _, c := range cases {
		g := gitleaksFinding{Description: c.desc, RuleID: c.rule}
		if got := g.normalize().Severity; got != c.want {
			t.Errorf("%s/%s: got %v want %v", c.desc, c.rule, got, c.want)
		}
	}
}

// Aceitação: AddFinding + counts.
func TestAddFinding(t *testing.T) {
	r := &Report{}
	r.AddFinding(Finding{Severity: SevHigh})
	r.AddFinding(Finding{Severity: SevHigh})
	r.AddFinding(Finding{Severity: SevMedium})
	if r.Counts.Total != 3 {
		t.Errorf("total = %d", r.Counts.Total)
	}
	if r.Counts.BySeverity[SevHigh] != 2 {
		t.Errorf("high = %d", r.Counts.BySeverity[SevHigh])
	}
}

// Aceitação: HasHighSeverity.
func TestHasHighSeverity(t *testing.T) {
	r := &Report{}
	r.AddFinding(Finding{Severity: SevLow})
	if r.HasHighSeverity() {
		t.Errorf("sem high devia dar false")
	}
	r.AddFinding(Finding{Severity: SevHigh})
	if !r.HasHighSeverity() {
		t.Errorf("com high devia dar true")
	}
}

// Aceitação: SortFindings por severity.
func TestSortFindings(t *testing.T) {
	r := &Report{}
	r.AddFinding(Finding{Severity: SevLow, FilePath: "a"})
	r.AddFinding(Finding{Severity: SevHigh, FilePath: "b"})
	r.AddFinding(Finding{Severity: SevMedium, FilePath: "c"})
	r.SortFindings()
	if r.Findings[0].Severity != SevHigh {
		t.Errorf("ordem: %v", r.Findings)
	}
}

// Aceitação: findingRank.
func TestFindingRank(t *testing.T) {
	if findingRank(Finding{Severity: SevHigh}) >= findingRank(Finding{Severity: SevInfo}) {
		t.Errorf("rank ordem errada")
	}
}

// Aceitação: log2 via math.Log2 (testado via shannonEntropy).

// Aceitação: lnApprox removido (usa math.Log2).

// Aceitação: shannonEntropy distribution.
func TestShannonEntropyDistribution(t *testing.T) {
	// 4 chars diferentes equally distributed = 2 bits.
	e := shannonEntropy("abcd")
	if e < 1.9 || e > 2.1 {
		t.Errorf("entropy abcd = %v, quero ~2", e)
	}
}

// Aceitação: RedactSecret preserva prefix e suffix.
func TestRedactSecretPrefixSuffix(t *testing.T) {
	s := "AKIA" + strings.Repeat("X", 30) + "YZ"
	got := RedactSecret(s)
	if !strings.HasPrefix(got, "AKIA") {
		t.Errorf("prefix = %q", got)
	}
	if !strings.HasSuffix(got, "YZ") {
		t.Errorf("suffix = %q", got)
	}
}

// Aceitação: Gitleaks com Tags.
func TestParseGitleaksWithTags(t *testing.T) {
	data := `[{"Description":"x","RuleID":"y","File":"f","Line":1,"Entropy":3.0,"Tags":["aws","secret"]}]`
	rep, _ := ParseGitleaksJSON([]byte(data))
	if rep.Findings[0].Severity != SevMedium {
		t.Errorf("sev = %v", rep.Findings[0].Severity)
	}
}

// Aceitação: gitleaks description contém "high" → SevHigh.
func TestParseGitleaksDescriptionHigh(t *testing.T) {
	data := `[{"Description":"HIGH risk","RuleID":"x","File":"f","Line":1,"Entropy":3.0}]`
	rep, _ := ParseGitleaksJSON([]byte(data))
	if rep.Findings[0].Severity != SevHigh {
		t.Errorf("description high não promovido")
	}
}
