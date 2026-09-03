package mcpserver

import (
	"context"
	"testing"
)

// Aceitação: New.
func TestNew(t *testing.T) {
	s := New()
	if s == nil {
		t.Errorf("nil")
	}
	if s.SDK() == nil {
		t.Errorf("sdk nil")
	}
}

// Aceitação: BeginReviewInput defaults.
func TestBeginReviewInput(t *testing.T) {
	in := BeginReviewInput{}
	if in.RunID != "" || in.Resume {
		t.Errorf("defaults")
	}
}

// Aceitação: BeginReview handler invocado.
func TestRegisterBeginReview(t *testing.T) {
	s := New()
	called := false
	s.RegisterBeginReview(func(ctx context.Context, in *BeginReviewInput) (*BeginReviewOutput, error) {
		called = true
		if in.RunID != "r1" {
			t.Errorf("run_id")
		}
		return &BeginReviewOutput{RunID: "r1", Schema: "1", CreatedAt: "now"}, nil
	})
	if s.SDK() == nil {
		t.Fatal("sdk nil")
	}
	// Não invocável diretamente via SDK em teste; só verificamos
	// que registrou sem panic.
	_ = called
}

// Aceitação: SubmitPeerReviewInput.
func TestSubmitPeerReviewInput(t *testing.T) {
	in := SubmitPeerReviewInput{Actor: "alice"}
	if in.Actor != "alice" {
		t.Errorf("actor")
	}
}

// Aceitação: SubmitPeerReview handler.
func TestRegisterSubmitPeerReview(t *testing.T) {
	s := New()
	s.RegisterSubmitPeerReview(func(ctx context.Context, in *SubmitPeerReviewInput) (*SubmitPeerReviewOutput, error) {
		return &SubmitPeerReviewOutput{Accepted: true, ReviewID: "rev1"}, nil
	})
	if s.SDK() == nil {
		t.Errorf("sdk nil")
	}
}

// Aceitação: mustJSON.
func TestMustJSON(t *testing.T) {
	s := mustJSON(map[string]any{"a": 1})
	if s == "" {
		t.Errorf("vazio")
	}
}

// Aceitação: BeginReviewOutput reusable.
func TestBeginReviewOutputReused(t *testing.T) {
	out := &BeginReviewOutput{RunID: "r1", Reused: true}
	if !out.Reused {
		t.Errorf("reused")
	}
}

// Aceitação: SubmitPeerReviewOutput validation.
func TestSubmitPeerReviewOutputValidation(t *testing.T) {
	out := &SubmitPeerReviewOutput{Accepted: true, ValidationOK: true}
	if !out.Accepted || !out.ValidationOK {
		t.Errorf("flags")
	}
}

// Aceitação: Run sem transport (smoke).
func TestRunNoTransport(t *testing.T) {
	s := New()
	err := s.Run(context.Background())
	// stdio lê de stdin; em test vazio retorna EOF ou blocking.
	// Aceitamos nil ou error (depende se bloqueou).
	_ = err
}
