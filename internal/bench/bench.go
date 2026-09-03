// Benchmark fixtures (SAI-096).
//
// Cenários: 1k files, 10MB diff, monorepo. Gera inputs
// sintéticos p/ pillar score, SOLID snapshot, robustness
// comparator.
package bench

import (
	"fmt"
	"math/rand"

	"github.com/diegoaraujo/solidify/internal/peer"
	"github.com/diegoaraujo/solidify/internal/score"
)

// Fixture sizes.
const (
	SmallFiles  = 100
	MediumFiles = 1000
	LargeFiles  = 5000
	Monorepo    = 10000
	Diff10MB    = 10 * 1024 * 1024
)

// FixtureProfile descreve cenário.
type FixtureProfile struct {
	Name      string
	Files     int
	DiffBytes int
}

// StandardProfiles cenários canônicos.
var StandardProfiles = []FixtureProfile{
	{"1k_files", MediumFiles, Diff10MB / 10},
	{"10MB_diff", MediumFiles, Diff10MB},
	{"monorepo", Monorepo, Diff10MB * 5},
}

// GeneratePillarInput sintético.
func GeneratePillarInput(n int, seed int64) score.PillarInput {
	r := rand.New(rand.NewSource(seed))
	pillars := []score.PillarScore{}
	for i := 0; i < n; i++ {
		pillars = append(pillars, score.PillarScore{
			Pillar:     score.Pillars[i%len(score.Pillars)],
			Score:      r.Float64() * 100,
			Weight:     1.0 / float64(n),
			Applicable: r.Float64() > 0.1,
		})
	}
	return score.PillarInput{Pillars: pillars}
}

// GenerateSOLIDSnapshot sintético (5 princípios × N).
func GenerateSOLIDSnapshot(n int, seed int64) score.Snapshot {
	r := rand.New(rand.NewSource(seed))
	letters := []score.LetterScore{}
	for _, p := range score.Principles {
		for i := 0; i < n; i++ {
			letters = append(letters, score.LetterScore{
				Principle:  p,
				Score:      r.Float64() * 100,
				Applicable: r.Float64() > 0.1,
			})
		}
	}
	return score.Snapshot{
		RunID:   "bench",
		Label:   "bench",
		Letters: letters,
	}
}

// GenerateFindingsLite sintético.
func GenerateFindingsLite(n int, seed int64) []peer.RobustFindingLite {
	r := rand.New(rand.NewSource(seed))
	fs := make([]peer.RobustFindingLite, n)
	for i := 0; i < n; i++ {
		fs[i] = peer.RobustFindingLite{
			ID:       fmt.Sprintf("f-%d", i),
			Symbol:   "x",
			Severity: []string{"low", "medium", "high"}[r.Intn(3)],
		}
	}
	return fs
}

// GenerateRunSnapshot sintético.
func GenerateRunSnapshot(id string, seed int64) *peer.RunSnapshot {
	r := rand.New(rand.NewSource(seed))
	return &peer.RunSnapshot{
		RunID:      id,
		Score:      r.Float64() * 100,
		GateStatus: []string{"PASS", "WARN", "FAIL"}[r.Intn(3)],
		Findings:   GenerateFindingsLite(100, seed),
		SOLID:      GenerateSOLIDSnapshot(10, seed),
	}
}

// GenerateDiffBytes texto pseudo-aleatório de N bytes.
func GenerateDiffBytes(n int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	out := make([]byte, n)
	chars := []byte("abcdefghijklmnopqrstuvwxyz\n")
	for i := 0; i < n; i++ {
		out[i] = chars[r.Intn(len(chars))]
	}
	return out
}
