package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/diegoaraujo/solidify/internal/load"
)

// helper para criar LoadResult.
func mkResult(fp, target string, p95 time.Duration) *load.LoadResult {
	return &load.LoadResult{
		Script: "test.js", Target: target, VUs: 10, Duration: 30 * time.Second,
		P50: 200 * time.Millisecond, P95: p95, P99: 800 * time.Millisecond,
		Throughput: 100, ErrorRate: 0.001, Fingerprint: fp,
		Timestamp: time.Now(),
	}
}

// Aceitação: NewBaselineStore.
func TestNewBaselineStore(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBaselineStore(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s == nil {
		t.Fatal("store nil")
	}
}

// Aceitação: NewBaselineStore dir vazio.
func TestNewBaselineStoreEmptyDir(t *testing.T) {
	if _, err := NewBaselineStore(""); err == nil {
		t.Errorf("vazio devia falhar")
	}
}

// Aceitação: Save + Load round-trip.
func TestSaveLoadRoundTrip(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	r := mkResult("fp1", "http://x", 400*time.Millisecond)
	m := &LoadResultMeta{Result: r, CapturedAt: time.Now(), RefRunID: "run1", Branch: BranchMain}
	if err := s.Save(m); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load("fp1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Result.Fingerprint != "fp1" {
		t.Errorf("fingerprint")
	}
	if got.Branch != BranchMain {
		t.Errorf("branch")
	}
}

// Aceitação: Save nil.
func TestSaveNil(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	if err := s.Save(nil); err == nil {
		t.Errorf("nil devia falhar")
	}
}

// Aceitação: Save fingerprint vazio.
func TestSaveEmptyFingerprint(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	if err := s.Save(&LoadResultMeta{Result: &load.LoadResult{}}); err == nil {
		t.Errorf("fingerprint vazio devia falhar")
	}
}

// Aceitação: Save result nil.
func TestSaveResultNil(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	if err := s.Save(&LoadResultMeta{}); err == nil {
		t.Errorf("result nil devia falhar")
	}
}

// Aceitação: Load missing.
func TestLoadMissing(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	if _, err := s.Load("missing"); err == nil {
		t.Errorf("missing devia falhar")
	}
}

// Aceitação: Load empty fingerprint.
func TestLoadEmptyFingerprint(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	if _, err := s.Load(""); err == nil {
		t.Errorf("fingerprint vazio devia falhar")
	}
}

// Aceitação: Delete.
func TestDelete(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	r := mkResult("fp1", "http://x", 400*time.Millisecond)
	s.Save(&LoadResultMeta{Result: r, CapturedAt: time.Now(), Branch: BranchMain})
	if err := s.Delete("fp1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Load("fp1"); err == nil {
		t.Errorf("depois de delete devia falhar")
	}
}

// Aceitação: Delete missing não falla.
func TestDeleteMissing(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	if err := s.Delete("missing"); err != nil {
		t.Errorf("delete missing = no-op: %v", err)
	}
}

// Aceitação: List.
func TestList(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	for i, fp := range []string{"fp1", "fp2", "fp3"} {
		s.Save(&LoadResultMeta{Result: mkResult(fp, "http://x", 400*time.Millisecond), CapturedAt: time.Now(), Branch: BranchMain, RefRunID: string(rune('a' + i))})
	}
	got, err := s.List()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len = %d", len(got))
	}
}

// Aceitação: List filter por branch.
func TestListFilterBranch(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	s.Save(&LoadResultMeta{Result: mkResult("fp1", "http://x", 400*time.Millisecond), CapturedAt: time.Now(), Branch: BranchMain})
	s.Save(&LoadResultMeta{Result: mkResult("fp2", "http://x", 400*time.Millisecond), CapturedAt: time.Now(), Branch: BranchDevelop})
	got, _ := s.List(BranchMain)
	if len(got) != 1 {
		t.Errorf("filter = %d", len(got))
	}
}

// Aceitação: SelectBaselineForBranch.
func TestSelectBaselineForBranch(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	r := mkResult("fp1", "http://x", 400*time.Millisecond)
	s.Save(&LoadResultMeta{Result: r, CapturedAt: time.Now(), Branch: BranchDevelop})
	got, err := s.SelectBaselineForBranch("fp1", []Branch{BranchMain, BranchDevelop})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil {
		t.Errorf("devia achar baseline develop")
	}
	if got.Branch != BranchDevelop {
		t.Errorf("branch = %s", got.Branch)
	}
}

// Aceitação: SelectBaselineForBranch sem match.
func TestSelectBaselineForBranchNoMatch(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	got, _ := s.SelectBaselineForBranch("fp1", []Branch{BranchMain})
	if got != nil {
		t.Errorf("sem match = nil")
	}
}

// Aceitação: SelectBaselineForBranch empty fp.
func TestSelectBaselineForBranchEmpty(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	if _, err := s.SelectBaselineForBranch("", []Branch{BranchMain}); err == nil {
		t.Errorf("vazio devia falhar")
	}
}

// Aceitação: SelectHighestPriorityBaseline.
func TestSelectHighestPriority(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	r := mkResult("fp1", "http://x", 400*time.Millisecond)
	s.Save(&LoadResultMeta{Result: r, CapturedAt: time.Now(), Branch: BranchDevelop})
	s.Save(&LoadResultMeta{Result: r, CapturedAt: time.Now().Add(time.Second), Branch: BranchMain})
	got, _ := s.SelectHighestPriorityBaseline("fp1")
	if got == nil {
		t.Fatalf("nil")
	}
	if got.Branch != BranchMain {
		t.Errorf("branch = %s, main devia vencer", got.Branch)
	}
}

// Aceitação: SelectHighestPriorityBaseline no match.
func TestSelectHighestPriorityNoMatch(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	got, _ := s.SelectHighestPriorityBaseline("fp1")
	if got != nil {
		t.Errorf("sem match = nil")
	}
}

// Aceitação: CompareDecision sem baseline = Promote.
func TestCompareDecisionNoBaseline(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	d, err := s.CompareDecision(mkResult("fp1", "http://x", 400*time.Millisecond), []Branch{BranchMain}, load.DefaultRegressionThreshold())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d.Kind != DecisionPromote {
		t.Errorf("sem baseline = promote, got %s", d.Kind)
	}
}

// Aceitação: CompareDecision nil current.
func TestCompareDecisionNilCurrent(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	d, _ := s.CompareDecision(nil, []Branch{BranchMain}, load.DefaultRegressionThreshold())
	if d.Kind != DecisionReject {
		t.Errorf("nil = reject")
	}
}

// Aceitação: CompareDecision regressão.
func TestCompareDecisionRegression(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	s.Save(&LoadResultMeta{Result: mkResult("fp1", "http://x", 400*time.Millisecond), CapturedAt: time.Now(), Branch: BranchMain})
	cur := mkResult("fp1", "http://x", 600*time.Millisecond) // +50%
	d, _ := s.CompareDecision(cur, []Branch{BranchMain}, load.DefaultRegressionThreshold())
	if d.Kind != DecisionReject {
		t.Errorf("regressão = reject, got %s", d.Kind)
	}
}

// Aceitação: CompareDecision pass.
func TestCompareDecisionPass(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	s.Save(&LoadResultMeta{Result: mkResult("fp1", "http://x", 400*time.Millisecond), CapturedAt: time.Now(), Branch: BranchMain})
	cur := mkResult("fp1", "http://x", 380*time.Millisecond)
	d, _ := s.CompareDecision(cur, []Branch{BranchMain}, load.DefaultRegressionThreshold())
	if d.Kind != DecisionPromote {
		t.Errorf("pass = promote, got %s", d.Kind)
	}
}

// Aceitação: CompareDecision fingerprints incompatíveis.
func TestCompareDecisionIncompatible(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	s.Save(&LoadResultMeta{Result: mkResult("fp1", "http://x", 400*time.Millisecond), CapturedAt: time.Now(), Branch: BranchMain})
	cur := mkResult("fp2", "http://x", 400*time.Millisecond)
	d, _ := s.CompareDecision(cur, []Branch{BranchMain}, load.DefaultRegressionThreshold())
	if d.Kind != DecisionPromote {
		t.Errorf("sem match = promote, got %s", d.Kind)
	}
}

// Aceitação: PromoteIfBetter.
func TestPromoteIfBetter(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	d := &Decision{Kind: DecisionPromote, Current: mkResult("fp1", "http://x", 380*time.Millisecond)}
	if err := s.PromoteIfBetter(d, "run1", BranchMain, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	got, _ := s.Load("fp1")
	if got.Result.P95 != 380*time.Millisecond {
		t.Errorf("p95 = %v", got.Result.P95)
	}
}

// Aceitação: PromoteIfBetter não promove se baseline é melhor.
func TestPromoteIfBetterNotBetter(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	s.Save(&LoadResultMeta{Result: mkResult("fp1", "http://x", 400*time.Millisecond), CapturedAt: time.Now(), Branch: BranchMain})
	d := &Decision{Kind: DecisionPromote, Baseline: &LoadResultMeta{Result: mkResult("fp1", "http://x", 400*time.Millisecond), Branch: BranchMain}, Current: mkResult("fp1", "http://x", 500*time.Millisecond)}
	if err := s.PromoteIfBetter(d, "run2", BranchMain, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	got, _ := s.Load("fp1")
	if got.Result.P95 != 400*time.Millisecond {
		t.Errorf("baseline preservado = 400ms, got %v", got.Result.P95)
	}
}

// Aceitação: PromoteIfBetter não promove se decision != Promote.
func TestPromoteIfBetterReject(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	d := &Decision{Kind: DecisionReject}
	if err := s.PromoteIfBetter(d, "run1", BranchMain, ""); err == nil {
		t.Errorf("reject não promove")
	}
}

// Aceitação: PromoteIfBetter nil.
func TestPromoteIfBetterNil(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	if err := s.PromoteIfBetter(nil, "run1", BranchMain, ""); err == nil {
		t.Errorf("nil devia falhar")
	}
}

// Aceitação: Validate.
func TestValidate(t *testing.T) {
	m := &LoadResultMeta{Result: mkResult("fp1", "http://x", 400*time.Millisecond)}
	if err := m.Validate(); err != nil {
		t.Errorf("válido: %v", err)
	}
	if err := (&LoadResultMeta{}).Validate(); err == nil {
		t.Errorf("result nil devia falhar")
	}
	if err := (&LoadResultMeta{Result: &load.LoadResult{}}).Validate(); err == nil {
		t.Errorf("fingerprint vazio devia falhar")
	}
	if err := (&LoadResultMeta{Result: &load.LoadResult{Fingerprint: "x"}}).Validate(); err == nil {
		t.Errorf("target vazio devia falhar")
	}
	if err := (&LoadResultMeta{Result: &load.LoadResult{Fingerprint: "x", Target: "t"}}).Validate(); err == nil {
		t.Errorf("P95 zero devia falhar")
	}
}

// Aceitação: ShouldRun.
func TestShouldRun(t *testing.T) {
	if (&LoadResultMeta{}).ShouldRun() {
		t.Errorf("vazio não roda")
	}
	if !(&LoadResultMeta{Result: mkResult("fp", "x", 400*time.Millisecond)}).ShouldRun() {
		t.Errorf("com p95 roda")
	}
}

// Aceitação: AgingDays.
func TestAgingDays(t *testing.T) {
	m := &LoadResultMeta{CapturedAt: time.Now().Add(-48 * time.Hour)}
	if m.AgingDays(time.Now()) < 1.9 || m.AgingDays(time.Now()) > 2.1 {
		t.Errorf("aging = %f", m.AgingDays(time.Now()))
	}
	var nilMeta *LoadResultMeta
	if nilMeta.AgingDays(time.Now()) != 0 {
		t.Errorf("nil pointer aging = 0")
	}
}

// Aceitação: IsStale.
func TestIsStale(t *testing.T) {
	m := &LoadResultMeta{CapturedAt: time.Now().Add(-10 * 24 * time.Hour)}
	if !m.IsStale(time.Now(), 7*24*time.Hour) {
		t.Errorf("10 dias > 7 dias = stale")
	}
	if m.IsStale(time.Now(), 30*24*time.Hour) {
		t.Errorf("10 dias < 30 dias = fresh")
	}
	if !(&LoadResultMeta{}).IsStale(time.Now(), 0) {
		t.Errorf("nil = stale")
	}
}

// Aceitação: DecisionToReason.
func TestDecisionToReason(t *testing.T) {
	if DecisionToReason(nil) != "" {
		t.Errorf("nil = ''")
	}
	if DecisionToReason(&Decision{Reason: "x"}) != "x" {
		t.Errorf("reason")
	}
}

// Aceitação: SortByCapturedAt.
func TestSortByCapturedAt(t *testing.T) {
	now := time.Now()
	bs := []*LoadResultMeta{
		{CapturedAt: now.Add(-time.Hour)},
		{CapturedAt: now},
		{CapturedAt: now.Add(-2 * time.Hour)},
	}
	SortByCapturedAt(bs)
	if bs[0].CapturedAt != now {
		t.Errorf("mais recente primeiro")
	}
}

// Aceitação: FilterByFingerprint.
func TestFilterByFingerprint(t *testing.T) {
	bs := []*LoadResultMeta{
		{Result: mkResult("fp1", "x", 400*time.Millisecond)},
		{Result: mkResult("fp2", "x", 400*time.Millisecond)},
		{Result: mkResult("fp1", "x", 400*time.Millisecond)},
	}
	got := FilterByFingerprint(bs, "fp1")
	if len(got) != 2 {
		t.Errorf("len = %d", len(got))
	}
}

// Aceitação: Path.
func TestPath(t *testing.T) {
	s, _ := NewBaselineStore(t.TempDir())
	p := s.Path("abc")
	if !strings.HasSuffix(p, "abc.json") {
		t.Errorf("suffix: %s", p)
	}
}

// Aceitação: JSON Marshal/Unmarshal.
func TestJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewBaselineStore(dir)
	r := mkResult("fp1", "http://x", 400*time.Millisecond)
	m := &LoadResultMeta{Result: r, CapturedAt: time.Now(), Branch: BranchMain, RefRunID: "run1"}
	s.Save(m)
	data, err := os.ReadFile(filepath.Join(dir, "fp1.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got LoadResultMeta
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Branch != BranchMain {
		t.Errorf("branch")
	}
}
