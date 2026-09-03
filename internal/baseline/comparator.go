// Package baseline — load test baseline storage + comparator.
//
// SAI-049: persiste LoadResult como baseline canônico por
// fingerprint; compara novo run contra baseline selecionado
// por branch policy; decisão de promoção/update.
package baseline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/diegoaraujo/solidify/internal/load"
)

// Branch branch policy.
type Branch string

const (
	BranchMain    Branch = "main"
	BranchDevelop Branch = "develop"
	BranchFeature Branch = "feature"
	BranchRelease Branch = "release"
)

// promotionPriority higher wins.
var promotionPriority = map[Branch]int{
	BranchMain:    100,
	BranchRelease: 80,
	BranchDevelop: 50,
	BranchFeature: 10,
}

// LoadResultMeta wraps load.LoadResult com metadata de captura.
type LoadResultMeta struct {
	Result     *load.LoadResult `json:"result"`
	CapturedAt time.Time        `json:"captured_at"`
	RefRunID   string           `json:"ref_run_id"`
	Branch     Branch           `json:"branch"`
	Notes      string           `json:"notes,omitempty"`
}

// BaselineStore gerencia baselines em filesystem.
type BaselineStore struct {
	dir string
	mu  sync.Mutex
}

// NewBaselineStore cria store em dir (criado se não existe).
func NewBaselineStore(dir string) (*BaselineStore, error) {
	if dir == "" {
		return nil, errors.New("baseline: dir vazio")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("baseline: mkdir: %w", err)
	}
	return &BaselineStore{dir: dir}, nil
}

// Path resolve path do baseline por fingerprint.
func (s *BaselineStore) Path(fingerprint string) string {
	return filepath.Join(s.dir, fingerprint+".json")
}

// Save escreve baseline atomicamente (write tmp + rename).
func (s *BaselineStore) Save(b *LoadResultMeta) error {
	if b == nil {
		return errors.New("baseline: nil")
	}
	if b.Result == nil {
		return errors.New("baseline: result nil")
	}
	if b.Result.Fingerprint == "" {
		return errors.New("baseline: fingerprint vazio")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("baseline: marshal: %w", err)
	}
	final := s.Path(b.Result.Fingerprint)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("baseline: write tmp: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("baseline: rename: %w", err)
	}
	return nil
}

// Load lê baseline por fingerprint.
func (s *BaselineStore) Load(fingerprint string) (*LoadResultMeta, error) {
	if fingerprint == "" {
		return nil, errors.New("baseline: fingerprint vazio")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.Path(fingerprint))
	if err != nil {
		return nil, fmt.Errorf("baseline: read: %w", err)
	}
	var b LoadResultMeta
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("baseline: parse: %w", err)
	}
	return &b, nil
}

// Delete remove baseline.
func (s *BaselineStore) Delete(fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.Path(fingerprint))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// List retorna todos baselines, opcionalmente filtrado por branch.
func (s *BaselineStore) List(branches ...Branch) ([]*LoadResultMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("baseline: readdir: %w", err)
	}
	want := make(map[Branch]bool)
	for _, b := range branches {
		want[b] = true
	}
	var out []*LoadResultMeta
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var b LoadResultMeta
		if err := json.Unmarshal(data, &b); err != nil {
			continue
		}
		if len(want) > 0 && !want[b.Branch] {
			continue
		}
		out = append(out, &b)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CapturedAt.After(out[j].CapturedAt)
	})
	return out, nil
}

// SelectBaselineForBranch escolhe baseline de maior prioridade
// entre branches disponíveis. branches é ordered: tentar
// [main, develop, feature] — pegar o primeiro match.
func (s *BaselineStore) SelectBaselineForBranch(fingerprint string, branches []Branch) (*LoadResultMeta, error) {
	if fingerprint == "" {
		return nil, errors.New("baseline: fingerprint vazio")
	}
	// First try exact fingerprint match preferring branch priority.
	for _, br := range branches {
		all, err := s.List(br)
		if err != nil {
			continue
		}
		for _, b := range all {
			if b.Result != nil && b.Result.Fingerprint == fingerprint {
				return b, nil
			}
		}
	}
	return nil, nil
}

// SelectHighestPriorityBaseline retorna baseline mais prioritário
// (= branch com promotionPriority maior) entre os disponíveis
// para o fingerprint.
func (s *BaselineStore) SelectHighestPriorityBaseline(fingerprint string) (*LoadResultMeta, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	var best *LoadResultMeta
	bestScore := -1
	for _, b := range all {
		if b.Result == nil || b.Result.Fingerprint != fingerprint {
			continue
		}
		score := promotionPriority[b.Branch]
		if score > bestScore {
			best = b
			bestScore = score
		}
	}
	return best, nil
}

// DecisionKind tipo de decisão.
type DecisionKind string

const (
	DecisionPromote       DecisionKind = "promote"         // novo run vira baseline
	DecisionKeep          DecisionKind = "keep"            // baseline atual mantido
	DecisionReject        DecisionKind = "reject"          // regressão detectada
	DecisionRequireNewRun DecisionKind = "require_new_run" // erro/inconsistência
)

// Decision resultado da comparação.
type Decision struct {
	Kind       DecisionKind     `json:"kind"`
	Baseline   *LoadResultMeta  `json:"baseline,omitempty"`
	Current    *load.LoadResult `json:"current,omitempty"`
	Delta      *load.Delta      `json:"delta,omitempty"`
	Reason     string           `json:"reason"`
	ComparedAt time.Time        `json:"compared_at"`
}

// CompareDecision compara current contra baseline selecionado e
// decide promoção. th opcional — default = DefaultRegressionThreshold.
func (s *BaselineStore) CompareDecision(current *load.LoadResult, branches []Branch, th load.RegressionThreshold) (*Decision, error) {
	if current == nil {
		return &Decision{
			Kind:       DecisionReject,
			Reason:     "current nil",
			ComparedAt: time.Now(),
		}, nil
	}
	baseline, err := s.SelectBaselineForBranch(current.Fingerprint, branches)
	if err != nil {
		return nil, err
	}
	d := &Decision{
		Current:    current,
		Baseline:   baseline,
		ComparedAt: time.Now(),
	}
	if baseline == nil {
		d.Kind = DecisionPromote
		d.Reason = "sem baseline para fingerprint + branch"
		return d, nil
	}
	delta, err := load.CompareToBaseline(current, baseline.Result, th)
	if err != nil {
		d.Kind = DecisionRequireNewRun
		d.Reason = fmt.Sprintf("comparação falhou: %v", err)
		return d, nil
	}
	d.Delta = delta
	if delta.Regression {
		d.Kind = DecisionReject
		d.Reason = "regressão detectada"
		return d, nil
	}
	d.Kind = DecisionPromote
	d.Reason = "passou thresholds"
	return d, nil
}

// PromoteIfBetter atualiza baseline se Decision.Kind == Promote
// + current é melhor (P95 menor) ou primeiro registro.
func (s *BaselineStore) PromoteIfBetter(d *Decision, refRunID string, branch Branch, notes string) error {
	if d == nil {
		return errors.New("baseline: decision nil")
	}
	if d.Kind != DecisionPromote {
		return fmt.Errorf("baseline: decisão %s não promove", d.Kind)
	}
	// Se já tem baseline E current não é melhor, mantém.
	if d.Baseline != nil && d.Current != nil {
		if d.Current.P95 >= d.Baseline.Result.P95 {
			return nil
		}
	}
	return s.Save(&LoadResultMeta{
		Result:     d.Current,
		CapturedAt: time.Now(),
		RefRunID:   refRunID,
		Branch:     branch,
		Notes:      notes,
	})
}

// ValidateChecksums checagens básicas de metadata.
func (b *LoadResultMeta) Validate() error {
	if b == nil {
		return errors.New("baseline nil")
	}
	if b.Result == nil {
		return errors.New("result nil")
	}
	if b.Result.Fingerprint == "" {
		return errors.New("fingerprint vazio")
	}
	if b.Result.Target == "" {
		return errors.New("target vazio")
	}
	if b.Result.P95 == 0 {
		return errors.New("P95 zero")
	}
	return nil
}

// ShouldRun verifica condições mínimas pra baseline.
func (b *LoadResultMeta) ShouldRun() bool {
	return b != nil && b.Result != nil && b.Result.P95 > 0
}

// AgingDays retorna idade do baseline em dias.
func (b *LoadResultMeta) AgingDays(now time.Time) float64 {
	if b == nil {
		return 0
	}
	return now.Sub(b.CapturedAt).Hours() / 24
}

// IsStale devolve true se baseline é mais velho que maxAge.
func (b *LoadResultMeta) IsStale(now time.Time, maxAge time.Duration) bool {
	if b == nil {
		return true
	}
	return now.Sub(b.CapturedAt) > maxAge
}

// DecisionToReason helper de serialização.
func DecisionToReason(d *Decision) string {
	if d == nil {
		return ""
	}
	return d.Reason
}

// SortByCapturedAt ordena Desc (mais recente primeiro).
func SortByCapturedAt(bs []*LoadResultMeta) {
	sort.Slice(bs, func(i, j int) bool {
		return bs[i].CapturedAt.After(bs[j].CapturedAt)
	})
}

// FilterByFingerprint retorna subset com fingerprint match.
func FilterByFingerprint(bs []*LoadResultMeta, fingerprint string) []*LoadResultMeta {
	out := make([]*LoadResultMeta, 0)
	for _, b := range bs {
		if b.Result != nil && b.Result.Fingerprint == fingerprint {
			out = append(out, b)
		}
	}
	return out
}
