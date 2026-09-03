package peer

import (
	"strings"
	"testing"
	"time"
)

// Aceitação: NewRecorder.
func TestNewRecorder(t *testing.T) {
	r := NewRecorder()
	if r == nil {
		t.Errorf("nil")
	}
}

// Aceitação: Record basic.
func TestRecord(t *testing.T) {
	r := NewRecorder()
	err := r.Record(PeerMetadata{Label: "A", Provider: "p1", Model: "gpt-4"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	got, ok := r.Get("A")
	if !ok || got.Model != "gpt-4" {
		t.Errorf("get")
	}
}

// Aceitação: Record valida campos.
func TestRecordValidate(t *testing.T) {
	r := NewRecorder()
	if err := r.Record(PeerMetadata{}); err == nil {
		t.Errorf("label vazio")
	}
	if err := r.Record(PeerMetadata{Label: "A"}); err == nil {
		t.Errorf("provider vazio")
	}
	if err := r.Record(PeerMetadata{Label: "A", Provider: "p"}); err == nil {
		t.Errorf("model vazio")
	}
}

// Aceitação: Record family auto-inferida.
func TestRecordAutoFamily(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "p", Model: "gpt-4"})
	got, _ := r.Get("A")
	if got.Family != "gpt" {
		t.Errorf("family auto: %s", got.Family)
	}
}

// Aceitação: Record RecordedAt populated.
func TestRecordTimestamp(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "p", Model: "x"})
	got, _ := r.Get("A")
	if got.RecordedAt.IsZero() {
		t.Errorf("timestamp vazio")
	}
}

// Aceitação: Labels.
func TestLabels(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "p", Model: "x"})
	r.Record(PeerMetadata{Label: "B", Provider: "q", Model: "y"})
	labels := r.Labels()
	if len(labels) != 2 {
		t.Errorf("labels: %d", len(labels))
	}
}

// Aceitação: Audit básico OK.
func TestAuditOK(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "openai", Model: "gpt-4"})
	r.Record(PeerMetadata{Label: "B", Provider: "anthropic", Model: "claude-3"})
	a, err := r.Audit(AuditOptions{DistinctnessPolicy: "provider"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !a.IndependenceOK {
		t.Errorf("independence: %v", a.DegradedReasons)
	}
}

// Aceitação: Audit missing peer.
func TestAuditMissing(t *testing.T) {
	r := NewRecorder()
	if _, err := r.Audit(AuditOptions{}); err == nil {
		t.Errorf("sem peer")
	}
}

// Aceitação: Audit detecta same provider.
func TestAuditSameProvider(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "openai", Model: "gpt-4"})
	r.Record(PeerMetadata{Label: "B", Provider: "openai", Model: "gpt-3.5"})
	a, _ := r.Audit(AuditOptions{DistinctnessPolicy: "provider"})
	if !a.Degraded {
		t.Errorf("devia degradar")
	}
	if !containsReason(a.DegradedReasons, "same provider") {
		t.Errorf("razão: %v", a.DegradedReasons)
	}
}

// Aceitação: Audit detecta same model.
func TestAuditSameModel(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "openai", Model: "gpt-4"})
	r.Record(PeerMetadata{Label: "B", Provider: "anthropic", Model: "gpt-4"})
	a, _ := r.Audit(AuditOptions{DistinctnessPolicy: "model"})
	if !containsReason(a.DegradedReasons, "same model") {
		t.Errorf("model reason: %v", a.DegradedReasons)
	}
}

// Aceitação: Audit detecta same family.
func TestAuditSameFamily(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "p1", Model: "gpt-4"})
	r.Record(PeerMetadata{Label: "B", Provider: "p2", Model: "gpt-3.5"})
	a, _ := r.Audit(AuditOptions{DistinctnessPolicy: "family"})
	if !containsReason(a.DegradedReasons, "same family") {
		t.Errorf("family reason: %v", a.DegradedReasons)
	}
}

// Aceitação: Audit detecta same context hash.
func TestAuditSameContext(t *testing.T) {
	r := NewRecorder()
	ctxHash := ComputeContextHash("shared-ctx")
	r.Record(PeerMetadata{Label: "A", Provider: "p1", Model: "x", ContextHash: ctxHash})
	r.Record(PeerMetadata{Label: "B", Provider: "p2", Model: "y", ContextHash: ctxHash})
	a, _ := r.Audit(AuditOptions{DistinctnessPolicy: "context"})
	if !containsReason(a.DegradedReasons, "same context") {
		t.Errorf("context: %v", a.DegradedReasons)
	}
}

// Aceitação: Audit cross-contamination.
func TestAuditCross(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "p1", Model: "x"})
	r.Record(PeerMetadata{Label: "B", Provider: "p2", Model: "y"})
	a, _ := r.Audit(AuditOptions{CrossContamination: []string{"PEER_A_OUTPUT found"}})
	if !containsReason(a.DegradedReasons, "cross-contamination") {
		t.Errorf("cross: %v", a.DegradedReasons)
	}
}

// Aceitação: Cross contamination ignore empty.
func TestAuditCrossEmpty(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "p1", Model: "x"})
	r.Record(PeerMetadata{Label: "B", Provider: "p2", Model: "y"})
	a, _ := r.Audit(AuditOptions{CrossContamination: []string{""}})
	if a.Degraded {
		t.Errorf("empty cross não devia degradar")
	}
}

// Aceitação: ComputeContextHash determinístico.
func TestContextHashStable(t *testing.T) {
	h1 := ComputeContextHash("a", "b")
	h2 := ComputeContextHash("a", "b")
	if h1 != h2 {
		t.Errorf("stable")
	}
}

// Aceitação: ComputeContextHash sensitive.
func TestContextHashSensitive(t *testing.T) {
	h1 := ComputeContextHash("a")
	h2 := ComputeContextHash("b")
	if h1 == h2 {
		t.Errorf("sensitive")
	}
}

// Aceitação: DetectCrossContamination marker.
func TestDetectCrossMarker(t *testing.T) {
	hits := DetectCrossContamination("any output", "PEER_A_OUTPUT: data")
	if len(hits) == 0 {
		t.Errorf("marker")
	}
}

// Aceitação: DetectCrossContamination substring.
func TestDetectCrossSubstring(t *testing.T) {
	hits := DetectCrossContamination("this is the actual peer A output", "review includes this is the actual peer A output by alice")
	if len(hits) == 0 {
		t.Errorf("substring")
	}
}

// Aceitação: DetectCrossContamination clean.
func TestDetectCrossClean(t *testing.T) {
	hits := DetectCrossContamination("output", "totally different prompt")
	if len(hits) != 0 {
		t.Errorf("clean")
	}
}

// Aceitação: DetectCross empty inputs.
func TestDetectCrossEmpty(t *testing.T) {
	if hits := DetectCrossContamination("", ""); len(hits) != 0 {
		t.Errorf("empty")
	}
	if hits := DetectCrossContamination("x", ""); len(hits) != 0 {
		t.Errorf("empty prompt")
	}
	if hits := DetectCrossContamination("", "x"); len(hits) != 0 {
		t.Errorf("empty output")
	}
}

// Aceitação: RenderReport OK.
func TestRenderReportOK(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "openai", Model: "gpt-4"})
	r.Record(PeerMetadata{Label: "B", Provider: "anthropic", Model: "claude-3"})
	a, _ := r.Audit(AuditOptions{DistinctnessPolicy: "provider"})
	out := RenderReport(a)
	if !strings.Contains(out, "OK") {
		t.Errorf("ok")
	}
	if RenderReport(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: RenderReport degraded.
func TestRenderReportDegraded(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "openai", Model: "gpt-4"})
	r.Record(PeerMetadata{Label: "B", Provider: "openai", Model: "gpt-3.5"})
	a, _ := r.Audit(AuditOptions{DistinctnessPolicy: "provider"})
	out := RenderReport(a)
	if !strings.Contains(out, "DEGRADED") {
		t.Errorf("degraded")
	}
	if !strings.Contains(out, "Reasons") {
		t.Errorf("reasons")
	}
}

// Aceitação: MarkDegraded manual.
func TestMarkDegraded(t *testing.T) {
	r := NewRecorder()
	r.Record(PeerMetadata{Label: "A", Provider: "p1", Model: "x"})
	r.Record(PeerMetadata{Label: "B", Provider: "p2", Model: "y"})
	a, _ := r.Audit(AuditOptions{})
	MarkDegraded(a, "manual flag")
	if !a.Degraded {
		t.Errorf("manual")
	}
}

// Aceitação: MarkDegraded nil safe.
func TestMarkDegradedNil(t *testing.T) {
	MarkDegraded(nil, "x") // não panic
}

// Aceitação: HasDegradation helper.
func TestHasDegradation(t *testing.T) {
	if HasDegradation(nil) {
		t.Errorf("nil")
	}
	a := &IndependenceAudit{}
	if HasDegradation(a) {
		t.Errorf("zero")
	}
	a.Degraded = true
	if !HasDegradation(a) {
		t.Errorf("true")
	}
}

// Aceitação: inferFamily.
func TestInferFamily(t *testing.T) {
	if inferFamily("gpt-4") != "gpt" {
		t.Errorf("gpt")
	}
	if inferFamily("CLAUDE-3") != "claude" {
		t.Errorf("claude")
	}
	if inferFamily("random") != "other" {
		t.Errorf("other")
	}
}

// Aceitação: PeerMetadata RecordedAt dentro de margem.
func TestRecordRecent(t *testing.T) {
	r := NewRecorder()
	before := time.Now()
	r.Record(PeerMetadata{Label: "A", Provider: "p", Model: "x"})
	got, _ := r.Get("A")
	if got.RecordedAt.Before(before) {
		t.Errorf("before")
	}
}

// helper.
func containsReason(reasons []string, substr string) bool {
	for _, r := range reasons {
		if strings.Contains(r, substr) {
			return true
		}
	}
	return false
}
