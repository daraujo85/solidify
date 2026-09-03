// Package load — load test result normalization.
//
// SAI-048/049: normaliza resultado k6 em LoadResult com
// fingerprint (script+target+config) para comparação baseline.
// Sem baseline = não afirma regressão.
package load

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// LoadResult resultado normalizado cross-run.
type LoadResult struct {
	Script        string        `json:"script"` // path ou "auto"
	Target        string        `json:"target"` // URL
	VUs           int           `json:"vus"`
	Duration      time.Duration `json:"duration_ns"`
	P50           time.Duration `json:"p50_ns"`
	P90           time.Duration `json:"p90_ns"`
	P95           time.Duration `json:"p95_ns"`
	P99           time.Duration `json:"p99_ns"`
	ErrorRate     float64       `json:"error_rate"`
	Requests      int           `json:"requests"`
	Throughput    float64       `json:"throughput_rps"`
	Fingerprint   string        `json:"fingerprint"` // sha256(script+target+vus+duration)
	Timestamp     time.Time     `json:"timestamp"`
	ThresholdOK   bool          `json:"threshold_ok"`
	ThresholdFail []string      `json:"threshold_failed,omitempty"`
}

// Normalize converte dados brutos k6 + metadata em LoadResult.
type NormalizeInput struct {
	Script        string
	Target        string
	VUs           int
	Duration      time.Duration
	P50           time.Duration
	P90           time.Duration
	P95           time.Duration
	P99           time.Duration
	ErrorRate     float64
	Requests      int
	Throughput    float64
	ThresholdOK   bool
	ThresholdFail []string
}

// Normalize monta LoadResult + calcula fingerprint.
func Normalize(in NormalizeInput) (*LoadResult, error) {
	if in.Target == "" {
		return nil, errors.New("load: target vazio")
	}
	r := &LoadResult{
		Script:        in.Script,
		Target:        in.Target,
		VUs:           in.VUs,
		Duration:      in.Duration,
		P50:           in.P50,
		P90:           in.P90,
		P95:           in.P95,
		P99:           in.P99,
		ErrorRate:     in.ErrorRate,
		Requests:      in.Requests,
		Throughput:    in.Throughput,
		Timestamp:     time.Now(),
		ThresholdOK:   in.ThresholdOK,
		ThresholdFail: in.ThresholdFail,
	}
	r.Fingerprint = ComputeFingerprint(in.Script, in.Target, in.VUs, in.Duration)
	return r, nil
}

// ComputeFingerprint hash estável para comparação baseline.
// NÃO inclui metrics — fingerprint identifica o "tipo de run";
// valores podem mudar entre runs.
func ComputeFingerprint(script, target string, vus int, dur time.Duration) string {
	payload := struct {
		Script   string        `json:"script"`
		Target   string        `json:"target"`
		VUs      int           `json:"vus"`
		Duration time.Duration `json:"duration_ns"`
	}{script, target, vus, dur}
	data, _ := json.Marshal(payload)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// CompatibleWith devolve true se fingerprint bate — runs comparáveis.
func (r *LoadResult) CompatibleWith(other *LoadResult) bool {
	if r == nil || other == nil {
		return false
	}
	return r.Fingerprint == other.Fingerprint
}

// DeltaPercent compara dois LoadResults compatíveis; retorna %
// mudança entre métricas (positive = regressão em duration).
type Delta struct {
	P50Delta        float64 `json:"p50_delta"` // %
	P95Delta        float64 `json:"p95_delta"`
	P99Delta        float64 `json:"p99_delta"`
	ThroughputDelta float64 `json:"throughput_delta"` // %
	ErrorRateDelta  float64 `json:"error_rate_delta"` // absoluto (não %)
	Regression      bool    `json:"regression"`       // any regressão > threshold
}

// CompareToBaseline retorna delta entre current vs baseline.
// Caller passa regression thresholds.
type RegressionThreshold struct {
	MaxP95Increase       float64 // ex.: 0.20 = 20% pior
	MaxP99Increase       float64
	MaxErrorRateIncrease float64 // absoluto
	MaxThroughputDrop    float64 // negativo é drop (ex.: 0.15 = 15% drop)
}

// DefaultRegressionThreshold devolve defaults.
func DefaultRegressionThreshold() RegressionThreshold {
	return RegressionThreshold{
		MaxP95Increase:       0.20,
		MaxP99Increase:       0.30,
		MaxErrorRateIncrease: 0.02,
		MaxThroughputDrop:    0.15,
	}
}

// CompareToBaseline calcula deltas. baseline deve ser CompatibleWith.
func CompareToBaseline(current, baseline *LoadResult, th RegressionThreshold) (*Delta, error) {
	if !current.CompatibleWith(baseline) {
		return nil, errors.New("load: fingerprints incompatíveis")
	}
	if baseline.P50 == 0 || baseline.P95 == 0 || baseline.P99 == 0 {
		return nil, errors.New("load: baseline sem métricas")
	}
	d := &Delta{}
	d.P50Delta = percentChange(baseline.P50, current.P50)
	d.P95Delta = percentChange(baseline.P95, current.P95)
	d.P99Delta = percentChange(baseline.P99, current.P99)
	if baseline.Throughput > 0 {
		d.ThroughputDelta = percentChange(baseline.Throughput, current.Throughput)
	}
	d.ErrorRateDelta = current.ErrorRate - baseline.ErrorRate
	// Regressão?
	if d.P95Delta > th.MaxP95Increase {
		d.Regression = true
	}
	if d.P99Delta > th.MaxP99Increase {
		d.Regression = true
	}
	if d.ErrorRateDelta > th.MaxErrorRateIncrease {
		d.Regression = true
	}
	if d.ThroughputDelta < -th.MaxThroughputDrop {
		d.Regression = true
	}
	return d, nil
}

// percentChange helper: (current - baseline) / baseline.
// Retorna 0 se baseline = 0.
func percentChange[T ~int64 | ~float64](base, current T) float64 {
	if base == 0 {
		return 0
	}
	return float64(current-base) / float64(base)
}

// SummaryStats snapshot rápido.
type SummaryStats struct {
	Pass       bool    `json:"pass"`
	P95Ms      int64   `json:"p95_ms"`
	P99Ms      int64   `json:"p99_ms"`
	Throughput float64 `json:"throughput_rps"`
	ErrorRate  float64 `json:"error_rate"`
}

// Summary devolve snapshot rápido.
func (r *LoadResult) Summary() SummaryStats {
	return SummaryStats{
		Pass:       r.ThresholdOK,
		P95Ms:      r.P95.Milliseconds(),
		P99Ms:      r.P99.Milliseconds(),
		Throughput: r.Throughput,
		ErrorRate:  r.ErrorRate,
	}
}

// EqualMetadata compara apenas campos metadata (não métricas).
// Útil pra validar que dois runs são do "mesmo setup".
func (r *LoadResult) EqualMetadata(other *LoadResult) bool {
	if r == nil || other == nil {
		return false
	}
	return r.Script == other.Script &&
		r.Target == other.Target &&
		r.VUs == other.VUs &&
		r.Duration == other.Duration
}

// MarshalJSON garante timestamp em UTC ISO8601.
func (r *LoadResult) MarshalJSON2() ([]byte, error) {
	type Alias LoadResult
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp_iso"`
		*Alias
	}{
		Timestamp: r.Timestamp.UTC().Format(time.RFC3339Nano),
		Alias:     (*Alias)(r),
	})
}

// SortByP95 ordena results pelo P95 asc.
func SortByP95(rs []*LoadResult) {
	sort.Slice(rs, func(i, j int) bool {
		return rs[i].P95 < rs[j].P95
	})
}

// AggregateStats agrega múltiplos LoadResults compatíveis (mesmo fingerprint).
func AggregateStats(rs []*LoadResult) *Delta {
	if len(rs) == 0 {
		return &Delta{}
	}
	// Median P95, median throughput.
	sorted := make([]*LoadResult, len(rs))
	copy(sorted, rs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].P95 < sorted[j].P95 })
	medP95 := sorted[len(sorted)/2].P95
	d := &Delta{}
	d.P95Delta = float64(medP95.Milliseconds())
	return d
}

// String helper.
func (r *LoadResult) String() string {
	return fmt.Sprintf("LoadResult{target=%s vus=%d p95=%v err=%.3f req=%d fingerprint=%s}",
		r.Target, r.VUs, r.P95, r.ErrorRate, r.Requests, r.Fingerprint[:8])
}

// IsCompatibleWith é alias de CompatibleWith (preferir esse).
func (r *LoadResult) IsCompatibleWith(other *LoadResult) bool {
	return r.CompatibleWith(other)
}
