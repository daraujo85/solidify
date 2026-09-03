package bench

import (
	"testing"

	"github.com/diegoaraujo/solidify/internal/peer"
	"github.com/diegoaraujo/solidify/internal/score"
)

// Bench pillar score com 1k entries.
func BenchmarkPillarScore_1k(b *testing.B) {
	in := GeneratePillarInput(1000, 42)
	in.RunID = "bench"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = score.ComputeGlobalScore(in)
	}
}

// Bench pillar score com 10k entries (monorepo).
func BenchmarkPillarScore_10k(b *testing.B) {
	in := GeneratePillarInput(10000, 42)
	in.RunID = "bench"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = score.ComputeGlobalScore(in)
	}
}

// Bench robustness compare (100 findings each).
func BenchmarkRobustnessCompare_100(b *testing.B) {
	orig := GenerateRunSnapshot("orig", 42)
	sw := GenerateRunSnapshot("swap", 43)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = peer.Compare(orig, sw)
	}
}

// Bench findings overlap puro (500 findings).
func BenchmarkFindingsOverlap_500(b *testing.B) {
	a := GenerateFindingsLite(500, 42)
	bb := GenerateFindingsLite(500, 43)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = peer.FindingsOverlapPublic(a, bb)
	}
}

// Bench diff byte generation (10MB).
func BenchmarkGenerateDiff_10MB(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateDiffBytes(Diff10MB, 42)
	}
}
