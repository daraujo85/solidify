package lighthouse

// PillarThreshold configura gate por pilar frontend.
//
// SAI-045: thresholds por pilar — só bloqueia se pillar está
// configurado (weight > 0). SEO default informativo (weight=0).
type PillarThreshold struct {
	MinPerformance   Score `json:"min_performance"`
	MinAccessibility Score `json:"min_accessibility"`
	MinBestPractices Score `json:"min_best_practices"`
	MinSEO           Score `json:"min_seo"` // default 0 (informativo)
	MinAggregate     Score `json:"min_aggregate"`
}

// DefaultPillarThreshold devolve gate conservador: pilares chave
// ≥ 0.9, SEO informativo (sem min).
func DefaultPillarThreshold() PillarThreshold {
	return PillarThreshold{
		MinPerformance:   0.9,
		MinAccessibility: 0.9,
		MinBestPractices: 0.9,
		MinSEO:           0,
		MinAggregate:     0.8,
	}
}

// LenientPillarThreshold gate permissivo: 0.5 em tudo.
func LenientPillarThreshold() PillarThreshold {
	return PillarThreshold{
		MinPerformance:   0.5,
		MinAccessibility: 0.5,
		MinBestPractices: 0.5,
		MinSEO:           0,
		MinAggregate:     0.5,
	}
}

// PillarGateResult decisão.
type PillarGateResult struct {
	Allow         bool     `json:"allow"`
	Reason        string   `json:"reason,omitempty"`
	Failed        []string `json:"failed,omitempty"`  // pillars que falharam
	Skipped       bool     `json:"skipped,omitempty"` // true se pillar foi skip (backend-only)
	SkippedReason string   `json:"skipped_reason,omitempty"`
}

// EvaluatePillarGate avalia Report contra threshold.
// Report com Mode=Disabled → Skipped=true, Allow=true (não bloqueia).
func EvaluatePillarGate(rep *Report, t PillarThreshold) PillarGateResult {
	if rep == nil {
		return PillarGateResult{Allow: true, Skipped: true, SkippedReason: "report nil"}
	}
	if rep.Mode == ModeDisabled {
		return PillarGateResult{
			Allow:         true,
			Skipped:       true,
			SkippedReason: rep.Error,
		}
	}
	var failed []string
	if t.MinAggregate > 0 && rep.AggregateScore < t.MinAggregate {
		failed = append(failed, "aggregate")
	}
	if t.MinPerformance > 0 && rep.Pillar.Performance > 0 && rep.Pillar.Performance < t.MinPerformance {
		failed = append(failed, "performance")
	}
	if t.MinAccessibility > 0 && rep.Pillar.Accessibility > 0 && rep.Pillar.Accessibility < t.MinAccessibility {
		failed = append(failed, "accessibility")
	}
	if t.MinBestPractices > 0 && rep.Pillar.BestPractices > 0 && rep.Pillar.BestPractices < t.MinBestPractices {
		failed = append(failed, "best-practices")
	}
	if t.MinSEO > 0 && rep.Pillar.SEO > 0 && rep.Pillar.SEO < t.MinSEO {
		failed = append(failed, "seo")
	}
	if len(failed) > 0 {
		return PillarGateResult{
			Allow:  false,
			Reason: "pillars abaixo do threshold: " + joinStrings(failed, ","),
			Failed: failed,
		}
	}
	return PillarGateResult{Allow: true, Reason: "all pillars passed"}
}

// joinStrings helper sem importar strings (evita conflito com tests).
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}

// GateLenient devolve true se report passa no gate lenient.
func GateLenient(rep *Report) bool {
	return EvaluatePillarGate(rep, LenientPillarThreshold()).Allow
}

// GateDefault devolve true se report passa no gate default.
func GateDefault(rep *Report) bool {
	return EvaluatePillarGate(rep, DefaultPillarThreshold()).Allow
}

// WorstPillar devolve o pilar com menor score (entre os não-zero).
func WorstPillar(p Pillar) (CategoryKey, Score) {
	worst := Score(2.0)
	var key CategoryKey
	if p.Performance > 0 && p.Performance < worst {
		worst = p.Performance
		key = CatPerformance
	}
	if p.Accessibility > 0 && p.Accessibility < worst {
		worst = p.Accessibility
		key = CatAccessibility
	}
	if p.BestPractices > 0 && p.BestPractices < worst {
		worst = p.BestPractices
		key = CatBestPractices
	}
	if p.SEO > 0 && p.SEO < worst {
		worst = p.SEO
		key = CatSEO
	}
	if p.PWA > 0 && p.PWA < worst {
		worst = p.PWA
		key = CatPWA
	}
	if key == "" {
		return "", 0
	}
	return key, worst
}

// AggregatePercent helper — score 0-1 → 0-100, retorna 0 se vazio.
func AggregatePercent(rep *Report) int {
	if rep == nil {
		return 0
	}
	return rep.AggregateScore.ToPercent()
}
