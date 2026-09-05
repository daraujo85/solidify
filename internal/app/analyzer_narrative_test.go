package app

import (
	"strings"
	"testing"

	"github.com/diegoaraujo/solidify/internal/report"
)

// TestBuildReleaseNotes_AggregatesByType: 3 commits de tipos
// diferentes viram 3 ChangeGroup; commit breaking vira BreakingChange;
// executive_summary é fallback determinístico (zero LLM).
func TestBuildReleaseNotes_AggregatesByType(t *testing.T) {
	rn := buildReleaseNotes([]report.Commit{
		{ShortSHA: "aaa1111", Subject: "feat: add login", Type: "feat"},
		{ShortSHA: "bbb2222", Subject: "fix: crash on null", Type: "fix"},
		{ShortSHA: "ccc3333", Subject: "feat!: drop v1 API", Type: "feat", Breaking: true},
	}, nil)

	if len(rn.Groups) != 2 {
		t.Fatalf("esperava 2 groups (feat, fix), got %d", len(rn.Groups))
	}
	if rn.Groups[0].Type != "feat" || len(rn.Groups[0].Items) != 2 {
		t.Errorf("group[0] = %+v", rn.Groups[0])
	}
	if rn.Groups[1].Type != "fix" || len(rn.Groups[1].Items) != 1 {
		t.Errorf("group[1] = %+v", rn.Groups[1])
	}
	if len(rn.BreakingChanges) != 1 || rn.BreakingChanges[0].Title != "feat!: drop v1 API" {
		t.Errorf("breaking = %+v", rn.BreakingChanges)
	}
	if !strings.Contains(rn.ExecutiveSummary, "2 feature(s)") || !strings.Contains(rn.ExecutiveSummary, "1 fix(es)") {
		t.Errorf("executive_summary = %q", rn.ExecutiveSummary)
	}
}

// TestBuildReleaseNotes_Empty: sem commits/migrations → ReleaseNotes
// com arrays vazios (nunca fabricada com placeholder).
func TestBuildReleaseNotes_Empty(t *testing.T) {
	rn := buildReleaseNotes(nil, nil)
	if rn.Groups == nil || rn.BreakingChanges == nil {
		t.Fatalf("esperava slices não-nil vazios, got %+v", rn)
	}
	if len(rn.Groups) != 0 || len(rn.BreakingChanges) != 0 || rn.ExecutiveSummary != "" {
		t.Errorf("esperava tudo vazio, got %+v", rn)
	}
}

// TestBuildNarrativePrompt_EmptyWhenNoEvidence: sem commits, sem
// analyzers, sem SOLID score → prompt vazio (caller não gasta tokens).
func TestBuildNarrativePrompt_EmptyWhenNoEvidence(t *testing.T) {
	if got := buildNarrativePrompt(&report.BuilderInput{RunID: "r1"}); got != "" {
		t.Fatalf("esperava prompt vazio, got %q", got[:60])
	}
}

// TestBuildNarrativePrompt_IncludesScoreAndSolid: prompt inclui o score
// global + 1 linha por princípio aplicável (sem fabricar dados).
func TestBuildNarrativePrompt_IncludesScoreAndSolid(t *testing.T) {
	score := 75.0
	in := &report.BuilderInput{
		RunID: "run-test", Profile: "quick", ProjectName: "demo",
		Git: report.GitInfo{BaseRef: "main", HeadRef: "HEAD"},
		Scores: report.ScoresBlock{Quality: &score, ScoreStatus: "available", Grade: "B"},
		Analyzers: []report.Analyzer{
			{ID: "sonar", Applicability: "APPLICABLE", Score: &score, Findings: []map[string]any{}},
		},
		SOLID: report.SOLIDBlock{
			Principles: map[string]report.Principle{
				"S": {Applicability: "APPLICABLE", Reason: "boa separação"},
				"O": {Applicability: "NOT_APPLICABLE"},
			},
		},
	}
	p := buildNarrativePrompt(in)
	for _, must := range []string{"run-test", "quality=75.0", "sonar", "S: applicability=APPLICABLE", "reason=\"boa separação\""} {
		if !strings.Contains(p, must) {
			t.Errorf("prompt missing %q\n--- prompt ---\n%s", must, p)
		}
	}
}
