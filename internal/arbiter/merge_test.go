package arbiter

import (
	"strings"
	"testing"
)

// Aceitação: Merge basic OK.
func TestMergeOK(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerAFindings: []Finding{
			{ID: "f1", Kind: "solid", Principle: "SRP", Severity: "high", Symbol: "X"},
		},
		PeerBFindings: []Finding{},
		Verdict: &Verdict{
			RunID: "r1", Actor: "arbiter", Verdict: "resolved",
			Resolutions: []Resolution{
				{Topic: "SRP:f1", Decision: DecisionAcceptPeerA},
			},
		},
	}
	r, err := Merge(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(r.Accepted) != 1 {
		t.Errorf("accepted: %d", len(r.Accepted))
	}
	if r.Accepted[0].ID != "f1" {
		t.Errorf("f1")
	}
}

// Aceitação: Merge RunID vazio.
func TestMergeEmptyRunID(t *testing.T) {
	if _, err := Merge(MergeInput{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: Merge sem Verdict.
func TestMergeNoVerdict(t *testing.T) {
	if _, err := Merge(MergeInput{RunID: "r1"}); err == nil {
		t.Errorf("no verdict")
	}
}

// Aceitação: Immutable preservado.
func TestImmutablePreserved(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerAFindings: []Finding{
			{ID: "test1", Kind: KindTest, Severity: "high", Immutable: true},
		},
		PeerBFindings: []Finding{
			{ID: "sonar1", Kind: KindSonar, Severity: "critical", Immutable: true},
		},
		Verdict: &Verdict{
			RunID: "r1", Verdict: "resolved",
			Resolutions: []Resolution{
				{Topic: "SRP:test1", Decision: DecisionRejectBoth},
			},
		},
	}
	r, _ := Merge(in)
	if len(r.Immutable) != 2 {
		t.Errorf("immutable: %d", len(r.Immutable))
	}
}

// Aceitação: Immutable NÃO alterado pelo árbitro (aceitação).
func TestImmutableNotAlteredByVerdict(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerAFindings: []Finding{
			{ID: "zap1", Kind: KindZAP, Severity: "critical", Immutable: true},
		},
		Verdict: &Verdict{
			RunID: "r1", Verdict: "resolved",
			Resolutions: []Resolution{
				{Topic: "SOLID:zap1", Decision: DecisionRejectBoth}, // tenta rejeitar
			},
		},
	}
	r, _ := Merge(in)
	if len(r.Immutable) != 1 {
		t.Errorf("deveria preservar: %d", len(r.Immutable))
	}
	if len(r.Rejected) != 0 {
		t.Errorf("árbitro não pode rejeitar immutable: %d", len(r.Rejected))
	}
}

// Aceitação: AcceptPeerA.
func TestAcceptPeerA(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerAFindings: []Finding{
			{ID: "f1", Kind: "solid", Principle: "OCP", Severity: "medium"},
		},
		Verdict: &Verdict{
			RunID: "r1", Verdict: "resolved",
			Resolutions: []Resolution{{Topic: "OCP:f1", Decision: DecisionAcceptPeerA}},
		},
	}
	r, _ := Merge(in)
	if len(r.Accepted) != 1 {
		t.Errorf("accepted")
	}
}

// Aceitação: AcceptPeerB.
func TestAcceptPeerB(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerBFindings: []Finding{
			{ID: "f1", Kind: "solid", Principle: "LSP", Severity: "low"},
		},
		Verdict: &Verdict{
			RunID: "r1", Verdict: "resolved",
			Resolutions: []Resolution{{Topic: "LSP:f1", Decision: DecisionAcceptPeerB}},
		},
	}
	r, _ := Merge(in)
	if len(r.Accepted) != 1 {
		t.Errorf("accepted B")
	}
}

// Aceitação: AcceptBoth.
func TestAcceptBoth(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerAFindings: []Finding{
			{ID: "f1", Kind: "solid", Principle: "ISP", Severity: "medium"},
		},
		PeerBFindings: []Finding{
			{ID: "f1", Kind: "solid", Principle: "ISP", Severity: "medium"},
		},
		Verdict: &Verdict{
			RunID: "r1", Verdict: "resolved",
			Resolutions: []Resolution{{Topic: "ISP:f1", Decision: DecisionAcceptBoth}},
		},
	}
	r, _ := Merge(in)
	if len(r.Merged) != 1 {
		t.Errorf("merged: %d", len(r.Merged))
	}
	if !r.Merged[0].Merged {
		t.Errorf("merged flag")
	}
}

// Aceitação: RejectBoth.
func TestRejectBoth(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerAFindings: []Finding{
			{ID: "f1", Kind: "solid", Principle: "DIP", Severity: "low"},
		},
		Verdict: &Verdict{
			RunID: "r1", Verdict: "resolved",
			Resolutions: []Resolution{{Topic: "DIP:f1", Decision: DecisionRejectBoth}},
		},
	}
	r, _ := Merge(in)
	if len(r.Rejected) != 1 {
		t.Errorf("rejected: %d", len(r.Rejected))
	}
	if !r.Rejected[0].Rejected {
		t.Errorf("rejected flag")
	}
}

// Aceitação: Topic inválido.
func TestInvalidTopic(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerAFindings: []Finding{
			{ID: "f1", Kind: "solid", Principle: "SRP"},
		},
		Verdict: &Verdict{
			RunID: "r1", Verdict: "resolved",
			Resolutions: []Resolution{{Topic: "no-colon", Decision: DecisionAcceptPeerA}},
		},
	}
	r, _ := Merge(in)
	if len(r.Accepted) != 0 {
		t.Errorf("topic inválido devia ser ignorado")
	}
}

// Aceitação: isImmutableKind.
func TestIsImmutableKind(t *testing.T) {
	if !isImmutableKind(KindTest) {
		t.Errorf("test")
	}
	if !isImmutableKind(KindSonar) {
		t.Errorf("sonar")
	}
	if !isImmutableKind(KindZAP) {
		t.Errorf("zap")
	}
	if isImmutableKind(KindSOLID) {
		t.Errorf("solid não immutable")
	}
	if isImmutableKind("random") {
		t.Errorf("random")
	}
}

// Aceitação: Final status S (sem findings).
func TestFinalStatusShip(t *testing.T) {
	r := &MergeResult{}
	r.FinalStatus = ComputeFinalStatus(r)
	if r.FinalStatus != StatusShip {
		t.Errorf("vazio devia ship: %s", r.FinalStatus)
	}
}

// Aceitação: Final status L (immutable critical).
func TestFinalStatusLimit(t *testing.T) {
	r := &MergeResult{
		Immutable: []Finding{{Severity: "critical"}},
	}
	r.FinalStatus = ComputeFinalStatus(r)
	if r.FinalStatus != StatusLimit {
		t.Errorf("critical: %s", r.FinalStatus)
	}
}

// Aceitação: Final status L (accepted critical).
func TestFinalStatusLimitAccepted(t *testing.T) {
	r := &MergeResult{
		Accepted: []Finding{{Severity: "high"}},
	}
	r.FinalStatus = ComputeFinalStatus(r)
	if r.FinalStatus != StatusLimit {
		t.Errorf("accepted high: %s", r.FinalStatus)
	}
}

// Aceitação: Final status O (medium).
func TestFinalStatusObserve(t *testing.T) {
	r := &MergeResult{
		Accepted: []Finding{{Severity: "medium"}},
	}
	r.FinalStatus = ComputeFinalStatus(r)
	if r.FinalStatus != StatusObserve {
		t.Errorf("medium: %s", r.FinalStatus)
	}
}

// Aceitação: Final status D (defer).
func TestFinalStatusDefer(t *testing.T) {
	r := &MergeResult{
		Accepted: []Finding{{Severity: "low"}},
	}
	r.FinalStatus = ComputeFinalStatus(r)
	if r.FinalStatus != StatusDefer {
		t.Errorf("low: %s", r.FinalStatus)
	}
}

// Aceitação: HasImmutableCritical.
func TestHasImmutableCritical(t *testing.T) {
	if (&MergeResult{}).HasImmutableCritical() {
		t.Errorf("vazio")
	}
	r := &MergeResult{Immutable: []Finding{{Severity: "high"}}}
	if !r.HasImmutableCritical() {
		t.Errorf("high")
	}
}

// Aceitação: Counters.
func TestCounters(t *testing.T) {
	var r *MergeResult
	if r.TotalFindings() != 0 {
		t.Errorf("nil")
	}
	r = &MergeResult{
		Accepted: []Finding{{}, {}},
		Rejected: []Finding{{}},
		Merged:   []Finding{{}},
	}
	if r.TotalFindings() != 4 {
		t.Errorf("total: %d", r.TotalFindings())
	}
	if r.AcceptedCount() != 2 {
		t.Errorf("acc: %d", r.AcceptedCount())
	}
	if r.RejectedCount() != 1 {
		t.Errorf("rej: %d", r.RejectedCount())
	}
	if r.MergedCount() != 1 {
		t.Errorf("mer: %d", r.MergedCount())
	}
	if r.ImmutableCount() != 0 {
		t.Errorf("imm: %d", r.ImmutableCount())
	}
}

// Aceitação: RenderMerge.
func TestRenderMerge(t *testing.T) {
	r := &MergeResult{
		RunID:       "r1",
		FinalStatus: StatusLimit,
		Accepted:    []Finding{{ID: "f1"}},
		Immutable:   []Finding{{ID: "t1"}},
		Reasoning:   "needs review",
	}
	out := RenderMerge(r)
	if !strings.Contains(out, "r1") {
		t.Errorf("runid")
	}
	if !strings.Contains(out, StatusLimit) {
		t.Errorf("status")
	}
	if !strings.Contains(out, "reasoning") {
		t.Errorf("reasoning")
	}
	if RenderMerge(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: dedup.
func TestDedup(t *testing.T) {
	in := []Finding{{ID: "f1"}, {ID: "f1"}, {ID: "f2"}}
	out := dedup(in)
	if len(out) != 2 {
		t.Errorf("dedup: %d", len(out))
	}
}

// Aceitação: SortFindingsByID.
func TestSortFindingsByID(t *testing.T) {
	in := []Finding{{ID: "z"}, {ID: "a"}, {ID: "m"}}
	SortFindingsByID(in)
	if in[0].ID != "a" || in[1].ID != "m" || in[2].ID != "z" {
		t.Errorf("sort: %v", in)
	}
}

// Aceitação: itoaCount.
func TestItoaCount(t *testing.T) {
	if itoaCount(0) != "0" {
		t.Errorf("0")
	}
	if itoaCount(123) != "123" {
		t.Errorf("123")
	}
}

// Aceitação: AcceptPeerA com finding inexistente.
func TestAcceptPeerANotFound(t *testing.T) {
	in := MergeInput{
		RunID: "r1",
		PeerAFindings: []Finding{
			{ID: "f1", Kind: "solid", Principle: "SRP"},
		},
		Verdict: &Verdict{
			RunID: "r1", Verdict: "resolved",
			Resolutions: []Resolution{{Topic: "OCP:notfound", Decision: DecisionAcceptPeerA}},
		},
	}
	r, _ := Merge(in)
	if len(r.Accepted) != 0 {
		t.Errorf("não devia aceitar")
	}
}

// Aceitação: Final status L (merged critical).
func TestFinalStatusMergedCritical(t *testing.T) {
	r := &MergeResult{
		Merged: []Finding{{Severity: "critical"}},
	}
	r.FinalStatus = ComputeFinalStatus(r)
	if r.FinalStatus != StatusLimit {
		t.Errorf("merged critical: %s", r.FinalStatus)
	}
}
