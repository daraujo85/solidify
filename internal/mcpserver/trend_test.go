// Tests para PeerReviewTrend (SAI-125).
package mcpserver

import (
	"testing"
	"time"
)

func TestPeerReviewTrend_Empty(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	trend := ComputeTrend(nil, 30, now, 0.05)
	if trend.WindowDays != 30 {
		t.Errorf("window: %d", trend.WindowDays)
	}
	if trend.Threshold != 0.05 {
		t.Errorf("threshold: %v", trend.Threshold)
	}
	if len(trend.Points) != 30 {
		t.Fatalf("expected 30 points, got %d", len(trend.Points))
	}
	for i, p := range trend.Points {
		if p.Total != 0 || p.V1Fraction != 0 {
			t.Errorf("point[%d] deveria estar zerado: %+v", i, p)
		}
	}
}

func TestPeerReviewTrend_DaysAreOldestFirst(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	trend := ComputeTrend(nil, 30, now, 0.05)
	if trend.Points[0].Date >= trend.Points[29].Date {
		t.Errorf("points[0]=%s devia ser < points[29]=%s",
			trend.Points[0].Date, trend.Points[29].Date)
	}
	// sanity: último ponto = data de `now` (UTC)
	if trend.Points[29].Date != "2026-08-21" {
		t.Errorf("último ponto devia ser 2026-08-21, got %s", trend.Points[29].Date)
	}
}

func TestPeerReviewTrend_SingleBucketWithEvents(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	// 5 v2 events dentro dos últimos 30 dias
	events := []Metric{}
	for i := 0; i < 5; i++ {
		events = append(events, Metric{
			Timestamp: now.Add(-time.Duration(i+1) * 24 * time.Hour),
			Schema:    "2",
		})
	}
	trend := ComputeTrend(events, 30, now, 0.05)
	// algum bucket recente deve ter Total>0 e V2Count>0
	last := trend.Points[29]
	if last.Total == 0 || last.V2Count == 0 {
		t.Errorf("último ponto devia ter eventos: %+v", last)
	}
	if last.V1Fraction != 0 {
		t.Errorf("sem v1 events, fraction devia ser 0, got %v", last.V1Fraction)
	}
	// bucket mais antigo (30 dias atrás) deve estar zerado
	if trend.Points[0].Total != 0 {
		t.Errorf("ponto mais antigo deveria estar vazio (eventos de hoje não contam): %+v", trend.Points[0])
	}
}

func TestPeerReviewTrend_MixedV1V2(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	events := []Metric{
		// 2 v1 antigos (dentro da janela do dia 21)
		{Timestamp: time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC), Schema: "1"},
		{Timestamp: time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC), Schema: "1"},
		// 1 v2 recente
		{Timestamp: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC), Schema: "2"},
	}
	trend := ComputeTrend(events, 30, now, 0.05)
	last := trend.Points[29]
	if last.V1Count != 2 || last.V2Count != 1 {
		t.Errorf("último ponto: esperado v1=2 v2=1, got v1=%d v2=%d",
			last.V1Count, last.V2Count)
	}
	if last.V1Fraction < 0.65 || last.V1Fraction > 0.68 {
		t.Errorf("v1_fraction esperado ≈0.667 (2/3), got %v", last.V1Fraction)
	}
}

func TestPeerReviewTrend_DefaultsDaysTo30(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	trend := ComputeTrend(nil, 0, now, 0.05)
	if trend.WindowDays != 30 {
		t.Errorf("days=0 devia default 30, got %d", trend.WindowDays)
	}
}
