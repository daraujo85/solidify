package errs

import "testing"

func TestPublicExitCodeNormalizesIncomplete(t *testing.T) {
	if got := PublicExitCode(New(CodeIncomplete, "incomplete")); got != 2 {
		t.Fatalf("public exit=%d, want 2", got)
	}
	if got := PublicExitCode(New(CodeProvider, "provider")); got != 1 {
		t.Fatalf("public provider exit=%d, want 1", got)
	}
}
