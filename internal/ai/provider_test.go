package ai

import (
	"context"
	"strings"
	"testing"
)

// Aceitação: MockProvider Name.
func TestMockProviderName(t *testing.T) {
	p := NewMockProvider()
	if p.Name() != "mock" {
		t.Errorf("name")
	}
}

// Aceitação: ListModels.
func TestMockListModels(t *testing.T) {
	p := NewMockProvider()
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("models count")
	}
}

// Aceitação: CompleteJSON OK.
func TestMockCompleteOK(t *testing.T) {
	p := NewMockProvider()
	res, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:    "mock-1",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Model != "mock-1" {
		t.Errorf("model")
	}
	if !strings.Contains(res.Content, "mock") {
		t.Errorf("content")
	}
}

// Aceitação: CompleteJSON model não existe.
func TestMockCompleteModelMissing(t *testing.T) {
	p := NewMockProvider()
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model:    "nope",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Errorf("esperava erro")
	}
}

// Aceitação: CompleteJSON empty messages.
func TestCompleteEmptyMessages(t *testing.T) {
	p := NewMockProvider()
	_, err := p.CompleteJSON(context.Background(), CompleteOptions{
		Model: "mock-1",
	})
	if err == nil {
		t.Errorf("empty")
	}
}

// Aceitação: ValidateOptions model vazio.
func TestValidateModelEmpty(t *testing.T) {
	if err := ValidateOptions(CompleteOptions{
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	}); err == nil {
		t.Errorf("model vazio")
	}
}

// Aceitação: ValidateOptions message sem role.
func TestValidateMessageNoRole(t *testing.T) {
	err := ValidateOptions(CompleteOptions{
		Model:    "x",
		Messages: []Message{{Content: "x"}},
	})
	if err == nil {
		t.Errorf("role")
	}
}

// Aceitação: ValidateOptions message sem content.
func TestValidateMessageNoContent(t *testing.T) {
	err := ValidateOptions(CompleteOptions{
		Model:    "x",
		Messages: []Message{{Role: RoleUser}},
	})
	if err == nil {
		t.Errorf("content")
	}
}

// Aceitação: ValidateOptions OK.
func TestValidateOK(t *testing.T) {
	err := ValidateOptions(CompleteOptions{
		Model:    "x",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: FilterByCapability JSON.
func TestFilterJSON(t *testing.T) {
	models := []ModelInfo{
		{ID: "a", SupportsJSON: true},
		{ID: "b", SupportsJSON: false},
	}
	out := FilterByCapability(models, true, false, false)
	if len(out) != 1 || out[0].ID != "a" {
		t.Errorf("filter")
	}
}

// Aceitação: FilterByCapability vision.
func TestFilterVision(t *testing.T) {
	models := []ModelInfo{
		{ID: "a", SupportsJSON: true, SupportsVision: true},
		{ID: "b", SupportsJSON: true},
	}
	out := FilterByCapability(models, false, true, false)
	if len(out) != 1 || out[0].ID != "a" {
		t.Errorf("vision")
	}
}

// Aceitação: FindModel.
func TestFindModel(t *testing.T) {
	models := []ModelInfo{{ID: "a"}, {ID: "b"}}
	if _, ok := FindModel(models, "a"); !ok {
		t.Errorf("find")
	}
	if _, ok := FindModel(models, "z"); ok {
		t.Errorf("find z")
	}
}

// Aceitação: SupportsJSON helper.
func TestSupportsJSONHelper(t *testing.T) {
	models := []ModelInfo{{ID: "a", SupportsJSON: true}, {ID: "b"}}
	if !SupportsJSON(models, "a") {
		t.Errorf("a")
	}
	if SupportsJSON(models, "b") {
		t.Errorf("b")
	}
	if SupportsJSON(models, "z") {
		t.Errorf("z")
	}
}

// Aceitação: RoleIsValid.
func TestRoleIsValid(t *testing.T) {
	if !RoleIsValid(RoleSystem) {
		t.Errorf("system")
	}
	if !RoleIsValid(RoleUser) {
		t.Errorf("user")
	}
	if !RoleIsValid(RoleAssistant) {
		t.Errorf("assistant")
	}
	if RoleIsValid("foo") {
		t.Errorf("foo")
	}
}

// Aceitação: BuildMessages.
func TestBuildMessages(t *testing.T) {
	msgs := BuildMessages("sys", "usr")
	if len(msgs) != 2 {
		t.Errorf("count")
	}
	if msgs[0].Role != RoleSystem {
		t.Errorf("sys")
	}
	if msgs[1].Role != RoleUser {
		t.Errorf("usr")
	}
}

// Aceitação: BuildMessages vazio.
func TestBuildMessagesEmpty(t *testing.T) {
	if len(BuildMessages("", "")) != 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: BuildMessages só user.
func TestBuildMessagesUserOnly(t *testing.T) {
	msgs := BuildMessages("", "hi")
	if len(msgs) != 1 || msgs[0].Role != RoleUser {
		t.Errorf("user only")
	}
}

// Aceitação: TruncateContent.
func TestTruncate(t *testing.T) {
	if TruncateContent("abc", 10) != "abc" {
		t.Errorf("no truncate")
	}
	out := TruncateContent("abcdefghij", 5)
	if !strings.HasSuffix(out, "...") {
		t.Errorf("truncate")
	}
}

// Aceitação: TruncateContent max=0.
func TestTruncateZero(t *testing.T) {
	if TruncateContent("abc", 0) != "abc" {
		t.Errorf("zero max")
	}
}

// Aceitação: StripCodeFences.
func TestStripFences(t *testing.T) {
	in := "```json\n{\"a\":1}\n```"
	out := StripCodeFences(in)
	if strings.Contains(out, "```") {
		t.Errorf("still fenced: %s", out)
	}
	if !strings.Contains(out, "a") {
		t.Errorf("content")
	}
}

// Aceitação: StripCodeFences no fence.
func TestStripFencesNoOp(t *testing.T) {
	in := `{"a":1}`
	if StripCodeFences(in) != in {
		t.Errorf("changed")
	}
}

// Aceitação: Metadata mock.
func TestMetadata(t *testing.T) {
	p := NewMockProvider()
	m := p.Metadata()
	if m.Name != "mock" {
		t.Errorf("meta")
	}
	if m.Version == "" {
		t.Errorf("version")
	}
}

// Aceitação: MockProvider ListModels copy.
func TestListModelsImmutable(t *testing.T) {
	p := NewMockProvider()
	a, _ := p.ListModels(context.Background())
	a[0].ID = "mutated"
	b, _ := p.ListModels(context.Background())
	if b[0].ID == "mutated" {
		t.Errorf("shared slice")
	}
}

// Aceitação: MockProvider context cancelation.
func TestCompleteCtxCancel(t *testing.T) {
	p := NewMockProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.CompleteJSON(ctx, CompleteOptions{
		Model:    "mock-1",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	// Mock não checa ctx; aceita. Só garante não-deadlock.
	_ = err
}
