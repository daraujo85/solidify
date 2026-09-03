// E2E full flow — SAI-108 contractual peer review.
//
// Simula Peer B + Arbiter via httptest.Server (LLM mockado). Peer A é
// simulado como o agente terminal (produz PeerScores direto, sem LLM
// externo — consistente com AGENT_HANDOFF.md).
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
	"github.com/diegoaraujo/solidify/internal/arbiter"
	"github.com/diegoaraujo/solidify/internal/peer"
)

type mockChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mockChatRequest struct {
	Model    string            `json:"model"`
	Messages []mockChatMessage `json:"messages"`
}

// mockLLMServer devolve peerBContent p/ requests do Peer B e
// arbiterContent p/ requests do Arbiter (routing pela 1a system message).
func mockLLMServer(peerBContent, arbiterContent string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req mockChatRequest
		_ = json.Unmarshal(body, &req)

		content := peerBContent
		if len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "Arbiter") {
			content = arbiterContent
		}

		resp := map[string]any{
			"id": "mock", "object": "chat.completion", "model": req.Model,
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": content},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// extractSolidScores lê solid.{S,O,L,I,D}.after_score.value do output
// canônico do Peer B.
func extractSolidScores(t *testing.T, parsed map[string]any) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	solid, _ := parsed["solid"].(map[string]any)
	for _, k := range []string{"S", "O", "L", "I", "D"} {
		node, _ := solid[k].(map[string]any)
		after, _ := node["after_score"].(map[string]any)
		if v, ok := after["value"].(float64); ok {
			out[k] = v
		}
	}
	return out
}

// Aceitação SAI-108: fluxo contratual completo — Peer A (terminal, sem
// LLM), Peer B (LLM isolado via provider mockado), divergence map
// determinístico, Arbiter C (LLM isolado, contexto separado), gate
// contractual PASS.
func TestE2E_ContractualPeerReviewFullFlow(t *testing.T) {
	const runID = "run-e2e-108"

	peerBJSON := `{"solid":{"S":{"applicable":true,"after_score":{"value":82}},` +
		`"O":{"applicable":true,"after_score":{"value":78}},` +
		`"L":{"applicable":true,"after_score":{"value":90}},` +
		`"I":{"applicable":true,"after_score":{"value":86}},` +
		`"D":{"applicable":true,"after_score":{"value":71}}},` +
		`"quality_score":81.4,"confidence":0.9,"summary":"peer b review"}`

	arbiterJSON := fmt.Sprintf(
		`{"run_id":%q,"actor":"arbiter","verdict":"resolved",`+
			`"resolutions":[{"topic":"D score divergence","decision":"accept_both",`+
			`"reasoning":"minor delta, both peers converge"}],`+
			`"reasoning":"peers agree with acceptable divergence"}`, runID)

	srv := mockLLMServer(peerBJSON, arbiterJSON)
	defer srv.Close()

	provider := ai.NewOpenAIProvider(srv.URL, "test-key").WithProviderName("mock-9router")

	// Evidence shards compartilhados entre peers (SAI-064).
	shards := []peer.EvidenceShard{
		{ID: "shard-1", Kind: "diff", Content: "func Foo() {}", SourceFile: "src/api/users.go"},
	}
	evidence, err := peer.NewEvidenceShardSet(runID, shards)
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}

	// Peer A: simulado como agente terminal (não passa por LLM externo).
	peerA := peer.PeerScores{
		RunID:      runID,
		Source:     "A",
		Scores:     map[string]float64{"S": 85, "O": 75, "L": 90, "I": 88, "D": 70},
		Applicable: map[string]bool{"S": true, "O": true, "L": true, "I": true, "D": true},
	}

	// Peer B: executor isolado (SAI-065), nunca vê output de A.
	builder := peer.NewRequestBuilder("Review evidence for run {{run_id}}", "1")
	reqB, err := builder.Build(peer.RequestOptions{RunID: runID, Actor: "peer_b", Evidence: evidence})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	execB := peer.NewExecutor()
	resB, err := execB.Execute(context.Background(), peer.ExecutorOptions{
		Request: reqB, Provider: provider, Model: "mock-model-b", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("peer b execute: %v", err)
	}
	if resB.ScoreStatus != peer.ScoreStatusAvailable {
		t.Fatalf("peer b score status: %s errs=%v verrs=%v", resB.ScoreStatus, resB.Errors, resB.ValidationErrors)
	}

	peerB := peer.PeerScores{
		RunID:      runID,
		Source:     "B",
		Scores:     extractSolidScores(t, resB.ParsedContent),
		Applicable: map[string]bool{"S": true, "O": true, "L": true, "I": true, "D": true},
	}

	// Divergence map determinístico (SAI-067).
	dm, err := peer.Compute(peerA, peerB)
	if err != nil {
		t.Fatalf("divergence: %v", err)
	}

	// Arbiter C: contexto isolado, recebe evidence + reviews A/B + divmap.
	arb, err := arbiter.NewExecutor(arbiter.ExecutorOptions{
		RunID:       runID,
		Evidence:    "shard-1",
		PeerAOutput: fmt.Sprintf("%v", peerA.Scores),
		PeerBOutput: resB.Content,
		DivMap:      peer.RenderDivergence(dm),
		Provider:    provider, Model: "mock-model-arbiter", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("new arbiter: %v", err)
	}
	resArb, err := arb.Execute(context.Background())
	if err != nil {
		t.Fatalf("arbiter execute: %v", err)
	}
	if resArb.ScoreStatus != peer.ScoreStatusAvailable || resArb.Verdict == nil {
		t.Fatalf("arbiter status: %s errs=%v verrs=%v", resArb.ScoreStatus, resArb.Errors, resArb.ValidationErrors)
	}

	// Gate contractual: 2+ peers, 1 arbiter (SAI-108/SAI-093).
	out, err := VerifyContractualPeer(PeersInput{
		Peers: 2, Arbiters: 1,
		RobustnessStatus: "stable",
		SwapApplied:      false,
	})
	if err != nil {
		t.Fatalf("verify contractual: %v", err)
	}
	if out.GateStatus != "PASS" {
		t.Errorf("gate: %+v", out)
	}
	if dm.ArbiterRequired && resArb.Verdict.Verdict != "resolved" {
		t.Errorf("arbiter deveria resolver divergência: %+v", resArb.Verdict)
	}
}
