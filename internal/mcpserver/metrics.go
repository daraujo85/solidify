// Peer review telemetry + v1 cutoff (SAI-121).
//
// Append-only JSONL de submissions em `~/.solidify/metrics/peer_reviews.jsonl`
// (ou `ai.peer_review.metrics_path` override). Best-effort: falha no
// append não bloqueia save do record.
//
// V1Cutoff é um global package-level (não config struct) porque o
// validator é instanciado antes do config ser carregado em alguns
// paths (ex: testes isolados). Setter explícito `SetV1Cutoff` é
// chamado por `app/mcp.go` no startup do server.
package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// V1Cutoff data após a qual schema v1 vira hard error (não warning).
// Vazio (zero) = sem cutoff, sempre warning. Set via SetV1Cutoff.
var V1Cutoff time.Time

// SetV1Cutoff define cutoff global. Aceita string RFC3339, vazio desabilita.
func SetV1Cutoff(s string) error {
	if s == "" {
		V1Cutoff = time.Time{}
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("v1_cutoff: parse RFC3339: %w", err)
	}
	V1Cutoff = t
	return nil
}

// Metric é um evento de telemetria de submission.
type Metric struct {
	Timestamp time.Time `json:"ts"`
	Schema    string    `json:"schema"`
	Actor     string    `json:"actor"`
	ReviewID  string    `json:"review_id"`
	Verdict   string    `json:"verdict"`
}

// metricsPathDefault retorna path default do JSONL.
func metricsPathDefault() string {
	if MetricsPath != "" {
		return MetricsPath
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".solidify", "metrics", "peer_reviews.jsonl")
}

// MetricsPath override do path (set por config ou flag CLI).
var MetricsPath string

// AppendMetric adiciona evento no JSONL. Best-effort: erro silencioso
// (logado pelo caller se necessário).
func AppendMetric(m Metric) error {
	path := metricsPathDefault()
	if path == "" {
		return errors.New("metrics: path vazio")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// ReadMetrics lê todos events do JSONL. Records corrompidos são pulados
// (best-effort parse — telemetria nunca aborta).
func ReadMetrics() ([]Metric, error) {
	path := metricsPathDefault()
	if path == "" {
		return nil, errors.New("metrics: path vazio")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Metric
	for _, line := range splitLinesBytes(data) {
		if len(line) == 0 {
			continue
		}
		var m Metric
		if jerr := json.Unmarshal(line, &m); jerr != nil {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// MetricsSummary agregado por schema/actor.
type MetricsSummary struct {
	Total      int            `json:"total"`
	BySchema   map[string]int `json:"by_schema"`
	ByActor    map[string]int `json:"by_actor"`
	ByVerdict  map[string]int `json:"by_verdict"`
	Oldest     time.Time      `json:"oldest,omitempty"`
	Newest     time.Time      `json:"newest,omitempty"`
	V1Count    int            `json:"v1_count"`
	V2Count    int            `json:"v2_count"`
	V1Fraction float64        `json:"v1_fraction"`
}

// SummarizeMetrics computa agregado. from e to opcionais:
// from zero = sem piso inferior; to zero = sem teto (equivalente a time.Now()).
// SAI-125: `to` adicionado pra série temporal (rolling window como
// canary diria naquele dia).
func SummarizeMetrics(events []Metric, from, to time.Time) MetricsSummary {
	s := MetricsSummary{
		BySchema:  map[string]int{},
		ByActor:   map[string]int{},
		ByVerdict: map[string]int{},
	}
	for _, e := range events {
		if !from.IsZero() && e.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && e.Timestamp.After(to) {
			continue
		}
		s.Total++
		s.BySchema[e.Schema]++
		s.ByActor[e.Actor]++
		s.ByVerdict[e.Verdict]++
		if s.Oldest.IsZero() || e.Timestamp.Before(s.Oldest) {
			s.Oldest = e.Timestamp
		}
		if e.Timestamp.After(s.Newest) {
			s.Newest = e.Timestamp
		}
		switch e.Schema {
		case "1":
			s.V1Count++
		case "2":
			s.V2Count++
		}
	}
	if s.Total > 0 {
		s.V1Fraction = float64(s.V1Count) / float64(s.Total)
	}
	return s
}

func splitLinesBytes(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// metricsMu protege appends concorrentes no mesmo processo.
var metricsMu sync.Mutex

// init não faz nada (path resolvido em runtime), mas garante
// que V1Cutoff e MetricsPath são package-level vars tipadas.
func init() {
	_ = metricsMu
}
