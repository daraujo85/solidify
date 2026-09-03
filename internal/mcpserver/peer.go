// Peer review submission (SAI-053).
//
// Schema validation + actor metadata + evidence hash + atomic save.
package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PeerReviewSchemaVersion schema atual (SAI-120: bumped 1→2).
//
// v1: payload canônico embutido em `Notes` (string JSON).
// v2: payload canônico em `CanonicalPayload` (map nativo).
//     `Notes` mantido como campo livre pra comentários humanos.
//     Records v1 existentes continuam legíveis (recordToExecutorResult
//     parseia Notes como fallback), mas validator loga warning
//     em novos submissions v1 (deprecation).
const PeerReviewSchemaVersion = "2"

// PeerReviewValidator valida input.
type PeerReviewValidator struct {
	MinFindings     int      // mínimo findings (default 0)
	AllowedVerdicts []string // default ["approve", "request_changes", "comment"]
}

// DefaultPeerReviewValidator.
func DefaultPeerReviewValidator() *PeerReviewValidator {
	return &PeerReviewValidator{
		MinFindings:     0,
		AllowedVerdicts: []string{"approve", "request_changes", "comment"},
	}
}

// ValidationError item erros de validação.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors slice.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no errors"
	}
	parts := make([]string, len(e))
	for i, v := range e {
		parts[i] = v.Field + ": " + v.Message
	}
	return strings.Join(parts, "; ")
}

// Errors() pra error interface.
func (e ValidationErrors) Errors() []ValidationError { return e }

// Validate valida input conforme schema.
// SAI-120: aceita "1" (deprecated, Notes JSON-string) e "2" (canonical_payload nativo).
func (v *PeerReviewValidator) Validate(in *SubmitPeerReviewInput) ValidationErrors {
	var errs ValidationErrors
	if in.RunID == "" {
		errs = append(errs, ValidationError{Field: "run_id", Message: "required"})
	}
	if in.Actor == "" {
		errs = append(errs, ValidationError{Field: "actor", Message: "required"})
	}
	if in.Schema == "" {
		errs = append(errs, ValidationError{Field: "schema", Message: "required"})
	}
	if in.Schema != "" && in.Schema != PeerReviewSchemaVersion && in.Schema != "1" {
		errs = append(errs, ValidationError{Field: "schema", Message: "unsupported: " + in.Schema})
	}
	if in.EvidenceHash == "" {
		errs = append(errs, ValidationError{Field: "evidence_hash", Message: "required"})
	}
	if in.Verdict == "" {
		errs = append(errs, ValidationError{Field: "verdict", Message: "required"})
	}
	if in.Verdict != "" && !containsStr(v.AllowedVerdicts, in.Verdict) {
		errs = append(errs, ValidationError{Field: "verdict", Message: "must be one of: " + strings.Join(v.AllowedVerdicts, ", ")})
	}
	if v.MinFindings > 0 {
		count := len(in.Findings)
		if count < v.MinFindings {
			errs = append(errs, ValidationError{Field: "findings", Message: fmt.Sprintf("min %d required", v.MinFindings)})
		}
	}
	if len(in.Actor) > 200 {
		errs = append(errs, ValidationError{Field: "actor", Message: "max 200 chars"})
	}
	if len(in.EvidenceHash) != 64 {
		errs = append(errs, ValidationError{Field: "evidence_hash", Message: "must be 64 hex chars (sha256)"})
	} else if !isHex(in.EvidenceHash) {
		errs = append(errs, ValidationError{Field: "evidence_hash", Message: "must be hex"})
	}
	return errs
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// PeerReviewRecord persisted JSON.
type PeerReviewRecord struct {
	ReviewID     string         `json:"review_id"`
	RunID        string         `json:"run_id"`
	Actor        string         `json:"actor"`
	Schema       string         `json:"schema"`
	EvidenceHash string         `json:"evidence_hash"`
	Verdict      string         `json:"verdict"`
	Findings     map[string]any `json:"findings,omitempty"`
	// Notes é campo livre pra comentário humano (markdown livre).
	// Em v1 carregava o payload canônico JSON-string — deprecated.
	Notes string `json:"notes,omitempty"`
	// CanonicalPayload é o peer review canônico (SAI-116 schema)
	// como objeto nativo. SAI-120: substituiu `Notes` JSON-string.
	// v1 records continuam funcionando via fallback (Notes → parsed JSON).
	CanonicalPayload map[string]any `json:"canonical_payload,omitempty"`
	SubmittedAt      time.Time      `json:"submitted_at"`
	ContentHash      string         `json:"content_hash"`
}

// ComputeContentHash hash estável do conteúdo (sem timestamp).
// SAI-120: inclui CanonicalPayload no hash (campo nativo v2).
func ComputeContentHash(in *SubmitPeerReviewInput) string {
	data := struct {
		RunID            string         `json:"run_id"`
		Actor            string         `json:"actor"`
		Schema           string         `json:"schema"`
		EvidenceHash     string         `json:"evidence_hash"`
		Verdict          string         `json:"verdict"`
		Findings         map[string]any `json:"findings,omitempty"`
		Notes            string         `json:"notes,omitempty"`
		CanonicalPayload map[string]any `json:"canonical_payload,omitempty"`
	}{
		in.RunID, in.Actor, in.Schema, in.EvidenceHash, in.Verdict,
		in.Findings, in.Notes, in.CanonicalPayload,
	}
	b, _ := json.Marshal(data)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// PeerReviewStore persiste records atomicamente.
type PeerReviewStore struct {
	dir string
	mu  sync.Mutex
}

// NewPeerReviewStore cria store em dir.
func NewPeerReviewStore(dir string) (*PeerReviewStore, error) {
	if dir == "" {
		return nil, errors.New("peer: dir vazio")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &PeerReviewStore{dir: dir}, nil
}

// Path resolve path do review.
func (s *PeerReviewStore) Path(reviewID string) string {
	return filepath.Join(s.dir, reviewID+".json")
}

// Save atomic write (tmp + rename).
func (s *PeerReviewStore) Save(rec *PeerReviewRecord) error {
	if rec == nil {
		return errors.New("peer: nil record")
	}
	if rec.ReviewID == "" {
		return errors.New("peer: review_id vazio")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	final := s.Path(rec.ReviewID)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Load lê record.
func (s *PeerReviewStore) Load(reviewID string) (*PeerReviewRecord, error) {
	if reviewID == "" {
		return nil, errors.New("peer: review_id vazio")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.Path(reviewID))
	if err != nil {
		return nil, err
	}
	var rec PeerReviewRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// List devolve todos records persistidos (SAI-119: usado pelo
// orchestrator pra poll peer_a submission). Erro só se a varredura
// do diretório falhar — records corrompidos são pulados (best-effort).
func (s *PeerReviewStore) List() ([]*PeerReviewRecord, error) {
	if s == nil || s.dir == "" {
		return nil, errors.New("peer: store não inicializado")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := make([]*PeerReviewRecord, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if rerr != nil {
			continue
		}
		var rec PeerReviewRecord
		if uerr := json.Unmarshal(data, &rec); uerr != nil {
			continue // skip corrompido — auditoria melhor que abort
		}
		out = append(out, &rec)
	}
	return out, nil
}

// DefaultSubmitPeerReviewHandler handler default com validação + save.
// SAI-120: v2 aceita CanonicalPayload nativo; v1 ainda aceito pra
// backward compat (Notes JSON-string). Validator loga warning pra
// submissions v1 sinalizando deprecation.
// SAI-121: honra cutoff (fail V1AfterCutoff); após cutoff, v1
// retorna erro (não warning). Telemetry append em metricsSink.
func DefaultSubmitPeerReviewHandler(store *PeerReviewStore, validator *PeerReviewValidator) SubmitPeerReviewHandler {
	if validator == nil {
		validator = DefaultPeerReviewValidator()
	}
	return func(ctx context.Context, in *SubmitPeerReviewInput) (*SubmitPeerReviewOutput, error) {
		if in == nil {
			return nil, errors.New("input nil")
		}
		errs := validator.Validate(in)
		if len(errs) > 0 {
			return &SubmitPeerReviewOutput{
				Accepted:     false,
				ValidationOK: false,
				Errors:       errStrings(errs),
			}, nil
		}
		// SAI-120: warning deprecation pra v1 sem CanonicalPayload
		var warnings []string
		if in.Schema == "1" && len(in.CanonicalPayload) == 0 {
			warnings = append(warnings, "schema v1 deprecated: use canonical_payload (schema v2)")
		}
		// SAI-121: se cutoff venceu, v1 vira erro (hard fail).
		if v1AfterCutoff(in.Schema) {
			return &SubmitPeerReviewOutput{
				Accepted:     false,
				ValidationOK: false,
				Errors:       []string{"schema v1 rejected: cutoff date passed; use canonical_payload (schema v2)"},
				Warnings:     warnings,
			}, nil
		}
		reviewID := deriveReviewID(in)
		rec := &PeerReviewRecord{
			ReviewID:         reviewID,
			RunID:            in.RunID,
			Actor:            in.Actor,
			Schema:           in.Schema,
			EvidenceHash:     in.EvidenceHash,
			Verdict:          in.Verdict,
			Findings:         in.Findings,
			Notes:            in.Notes,
			CanonicalPayload: in.CanonicalPayload,
			SubmittedAt:      time.Now(),
			ContentHash:      ComputeContentHash(in),
		}
		if err := store.Save(rec); err != nil {
			return &SubmitPeerReviewOutput{
				Accepted: false,
				Errors:   []string{"save: " + err.Error()},
			}, nil
		}
		// SAI-121: telemetry append (best-effort, não bloqueia save).
		_ = AppendMetric(Metric{
			Timestamp: rec.SubmittedAt,
			Schema:    rec.Schema,
			Actor:     rec.Actor,
			ReviewID:  rec.ReviewID,
			Verdict:   rec.Verdict,
		})
		return &SubmitPeerReviewOutput{
			Accepted:     true,
			ReviewID:     reviewID,
			SavedAt:      rec.SubmittedAt.Format(time.RFC3339Nano),
			ValidationOK: true,
			Warnings:     warnings,
		}, nil
	}
}

// v1AfterCutoff checa se schema v1 deve ser rejeitado (hard fail).
// SAI-121: cutoff carregado de V1Cutoff global; se zero (não setado),
// nunca rejeita. Função pública pra testes.
func v1AfterCutoff(schema string) bool {
	if schema != "1" {
		return false
	}
	if V1Cutoff.IsZero() {
		return false
	}
	return time.Now().After(V1Cutoff)
}

func errStrings(errs ValidationErrors) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Field + ": " + e.Message
	}
	return out
}

func deriveReviewID(in *SubmitPeerReviewInput) string {
	// review_id = sha256(runID|actor|contentHash)[:16]
	sum := sha256.Sum256([]byte(in.RunID + "|" + in.Actor + "|" + ComputeContentHash(in)))
	return hex.EncodeToString(sum[:])[:16]
}

// ValidateEvidenceHash checa consistência do hash externo.
func ValidateEvidenceHash(in *SubmitPeerReviewInput, expected string) bool {
	return in.EvidenceHash == expected
}
