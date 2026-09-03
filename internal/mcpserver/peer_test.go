package mcpserver

import (
	"context"
	"testing"
)

// Aceitação: Validator básico válido.
func TestValidateValid(t *testing.T) {
	v := DefaultPeerReviewValidator()
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "alice", Schema: "1",
		EvidenceHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Verdict:      "approve",
	}
	if errs := v.Validate(in); len(errs) > 0 {
		t.Errorf("esperado válido, got %v", errs)
	}
}

// Aceitação: Validator run_id ausente.
func TestValidateRunIDMissing(t *testing.T) {
	v := DefaultPeerReviewValidator()
	in := &SubmitPeerReviewInput{}
	if errs := v.Validate(in); len(errs) == 0 {
		t.Errorf("esperado erros")
	}
}

// Aceitação: Validator actor missing.
func TestValidateActorMissing(t *testing.T) {
	v := DefaultPeerReviewValidator()
	in := &SubmitPeerReviewInput{
		RunID: "r1", Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "approve",
	}
	errs := v.Validate(in)
	found := false
	for _, e := range errs {
		if e.Field == "actor" {
			found = true
		}
	}
	if !found {
		t.Errorf("actor error")
	}
}

// Aceitação: Validator schema unsupported.
func TestValidateSchemaUnsupported(t *testing.T) {
	v := DefaultPeerReviewValidator()
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "a", Schema: "99",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "approve",
	}
	errs := v.Validate(in)
	if len(errs) == 0 {
		t.Errorf("esperado erro schema")
	}
}

// Aceitação: Validator verdict unknown.
func TestValidateVerdictUnknown(t *testing.T) {
	v := DefaultPeerReviewValidator()
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "a", Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "wat",
	}
	errs := v.Validate(in)
	if len(errs) == 0 {
		t.Errorf("verdict")
	}
}

// Aceitação: Validator evidence_hash não-hex.
func TestValidateHashNotHex(t *testing.T) {
	v := DefaultPeerReviewValidator()
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "a", Schema: "1",
		EvidenceHash: "zz", Verdict: "approve",
	}
	errs := v.Validate(in)
	if len(errs) == 0 {
		t.Errorf("hash nan hex")
	}
}

// Aceitação: Validator hash 64 chars.
func TestValidateHashLen(t *testing.T) {
	v := DefaultPeerReviewValidator()
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "a", Schema: "1",
		EvidenceHash: "abc", Verdict: "approve",
	}
	errs := v.Validate(in)
	if len(errs) == 0 {
		t.Errorf("hash curto")
	}
}

// Aceitação: Validator actor muito longo.
func TestValidateActorLong(t *testing.T) {
	v := DefaultPeerReviewValidator()
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: string(make([]byte, 201)), Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "approve",
	}
	errs := v.Validate(in)
	if len(errs) == 0 {
		t.Errorf("actor longo")
	}
}

// Aceitação: Validator min findings.
func TestValidateMinFindings(t *testing.T) {
	v := &PeerReviewValidator{MinFindings: 2, AllowedVerdicts: []string{"approve"}}
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "a", Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "approve",
	}
	errs := v.Validate(in)
	if len(errs) == 0 {
		t.Errorf("min findings")
	}
}

// Aceitação: ComputeContentHash estável.
func TestComputeContentHashStable(t *testing.T) {
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "a", Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "approve",
	}
	h1 := ComputeContentHash(in)
	h2 := ComputeContentHash(in)
	if h1 != h2 {
		t.Errorf("mesmo input devia mesmo hash")
	}
}

// Aceitação: ComputeContentHash sensível.
func TestComputeContentHashSensitive(t *testing.T) {
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "a", Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "approve",
	}
	h1 := ComputeContentHash(in)
	in2 := *in
	in2.Actor = "b"
	h2 := ComputeContentHash(&in2)
	if h1 == h2 {
		t.Errorf("actor diferente devia mudar hash")
	}
}

// Aceitação: NewPeerReviewStore.
func TestNewPeerReviewStore(t *testing.T) {
	if _, err := NewPeerReviewStore(""); err == nil {
		t.Errorf("vazio")
	}
	s, err := NewPeerReviewStore(t.TempDir())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s == nil {
		t.Errorf("nil")
	}
}

// Aceitação: Save/Load round-trip.
func TestPeerReviewSaveLoad(t *testing.T) {
	s, _ := NewPeerReviewStore(t.TempDir())
	rec := &PeerReviewRecord{
		ReviewID: "rev1", RunID: "r1", Actor: "alice",
		Schema: "1", EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict: "approve", ContentHash: "abc",
	}
	if err := s.Save(rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load("rev1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Actor != "alice" {
		t.Errorf("actor")
	}
}

// Aceitação: Save nil.
func TestPeerReviewSaveNil(t *testing.T) {
	s, _ := NewPeerReviewStore(t.TempDir())
	if err := s.Save(nil); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: Save review_id vazio.
func TestPeerReviewSaveEmptyID(t *testing.T) {
	s, _ := NewPeerReviewStore(t.TempDir())
	if err := s.Save(&PeerReviewRecord{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Load missing.
func TestPeerReviewLoadMissing(t *testing.T) {
	s, _ := NewPeerReviewStore(t.TempDir())
	if _, err := s.Load("missing"); err == nil {
		t.Errorf("missing")
	}
}

// Aceitação: Load empty.
func TestPeerReviewLoadEmpty(t *testing.T) {
	s, _ := NewPeerReviewStore(t.TempDir())
	if _, err := s.Load(""); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: DefaultSubmitPeerReviewHandler accept.
func TestDefaultSubmitHandlerAccept(t *testing.T) {
	s, _ := NewPeerReviewStore(t.TempDir())
	h := DefaultSubmitPeerReviewHandler(s, nil)
	in := &SubmitPeerReviewInput{
		RunID: "r1", Actor: "alice", Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Verdict:      "approve",
	}
	out, err := h(context.Background(), in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.Accepted {
		t.Errorf("accept")
	}
	if !out.ValidationOK {
		t.Errorf("validation")
	}
	if out.ReviewID == "" {
		t.Errorf("review_id")
	}
}

// Aceitação: DefaultSubmitPeerReviewHandler reject.
func TestDefaultSubmitHandlerReject(t *testing.T) {
	s, _ := NewPeerReviewStore(t.TempDir())
	h := DefaultSubmitPeerReviewHandler(s, nil)
	in := &SubmitPeerReviewInput{Actor: "alice"}
	out, err := h(context.Background(), in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.Accepted {
		t.Errorf("deveria rejeitar")
	}
	if len(out.Errors) == 0 {
		t.Errorf("errors")
	}
}

// Aceitação: DefaultSubmitPeerReviewHandler nil input.
func TestDefaultSubmitHandlerNilInput(t *testing.T) {
	s, _ := NewPeerReviewStore(t.TempDir())
	h := DefaultSubmitPeerReviewHandler(s, nil)
	if _, err := h(context.Background(), nil); err == nil {
		t.Errorf("nil devia erro")
	}
}

// Aceitação: validateEvidenceHash.
func TestValidateEvidenceHash(t *testing.T) {
	in := &SubmitPeerReviewInput{EvidenceHash: "abc"}
	if !ValidateEvidenceHash(in, "abc") {
		t.Errorf("match")
	}
	if ValidateEvidenceHash(in, "xyz") {
		t.Errorf("mismatch")
	}
}

// Aceitação: ValidationErrors.Error().
func TestValidationErrorsString(t *testing.T) {
	e := ValidationErrors{{Field: "f1", Message: "m1"}}
	if e.Error() == "" {
		t.Errorf("string")
	}
	var empty ValidationErrors
	if empty.Error() != "no errors" {
		t.Errorf("empty")
	}
}

// Aceitação: deriveReviewID unique.
func TestDeriveReviewIDUnique(t *testing.T) {
	in1 := &SubmitPeerReviewInput{RunID: "r1", Actor: "a", Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000", Verdict: "approve"}
	in2 := &SubmitPeerReviewInput{RunID: "r2", Actor: "a", Schema: "1",
		EvidenceHash: "0000000000000000000000000000000000000000000000000000000000000000", Verdict: "approve"}
	id1 := deriveReviewID(in1)
	id2 := deriveReviewID(in2)
	if id1 == id2 {
		t.Errorf("diferentes inputs deviam IDs diferentes")
	}
	if len(id1) != 16 {
		t.Errorf("16 chars")
	}
}
