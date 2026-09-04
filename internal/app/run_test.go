package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/diegoaraujo/solidify/internal/arbiter"
	"github.com/diegoaraujo/solidify/internal/peer"
)

// setupGitRepo cria um repo git temporário com 2 commits (base/head reais)
// pra gitx.Diff ter o que processar.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "c1")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "c2")
	return dir
}

// TestRunRun_NoProviderProducesIncompleteReport smoke test SAI-127: sem
// 9Router disponível, runRun deve falhar de forma limpa (não mais o erro
// fake de quota hardcoded) e escrever release-report.json com
// status=INCOMPLETE via PartialReportDeferred.
func TestProjectPeerResultToReportPreservesApplicabilityAndEvidence(t *testing.T) {
	parsed := map[string]any{
		"summary": "switch por tipo dificulta extensão",
		"issues": []any{map[string]any{
			"id": "OCP-1", "severity": "high", "title": "Switch concreto", "evidence_refs": []any{"service.go:12"},
		}},
		"solid": map[string]any{
			"S": map[string]any{"applicability": "NOT_APPLICABLE", "score": nil, "reason": "mudança sem responsabilidade adicional"},
			"O": map[string]any{"applicability": "APPLICABLE", "score": 42.0, "reason": "novo switch", "evidence_refs": []any{"service.go:12"}},
		},
	}

	solid, pillars, risk, recommendations, summary := projectPeerResultToReport(parsed)
	if got := solid.Principles["S"]; got.Applicable || got.Applicability != "NOT_APPLICABLE" || got.Reason == "" || got.AfterScore != nil {
		t.Fatalf("S projectado incorretamente: %#v", got)
	}
	if got := solid.Principles["O"]; !got.Applicable || got.Applicability != "APPLICABLE" || got.AfterScore == nil || *got.AfterScore != 42 || len(got.EvidenceRefs) != 1 {
		t.Fatalf("O projectado incorretamente: %#v", got)
	}
	if len(pillars) != 5 || !pillars[1].Applicable || !pillars[1].Scored || pillars[1].Score == nil {
		t.Fatalf("pillars incorretos: %#v", pillars)
	}
	if risk.Level != "HIGH" || len(risk.Factors) != 1 || risk.Factors[0].ID != "OCP-1" {
		t.Fatalf("risco incorreto: %#v", risk)
	}
	if len(recommendations) != 1 || recommendations[0].Title != "Switch concreto" {
		t.Fatalf("recomendação incorreta: %#v", recommendations)
	}
	if summary != "switch por tipo dificulta extensão" {
		t.Fatalf("summary = %q", summary)
	}
}

func TestRunRun_NoProviderProducesIncompleteReport(t *testing.T) {
	repoDir := setupGitRepo(t)

	// runRun grava .solidify/out/release-report.json relativo ao cwd do
	// processo (mesmo padrão de PartialReportDeferred) — isola em tmp.
	workDir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	var stdout, stderr bytes.Buffer
	env := Env{Stdout: &stdout, Stderr: &stderr}
	logger := slog.New(slog.NewTextHandler(&stderr, nil))

	args := []string{"--profile", "quick", "--base", "HEAD~1", "--head", "HEAD", "--dir", repoDir}
	runErr := runRun(args, env, logger)
	if runErr == nil {
		t.Fatal("esperava erro (sem 9Router disponível), got nil")
	}

	reportPath := filepath.Join(workDir, ".solidify", "out", "release-report.json")
	b, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("release-report.json não foi escrito: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatalf("release-report.json inválido: %v", err)
	}
	if rep["status"] != "INCOMPLETE" {
		t.Fatalf("status = %v, esperava INCOMPLETE", rep["status"])
	}
}

// setupGitRepoNonTrivial cria diff que NÃO aciona o shortcut heurístico
// (SAI-129B classifica rename/whitespace puro como CLEARLY_NOT_APPLICABLE
// pros 5 princípios e pula o LLM inteiro) — este teste precisa que o LLM
// seja de fato chamado, pra diagnosticar o wiring real do score.
func setupGitRepoNonTrivial(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	base := "package calc\n\nfunc Add(a, b int) int { return a + b }\n"
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte(base), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "c1")
	head := base + "\nfunc Mul(a, b int) int { return a * b }\n"
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte(head), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "c2")
	return dir
}

// runScoreSourceScenario roda runRun contra um servidor fake devolvendo
// `content` (JSON canônico do peer), profile quick/peer_a-only (o path
// comum a TODOS os cenários já validados nesta sessão), e retorna
// scores.quality + score_status lidos do release-report.json. Erro de
// gate (FAIL por score baixo) é esperado em alguns casos e ignorado —
// o que estes testes verificam é a FONTE do score, não o veredito do gate.
func runScoreSourceScenario(t *testing.T, content string) (quality *float64, scoreStatus, gateStatus string) {
	t.Helper()
	repoDir := setupGitRepoNonTrivial(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id":    "x",
			"model": "test-model",
			"choices": []map[string]any{{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": content},
				"finish_reason": "stop",
			}},
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}))
	defer srv.Close()

	workDir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	t.Setenv("SAI129_TEST_TOKEN", "testkey")

	cfg := fmt.Sprintf(`{
		"project": {"name": "diag"},
		"ai": {
			"external_provider": {"base_url_host": %q, "base_url_docker": %q, "api_key_env": "SAI129_TEST_TOKEN"},
			"peer_a": {"source": "http-combo"},
			"selection": {"peer_a": {"preferred": ["test-model"]}, "peer_b": {"preferred": ["test-model"]}}
		},
		"profiles": {
			"quick": {"require_peer_a": true, "require_peer_b": false, "require_arbiter": false}
		}
	}`, srv.URL, srv.URL)
	if err := os.WriteFile(filepath.Join(workDir, "solidify.json"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	env := Env{Stdout: &stdout, Stderr: &stderr}
	logger := slog.New(slog.NewTextHandler(&stderr, nil))

	args := []string{"--profile", "quick", "--base", "HEAD~1", "--head", "HEAD", "--dir", repoDir}
	_ = runRun(args, env, logger)

	b, err := os.ReadFile(filepath.Join(workDir, ".solidify", "out", "release-report.json"))
	if err != nil {
		t.Fatalf("release-report.json não foi escrito: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatalf("release-report.json inválido: %v", err)
	}
	scores, _ := rep["scores"].(map[string]any)
	if raw, ok := scores["quality"].(float64); ok {
		quality = &raw
	}
	scoreStatus, _ = scores["score_status"].(string)
	qualityGate, _ := rep["quality_gate"].(map[string]any)
	gateStatus, _ = qualityGate["status"].(string)
	return quality, scoreStatus, gateStatus
}

// TestRunRun_GlobalScoreSourceOfTruth — 3 testes definitivos pedidos pelo
// usuário (achado, 2026-09-02): o path comum a TODOS os 5 cenários já
// validados nesta sessão é profile "quick" com só peer_a chamado (sem
// peer_b, sem arbiter). Antes do fix, SAI-129C's peer.AggregateSolidScore()
// só era invocado em run.go quando requirePrincipleVerdicts && arbiterCalled
// — condição que exige peer_a+peer_b divergirem em applicability E o
// arbiter ser chamado. Fora dessa faixa estreita, scores.quality vinha
// direto do quality_score autorreportado (confirmado objetivamente: caso
// 99-vs-30 abaixo deu quality=99 antes do fix em run.go). Corrigido:
// buildResolvedSolid()+AggregateSolidScore() agora rodam sempre que há
// princípios legíveis e o schema do peer vencedor validou.
func TestRunRun_GlobalScoreSourceOfTruth(t *testing.T) {
	t.Run("quality_score_autorreportado_diverge_da_media_dos_principios", func(t *testing.T) {
		content := `{"solid":{` +
			`"S":{"applicability":"APPLICABLE","score":10,"evidence_refs":["calc.go:3"]},` +
			`"O":{"applicability":"APPLICABLE","score":20,"evidence_refs":["calc.go:3"]},` +
			`"L":{"applicability":"APPLICABLE","score":30,"evidence_refs":["calc.go:3"]},` +
			`"I":{"applicability":"APPLICABLE","score":40,"evidence_refs":["calc.go:3"]},` +
			`"D":{"applicability":"APPLICABLE","score":50,"evidence_refs":["calc.go:3"]}` +
			`},"quality_score":99,"confidence":0.9,"summary":"teste diagnostico","issues":[]}`
		quality, _, _ := runScoreSourceScenario(t, content)
		if quality == nil || *quality != 30 {
			t.Fatalf("scores.quality = %v; esperava 30 (media determinística S=10,O=20,L=30,I=40,D=50), não o quality_score=99 autorreportado", quality)
		}
	})

	t.Run("mix_applicable_e_not_applicable_renormaliza_media", func(t *testing.T) {
		content := `{"solid":{` +
			`"S":{"applicability":"APPLICABLE","score":80,"evidence_refs":["calc.go:3"]},` +
			`"O":{"applicability":"APPLICABLE","score":60,"evidence_refs":["calc.go:3"]},` +
			`"L":{"applicability":"NOT_APPLICABLE","reason":"sem mudança de contrato"},` +
			`"I":{"applicability":"NOT_APPLICABLE","reason":"sem mudança de contrato"},` +
			`"D":{"applicability":"NOT_APPLICABLE","reason":"sem mudança de contrato"}` +
			`},"quality_score":10,"confidence":0.9,"summary":"teste diagnostico","issues":[]}`
		quality, status, _ := runScoreSourceScenario(t, content)
		if quality == nil || *quality != 70 {
			t.Fatalf("scores.quality = %v; esperava 70 (media só de S=80,O=60, renormalizada), não o quality_score=10 autorreportado", quality)
		}
		if status != peer.ScoreStatusAvailable {
			t.Fatalf("score_status = %q; esperava %q (2/5 aplicáveis, sem insuficiência)", status, peer.ScoreStatusAvailable)
		}
	})

	t.Run("cinco_de_cinco_not_applicable_nao_bloqueia_gate", func(t *testing.T) {
		content := `{"solid":{` +
			`"S":{"applicability":"NOT_APPLICABLE","reason":"sem mudança de contrato"},` +
			`"O":{"applicability":"NOT_APPLICABLE","reason":"sem mudança de contrato"},` +
			`"L":{"applicability":"NOT_APPLICABLE","reason":"sem mudança de contrato"},` +
			`"I":{"applicability":"NOT_APPLICABLE","reason":"sem mudança de contrato"},` +
			`"D":{"applicability":"NOT_APPLICABLE","reason":"sem mudança de contrato"}` +
			`},"quality_score":92,"confidence":0.9,"summary":"teste diagnostico","issues":[]}`
		quality, status, gateStatus := runScoreSourceScenario(t, content)
		if quality != nil {
			t.Fatalf("scores.quality = %v; esperava null (sem princípio aplicável), não o quality_score=92 autorreportado", *quality)
		}
		if status != peer.SolidScoreNotApplicable {
			t.Fatalf("score_status = %q; esperava %q", status, peer.SolidScoreNotApplicable)
		}
		if gateStatus != "PASS" {
			t.Fatalf("quality_gate.status = %q; esperava PASS/non-blocking para 5/5 NOT_APPLICABLE", gateStatus)
		}
	})
}

// TestBuildActors_PopulatesRequestedAndResolvedModel (SAI-129, 2026-09-04):
// buildActors() só copiava peer.ExecutorResult.Model/arbiter.ExecutorResult.Model
// (o combo PEDIDO) pro report.Actor.ModelID — RequestedModel/ExecutedModel do
// report ficavam sempre vazios, mesmo com peer/arbiter já guardando o model
// REAL resolvido pelo 9router em ResolvedModel. buildActors precisa propagar
// os dois.
func TestBuildActors_PopulatesRequestedAndResolvedModel(t *testing.T) {
	peerA := &peer.ExecutorResult{
		Provider: "mock", Model: "claude-premium-a",
		RequestedModel: "claude-premium-a", ResolvedModel: "gpt-5.4-real-a",
		ScoreStatus: "available",
	}
	peerB := &peer.ExecutorResult{
		Provider: "mock", Model: "claude-premium-b",
		RequestedModel: "claude-premium-b", ResolvedModel: "gpt-5.4-real-b",
		ScoreStatus: "available",
	}
	arb := &arbiter.ExecutorResult{
		Provider: "mock", Model: "claude-premium-arbiter",
		RequestedModel: "claude-premium-arbiter", ResolvedModel: "gpt-5.4-real-arb",
		ScoreStatus: "available",
	}

	actors := buildActors(peerA, peerB, arb)
	if len(actors) != 3 {
		t.Fatalf("actors: %d, esperava 3", len(actors))
	}
	for _, a := range actors {
		var wantReq, wantResolved string
		switch a.Role {
		case "peer_a":
			wantReq, wantResolved = "claude-premium-a", "gpt-5.4-real-a"
		case "peer_b":
			wantReq, wantResolved = "claude-premium-b", "gpt-5.4-real-b"
		case "arbiter":
			wantReq, wantResolved = "claude-premium-arbiter", "gpt-5.4-real-arb"
		default:
			t.Fatalf("role inesperado: %q", a.Role)
		}
		if a.RequestedModel != wantReq {
			t.Errorf("%s: RequestedModel = %q, esperava %q", a.Role, a.RequestedModel, wantReq)
		}
		if a.ExecutedModel != wantResolved {
			t.Errorf("%s: ExecutedModel = %q, esperava %q", a.Role, a.ExecutedModel, wantResolved)
		}
	}
}
