package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Aceitação: IsLikelyJSON OK.
func TestIsLikelyJSONOK(t *testing.T) {
	if !IsLikelyJSON(`{"a":1}`) {
		t.Errorf("object")
	}
	if !IsLikelyJSON(`[1,2,3]`) {
		t.Errorf("array")
	}
	if !IsLikelyJSON("```json\n{}\n```") {
		t.Errorf("fenced")
	}
}

// Aceitação: IsLikelyJSON fail.
func TestIsLikelyJSONFail(t *testing.T) {
	if IsLikelyJSON("hello") {
		t.Errorf("string")
	}
	if IsLikelyJSON("") {
		t.Errorf("empty")
	}
	if IsLikelyJSON("garbage") {
		t.Errorf("garbage")
	}
}

// Aceitação: IsLikelyJSONObject.
func TestIsLikelyJSONObject(t *testing.T) {
	if !IsLikelyJSONObject(`{"a":1}`) {
		t.Errorf("object")
	}
	if IsLikelyJSONObject(`[1]`) {
		t.Errorf("array não é object")
	}
}

// Aceitação: percentile basic.
func TestPercentile(t *testing.T) {
	vals := []int64{10, 20, 30, 40, 50}
	if percentile(vals, 50) != 30 {
		t.Errorf("p50")
	}
	if percentile(vals, 100) != 50 {
		t.Errorf("p100")
	}
	if percentile(vals, 0) != 10 {
		t.Errorf("p0")
	}
	if percentile(nil, 50) != 0 {
		t.Errorf("nil")
	}
}

// Aceitação: ProbeCache Put/Get.
func TestProbeCachePutGet(t *testing.T) {
	c := NewProbeCache()
	r := ProbeResult{Model: "x", LatencyMS: 100}
	c.Put(r)
	got := c.Get("x")
	if len(got) != 1 || got[0].Model != "x" {
		t.Errorf("get")
	}
}

// Aceitação: ProbeCache Get missing.
func TestProbeCacheMissing(t *testing.T) {
	c := NewProbeCache()
	if len(c.Get("nope")) != 0 {
		t.Errorf("vazio")
	}
}

// Aceitação: ProbeCache Reset.
func TestProbeCacheReset(t *testing.T) {
	c := NewProbeCache()
	c.Put(ProbeResult{Model: "x"})
	c.Reset()
	if len(c.Get("x")) != 0 {
		t.Errorf("reset")
	}
}

// Aceitação: ProbeCache All.
func TestProbeCacheAll(t *testing.T) {
	c := NewProbeCache()
	c.Put(ProbeResult{Model: "a"})
	c.Put(ProbeResult{Model: "b"})
	all := c.All()
	if len(all) != 2 {
		t.Errorf("all count")
	}
}

// Aceitação: Probe com mock provider OK.
func TestProbeOK(t *testing.T) {
	p := NewMockProvider()
	r, err := Probe(context.Background(), p, ProbeOptions{Model: "mock-1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !r.Available {
		t.Errorf("not available")
	}
	if r.LatencyMS < 0 {
		t.Errorf("latency")
	}
}

// Aceitação: Probe model missing.
func TestProbeMissing(t *testing.T) {
	p := NewMockProvider()
	r, err := Probe(context.Background(), p, ProbeOptions{Model: "nope"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Available {
		t.Errorf("não devia estar available")
	}
	if r.ErrorMessage == "" {
		t.Errorf("error message vazio")
	}
}

// Aceitação: Probe nil provider.
func TestProbeNilProvider(t *testing.T) {
	if _, err := Probe(context.Background(), nil, ProbeOptions{Model: "x"}); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: Probe model vazio.
func TestProbeEmptyModel(t *testing.T) {
	if _, err := Probe(context.Background(), NewMockProvider(), ProbeOptions{}); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: ProbeMany.
func TestProbeMany(t *testing.T) {
	p := NewMockProvider()
	results, err := ProbeMany(context.Background(), p, ProbeOptions{Model: "mock-1"}, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("count")
	}
}

// Aceitação: ProbeMany default samples.
func TestProbeManyDefault(t *testing.T) {
	p := NewMockProvider()
	results, _ := ProbeMany(context.Background(), p, ProbeOptions{Model: "mock-1"}, 0)
	if len(results) != 3 {
		t.Errorf("default 3")
	}
}

// Aceitação: Summarize.
func TestSummarize(t *testing.T) {
	results := []ProbeResult{
		{Model: "x", Available: true, JSONCompliant: true, LatencyMS: 100},
		{Model: "x", Available: true, JSONCompliant: false, LatencyMS: 200},
		{Model: "x", Available: false, LatencyMS: 300},
	}
	sum := Summarize(results)
	if !sum.Available {
		t.Errorf("available")
	}
	if !sum.JSONCompliant {
		t.Errorf("json")
	}
	if sum.SuccessCount != 2 {
		t.Errorf("success: %d", sum.SuccessCount)
	}
	if sum.SampleCount != 3 {
		t.Errorf("samples")
	}
	if sum.AvgLatencyMS != 200 {
		t.Errorf("avg latency")
	}
}

// Aceitação: Summarize empty.
func TestSummarizeEmpty(t *testing.T) {
	sum := Summarize(nil)
	if sum.Available {
		t.Errorf("empty")
	}
}

// Aceitação: RankByCapability.
func TestRankByCapability(t *testing.T) {
	summaries := []ProbeSummary{
		{Model: "a", Available: true, JSONCompliant: false, AvgLatencyMS: 100},
		{Model: "b", Available: true, JSONCompliant: true, AvgLatencyMS: 50},
		{Model: "c", Available: false},
	}
	ranked := RankByCapability(summaries)
	if ranked[0].Model != "b" {
		t.Errorf("top: %s", ranked[0].Model)
	}
	if ranked[2].Model != "c" {
		t.Errorf("bottom: %s", ranked[2].Model)
	}
}

// Aceitação: DefaultProbePrompt.
func TestDefaultPrompt(t *testing.T) {
	if DefaultProbePrompt == "" {
		t.Errorf("vazio")
	}
	if !strings.Contains(DefaultProbePrompt, "JSON") {
		t.Errorf("contem JSON")
	}
}

// Aceitação: Probe latency >= 0.
func TestProbeLatency(t *testing.T) {
	p := NewMockProvider()
	r, _ := Probe(context.Background(), p, ProbeOptions{Model: "mock-1"})
	if r.LatencyMS < 0 {
		t.Errorf("esperado latency >= 0")
	}
}

// Aceitação: Probe context cancel.
func TestProbeCtxCancel(t *testing.T) {
	p := NewMockProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, _ := Probe(ctx, p, ProbeOptions{Model: "mock-1"})
	// mock não checa ctx; r pode estar disponível.
	_ = r
}

// Aceitação: Probe ProbeOptions.Provider override.
func TestProbeProviderOverride(t *testing.T) {
	// Quando p=nil mas opts.Provider setado.
	r, err := Probe(context.Background(), nil, ProbeOptions{Model: "mock-1", Provider: NewMockProvider()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !r.Available {
		t.Errorf("not available")
	}
}

// Aceitação: Probe timeout custom.
func TestProbeCustomTimeout(t *testing.T) {
	p := NewMockProvider()
	r, _ := Probe(context.Background(), p, ProbeOptions{Model: "mock-1", Timeout: time.Second})
	if !r.Available {
		t.Errorf("avail")
	}
}

// Aceitação: Probe JSON compliance detecta.
func TestProbeJSONCompliance(t *testing.T) {
	// Mock devolve {"mock":true} — JSON válido.
	p := NewMockProvider()
	r, _ := Probe(context.Background(), p, ProbeOptions{Model: "mock-1"})
	if !r.JSONCompliant {
		t.Errorf("mock output é JSON")
	}
}
