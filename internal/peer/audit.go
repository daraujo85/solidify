// Independence audit (SAI-066).
//
// Registra metadata de model/provider/context para Peer A e B.
// Marca `independence_degraded` quando:
//   - mesmo provider
//   - mesmo model
//   - mesma família
//   - mesma session/context (cross-contamination detectada)
package peer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

// PeerMetadata info do peer.
type PeerMetadata struct {
	Label       string    `json:"label"` // "A"|"B"|"Arbiter"
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Family      string    `json:"family"`
	ContextHash string    `json:"context_hash"`
	RecordedAt  time.Time `json:"recorded_at"`
	RequestHash string    `json:"request_hash,omitempty"`
}

// IndependenceAudit resultado.
type IndependenceAudit struct {
	PeerA              PeerMetadata `json:"peer_a"`
	PeerB              PeerMetadata `json:"peer_b"`
	IndependenceOK     bool         `json:"independence_ok"`
	DegradedReasons    []string     `json:"degraded_reasons,omitempty"`
	Degraded           bool         `json:"degraded"`
	DistinctnessPolicy string       `json:"distinctness_policy"`
	AuditedAt          time.Time    `json:"audited_at"`
}

// Recorder registra metadata + audita.
type Recorder struct {
	mu    sync.Mutex
	store map[string]PeerMetadata // key = label
}

// NewRecorder constrói.
func NewRecorder() *Recorder {
	return &Recorder{store: make(map[string]PeerMetadata)}
}

// Record adiciona/atualiza metadata.
func (r *Recorder) Record(m PeerMetadata) error {
	if m.Label == "" {
		return errors.New("recorder: label vazio")
	}
	if m.Provider == "" {
		return errors.New("recorder: provider vazio")
	}
	if m.Model == "" {
		return errors.New("recorder: model vazio")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m.RecordedAt = time.Now()
	if m.Family == "" {
		m.Family = inferFamily(m.Model)
	}
	r.store[m.Label] = m
	return nil
}

// Get retorna metadata.
func (r *Recorder) Get(label string) (PeerMetadata, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.store[label]
	return m, ok
}

// Labels retorna labels registrados.
func (r *Recorder) Labels() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.store))
	for k := range r.store {
		out = append(out, k)
	}
	return out
}

// AuditOptions opções.
type AuditOptions struct {
	DistinctnessPolicy string
	CrossContamination []string // detected cross-contamination strings (substrings)
}

// Audit compara Peer A vs B.
func (r *Recorder) Audit(opts AuditOptions) (*IndependenceAudit, error) {
	a, okA := r.store["A"]
	b, okB := r.store["B"]
	if !okA || !okB {
		return nil, errors.New("recorder: A ou B não registrados")
	}
	audit := &IndependenceAudit{
		PeerA:              a,
		PeerB:              b,
		DistinctnessPolicy: opts.DistinctnessPolicy,
		AuditedAt:          time.Now(),
	}
	audit.DegradedReasons = detectDegradation(a, b, opts.CrossContamination)
	audit.Degraded = len(audit.DegradedReasons) > 0
	audit.IndependenceOK = !audit.Degraded
	return audit, nil
}

// detectDegradation retorna lista de razões.
func detectDegradation(a, b PeerMetadata, cross []string) []string {
	var reasons []string
	if a.Provider == b.Provider {
		reasons = append(reasons, "same provider: "+a.Provider)
	}
	if a.Model == b.Model {
		reasons = append(reasons, "same model: "+a.Model)
	}
	if a.Family != "" && a.Family == b.Family && a.Family != "other" {
		reasons = append(reasons, "same family: "+a.Family)
	}
	if a.ContextHash != "" && a.ContextHash == b.ContextHash {
		reasons = append(reasons, "same context hash (cross-contamination suspected)")
	}
	for _, s := range cross {
		if s == "" {
			continue
		}
		reasons = append(reasons, "cross-contamination: "+truncate(s, 50))
	}
	return reasons
}

// inferFamily extrai família.
func inferFamily(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "gpt"):
		return "gpt"
	case strings.HasPrefix(m, "claude"):
		return "claude"
	case strings.HasPrefix(m, "gemini"):
		return "gemini"
	case strings.HasPrefix(m, "llama"):
		return "llama"
	case strings.HasPrefix(m, "mistral") || strings.HasPrefix(m, "mixtral"):
		return "mistral"
	}
	return "other"
}

// ComputeContextHash SHA256 do contexto serializado.
func ComputeContextHash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// DetectCrossContamination checa se output do Peer A aparece
// em inputs do Peer B (best-effort substring match).
func DetectCrossContamination(peerAOutput, peerBPrompt string) []string {
	if peerAOutput == "" || peerBPrompt == "" {
		return nil
	}
	var hits []string
	if strings.Contains(peerBPrompt, "PEER_A_OUTPUT:") {
		hits = append(hits, "PEER_A_OUTPUT marker")
	}
	marker := peerAOutput
	if len(marker) > 100 {
		marker = marker[:100]
	}
	if marker != "" && strings.Contains(peerBPrompt, marker) {
		hits = append(hits, "substring match")
	}
	return hits
}

// IndependenceReport textual render.
type IndependenceReport struct {
	Summary string
	Audit   *IndependenceAudit
}

// RenderReport human-readable.
func RenderReport(audit *IndependenceAudit) string {
	if audit == nil {
		return "<nil>"
	}
	var sb strings.Builder
	if audit.IndependenceOK {
		sb.WriteString("✓ Independence OK\n")
	} else {
		sb.WriteString("✗ Independence DEGRADED\n")
	}
	sb.WriteString("  Peer A: " + audit.PeerA.Provider + "/" + audit.PeerA.Model + "\n")
	sb.WriteString("  Peer B: " + audit.PeerB.Provider + "/" + audit.PeerB.Model + "\n")
	sb.WriteString("  Distinctness policy: " + audit.DistinctnessPolicy + "\n")
	if audit.Degraded {
		sb.WriteString("  Reasons:\n")
		for _, r := range audit.DegradedReasons {
			sb.WriteString("    - " + r + "\n")
		}
	}
	return sb.String()
}

// MarkDegraded força marcação manual.
func MarkDegraded(audit *IndependenceAudit, reason string) {
	if audit == nil {
		return
	}
	audit.Degraded = true
	audit.IndependenceOK = false
	if reason != "" {
		audit.DegradedReasons = append(audit.DegradedReasons, reason)
	}
}

// HasDegradation helper.
func HasDegradation(audit *IndependenceAudit) bool {
	if audit == nil {
		return false
	}
	return audit.Degraded
}

// _ = context unused
var _ = context.TODO

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
