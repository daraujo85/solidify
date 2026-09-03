package app

import (
	"testing"

	"github.com/diegoaraujo/solidify/internal/arbiter"
	"github.com/diegoaraujo/solidify/internal/peer"
)

// TestBuildResolvedSolid_ArbiterOverridesPrincipleGolden é a regressão
// direta do achado do plano SAI-129 §0: peer_a e peer_b divergem em S
// (applicable=false vs true); o arbiter resolve S como APPLICABLE com
// score próprio, aceitando peer_b. O princípio O (sem disputa) precisa
// continuar vindo do consenso do peer (peer_b, por precedência).
func TestBuildResolvedSolid_ArbiterOverridesPrincipleGolden(t *testing.T) {
	naNode := map[string]any{"applicability": peer.ApplicabilityNotApplicable, "score": nil}
	peerA := &peer.ExecutorResult{ParsedContent: map[string]any{
		"solid": map[string]any{
			"S": map[string]any{"applicability": peer.ApplicabilityNotApplicable, "score": nil},
			"O": map[string]any{"applicability": peer.ApplicabilityApplicable, "score": 40.0},
			"L": naNode, "I": naNode, "D": naNode,
		},
	}}
	peerB := &peer.ExecutorResult{ParsedContent: map[string]any{
		"solid": map[string]any{
			"S": map[string]any{"applicability": peer.ApplicabilityApplicable, "score": 30.0},
			"O": map[string]any{"applicability": peer.ApplicabilityApplicable, "score": 72.0},
			"L": naNode, "I": naNode, "D": naNode,
		},
	}}
	arb := &arbiter.ExecutorResult{Verdict: &arbiter.Verdict{
		ApplicabilityVerdicts: []arbiter.ApplicabilityVerdict{
			{Principle: "S", Applicability: peer.ApplicabilityApplicable, EvidenceRefs: []string{"a.go:1"}},
		},
		ScoreVerdicts: []arbiter.ScoreVerdict{
			{Principle: "S", Decision: arbiter.DecisionAcceptPeerB, Score: floatPtr(62)},
		},
	}}

	solid := buildResolvedSolid(true, peerA, true, peerB, arb)

	sNode, _ := solid["S"].(map[string]any)
	if sNode["applicability"] != peer.ApplicabilityApplicable {
		t.Fatalf("S.applicability = %v, esperado APPLICABLE (veredito do arbiter)", sNode["applicability"])
	}
	if sNode["score"] != 62.0 {
		t.Fatalf("S.score = %v, esperado 62 (score_verdicts do arbiter)", sNode["score"])
	}

	oNode, _ := solid["O"].(map[string]any)
	if oNode["score"] != 72.0 {
		t.Fatalf("O.score = %v, esperado 72 (consenso peer_b, arbiter não tocou)", oNode["score"])
	}

	score, status := peer.AggregateSolidScore(map[string]any{"solid": solid})
	if status != peer.SolidScoreAvailable {
		t.Fatalf("status = %s, esperado AVAILABLE", status)
	}
	if score == nil || *score != 67.0 {
		t.Fatalf("score agregado = %v, esperado 67 (média de 62 e 72)", score)
	}
}

// TestBuildResolvedSolid_RejectBothZeroesScore: decision=reject_both não
// pode inventar score — princípio some da agregação (SAI-129C, §7 do
// plano: peso zero, não reduz nem aumenta o score global).
func TestBuildResolvedSolid_RejectBothZeroesScore(t *testing.T) {
	arb := &arbiter.ExecutorResult{Verdict: &arbiter.Verdict{
		ApplicabilityVerdicts: []arbiter.ApplicabilityVerdict{
			{Principle: "S", Applicability: peer.ApplicabilityApplicable, EvidenceRefs: []string{"a.go:1"}},
		},
		ScoreVerdicts: []arbiter.ScoreVerdict{
			{Principle: "S", Decision: arbiter.DecisionRejectBoth},
		},
	}}
	solid := buildResolvedSolid(false, nil, false, nil, arb)
	sNode, _ := solid["S"].(map[string]any)
	if sNode["score"] != nil {
		t.Fatalf("S.score = %v, esperado nil (reject_both não tem score)", sNode["score"])
	}
	applicability, _, ok := peer.PrincipleApplicabilityAndScore(sNode)
	if ok {
		t.Fatalf("PrincipleApplicabilityAndScore ok=true pra reject_both sem score, esperado false")
	}
	if applicability != peer.ApplicabilityInsufficientEvidence {
		t.Fatalf("applicability derivada = %s, esperado INSUFFICIENT_EVIDENCE (APPLICABLE sem score numérico)", applicability)
	}
}

func TestMapSolidScoreStatus(t *testing.T) {
	cases := map[string]string{
		peer.SolidScoreAvailable:            peer.ScoreStatusAvailable,
		peer.SolidScorePartial:              peer.ScoreStatusAvailable,
		peer.SolidScoreNotApplicable:        peer.SolidScoreNotApplicable,
		peer.SolidScoreInsufficientEvidence: peer.ScoreStatusUnavailable,
	}
	for in, want := range cases {
		if got := mapSolidScoreStatus(in); got != want {
			t.Errorf("mapSolidScoreStatus(%s) = %s, esperado %s", in, got, want)
		}
	}
}

func floatPtr(f float64) *float64 { return &f }
