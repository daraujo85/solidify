// Testes de SAI-129A: applicability obrigatório, score condicional,
// AggregateSolidScore() determinístico (nunca lê quality_score).
package peer

import "testing"

func principleNode(applicability string, score any, evidence []any) map[string]any {
	m := map[string]any{"applicability": applicability}
	if score != nil {
		m["score"] = score
	}
	if evidence != nil {
		m["evidence_refs"] = evidence
	}
	return m
}

func solidPayload(quality float64, s, o, l, i, d map[string]any) map[string]any {
	return map[string]any{
		"quality_score": quality,
		"solid":         map[string]any{"S": s, "O": o, "L": l, "I": i, "D": d},
	}
}

func ref(s string) []any { return []any{s} }

// --- validatePrincipleNode: novo modo ---

func TestValidatePrincipleNode_ApplicableRequiresScoreAndEvidence(t *testing.T) {
	cases := []struct {
		name    string
		node    map[string]any
		wantErr bool
	}{
		{"applicable com score+evidence: ok", principleNode(ApplicabilityApplicable, 78.0, ref("file.go:1")), false},
		{"applicable sem score: erro", principleNode(ApplicabilityApplicable, nil, ref("file.go:1")), true},
		{"applicable sem evidence_refs: erro", principleNode(ApplicabilityApplicable, 78.0, nil), true},
		{"applicable com evidence_refs vazio: erro", principleNode(ApplicabilityApplicable, 78.0, []any{}), true},
		{"not_applicable sem score: ok", principleNode(ApplicabilityNotApplicable, nil, nil), false},
		{"not_applicable com score preenchido: erro", principleNode(ApplicabilityNotApplicable, 50.0, nil), true},
		{"not_applicable com score null explícito: ok", principleNode(ApplicabilityNotApplicable, nil, nil), false},
		{"insufficient_evidence sem score: ok", principleNode(ApplicabilityInsufficientEvidence, nil, nil), false},
		{"insufficient_evidence com score preenchido: erro", principleNode(ApplicabilityInsufficientEvidence, 10.0, nil), true},
		{"applicability inválida: erro", map[string]any{"applicability": "MAYBE"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := validatePrincipleNode("S", c.node)
			if c.wantErr && len(errs) == 0 {
				t.Errorf("esperava erro, nenhum retornado")
			}
			if !c.wantErr && len(errs) != 0 {
				t.Errorf("esperava sem erro, got %v", errs)
			}
		})
	}
}

func TestValidatePrincipleNode_ScoreNullExplicit(t *testing.T) {
	node := map[string]any{"applicability": ApplicabilityNotApplicable, "score": nil}
	if errs := validatePrincipleNode("O", node); len(errs) != 0 {
		t.Errorf("score:null explícito com NOT_APPLICABLE deveria ser válido, got %v", errs)
	}
}

// --- validatePrincipleNode: modo legado (compat) ---

func TestValidatePrincipleNode_LegacyShapeStillAccepted(t *testing.T) {
	cases := []struct {
		name    string
		node    map[string]any
		wantErr bool
	}{
		{"legacy object value", map[string]any{"after_score": map[string]any{"value": 85.0}}, false},
		{"legacy compact number", map[string]any{"after_score": 85.0}, false},
		{"legacy sem after_score: erro", map[string]any{"applicable": true}, true},
		{"legacy after_score.value não numérico: erro", map[string]any{"after_score": map[string]any{"value": "x"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := validatePrincipleNode("D", c.node)
			if c.wantErr != (len(errs) != 0) {
				t.Errorf("wantErr=%v got errs=%v", c.wantErr, errs)
			}
		})
	}
}

// --- validateCanonical: schema completo ---

func TestValidateCanonical_NewShapeAllApplicable(t *testing.T) {
	mk := func(s float64) map[string]any { return principleNode(ApplicabilityApplicable, s, ref("f.go:1")) }
	p := solidPayload(99.0, mk(10), mk(20), mk(30), mk(40), mk(50))
	if errs := validateCanonical(p); len(errs) != 0 {
		t.Fatalf("esperava válido, got %v", errs)
	}
}

func TestValidateCanonical_MixedLegacyAndNew(t *testing.T) {
	// Registro pré-SAI-129 (nenhum applicability) continua validando.
	p := map[string]any{
		"quality_score": 80.0,
		"solid": map[string]any{
			"S": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"O": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"L": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"I": map[string]any{"after_score": map[string]any{"value": 80.0}},
			"D": map[string]any{"after_score": map[string]any{"value": 80.0}},
		},
	}
	if errs := validateCanonical(p); len(errs) != 0 {
		t.Fatalf("registro legado deveria continuar válido, got %v", errs)
	}
}

// --- AggregateSolidScore: tabela do usuário, verbatim ---

func TestAggregateSolidScore_AllApplicable(t *testing.T) {
	mk := func(s float64) map[string]any { return principleNode(ApplicabilityApplicable, s, ref("f:1")) }
	p := solidPayload(1.0, mk(80), mk(80), mk(80), mk(80), mk(80))
	score, status := AggregateSolidScore(p)
	if score == nil || *score != 80 {
		t.Fatalf("score esperado 80, got %v", score)
	}
	if status != SolidScoreAvailable {
		t.Fatalf("status esperado AVAILABLE, got %s", status)
	}
}

func TestAggregateSolidScore_3Applicable2NotApplicable(t *testing.T) {
	na := principleNode(ApplicabilityNotApplicable, nil, nil)
	mk := func(s float64) map[string]any { return principleNode(ApplicabilityApplicable, s, ref("f:1")) }
	p := solidPayload(1.0, mk(60), mk(90), mk(90), na, na)
	score, status := AggregateSolidScore(p)
	want := (60.0 + 90.0 + 90.0) / 3.0
	if score == nil || *score != want {
		t.Fatalf("score esperado %v, got %v", want, score)
	}
	if status != SolidScoreAvailable {
		t.Fatalf("status esperado AVAILABLE, got %s", status)
	}
}

func TestAggregateSolidScore_3Applicable1NotApplicable1Insufficient(t *testing.T) {
	na := principleNode(ApplicabilityNotApplicable, nil, nil)
	ie := principleNode(ApplicabilityInsufficientEvidence, nil, nil)
	mk := func(s float64) map[string]any { return principleNode(ApplicabilityApplicable, s, ref("f:1")) }
	p := solidPayload(1.0, mk(60), mk(90), mk(90), na, ie)
	score, status := AggregateSolidScore(p)
	want := (60.0 + 90.0 + 90.0) / 3.0
	if score == nil || *score != want {
		t.Fatalf("score esperado %v, got %v", want, score)
	}
	if status != SolidScorePartial {
		t.Fatalf("status esperado PARTIAL, got %s", status)
	}
}

func TestAggregateSolidScore_5NotApplicable(t *testing.T) {
	na := principleNode(ApplicabilityNotApplicable, nil, nil)
	p := solidPayload(1.0, na, na, na, na, na)
	score, status := AggregateSolidScore(p)
	if score != nil {
		t.Fatalf("score esperado nil, got %v", *score)
	}
	if status != SolidScoreNotApplicable {
		t.Fatalf("status esperado NOT_APPLICABLE, got %s", status)
	}
}

func TestAggregateSolidScore_ZeroApplicableSomeInsufficient(t *testing.T) {
	na := principleNode(ApplicabilityNotApplicable, nil, nil)
	ie := principleNode(ApplicabilityInsufficientEvidence, nil, nil)
	p := solidPayload(1.0, ie, ie, na, na, na)
	score, status := AggregateSolidScore(p)
	if score != nil {
		t.Fatalf("score esperado nil, got %v", *score)
	}
	if status != SolidScoreInsufficientEvidence {
		t.Fatalf("status esperado INSUFFICIENT_EVIDENCE, got %s", status)
	}
}

// TestAggregateSolidScore_AuthorityCase é o teste de autoridade exigido
// pelo usuário: quality_score autorreportado (99) NÃO pode influenciar
// o resultado — AggregateSolidScore() ignora esse campo por completo.
func TestAggregateSolidScore_AuthorityCase(t *testing.T) {
	p := solidPayload(99.0,
		principleNode(ApplicabilityApplicable, 10.0, ref("s.go:1")),
		principleNode(ApplicabilityApplicable, 20.0, ref("o.go:1")),
		principleNode(ApplicabilityApplicable, 30.0, ref("l.go:1")),
		principleNode(ApplicabilityApplicable, 40.0, ref("i.go:1")),
		principleNode(ApplicabilityApplicable, 50.0, ref("d.go:1")),
	)
	score, status := AggregateSolidScore(p)
	if score == nil || *score != 30 {
		t.Fatalf("AggregateSolidScore() esperado 30 (ignorando quality_score=99), got %v", score)
	}
	if status != SolidScoreAvailable {
		t.Fatalf("status esperado AVAILABLE, got %s", status)
	}
}

// --- Compat: registros legados também agregam (sem applicability) ---

func TestAggregateSolidScore_LegacyShapeTreatedAsApplicable(t *testing.T) {
	mk := func(v float64) map[string]any { return map[string]any{"after_score": map[string]any{"value": v}} }
	p := solidPayload(999.0, mk(10), mk(20), mk(30), mk(40), mk(50))
	score, status := AggregateSolidScore(p)
	if score == nil || *score != 30 {
		t.Fatalf("legado: esperado 30 (média), got %v", score)
	}
	if status != SolidScoreAvailable {
		t.Fatalf("status esperado AVAILABLE, got %s", status)
	}
}

func TestAggregateSolidScore_MalformedNodeCountsAsInsufficient(t *testing.T) {
	// Nó ausente não contamina a média (só S entra), mas também não é
	// tratado como NOT_APPLICABLE silencioso — vira INSUFFICIENT_EVIDENCE,
	// que degrada o status pra PARTIAL. validateCanonical já rejeitaria
	// esse payload antes de chegar aqui em produção (chave ausente); este
	// teste cobre a função isoladamente contra input malformado.
	p := map[string]any{
		"quality_score": 1.0,
		"solid": map[string]any{
			"S": principleNode(ApplicabilityApplicable, 100.0, ref("f:1")),
			// O, L, I, D ausentes
		},
	}
	score, status := AggregateSolidScore(p)
	if score == nil || *score != 100 {
		t.Fatalf("esperado média só do S (100), got %v", score)
	}
	if status != SolidScorePartial {
		t.Fatalf("status esperado PARTIAL (nós ausentes viram INSUFFICIENT_EVIDENCE), got %s", status)
	}
}
