// Peer A executor (SAI-118 + SAI-119).
//
// peer_a é o revisor primário. Três fontes possíveis:
//
//   - SourceTerminalMCP: MCP stdio real (ADR-0119). Polls
//     `PeerReviewStore` no `StoreDir` configurado, esperando
//     um submission do agente humano/IA com
//     actor="peer_a" e run_id correspondente. Sem submission
//     dentro do timeout → skipped (called=false) — gate trata
//     como não-executado. Stub legado que lia
//     <artifact_dir>/peer_a_response.json foi removido em SAI-119.
//
//   - SourceHTTPCombo: chama peer.Executor com o combo
//     "solidai-peer-a" via 9Router. Mesmo transport do peer_b mas
//     combo diferente → distinct_models=2 se peer_b usa outro
//     combo → independence_gate passa.
//
//   - SourceAuto: Fase 1 = http-combo (libera contractual
//     sem exigir config do usuário).
package peer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
	"github.com/diegoaraujo/solidify/internal/mcpserver"
)

// Source identifica como peer_a é executado.
type Source string

const (
	SourceTerminalMCP Source = "terminal-mcp" // MCP stdio polling PeerReviewStore (SAI-119)
	SourceHTTPCombo   Source = "http-combo"   // 9Router combo solidai-peer-a
	SourceAuto        Source = "auto"         // Fase 1: resolve pra http-combo
)

// ResolveSource normaliza o source declarado na config. Qualquer
// valor desconhecido cai em SourceAuto (não falha run — graceful
// degradation pro usuário que erra o nome).
func ResolveSource(s string) Source {
	switch Source(s) {
	case SourceTerminalMCP, SourceHTTPCombo, SourceAuto:
		return Source(s)
	default:
		return SourceAuto
	}
}

// PeerAOptions entrada do dispatcher.
type PeerAOptions struct {
	Source      Source
	Request     *PeerRequest
	Provider    ai.Provider
	Model       string
	Timeout     time.Duration
	ArtifactDir string // raiz onde artifacts de evidence são escritos (informativo)
	RunID       string // run_id usado pra match no PeerReviewStore (ADR-0119)
	StoreDir    string // diretório do PeerReviewStore (ADR-0119; default ~/.solidify/peer_reviews)
}

// ExecutePeerA roda peer_a segundo o source. Retorna (result, called, err).
//
//   - result pode ser nil quando called=false (source pulou peer_a).
//   - called=false significa que peer_a NÃO EXECUTOU — gate deve
//     tratar como não-executado (independence_gate BLOCKED se
//     RequirePeerA=true).
//   - err só é retornado quando o source falhou de forma recuperável
//     (ex.: provider HTTP retornou erro). Caller decide se aborta run.
func ExecutePeerA(ctx context.Context, opts PeerAOptions) (*ExecutorResult, bool, error) {
	src := ResolveSource(string(opts.Source))
	switch src {
	case SourceTerminalMCP:
		return executeTerminalMCP(ctx, opts)
	case SourceHTTPCombo:
		return executeHTTPCombo(ctx, opts)
	default:
		// SourceAuto ou desconhecido: Fase 1 = HTTP combo.
		return executeHTTPCombo(ctx, opts)
	}
}

// executeHTTPCombo roda peer_a via 9Router. Reusa peer.Executor —
// mesma máquina (timeout/repair/score_status) do peer_b, garantindo
// comportamento uniforme.
func executeHTTPCombo(ctx context.Context, opts PeerAOptions) (*ExecutorResult, bool, error) {
	if opts.Request == nil {
		return nil, false, errors.New("peer_a http-combo: request nil")
	}
	if opts.Provider == nil {
		return nil, false, errors.New("peer_a http-combo: provider nil")
	}
	if opts.Model == "" {
		return nil, false, errors.New("peer_a http-combo: model vazio")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	exec := NewExecutor()
	res, err := exec.Execute(ctx, ExecutorOptions{
		Request:    opts.Request,
		Provider:   opts.Provider,
		Model:      opts.Model,
		Timeout:    timeout,
		MaxRetries: 0,
	})
	if err != nil {
		// Provider falhou (timeout/HTTP/etc) — peer_a não executou de fato.
		// Retornamos called=false pra gate não pontuar um modelo que não rodou.
		return nil, false, err
	}
	return res, true, nil
}

// executeTerminalMCP roda peer_a esperando submission via MCP server
// (ADR-0119). Lê PeerReviewStore em StoreDir a cada 2s, buscando
// record com `RunID == opts.RunID` e `Actor == "peer_a"`. Quando
// acha, monta ExecutorResult a partir do record.
//
// Sem submission dentro de opts.Timeout → skipped (called=false).
// Records corrompidos são pulados (best-effort, ADR-0119).
//
// SAI-120: lê `CanonicalPayload` (v2) primeiro; se vazio,
// parseia `Notes` como JSON (v1 legacy compat).
func executeTerminalMCP(ctx context.Context, opts PeerAOptions) (*ExecutorResult, bool, error) {
	dir := opts.StoreDir
	if dir == "" {
		// ADR-0119: default ~/.solidify/peer_reviews/ — resolvido
		// pelo caller (run.go) antes de chegar aqui. Se vazio,
		// ainda tenta resolver HOME; se falhar, skipped.
		dir = resolveDefaultStoreDir()
	}
	if dir == "" || opts.RunID == "" {
		return nil, false, nil
	}
	store, err := mcpserver.NewPeerReviewStore(dir)
	if err != nil {
		// store indisponível = sem submission possível.
		// Não é erro fatal — caller decide se BLOCKED é OK.
		return nil, false, nil
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	// Tick imediato evita esperar 2s quando submission já está no store.
	if res, called, ok := pollOnce(store, opts.RunID); ok {
		return res, called, nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-ticker.C:
			if res, called, ok := pollOnce(store, opts.RunID); ok {
				return res, called, nil
			}
			if time.Now().After(deadline) {
				return nil, false, nil
			}
		}
	}
}

// pollOnce varre o store uma vez; retorna (result, called, true)
// se achar record matching run_id+actor=peer_a.
func pollOnce(store *mcpserver.PeerReviewStore, runID string) (*ExecutorResult, bool, bool) {
	records, err := store.List()
	if err != nil {
		return nil, false, false
	}
	for _, rec := range records {
		if rec == nil {
			continue
		}
		if rec.RunID != runID {
			continue
		}
		if rec.Actor != "peer_a" {
			continue
		}
		return recordToExecutorResult(rec), true, true
	}
	return nil, false, false
}

// recordToExecutorResult converte PeerReviewRecord em ExecutorResult.
// SAI-120: prefere CanonicalPayload (v2 nativo); fallback pra
// Notes JSON-string (v1 legacy). Model fica
// "solidify-mcp://peer_a/<review_id>" — auditável e distinto de
// qualquer HTTP combo (independence_gate ok).
func recordToExecutorResult(rec *mcpserver.PeerReviewRecord) *ExecutorResult {
	parsed := rec.CanonicalPayload
	if parsed == nil && rec.Notes != "" {
		// v1 legacy: payload canônico estava em Notes como JSON-string.
		var vp map[string]any
		if jerr := json.Unmarshal([]byte(rec.Notes), &vp); jerr == nil {
			parsed = vp
		}
	}
	res := &ExecutorResult{
		Content:      rec.Notes, // livre (markdown comentário); preserva p/ arbiter
		ParsedContent: parsed,
		Model:        "solidify-mcp://peer_a/" + rec.ReviewID,
		Provider:     "terminal-mcp",
		StartedAt:    rec.SubmittedAt,
		CompletedAt:  rec.SubmittedAt,
	}
	if res.ParsedContent == nil {
		// payload nem v1 (Notes) nem v2 (CanonicalPayload) — sintetiza
		// map mínimo pra gate ter algo (verdict + review_id).
		res.ParsedContent = map[string]any{
			"review_id": rec.ReviewID,
			"verdict":   rec.Verdict,
		}
	}
	if verrs := validateCanonical(res.ParsedContent); len(verrs) > 0 {
		res.ScoreStatus = ScoreStatusUnavailable
		res.ValidationErrors = verrs
	} else {
		res.ScoreStatus = ScoreStatusAvailable
	}
	res.QualityScore = extractQualityScore(res.ParsedContent)
	return res
}

// resolveDefaultStoreDir fallback raro — caller já deve ter
// resolvido. Mantido pra quando PeerAOptions chega sem StoreDir.
func resolveDefaultStoreDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".solidify", "peer_reviews")
}
