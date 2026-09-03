// Peer B request builder (SAI-064).
//
// Constrói request para Peer B usando evidence shards
// equivalentes ao Peer A. **Nunca** inclui output do Peer A
// (independência).
package peer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// EvidenceShard fatia de evidência.
type EvidenceShard struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Content    string `json:"content"`
	SourceFile string `json:"source_file,omitempty"`
	LineStart  int    `json:"line_start,omitempty"`
	LineEnd    int    `json:"line_end,omitempty"`
}

// EvidenceShardSet conjunto de shards (read-only).
type EvidenceShardSet struct {
	RunID      string          `json:"run_id"`
	Shards     []EvidenceShard `json:"shards"`
	TotalBytes int             `json:"total_bytes"`
}

// NewEvidenceShardSet constrói set.
func NewEvidenceShardSet(runID string, shards []EvidenceShard) (*EvidenceShardSet, error) {
	if runID == "" {
		return nil, errors.New("evidence: run_id vazio")
	}
	total := 0
	for _, s := range shards {
		total += len(s.Content)
	}
	return &EvidenceShardSet{
		RunID:      runID,
		Shards:     shards,
		TotalBytes: total,
	}, nil
}

// Hash computa SHA256 do conteúdo combinado.
func (s *EvidenceShardSet) Hash() string {
	if s == nil {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(s.RunID))
	h.Write([]byte{0})
	for _, shard := range s.Shards {
		h.Write([]byte(shard.ID))
		h.Write([]byte{0})
		h.Write([]byte(shard.Content))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// IDs devolve ids.
func (s *EvidenceShardSet) IDs() []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s.Shards))
	for i, x := range s.Shards {
		out[i] = x.ID
	}
	return out
}

// FilterByKind filtra por kind.
func (s *EvidenceShardSet) FilterByKind(kind string) []EvidenceShard {
	if s == nil {
		return nil
	}
	out := make([]EvidenceShard, 0)
	for _, x := range s.Shards {
		if x.Kind == kind {
			out = append(out, x)
		}
	}
	return out
}

// PeerRequest requisição.
type PeerRequest struct {
	RunID    string            `json:"run_id"`
	Actor    string            `json:"actor"`
	Schema   string            `json:"schema"`
	Prompt   string            `json:"prompt"`
	Evidence *EvidenceShardSet `json:"evidence"`
	// Vars usadas no template (audit).
	Vars map[string]string `json:"vars"`
	// Hash do request.
	Hash string `json:"hash"`
}

// RequestOptions opções.
type RequestOptions struct {
	RunID    string
	Actor    string
	Evidence *EvidenceShardSet
	Vars     map[string]string
	// PeerB-specific overrides (ex: schema_version diferente).
	SchemaVersion string
}

// RequestBuilder constrói PeerRequest.
type RequestBuilder struct {
	promptTemplate string
	schemaVersion  string
}

// NewRequestBuilder builder.
func NewRequestBuilder(promptTemplate, schemaVersion string) *RequestBuilder {
	if schemaVersion == "" {
		schemaVersion = "1"
	}
	return &RequestBuilder{promptTemplate: promptTemplate, schemaVersion: schemaVersion}
}

// Build monta request.
func (b *RequestBuilder) Build(opts RequestOptions) (*PeerRequest, error) {
	if opts.RunID == "" {
		return nil, errors.New("peer: RunID vazio")
	}
	if opts.Actor == "" {
		return nil, errors.New("peer: Actor vazio")
	}
	if opts.Evidence == nil {
		return nil, errors.New("peer: Evidence nil")
	}
	vars := make(map[string]string)
	for k, v := range opts.Vars {
		vars[k] = v
	}
	vars["run_id"] = opts.RunID
	vars["actor"] = opts.Actor
	vars["evidence_hash"] = opts.Evidence.Hash()
	vars["schema_version"] = b.schemaVersion
	if opts.SchemaVersion != "" {
		vars["schema_version"] = opts.SchemaVersion
	}
	// Render prompt.
	prompt, err := renderTemplate(b.promptTemplate, vars)
	if err != nil {
		return nil, fmt.Errorf("peer: render: %w", err)
	}
	req := &PeerRequest{
		RunID:    opts.RunID,
		Actor:    opts.Actor,
		Schema:   b.schemaVersion,
		Prompt:   prompt,
		Evidence: opts.Evidence,
		Vars:     vars,
	}
	req.Hash = req.computeHash()
	return req, nil
}

// computeHash do request (sem Prompt raw para evitar expor PII).
func (r *PeerRequest) computeHash() string {
	h := sha256.New()
	h.Write([]byte(r.RunID))
	h.Write([]byte{0})
	h.Write([]byte(r.Actor))
	h.Write([]byte{0})
	h.Write([]byte(r.Schema))
	h.Write([]byte{0})
	if r.Evidence != nil {
		h.Write([]byte(r.Evidence.Hash()))
	}
	h.Write([]byte{0})
	// hash do vars ordenado
	keys := make([]string, 0, len(r.Vars))
	for k := range r.Vars {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(r.Vars[k]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// sortStrings ordena em ordem alfabética.
func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}

// renderTemplate helper mínimo: substitui {{name}}.
func renderTemplate(tpl string, vars map[string]string) (string, error) {
	if tpl == "" {
		return "", nil
	}
	out := tpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out, nil
}

// MarshalJSON serializa request.
func (r *PeerRequest) MarshalJSONBytes() ([]byte, error) {
	return json.Marshal(r)
}

// HasPeerAOutput checa se request contém output do Peer A
// (palavras-chave típicas). Spec: nunca deve incluir.
func (r *PeerRequest) HasPeerAOutput(peerAOutput string) bool {
	if r == nil || peerAOutput == "" {
		return false
	}
	// Check 1: prompt contém peer_a_marker
	if strings.Contains(r.Prompt, "PEER_A_OUTPUT:") {
		return true
	}
	// Check 2: prompt contém primeiros 100 chars do output.
	marker := peerAOutput
	if len(marker) > 100 {
		marker = marker[:100]
	}
	if marker != "" && strings.Contains(r.Prompt, marker) {
		return true
	}
	return false
}

// VerifyIndependence checa request não tem cross-contamination.
func (r *PeerRequest) VerifyIndependence(peerAOutput string) []string {
	var issues []string
	if r.HasPeerAOutput(peerAOutput) {
		issues = append(issues, "prompt contém output do Peer A")
	}
	if r.Evidence == nil {
		issues = append(issues, "evidence nil")
	}
	if r.Evidence != nil && len(r.Evidence.Shards) == 0 {
		issues = append(issues, "evidence sem shards")
	}
	if r.RunID == "" {
		issues = append(issues, "run_id vazio")
	}
	return issues
}

// FormatRequest human-readable para debug.
func FormatRequest(r *PeerRequest) string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PeerRequest[%s] actor=%s schema=%s\n", r.RunID, r.Actor, r.Schema))
	sb.WriteString(fmt.Sprintf("  hash=%s\n", r.Hash[:8]))
	if r.Evidence != nil {
		sb.WriteString(fmt.Sprintf("  evidence: %d shards, %d bytes, hash=%s\n",
			len(r.Evidence.Shards), r.Evidence.TotalBytes, r.Evidence.Hash()[:8]))
	}
	return sb.String()
}
