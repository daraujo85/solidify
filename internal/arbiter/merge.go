// Arbitration merge (SAI-070).
//
// Accepted/rejected/merged findings; final S/O/L/I/D.
// Aceitação: árbitro não pode alterar resultado objetivo
// de teste/Sonar/ZAP (imutável).
package arbiter

import (
	"errors"
	"sort"
	"strings"
)

// FindingKind tipos.
const (
	KindTest       = "test"
	KindSonar      = "sonar"
	KindZAP        = "zap"
	KindSOLID      = "solid"
	KindSubjective = "subjective"
)

// FinalStatus codes (S/O/L/I/D).
const (
	StatusShip        = "S" // ship
	StatusObserve     = "O" // observe
	StatusLimit       = "L" // limit
	StatusInvestigate = "I" // investigate
	StatusDefer       = "D" // defer
)

// Finding input p/ merge.
type Finding struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"` // test/sonar/zap/solid/subjective
	Principle  string `json:"principle,omitempty"`
	Severity   string `json:"severity"` // low|medium|high|critical
	Symbol     string `json:"symbol,omitempty"`
	Note       string `json:"note,omitempty"`
	Immutable  bool   `json:"immutable"` // test/sonar/zap = true
	Accepted   bool   `json:"accepted"`
	Rejected   bool   `json:"rejected"`
	Merged     bool   `json:"merged"`
	SourcePeer string `json:"source_peer,omitempty"` // A|B|both|none
}

// MergeInput input do merge.
type MergeInput struct {
	RunID         string    `json:"run_id"`
	PeerAFindings []Finding `json:"peer_a_findings"`
	PeerBFindings []Finding `json:"peer_b_findings"`
	Verdict       *Verdict  `json:"verdict"`
}

// MergeResult saída.
type MergeResult struct {
	RunID       string    `json:"run_id"`
	Accepted    []Finding `json:"accepted"`
	Rejected    []Finding `json:"rejected"`
	Merged      []Finding `json:"merged"`
	Immutable   []Finding `json:"immutable"`
	FinalStatus string    `json:"final_status"`
	Reasoning   string    `json:"reasoning,omitempty"`
}

// Merge aplica veredito do árbitro sobre findings.
// Aceitação: findings imutáveis (test/sonar/zap) não podem ser
// alteradas pelo árbitro — sempre preservadas.
func Merge(in MergeInput) (*MergeResult, error) {
	if in.RunID == "" {
		return nil, errors.New("arbiter merge: RunID vazio")
	}
	if in.Verdict == nil {
		return nil, errors.New("arbiter merge: Verdict obrigatório")
	}
	res := &MergeResult{
		RunID:     in.RunID,
		Accepted:  []Finding{},
		Rejected:  []Finding{},
		Merged:    []Finding{},
		Immutable: []Finding{},
	}
	// Index immutable findings first.
	res.Immutable = collectImmutable(in.PeerAFindings, in.PeerBFindings)
	// Process resolutions do verdict.
	for _, r := range in.Verdict.Resolutions {
		switch r.Decision {
		case DecisionAcceptPeerA:
			acceptFromPeer(&res.Accepted, in.PeerAFindings, r.Topic)
		case DecisionAcceptPeerB:
			acceptFromPeer(&res.Accepted, in.PeerBFindings, r.Topic)
		case DecisionAcceptBoth:
			merged := mergeBoth(in.PeerAFindings, in.PeerBFindings, r.Topic)
			if merged != nil {
				merged.Merged = true
				res.Merged = append(res.Merged, *merged)
			}
		case DecisionRejectBoth:
			rej := collectBoth(in.PeerAFindings, in.PeerBFindings, r.Topic)
			res.Rejected = append(res.Rejected, rej...)
		}
	}
	// Dedup.
	res.Accepted = dedup(res.Accepted)
	res.Rejected = dedup(res.Rejected)
	res.Merged = dedup(res.Merged)
	res.Immutable = dedup(res.Immutable)
	// Final status.
	res.FinalStatus = ComputeFinalStatus(res)
	res.Reasoning = in.Verdict.Reasoning
	return res, nil
}

// collectImmutable findings imutáveis de ambos os peers (test/sonar/zap).
func collectImmutable(a, b []Finding) []Finding {
	var out []Finding
	for _, f := range a {
		if f.Immutable || isImmutableKind(f.Kind) {
			out = append(out, f)
		}
	}
	for _, f := range b {
		if f.Immutable || isImmutableKind(f.Kind) {
			out = append(out, f)
		}
	}
	return out
}

func isImmutableKind(k string) bool {
	return k == KindTest || k == KindSonar || k == KindZAP
}

// acceptFromPeer moves matching finding to accepted.
func acceptFromPeer(target *[]Finding, source []Finding, topic string) {
	parts := strings.Split(topic, ":")
	if len(parts) < 2 {
		return
	}
	principle := parts[0]
	id := strings.Join(parts[1:], ":")
	for _, f := range source {
		if f.ID == id && f.Principle == principle {
			acc := f
			acc.Accepted = true
			*target = append(*target, acc)
			return
		}
	}
}

// mergeBoth pega finding de A ou B, marca merged=true.
func mergeBoth(a, b []Finding, topic string) *Finding {
	parts := strings.Split(topic, ":")
	if len(parts) < 2 {
		return nil
	}
	principle := parts[0]
	id := strings.Join(parts[1:], ":")
	// Tenta A primeiro, fallback B.
	for _, src := range [][]Finding{a, b} {
		for _, f := range src {
			if f.ID == id && f.Principle == principle {
				merged := f
				merged.SourcePeer = "both"
				return &merged
			}
		}
	}
	return nil
}

// collectBoth findings de ambos os peers com mesmo topic.
func collectBoth(a, b []Finding, topic string) []Finding {
	var out []Finding
	parts := strings.Split(topic, ":")
	if len(parts) < 2 {
		return nil
	}
	principle := parts[0]
	id := strings.Join(parts[1:], ":")
	for _, src := range [][]Finding{a, b} {
		for _, f := range src {
			if f.ID == id && f.Principle == principle {
				rej := f
				rej.Rejected = true
				out = append(out, rej)
			}
		}
	}
	return out
}

// dedup removes duplicates by ID.
func dedup(in []Finding) []Finding {
	seen := make(map[string]bool)
	out := make([]Finding, 0, len(in))
	for _, f := range in {
		if seen[f.ID] {
			continue
		}
		seen[f.ID] = true
		out = append(out, f)
	}
	return out
}

// computeFinalStatus deriva S/O/L/I/D.
func ComputeFinalStatus(r *MergeResult) string {
	if r == nil {
		return StatusShip
	}
	// Qualquer immutable crítico → I (investigate) ou L (limit).
	for _, f := range r.Immutable {
		if f.Severity == "critical" || f.Severity == "high" {
			return StatusLimit
		}
	}
	// Qualquer accepted/merged crítico → L.
	for _, f := range r.Accepted {
		if f.Severity == "critical" || f.Severity == "high" {
			return StatusLimit
		}
	}
	for _, f := range r.Merged {
		if f.Severity == "critical" || f.Severity == "high" {
			return StatusLimit
		}
	}
	// Medium → O (observe).
	for _, f := range r.Accepted {
		if f.Severity == "medium" {
			return StatusObserve
		}
	}
	for _, f := range r.Merged {
		if f.Severity == "medium" {
			return StatusObserve
		}
	}
	// Tudo low → S (ship).
	if len(r.Accepted) == 0 && len(r.Merged) == 0 && len(r.Rejected) == 0 {
		return StatusShip
	}
	return StatusDefer
}

// HasImmutableCritical helper.
func (r *MergeResult) HasImmutableCritical() bool {
	for _, f := range r.Immutable {
		if f.Severity == "critical" || f.Severity == "high" {
			return true
		}
	}
	return false
}

// TotalFindings count.
func (r *MergeResult) TotalFindings() int {
	if r == nil {
		return 0
	}
	return len(r.Accepted) + len(r.Rejected) + len(r.Merged) + len(r.Immutable)
}

// AcceptedCount helper.
func (r *MergeResult) AcceptedCount() int {
	if r == nil {
		return 0
	}
	return len(r.Accepted)
}

// MergedCount helper.
func (r *MergeResult) MergedCount() int {
	if r == nil {
		return 0
	}
	return len(r.Merged)
}

// RejectedCount helper.
func (r *MergeResult) RejectedCount() int {
	if r == nil {
		return 0
	}
	return len(r.Rejected)
}

// ImmutableCount helper.
func (r *MergeResult) ImmutableCount() int {
	if r == nil {
		return 0
	}
	return len(r.Immutable)
}

// RenderMerge textual.
func RenderMerge(r *MergeResult) string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("MergeResult[")
	sb.WriteString(r.RunID)
	sb.WriteString(" status=")
	sb.WriteString(r.FinalStatus)
	sb.WriteString("]\n")
	sb.WriteString("  accepted: ")
	sb.WriteString(itoaCount(len(r.Accepted)))
	sb.WriteString("\n")
	sb.WriteString("  rejected: ")
	sb.WriteString(itoaCount(len(r.Rejected)))
	sb.WriteString("\n")
	sb.WriteString("  merged:   ")
	sb.WriteString(itoaCount(len(r.Merged)))
	sb.WriteString("\n")
	sb.WriteString("  immutable: ")
	sb.WriteString(itoaCount(len(r.Immutable)))
	sb.WriteString("\n")
	if r.Reasoning != "" {
		sb.WriteString("  reasoning: ")
		sb.WriteString(r.Reasoning)
		sb.WriteString("\n")
	}
	return sb.String()
}

func itoaCount(n int) string {
	if n == 0 {
		return "0"
	}
	out := []byte{}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}

// sortableFindings helper.
func sortFindings(in []Finding) {
	sort.Slice(in, func(i, j int) bool {
		return in[i].ID < in[j].ID
	})
}

// SortFindingsByID exportável.
func SortFindingsByID(in []Finding) {
	sortFindings(in)
}
