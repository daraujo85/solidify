// Peer review canary doctor checks (SAI-123).
//
// Dois checks:
//
// 1. `peer_review_canary` — lê JSONL de telemetria, computa
//    v1_fraction nos últimos 30 dias. Se < 5% e >= 10 events,
//    sugere setar cutoff (status: OK com sugestão). Se >= 5%,
//    FAIL pedindo rollout v2. Sem dados suficientes (nenhuma
//    submission) = OK sem mensagem acionável.
//
// 2. `peer_review_store_v1` — conta records v1 no
//    PeerReviewStore. Se > 0, FAIL sugerindo rodar
//    `solidify peer-reviews migrate`. Se 0 ou store vazio,
//    OK.
//
// Thresholds ficam em vars package-level (ajustáveis em testes).
package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/diegoaraujo/solidify/internal/mcpserver"
)

var (
	// CanaryWindow janela de observação (default 30 dias).
	CanaryWindow = 30 * 24 * time.Hour
	// CanaryFractionThreshold abaixo do qual considera "v1 morrendo".
	CanaryFractionThreshold = 0.05
	// CanaryMinEvents mínimo de eventos pra sugestão ser confiável.
	CanaryMinEvents = 10
)

// checkPeerReviewCanary avalia v1_fraction na janela.
func checkPeerReviewCanary() CheckResult {
	events, err := mcpserver.ReadMetrics()
	if err != nil {
		return CheckResult{
			Name:    "peer_review_canary",
			OK:      false,
			Message: "erro ao ler telemetry: " + err.Error(),
		}
	}
	if len(events) == 0 {
		return CheckResult{
			Name:    "peer_review_canary",
			OK:      true,
			Message: "sem submissões registradas (nada a analisar)",
		}
	}
	from := time.Now().Add(-CanaryWindow)
	s := mcpserver.SummarizeMetrics(events, from, time.Time{})
	if s.Total < CanaryMinEvents {
		return CheckResult{
			Name:    "peer_review_canary",
			OK:      true,
			Message: fmt.Sprintf("dados insuficientes (%d events < %d mínimo); continue coletando", s.Total, CanaryMinEvents),
		}
	}
	if s.V1Fraction < CanaryFractionThreshold {
		// v1 morrendo — sugere cutoff
		suggested := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
		return CheckResult{
			Name: "peer_review_canary",
			OK:   true,
			Message: fmt.Sprintf(
				"v1_fraction=%.1f%% (< %.0f%%) em %d events nos últimos %d dias — considere setar ai.peer_review.v1_cutoff=%s",
				s.V1Fraction*100,
				CanaryFractionThreshold*100,
				s.Total,
				int(CanaryWindow.Hours()/24),
				suggested,
			),
		}
	}
	// v1 ainda em uso significativo
	return CheckResult{
		Name: "peer_review_canary",
		OK:   false,
		Message: fmt.Sprintf(
			"v1_fraction=%.1f%% (>= %.0f%%) em %d events nos últimos %d dias — rollout v2 ainda em andamento",
			s.V1Fraction*100,
			CanaryFractionThreshold*100,
			s.Total,
			int(CanaryWindow.Hours()/24),
		),
	}
}

// checkPeerReviewStoreV1 conta records v1 no PeerReviewStore.
// Se > 0, FAIL com sugestão de migrate. Se store não existe,
// OK sem ação.
func checkPeerReviewStoreV1() CheckResult {
	dir, err := resolvePeerReviewStoreDir()
	if err != nil || dir == "" {
		return CheckResult{
			Name:    "peer_review_store_v1",
			OK:      true,
			Message: "store dir não resolvível (sem home ou config)",
		}
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return CheckResult{
			Name:    "peer_review_store_v1",
			OK:      true,
			Message: "store vazio (nenhum record persistido ainda)",
		}
	}
	store, err := mcpserver.NewPeerReviewStore(dir)
	if err != nil {
		return CheckResult{
			Name:    "peer_review_store_v1",
			OK:      false,
			Message: "erro ao abrir store: " + err.Error(),
		}
	}
	records, err := store.List()
	if err != nil {
		return CheckResult{
			Name:    "peer_review_store_v1",
			OK:      false,
			Message: "erro ao listar store: " + err.Error(),
		}
	}
	v1Count := 0
	for _, r := range records {
		if r != nil && r.Schema == "1" {
			v1Count++
		}
	}
	if v1Count == 0 {
		return CheckResult{
			Name:    "peer_review_store_v1",
			OK:      true,
			Message: "0 records v1 no store (clean)",
		}
	}
	return CheckResult{
		Name: "peer_review_store_v1",
		OK:   false,
		Message: fmt.Sprintf(
			"%d records v1 no store — rode `solidify peer-reviews migrate --dry-run` para preview",
			v1Count,
		),
	}
}

// resolvePeerReviewStoreDir resolve dir do store, honrando
// override via env SOLIDIFY_PEER_REVIEW_STORE_DIR (testes usam).
func resolvePeerReviewStoreDir() (string, error) {
	if d := os.Getenv("SOLIDIFY_PEER_REVIEW_STORE_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".solidify", "peer_reviews"), nil
}
