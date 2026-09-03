package load

import (
	"strings"
	"testing"
	"time"
)

// Aceitação: Normalize básico.
func TestNormalizeBasic(t *testing.T) {
	r, err := Normalize(NormalizeInput{
		Script: "test.js", Target: "http://x", VUs: 10, Duration: 30 * time.Second,
		P95: 400 * time.Millisecond, P99: 800 * time.Millisecond,
		ErrorRate: 0.001, Requests: 1000, Throughput: 33.3,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Target != "http://x" {
		t.Errorf("target")
	}
	if r.Fingerprint == "" {
		t.Errorf("fingerprint vazio")
	}
}

// Aceitação: Normalize sem target.
func TestNormalizeNoTarget(t *testing.T) {
	if _, err := Normalize(NormalizeInput{}); err == nil {
		t.Errorf("sem target devia falhar")
	}
}

// Aceitação: ComputeFingerprint estável.
func TestComputeFingerprintStable(t *testing.T) {
	f1 := ComputeFingerprint("a.js", "http://x", 10, 30*time.Second)
	f2 := ComputeFingerprint("a.js", "http://x", 10, 30*time.Second)
	if f1 != f2 {
		t.Errorf("mesmo input mesmo fingerprint")
	}
}

// Aceitação: ComputeFingerprint sensível a mudanças.
func TestComputeFingerprintSensitive(t *testing.T) {
	f1 := ComputeFingerprint("a.js", "http://x", 10, 30*time.Second)
	f2 := ComputeFingerprint("b.js", "http://x", 10, 30*time.Second)
	if f1 == f2 {
		t.Errorf("script diferente devia mudar hash")
	}
	f3 := ComputeFingerprint("a.js", "http://y", 10, 30*time.Second)
	if f1 == f3 {
		t.Errorf("target diferente devia mudar hash")
	}
	f4 := ComputeFingerprint("a.js", "http://x", 20, 30*time.Second)
	if f1 == f4 {
		t.Errorf("vus diferentes devia mudar hash")
	}
	f5 := ComputeFingerprint("a.js", "http://x", 10, 60*time.Second)
	if f1 == f5 {
		t.Errorf("duração diferente devia mudar hash")
	}
}

// Aceitação: CompatibleWith.
func TestCompatibleWith(t *testing.T) {
	r1 := &LoadResult{Fingerprint: "abc"}
	r2 := &LoadResult{Fingerprint: "abc"}
	r3 := &LoadResult{Fingerprint: "xyz"}
	if !r1.CompatibleWith(r2) {
		t.Errorf("mesmo fingerprint devia bater")
	}
	if r1.CompatibleWith(r3) {
		t.Errorf("diferente devia falhar")
	}
	if r1.CompatibleWith(nil) {
		t.Errorf("nil")
	}
}

// Aceitação: CompareToBaseline sem regressão.
func TestCompareToBaselineNoRegression(t *testing.T) {
	base := &LoadResult{
		Fingerprint: "fp", P50: 100 * time.Millisecond, P95: 400 * time.Millisecond,
		P99: 800 * time.Millisecond, Throughput: 100, ErrorRate: 0.001,
	}
	cur := &LoadResult{
		Fingerprint: "fp", P50: 105 * time.Millisecond, P95: 410 * time.Millisecond,
		P99: 820 * time.Millisecond, Throughput: 98, ErrorRate: 0.002,
	}
	d, err := CompareToBaseline(cur, base, DefaultRegressionThreshold())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d.Regression {
		t.Errorf("variação pequena não devia flagar: %+v", d)
	}
}

// Aceitação: CompareToBaseline com regressão P95.
func TestCompareToBaselineRegressionP95(t *testing.T) {
	base := &LoadResult{
		Fingerprint: "fp", P50: 200 * time.Millisecond, P95: 400 * time.Millisecond, P99: 800 * time.Millisecond,
		Throughput: 100, ErrorRate: 0.001,
	}
	cur := &LoadResult{
		Fingerprint: "fp", P50: 200 * time.Millisecond, P95: 600 * time.Millisecond, P99: 800 * time.Millisecond,
		Throughput: 100, ErrorRate: 0.001,
	}
	d, err := CompareToBaseline(cur, base, DefaultRegressionThreshold())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !d.Regression {
		t.Errorf("P95 +50%% devia ser regressão")
	}
}

// Aceitação: CompareToBaseline regression error rate.
func TestCompareToBaselineRegressionErr(t *testing.T) {
	base := &LoadResult{
		Fingerprint: "fp", P50: 200 * time.Millisecond, P95: 400 * time.Millisecond, P99: 800 * time.Millisecond,
		Throughput: 100, ErrorRate: 0.001,
	}
	cur := &LoadResult{
		Fingerprint: "fp", P50: 200 * time.Millisecond, P95: 400 * time.Millisecond, P99: 800 * time.Millisecond,
		Throughput: 100, ErrorRate: 0.05,
	}
	d, err := CompareToBaseline(cur, base, DefaultRegressionThreshold())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !d.Regression {
		t.Errorf("+5%% err devia ser regressão")
	}
}

// Aceitação: CompareToBaseline throughput drop.
func TestCompareToBaselineThroughputDrop(t *testing.T) {
	base := &LoadResult{
		Fingerprint: "fp", P50: 200 * time.Millisecond, P95: 400 * time.Millisecond, P99: 800 * time.Millisecond,
		Throughput: 100, ErrorRate: 0.001,
	}
	cur := &LoadResult{
		Fingerprint: "fp", P50: 200 * time.Millisecond, P95: 400 * time.Millisecond, P99: 800 * time.Millisecond,
		Throughput: 50, ErrorRate: 0.001, // 50% drop
	}
	d, err := CompareToBaseline(cur, base, DefaultRegressionThreshold())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !d.Regression {
		t.Errorf("throughput drop 50%% devia flagar")
	}
}

// Aceitação: CompareToBaseline fingerprints incompatíveis.
func TestCompareToBaselineIncompatible(t *testing.T) {
	base := &LoadResult{Fingerprint: "fp"}
	cur := &LoadResult{Fingerprint: "other"}
	if _, err := CompareToBaseline(cur, base, DefaultRegressionThreshold()); err == nil {
		t.Errorf("incompatíveis devia falhar")
	}
}

// Aceitação: CompareToBaseline baseline sem métricas.
func TestCompareToBaselineNoMetrics(t *testing.T) {
	base := &LoadResult{Fingerprint: "fp"}
	cur := &LoadResult{Fingerprint: "fp"}
	if _, err := CompareToBaseline(cur, base, DefaultRegressionThreshold()); err == nil {
		t.Errorf("sem métricas devia falhar")
	}
}

// Aceitação: percentChange.
func TestPercentChange(t *testing.T) {
	if percentChange(int64(100), int64(150)) != 0.5 {
		t.Errorf("+50%%")
	}
	if percentChange(int64(100), int64(50)) != -0.5 {
		t.Errorf("-50%%")
	}
	if percentChange(int64(0), int64(50)) != 0 {
		t.Errorf("zero base")
	}
	if percentChange(float64(100), float64(120)) != 0.2 {
		t.Errorf("+20%%")
	}
}

// Aceitação: DefaultRegressionThreshold.
func TestDefaultRegressionThreshold(t *testing.T) {
	th := DefaultRegressionThreshold()
	if th.MaxP95Increase <= 0 || th.MaxP99Increase <= 0 {
		t.Errorf("thresholds vazios")
	}
}

// Aceitação: EqualMetadata.
func TestEqualMetadata(t *testing.T) {
	r1 := &LoadResult{Script: "a", Target: "t", VUs: 10, Duration: 30 * time.Second}
	r2 := &LoadResult{Script: "a", Target: "t", VUs: 10, Duration: 30 * time.Second}
	r3 := &LoadResult{Script: "b", Target: "t", VUs: 10, Duration: 30 * time.Second}
	if !r1.EqualMetadata(r2) {
		t.Errorf("mesmo metadata")
	}
	if r1.EqualMetadata(r3) {
		t.Errorf("script diferente")
	}
	if r1.EqualMetadata(nil) {
		t.Errorf("nil")
	}
}

// Aceitação: Summary.
func TestLoadResultSummary(t *testing.T) {
	r := &LoadResult{P95: 400 * time.Millisecond, P99: 800 * time.Millisecond, Throughput: 33.3, ErrorRate: 0.01}
	s := r.Summary()
	if s.P95Ms != 400 {
		t.Errorf("p95ms = %v", s.P95Ms)
	}
	if s.P99Ms != 800 {
		t.Errorf("p99ms = %v", s.P99Ms)
	}
}

// Aceitação: SortByP95.
func TestSortByP95(t *testing.T) {
	rs := []*LoadResult{
		{P95: 500 * time.Millisecond},
		{P95: 200 * time.Millisecond},
		{P95: 800 * time.Millisecond},
	}
	SortByP95(rs)
	if rs[0].P95 != 200*time.Millisecond {
		t.Errorf("asc")
	}
}

// Aceitação: AggregateStats.
func TestAggregateStats(t *testing.T) {
	rs := []*LoadResult{
		{P95: 200 * time.Millisecond},
		{P95: 400 * time.Millisecond},
		{P95: 600 * time.Millisecond},
	}
	d := AggregateStats(rs)
	if d.P95Delta != 400 {
		t.Errorf("median = 400ms, got %v", d.P95Delta)
	}
}

// Aceitação: AggregateStats empty.
func TestAggregateStatsEmpty(t *testing.T) {
	d := AggregateStats(nil)
	if d.P95Delta != 0 {
		t.Errorf("vazio = 0")
	}
}

// Aceitação: String().
func TestLoadResultString(t *testing.T) {
	r := &LoadResult{
		Target: "http://x", VUs: 10, P95: 400 * time.Millisecond,
		Requests: 100, Fingerprint: "abcdefghij",
	}
	s := r.String()
	if !strings.Contains(s, "http://x") {
		t.Errorf("target")
	}
	if !strings.Contains(s, "abcdefgh") {
		t.Errorf("fingerprint prefix")
	}
}

// Aceitação: IsCompatibleWith alias.
func TestIsCompatibleWith(t *testing.T) {
	r1 := &LoadResult{Fingerprint: "x"}
	r2 := &LoadResult{Fingerprint: "x"}
	if !r1.IsCompatibleWith(r2) {
		t.Errorf("alias")
	}
}

// Aceitação: Timestamp populated.
func TestNormalizeTimestamp(t *testing.T) {
	r, _ := Normalize(NormalizeInput{Target: "x"})
	if r.Timestamp.IsZero() {
		t.Errorf("timestamp vazio")
	}
}

// Aceitação: ThresholdFail preservado.
func TestNormalizeThresholdFail(t *testing.T) {
	r, _ := Normalize(NormalizeInput{
		Target: "x", ThresholdFail: []string{"p(95)"},
	})
	if len(r.ThresholdFail) != 1 {
		t.Errorf("preservado")
	}
	if r.ThresholdOK {
		t.Errorf("ThresholdFail != OK")
	}
}

// Aceitação: MarshalJSON2.
func TestMarshalJSON2(t *testing.T) {
	r, _ := Normalize(NormalizeInput{Target: "x"})
	data, err := r.MarshalJSON2()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(data), "timestamp_iso") {
		t.Errorf("timestamp_iso field")
	}
}
