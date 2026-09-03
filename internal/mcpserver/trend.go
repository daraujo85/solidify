// Peer review v1_fraction trend over time (SAI-125).
//
// Computa série de `days` pontos diários. Cada ponto D
// representa o `v1_fraction` rolling de 30 dias "como canary
// computaria naquele instante" — `SummarizeMetrics(events,
// D-30d, D)`. Último ponto usa `now` diretamente, então bate
// com `peer_review_canary` no momento da requisição.
//
// Sem nova storage — computa on-demand do JSONL append-only
// (ADR-0121 §1). Volume baixo (~1 evento/run), binning
// O(N) único passe. Threshold = doctor.CanaryFractionThreshold
// pra UI renderizar a mesma linha que canary usa.
package mcpserver

import "time"

// TrendPoint 1 dia da série. Date em UTC (YYYY-MM-DD).
type TrendPoint struct {
	Date       string  `json:"date"` // YYYY-MM-DD (UTC)
	V1Count    int     `json:"v1_count"`
	V2Count    int     `json:"v2_count"`
	Total      int     `json:"total"`
	V1Fraction float64 `json:"v1_fraction"` // 0 se Total==0
}

// PeerReviewTrend série completa. Points em ordem cronológica
// (oldest → newest), length == WindowDays.
type PeerReviewTrend struct {
	WindowDays int          `json:"window_days"`
	Threshold  float64      `json:"threshold"`
	Points     []TrendPoint `json:"points"`
}

// ComputeTrend computa série de `days` pontos. `now` é o
// "fim" da série (ponto mais recente). Cada ponto usa janela
// rolling de 30 dias — alinhado com `peer_review_canary`.
//
// Zero-value em `now` é tratado como `time.Now().UTC()`.
// `days <= 0` é tratado como 30.
func ComputeTrend(events []Metric, days int, now time.Time, threshold float64) PeerReviewTrend {
	if days <= 0 {
		days = 30
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	const (
		dayDur        = 24 * time.Hour
		rollingWindow = 30 * 24 * time.Hour
	)
	points := make([]TrendPoint, days)
	for i := 0; i < days; i++ {
		// i=0 = mais antigo (now - (days-1)*day), i=days-1 = now (most recent)
		dayEnd := now.Add(-time.Duration(days-1-i) * dayDur)
		dayStart := dayEnd.Add(-rollingWindow)
		s := SummarizeMetrics(events, dayStart, dayEnd)
		fraction := 0.0
		if s.Total > 0 {
			fraction = s.V1Fraction
		}
		points[i] = TrendPoint{
			Date:       dayEnd.UTC().Format("2006-01-02"),
			V1Count:    s.V1Count,
			V2Count:    s.V2Count,
			Total:      s.Total,
			V1Fraction: fraction,
		}
	}
	return PeerReviewTrend{
		WindowDays: days,
		Threshold:  threshold,
		Points:     points,
	}
}
