// Canonical report builder (SAI-077).
//
// Monta report canônico a partir das engines: SOLID, peer,
// arbiter, risk, confidence, gate. Zero UI/HTML aqui — só
// estrutura. UI lê o JSON.
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

// SchemaVersion report. SAI-129D: 1.1.0 adiciona `applicability` enum +
// `reason` + `evidence_refs` por princípio. `applicable` bool continua
// presente (derivado) por compat.
const SchemaVersion = "1.1.0"

// Report canônico.
type Report struct {
	SchemaVersion    string            `json:"schema_version"`
	Status           string            `json:"status,omitempty"`
	FailedStage      string            `json:"failed_stage,omitempty"`
	ReasonCode       int               `json:"reason_code,omitempty"`
	Failure          map[string]any    `json:"failure,omitempty"`
	Run              RunInfo           `json:"run"`
	Git              GitInfo           `json:"git"`
	Components       []Component       `json:"components"`
	ReleaseNotes     ReleaseNotes      `json:"release_notes"`
	Migrations       []Migration       `json:"migrations"`
	EnvChanges       []EnvChange       `json:"env_changes"`
	Analyzers        []Analyzer        `json:"analyzers"`
	SOLID            SOLIDBlock        `json:"solid"`
	AIReview         AIReview          `json:"ai_review"`
	Scores           ScoresBlock       `json:"scores"`
	Risk             RiskBlock         `json:"risk"`
	QualityGate      QualityGate       `json:"quality_gate"`
	IndependenceGate *IndependenceGate `json:"independence_gate,omitempty"` // SAI-117
	FinalGate        *FinalGate        `json:"final_gate,omitempty"`        // SAI-117
	Recommendations  []Recommendation  `json:"recommendations"`
	Limitations      []string          `json:"limitations"`
	Artifacts        []Artifact        `json:"artifacts"`
	Extra            map[string]any    `json:"-"`
}

// RunInfo identifica run.
type RunInfo struct {
	ID              string  `json:"id"`
	Profile         string  `json:"profile"`
	SolidifyVersion string  `json:"solidify_version"`
	StartedAt       string  `json:"started_at"`
	FinishedAt      string  `json:"finished_at"`
	ConfigHash      string  `json:"config_hash"`
	EvidenceHash    string  `json:"evidence_hash"`
	OS              *string `json:"os,omitempty"`
	Arch            *string `json:"arch,omitempty"`
}

// GitInfo contexto git.
type GitInfo struct {
	BaseRef string `json:"base_ref"`
	BaseSHA string `json:"base_sha"`
	HeadRef string `json:"head_ref"`
	HeadSHA string `json:"head_sha"`
	// MergeBaseSHA — SHA do merge-base(base, head). Distinto de BaseSHA
	// quando base avançou desde que head foi criado.
	MergeBaseSHA string `json:"merge_base_sha,omitempty"`
	// DiffStrategy — como changed_files/commits foram calculados
	// (ex.: "merge-base", "two-dot", "three-dot").
	DiffStrategy string        `json:"diff_strategy,omitempty"`
	Commits      []Commit      `json:"commits"`
	ChangedFiles []ChangedFile `json:"changed_files"`
}

// Commit info.
type Commit struct {
	SHA        string   `json:"sha"`
	ShortSHA   string   `json:"short_sha"`
	Subject    string   `json:"subject"`
	Type       string   `json:"type"`
	Scope      *string  `json:"scope,omitempty"`
	Breaking   bool     `json:"breaking"`
	Issues     []string `json:"issues,omitempty"`
	Components []string `json:"components,omitempty"`
}

// ChangedFile info.
type ChangedFile struct {
	Path         string  `json:"path"`
	OldPath      *string `json:"old_path,omitempty"`
	Status       string  `json:"status"`
	AddedLines   *int    `json:"added_lines,omitempty"`
	DeletedLines *int    `json:"deleted_lines,omitempty"`
	Binary       bool    `json:"binary"`
}

// Component info.
type Component struct {
	ID                string   `json:"id"`
	Root              string   `json:"root"`
	Types             []string `json:"types"`
	Stacks            []string `json:"stacks"`
	ChangedFilesCount int      `json:"changed_files_count"`
}

// ReleaseNotes agregado.
type ReleaseNotes struct {
	ExecutiveSummary string        `json:"executive_summary"`
	Groups           []ChangeGroup `json:"groups"`
	BreakingChanges  []ChangeItem  `json:"breaking_changes"`
}

// ChangeGroup features/bugfix/etc.
type ChangeGroup struct {
	Type  string       `json:"type"`
	Items []ChangeItem `json:"items"`
}

// ChangeItem individual.
type ChangeItem struct {
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	Type         string   `json:"type"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// Migration info.
type Migration struct {
	ID              string           `json:"id"`
	Path            string           `json:"path"`
	Framework       string           `json:"framework"`
	Status          string           `json:"status"`
	Operations      []map[string]any `json:"operations"`
	RollbackPresent *bool            `json:"rollback_present,omitempty"`
	Risk            string           `json:"risk"`
	EvidenceRefs    []string         `json:"evidence_refs,omitempty"`
}

// EnvChange info.
type EnvChange struct {
	Name             string           `json:"name"`
	Status           string           `json:"status"`
	Components       []string         `json:"components"`
	Required         *bool            `json:"required,omitempty"`
	HasDefault       *bool            `json:"has_default,omitempty"`
	Documented       bool             `json:"documented"`
	LikelySecret     bool             `json:"likely_secret"`
	References       []map[string]any `json:"references"`
	DeploymentAction string           `json:"deployment_action"`
}

// Analyzer info.
type Analyzer struct {
	ID              string           `json:"id"`
	Version         string           `json:"version"`
	Applicability   string           `json:"applicability"`
	ExecutionStatus *string          `json:"execution_status,omitempty"`
	Score           *float64         `json:"score,omitempty"`
	DurationMS      int              `json:"duration_ms"`
	Findings        []map[string]any `json:"findings"`
	Metrics         map[string]any   `json:"metrics"`
	Limitations     []string         `json:"limitations,omitempty"`
}

// SOLIDBlock aggregate.
type SOLIDBlock struct {
	Score       *float64             `json:"score,omitempty"`
	BeforeScore *float64             `json:"before_score,omitempty"`
	Delta       *float64             `json:"delta,omitempty"`
	Principles  map[string]Principle `json:"principles"`
}

// Principle detalhe.
//
// SAI-129D: Applicability (enum: APPLICABLE/NOT_APPLICABLE/INSUFFICIENT_EVIDENCE)
// é a verdade sobre se o princípio foi avaliado. `Applicable` bool é
// derivado por compat (Applicable = (Applicability == "APPLICABLE")).
// Reason + EvidenceRefs são a evidência mínima exigida quando
// Applicability == APPLICABLE — sem evidência não há score válido.
type Principle struct {
	Applicability string           `json:"applicability"`
	Applicable    bool             `json:"applicable"` // derivado de Applicability; mantido pra compat
	BeforeScore   *float64         `json:"before_score,omitempty"`
	AfterScore    *float64         `json:"after_score,omitempty"`
	Delta         *float64         `json:"delta,omitempty"`
	Confidence    float64          `json:"confidence"`
	Reason        string           `json:"reason"`
	EvidenceRefs  []string         `json:"evidence_refs,omitempty"`
	Summary       string           `json:"summary"`
	Findings      []map[string]any `json:"findings"`
}

// AIReview info.
type AIReview struct {
	Mode                 string      `json:"mode"`
	PeerReviewed         bool        `json:"peer_reviewed"`
	Actors               []Actor     `json:"actors"`
	Agreement            *float64    `json:"agreement,omitempty"`
	IndependenceDegraded bool        `json:"independence_degraded"`
	Divergences          []string    `json:"divergences,omitempty"`
	Robustness           *Robustness `json:"robustness,omitempty"`
}

// Actor peer/arbiter.
type Actor struct {
	Role            string         `json:"role"`
	Provider        string         `json:"provider"`
	ModelID         string         `json:"model_id"`
	RequestedModel  string         `json:"requested_model,omitempty"`
	ExecutedModel   string         `json:"executed_model,omitempty"`
	FallbackUsed    bool           `json:"fallback_used,omitempty"`
	FallbackCount   int            `json:"fallback_count,omitempty"`
	PromptVersion   string         `json:"prompt_version"`
	EvidenceHash    string         `json:"evidence_hash"`
	Status          string         `json:"status"`
	LatencyMS       *int           `json:"latency_ms,omitempty"`
	Usage           map[string]any `json:"usage,omitempty"`
	SelectionReason *string        `json:"selection_reason,omitempty"`
}

// Robustness aggregate.
type Robustness struct {
	Status            string         `json:"status"`
	QualityScoreDelta float64        `json:"quality_score_delta"`
	GateStable        bool           `json:"gate_stable"`
	Details           map[string]any `json:"details,omitempty"`
}

// ScoreScope — escopo do score (o que foi avaliado).
type ScoreScope string

// ScoreScopeDelivery — score cobre o delivery (branch/PR) inteiro,
// não só o diff isolado.
const ScoreScopeDelivery ScoreScope = "delivery"

// ScoresBlock agregado.
type ScoresBlock struct {
	Quality         *float64 `json:"quality"`
	Grade           string   `json:"grade"`
	Confidence      float64  `json:"confidence"`
	ConfidenceLevel string   `json:"confidence_level"`
	// SAI-116: status do score. `available` = score real; `unavailable` =
	// schema falhou (quality=null, grade="?"). Torna explícito quando o
	// número não veio de uma avaliação real.
	ScoreStatus string     `json:"score_status"`
	Scope       ScoreScope `json:"scope,omitempty"`
	Pillars     []Pillar   `json:"pillars"`
}

// Pillar info.
type Pillar struct {
	ID         string   `json:"id"`
	Weight     float64  `json:"weight"`
	Applicable bool     `json:"applicable"`
	Scored     bool     `json:"scored"`
	Score      *float64 `json:"score,omitempty"`
}

// RiskBlock agregado.
type RiskBlock struct {
	Level   string       `json:"level"`
	Factors []RiskFactor `json:"factors"`
}

// RiskFactor individual.
type RiskFactor struct {
	ID           string   `json:"id"`
	Severity     string   `json:"severity"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// QualityGate aggregate.
type QualityGate struct {
	Status string     `json:"status"`
	Rules  []GateRule `json:"rules"`
}

// IndependenceGate (SAI-117) auditável separado de quality_gate.
type IndependenceGate struct {
	Status         string   `json:"status"`
	Reason         string   `json:"reason"`
	DistinctModels int      `json:"distinct_models"`
	PeerACalled    bool     `json:"peer_a_called"`
	PeerBCalled    bool     `json:"peer_b_called"`
	ArbiterCalled  bool     `json:"arbiter_called"`
	Required       []string `json:"required,omitempty"`
}

// FinalGate agrega os 2 subgates (SAI-117).
type FinalGate struct {
	Status string     `json:"status"`
	Reason string     `json:"reason"`
	Rules  []GateRule `json:"rules"`
}

// GateRule individual.
type GateRule struct {
	ID           string   `json:"id"`
	Status       string   `json:"status"`
	Message      string   `json:"message"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// Recommendation individual.
type Recommendation struct {
	Priority     string   `json:"priority"`
	Title        string   `json:"title"`
	Reason       string   `json:"reason"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// Artifact info.
type Artifact struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int    `json:"size_bytes"`
}

// BuilderInput entrada crua.
type BuilderInput struct {
	RunID            string
	Profile          string
	SolidifyVersion  string
	StartedAt        time.Time
	FinishedAt       time.Time
	ConfigHash       string
	EvidenceHash     string
	Git              GitInfo
	Components       []Component
	ReleaseNotes     ReleaseNotes
	Migrations       []Migration
	EnvChanges       []EnvChange
	Analyzers        []Analyzer
	SOLID            SOLIDBlock
	AIReview         AIReview
	Scores           ScoresBlock
	Risk             RiskBlock
	QualityGate      QualityGate
	IndependenceGate *IndependenceGate // SAI-117
	FinalGate        *FinalGate        // SAI-117
	Recommendations  []Recommendation
	Limitations      []string
	Artifacts        []Artifact
}

// Builder monta report.
type Builder struct {
	in BuilderInput
}

// NewBuilder cria builder.
func NewBuilder(in BuilderInput) *Builder {
	return &Builder{in: in}
}

// Build monta Report canônico.
func (b *Builder) Build() (*Report, error) {
	if b.in.RunID == "" {
		return nil, errors.New("report: RunID vazio")
	}
	if b.in.Profile == "" {
		return nil, errors.New("report: Profile vazio")
	}
	r := &Report{
		SchemaVersion: SchemaVersion,
		Run: RunInfo{
			ID:              b.in.RunID,
			Profile:         b.in.Profile,
			SolidifyVersion: b.in.SolidifyVersion,
			StartedAt:       b.in.StartedAt.UTC().Format(time.RFC3339),
			FinishedAt:      b.in.FinishedAt.UTC().Format(time.RFC3339),
			ConfigHash:      b.in.ConfigHash,
			EvidenceHash:    b.in.EvidenceHash,
		},
		Git:              b.in.Git,
		Components:       b.in.Components,
		ReleaseNotes:     b.in.ReleaseNotes,
		Migrations:       b.in.Migrations,
		EnvChanges:       b.in.EnvChanges,
		Analyzers:        b.in.Analyzers,
		SOLID:            b.in.SOLID,
		AIReview:         b.in.AIReview,
		Scores:           b.in.Scores,
		Risk:             b.in.Risk,
		QualityGate:      b.in.QualityGate,
		IndependenceGate: b.in.IndependenceGate,
		FinalGate:        b.in.FinalGate,
		Recommendations:  b.in.Recommendations,
		Limitations:      b.in.Limitations,
		Artifacts:        b.in.Artifacts,
	}
	if r.Components == nil {
		r.Components = []Component{}
	}
	if r.Migrations == nil {
		r.Migrations = []Migration{}
	}
	if r.EnvChanges == nil {
		r.EnvChanges = []EnvChange{}
	}
	if r.Analyzers == nil {
		r.Analyzers = []Analyzer{}
	}
	if r.Recommendations == nil {
		r.Recommendations = []Recommendation{}
	}
	if r.Limitations == nil {
		r.Limitations = []string{}
	}
	if r.Artifacts == nil {
		r.Artifacts = []Artifact{}
	}
	if r.SOLID.Principles == nil {
		r.SOLID.Principles = map[string]Principle{}
	}
	return r, nil
}

// HashContent canonical hash do report.
func (r *Report) HashContent() (string, error) {
	if r == nil {
		return "", errors.New("report: nil")
	}
	// serializa c/ chaves ordenadas p/ hash determinístico.
	canon, err := marshalCanonical(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:]), nil
}

// MarshalJSON serializa c/ schema_version garantido.
func (r *Report) MarshalJSON() ([]byte, error) {
	type alias Report
	r.SchemaVersion = SchemaVersion
	return json.Marshal((*alias)(r))
}

func marshalCanonical(v any) ([]byte, error) {
	var buf []byte
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeCanonical(buf *[]byte, v any) error {
	switch x := v.(type) {
	case map[string]any:
		*buf = append(*buf, '{')
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				*buf = append(*buf, ',')
			}
			kb, _ := json.Marshal(k)
			*buf = append(*buf, kb...)
			*buf = append(*buf, ':')
			if err := writeCanonical(buf, x[k]); err != nil {
				return err
			}
		}
		*buf = append(*buf, '}')
	case []any:
		*buf = append(*buf, '[')
		for i, item := range x {
			if i > 0 {
				*buf = append(*buf, ',')
			}
			if err := writeCanonical(buf, item); err != nil {
				return err
			}
		}
		*buf = append(*buf, ']')
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		*buf = append(*buf, b...)
	}
	return nil
}
