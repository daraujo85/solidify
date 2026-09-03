// Artifact immutability + hash (SAI-078).
//
// Finalized report não é modificado: rerun produz novo
// run_id + content_hash. Store é append-only.
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Store append-only de reports finalizados.
type Store struct {
	mu     sync.RWMutex
	byID   map[string]*StoredReport
	byHash map[string][]string // hash → ids
}

// StoredReport entrada imutável.
type StoredReport struct {
	RunID       string    `json:"run_id"`
	ContentHash string    `json:"content_hash"`
	FinalizedAt time.Time `json:"finalized_at"`
	Path        string    `json:"path"`
	SizeBytes   int       `json:"size_bytes"`
	Report      *Report   `json:"report,omitempty"`
}

// NewStore cria store vazio.
func NewStore() *Store {
	return &Store{
		byID:   map[string]*StoredReport{},
		byHash: map[string][]string{},
	}
}

// ErrDuplicateRunID run_id já existe.
var ErrDuplicateRunID = errors.New("store: run_id duplicado")

// ErrReportMutated tentativa de alterar report finalizado.
var ErrReportMutated = errors.New("store: report finalizado é imutável")

// ErrHashMismatch conteúdo não bate com hash registrado.
var ErrHashMismatch = errors.New("store: hash não bate")

// Finalize registra report (imutável após isso).
func (s *Store) Finalize(r *Report, path string) (*StoredReport, error) {
	if r == nil {
		return nil, errors.New("store: report nil")
	}
	if r.Run.ID == "" {
		return nil, errors.New("store: run_id vazio")
	}
	h, err := r.HashContent()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.byID[r.Run.ID]; ok {
		// mesmo id: re-finalize = erro se hash mudou.
		if existing.ContentHash != h {
			return nil, fmt.Errorf("%w: run_id=%s hash divergente", ErrReportMutated, r.Run.ID)
		}
		return existing, nil
	}
	sr := &StoredReport{
		RunID:       r.Run.ID,
		ContentHash: h,
		FinalizedAt: time.Now().UTC(),
		Path:        path,
		Report:      r,
	}
	s.byID[r.Run.ID] = sr
	s.byHash[h] = append(s.byHash[h], r.Run.ID)
	return sr, nil
}

// FinalizeBytes finaliza a partir de bytes (re-valida hash).
func (s *Store) FinalizeBytes(runID string, data []byte, path string) (*StoredReport, error) {
	if runID == "" {
		return nil, errors.New("store: run_id vazio")
	}
	// hash do conteúdo bruto.
	sum := sha256.Sum256(data)
	h := hex.EncodeToString(sum[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.byID[runID]; ok {
		if existing.ContentHash != h {
			return nil, fmt.Errorf("%w: run_id=%s hash divergente", ErrReportMutated, runID)
		}
		return existing, nil
	}
	sr := &StoredReport{
		RunID:       runID,
		ContentHash: h,
		FinalizedAt: time.Now().UTC(),
		Path:        path,
		SizeBytes:   len(data),
	}
	s.byID[runID] = sr
	s.byHash[h] = append(s.byHash[h], runID)
	return sr, nil
}

// Get busca por id (retorna cópia rasa — caller não muta store).
func (s *Store) Get(runID string) (*StoredReport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byID[runID]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, ok
}

// ByHash retorna ids com hash.
func (s *Store) ByHash(h string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := append([]string{}, s.byHash[h]...)
	sort.Strings(ids)
	return ids
}

// VerifyHash checa se run_id bate com conteúdo.
func (s *Store) VerifyHash(runID string, data []byte) error {
	sr, ok := s.Get(runID)
	if !ok {
		return errors.New("store: run_id não-encontrado")
	}
	sum := sha256.Sum256(data)
	h := hex.EncodeToString(sum[:])
	if h != sr.ContentHash {
		return fmt.Errorf("%w: esperado=%s got=%s", ErrHashMismatch, sr.ContentHash, h)
	}
	return nil
}

// VerifyReport checa report (Report) vs hash.
func (s *Store) VerifyReport(r *Report) error {
	if r == nil {
		return errors.New("store: report nil")
	}
	sr, ok := s.Get(r.Run.ID)
	if !ok {
		return errors.New("store: run_id não-encontrado")
	}
	h, err := r.HashContent()
	if err != nil {
		return err
	}
	if h != sr.ContentHash {
		return fmt.Errorf("%w: esperado=%s got=%s", ErrHashMismatch, sr.ContentHash, h)
	}
	return nil
}

// List todos os runs (ordenado por RunID).
func (s *Store) List() []*StoredReport {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*StoredReport, 0, len(s.byID))
	for _, r := range s.byID {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RunID < out[j].RunID })
	return out
}

// Count retorna total de runs finalizados.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}

// MarshalSnapshot exporta estado imutável.
func (s *Store) MarshalSnapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]StoredReport, 0, len(s.byID))
	for _, r := range s.byID {
		// omite Report p/ snapshot leve.
		cp := *r
		cp.Report = nil
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RunID < out[j].RunID })
	return json.Marshal(out)
}

// NewRunID helper determinístico.
func NewRunID(prefix string, t time.Time) string {
	if prefix == "" {
		prefix = "run"
	}
	return fmt.Sprintf("%s-%d", prefix, t.UnixNano())
}
