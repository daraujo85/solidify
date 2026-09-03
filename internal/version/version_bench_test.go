package version

import "testing"

// Baseline de custo do caminho mais barato da CLI. Os benchmarks realistas
// (1k changed files, diff de 10 MB, etc.) entram em SAI-096.
func BenchmarkGet(b *testing.B) {
	for b.Loop() {
		_ = Get()
	}
}

func BenchmarkInfoString(b *testing.B) {
	info := Get()
	for b.Loop() {
		_ = info.String()
	}
}
