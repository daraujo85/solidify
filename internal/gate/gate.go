// Quality Gate (SAI-075).
//
// Perfis quick/release/contractual; status PASS/WARN/FAIL/
// BLOCKED/INCOMPLETE.
package gate

import (
	"errors"
	"strings"
)

// Profile enum.
const (
	ProfileQuick       = "quick"
	ProfileRelease     = "release"
	ProfileContractual = "contractual"
)

// Status enum.
const (
	StatusPass       = "PASS"
	StatusWarn       = "WARN"
	StatusFail       = "FAIL"
	StatusBlocked    = "BLOCKED"
	StatusIncomplete = "INCOMPLETE"
)

// Threshold perfil → min score global.
var ProfileThresholds = map[string]float64{
	ProfileQuick:       60,
	ProfileRelease:     75,
	ProfileContractual: 85,
}

// GateInput input.
type GateInput struct {
	RunID             string  `json:"run_id"`
	Profile           string  `json:"profile"`
	GlobalScore       float64 `json:"global_score"`
	ConfidenceScore   float64 `json:"confidence_score"`
	ImmutableCritical int     `json:"immutable_critical"` // count
	IndependenceOK    bool    `json:"independence_ok"`    // legacy — preferir IndependenceInput
	ArbiterResolved   bool    `json:"arbiter_resolved"`
	EvidenceComplete  bool    `json:"evidence_complete"`
	// SAI-116: status do score. "available" → score real; "unavailable" →
	// schema falhou (gate deve ser INCOMPLETE); "error" → provider falhou.
	ScoreStatus string `json:"score_status"`
	// SAI-117: controle fino de independência derivado dos atores
	// efetivamente executados. Se zero-value, fallback para IndependenceOK.
	Independence IndependenceInput `json:"independence"`
}

// IndependenceInput descreve quem rodou e o que o perfil exige (SAI-117).
type IndependenceInput struct {
	PeerACalled     bool   // peer_a executou (não skipped)
	PeerBCalled     bool
	ArbiterCalled   bool
	PeerAModel      string // "" se skipped
	PeerBModel      string
	RequirePeerA          bool // profile config
	RequirePeerB          bool
	RequireArbiter        bool
	RequireDistinctModels bool // contractual
}

// Evaluate computa distinct + checa requisitos. Retorna OK + reason
// + distinct model count (auditável no report).
func (i IndependenceInput) Evaluate() (ok bool, distinct int, reason string) {
	models := []string{}
	if i.PeerAModel != "" {
		models = append(models, i.PeerAModel)
	}
	if i.PeerBModel != "" && i.PeerBModel != i.PeerAModel {
		models = append(models, i.PeerBModel)
	}
	distinct = len(models)
	if i.RequirePeerA && !i.PeerACalled {
		return false, distinct, "peer_a required mas não executou"
	}
	if i.RequirePeerB && !i.PeerBCalled {
		return false, distinct, "peer_b required mas não executou"
	}
	if i.RequireArbiter && !i.ArbiterCalled {
		return false, distinct, "arbiter required mas não executou"
	}
	if i.RequireDistinctModels && distinct < 2 {
		return false, distinct, "distinct models < 2"
	}
	return true, distinct, ""
}

// SubgateResult status individual por subgate.
type SubgateResult struct {
	Status  string `json:"status"`  // PASS|WARN|FAIL|INCOMPLETE|BLOCKED
	Reason  string `json:"reason"`
	Blocking bool  `json:"-"`       // se true, falha do subgate bloqueia final
}

// Subgates divide a decisão em eixos auditáveis (SAI-117).
type Subgates struct {
	Quality      SubgateResult `json:"quality"`
	Independence SubgateResult `json:"independence"`
}

// GateResult saída.
type GateResult struct {
	RunID     string   `json:"run_id"`
	Profile   string   `json:"profile"`
	Status    string   `json:"status"`
	Reason    string   `json:"reason"`
	Score     float64  `json:"score"`
	Threshold float64  `json:"threshold"`
	Subgates  Subgates `json:"subgates"`
}

// Evaluate gate.
func Evaluate(in GateInput) (*GateResult, error) {
	if in.RunID == "" {
		return nil, errors.New("gate: RunID vazio")
	}
	if in.Profile == "" {
		return nil, errors.New("gate: Profile vazio")
	}
	threshold, ok := ProfileThresholds[in.Profile]
	if !ok {
		return nil, errors.New("gate: Profile inválido: " + in.Profile)
	}
	res := &GateResult{
		RunID:     in.RunID,
		Profile:   in.Profile,
		Score:     in.GlobalScore,
		Threshold: threshold,
	}
	// SAI-116: ScoreStatus=unavailable tem precedência sobre tudo. Schema
	// falhou → não sabemos a qualidade real. INCOMPLETE bloqueia mas não
	// é FAIL (score não é ruim, é desconhecido).
	if in.ScoreStatus == "unavailable" {
		res.Status = StatusIncomplete
		res.Reason = "score schema falhou; re-run necessário"
		res.Subgates.Quality = SubgateResult{Status: StatusIncomplete, Reason: res.Reason, Blocking: true}
		// SAI-117: ainda computa independence (auditável mesmo quando quality bloqueia)
		res.Subgates.Independence = computeIndependenceSubgate(in)
		return res, nil
	}
	if in.ScoreStatus == "error" {
		res.Status = StatusIncomplete
		res.Reason = "score provider falhou"
		res.Subgates.Quality = SubgateResult{Status: StatusIncomplete, Reason: res.Reason, Blocking: true}
		res.Subgates.Independence = computeIndependenceSubgate(in)
		return res, nil
	}
	// SAI-129B: ScoreStatus=NOT_APPLICABLE (heurística determinística
	// classificou os 5 princípios SOLID como CLEARLY_NOT_APPLICABLE —
	// LLM nem foi chamado) não é score ruim, é "nada a avaliar". PASS,
	// não-blocking — nunca FAIL por comparar GlobalScore=0 contra
	// threshold. Independence ainda computado (auditável).
	if in.ScoreStatus == "NOT_APPLICABLE" {
		res.Status = StatusPass
		res.Reason = "nenhum princípio SOLID aplicável neste diff — nada a avaliar"
		res.Subgates.Quality = SubgateResult{Status: StatusPass, Reason: res.Reason}
		res.Subgates.Independence = computeIndependenceSubgate(in)
		res.Status, res.Reason = combineSubgates(res.Subgates)
		return res, nil
	}
	if !in.EvidenceComplete {
		res.Status = StatusIncomplete
		res.Reason = "evidence incompleta"
		res.Subgates.Quality = SubgateResult{Status: StatusIncomplete, Reason: res.Reason, Blocking: true}
		res.Subgates.Independence = computeIndependenceSubgate(in)
		return res, nil
	}
	if !in.ArbiterResolved {
		res.Status = StatusIncomplete
		res.Reason = "arbiter não resolveu divergência"
		res.Subgates.Quality = SubgateResult{Status: StatusIncomplete, Reason: res.Reason, Blocking: true}
		res.Subgates.Independence = computeIndependenceSubgate(in)
		return res, nil
	}
	// Quality subgate (SAI-117).
	qStatus, qReason := qualitySubgate(in, threshold)
	res.Subgates.Quality = SubgateResult{
		Status:   qStatus,
		Reason:   qReason,
		Blocking: qStatus == StatusFail || qStatus == StatusBlocked,
	}
	// Independence subgate (SAI-117).
	res.Subgates.Independence = computeIndependenceSubgate(in)
	res.Status, res.Reason = combineSubgates(res.Subgates)
	return res, nil
}

// computeIndependenceSubgate retorna o subgate de independência.
// Lógica: se IndependenceInput não foi preenchida (zero-value RequirePeer*),
// usa o legacy IndependenceOK. Senão, avalia atores + distinct.
func computeIndependenceSubgate(in GateInput) SubgateResult {
	if !independenceInputUsed(in.Independence) {
		if !in.IndependenceOK {
			return SubgateResult{
				Status: StatusBlocked, Reason: "independência dos peers degradada",
				Blocking: true,
			}
		}
		return SubgateResult{Status: StatusPass, Reason: "independência ok (legacy)"}
	}
	ok, _, reason := in.Independence.Evaluate()
	if ok {
		return SubgateResult{Status: StatusPass, Reason: "atores distintos executados"}
	}
	// Falha de independence é BLOCKED (não FAIL) quando exige ator;
	// é FAIL quando distinct<2 (processo ruim, não ator faltando).
	st := StatusFail
	blocking := true
	if in.Independence.RequirePeerA || in.Independence.RequirePeerB || in.Independence.RequireArbiter {
		st = StatusBlocked
	}
	return SubgateResult{Status: st, Reason: reason, Blocking: blocking}
}

// qualitySubgate retorna status/razão do subgate de qualidade.
func qualitySubgate(in GateInput, threshold float64) (string, string) {
	if in.ImmutableCritical > 0 {
		return StatusBlocked, "immutable findings crítico impede release"
	}
	if in.GlobalScore < threshold {
		return StatusFail, "score abaixo do threshold"
	}
	if in.GlobalScore < threshold+5 {
		return StatusWarn, "score próximo do threshold"
	}
	return StatusPass, "score acima do threshold com margem"
}

// combineSubgates agrega Quality + Independence em status final.
// INCOMPLETE/FAIL/BLOCKED de qualquer subgate blocking → final idem.
// WARN de qualquer subgate + nada blocking → final WARN.
func combineSubgates(s Subgates) (string, string) {
	// Blocking tem precedência.
	blocking := []SubgateResult{}
	for _, sg := range []SubgateResult{s.Quality, s.Independence} {
		if sg.Blocking {
			blocking = append(blocking, sg)
		}
	}
	if len(blocking) > 0 {
		// Ordem de severidade: BLOCKED > INCOMPLETE > FAIL.
		for _, sg := range blocking {
			if sg.Status == StatusBlocked {
				return StatusBlocked, "independência bloqueada: " + sg.Reason
			}
		}
		for _, sg := range blocking {
			if sg.Status == StatusIncomplete {
				return StatusIncomplete, sg.Reason
			}
		}
		for _, sg := range blocking {
			if sg.Status == StatusFail {
				return StatusFail, sg.Reason
			}
		}
	}
	// Sem blocking → check WARN.
	for _, sg := range []SubgateResult{s.Quality, s.Independence} {
		if sg.Status == StatusWarn {
			return StatusWarn, sg.Reason
		}
	}
	return StatusPass, "todos subgates passam"
}

// independenceInputUsed detecta se o caller passou IndependenceInput
// (não zero-value). Se sim, usa. Senão, cai no legacy.
func independenceInputUsed(i IndependenceInput) bool {
	return i.RequirePeerA || i.RequirePeerB || i.RequireArbiter || i.RequireDistinctModels
}

// IsBlocking helper: FAIL/BLOCKED/INCOMPLETE bloqueiam.
func (r *GateResult) IsBlocking() bool {
	if r == nil {
		return true
	}
	return r.Status == StatusFail || r.Status == StatusBlocked || r.Status == StatusIncomplete
}

// IsPassing helper.
func (r *GateResult) IsPassing() bool {
	if r == nil {
		return false
	}
	return r.Status == StatusPass || r.Status == StatusWarn
}

// RenderGate textual.
func RenderGate(r *GateResult) string {
	if r == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("Gate[")
	sb.WriteString(r.RunID)
	sb.WriteString(" ")
	sb.WriteString(r.Profile)
	sb.WriteString("] ")
	sb.WriteString(r.Status)
	sb.WriteString(" (")
	sb.WriteString(r.Reason)
	sb.WriteString(")\n  score=")
	sb.WriteString(ftoa5(r.Score))
	sb.WriteString(" threshold=")
	sb.WriteString(ftoa5(r.Threshold))
	sb.WriteString("\n")
	return sb.String()
}

// ValidProfile check.
func ValidProfile(p string) bool {
	_, ok := ProfileThresholds[p]
	return ok
}

// helpers.
func ftoa5(f float64) string {
	intPart := int(f)
	frac := int((f - float64(intPart)) * 100)
	if frac < 0 {
		frac = -frac
	}
	out := itoa5(intPart) + "." + padLeft5(itoa5(frac), 2)
	return out
}

func itoa5(n int) string {
	if n == 0 {
		return "0"
	}
	out := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	if neg {
		out = append([]byte{'-'}, out...)
	}
	return string(out)
}

func padLeft5(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}


func checkIncompleteFields(in GateInput, status string) string {
	return status
}