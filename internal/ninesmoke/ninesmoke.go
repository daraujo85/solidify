// 9Router adapter smoke (SAI-109).
//
// Verifica comportamento esperado do adapter 9Router:
// - health endpoint responde.
// - combo resolve modelos em ordem.
// - fallback funciona quando primário falha.
package ninesmoke

import "errors"

// HealthStatus saúde do gateway.
type HealthStatus struct {
	Online   bool
	Version  string
	Combos   int
	Models   int
}

// SmokeResult verificação.
type SmokeResult struct {
	HealthOK       bool
	ComboResolved  bool
	FallbackWorked bool
	Passed         bool
	Reasons        []string
}

// Gateway stub do 9Router (substituível por real).
type Gateway struct {
	BaseURL    string
	AuthToken  string
	Combos     map[string][]string // combo → ordered models
}

// NewGateway cria gateway stub.
func NewGateway(url, token string) *Gateway {
	return &Gateway{
		BaseURL:   url,
		AuthToken: token,
		Combos:    map[string][]string{},
	}
}

// Health checa saúde (stub: sempre online se URL setada).
func (g *Gateway) Health() (HealthStatus, error) {
	if g.BaseURL == "" {
		return HealthStatus{}, errors.New("ninesmoke: URL vazia")
	}
	return HealthStatus{
		Online:  true,
		Version: "1.0",
		Combos:  len(g.Combos),
		Models:  countModels(g.Combos),
	}, nil
}

// ResolveCombo retorna primeiro modelo do combo.
//
// Se primário falhar (passado via `failed`), tenta
// próximo.
func (g *Gateway) ResolveCombo(combo string, failed []string) (string, error) {
	models, ok := g.Combos[combo]
	if !ok {
		return "", errors.New("ninesmoke: combo não existe: " + combo)
	}
	for _, m := range models {
		if !contains(failed, m) {
			return m, nil
		}
	}
	return "", errors.New("ninesmoke: combo exhausted: " + combo)
}

// RegisterCombo adiciona/atualiza combo.
func (g *Gateway) RegisterCombo(name string, models []string) {
	g.Combos[name] = models
}

// RunSmoke roda bateria completa.
func (g *Gateway) RunSmoke(combo string) SmokeResult {
	r := SmokeResult{}
	if _, err := g.Health(); err != nil {
		r.Reasons = append(r.Reasons, "health: "+err.Error())
		return r
	}
	r.HealthOK = true
	if _, err := g.ResolveCombo(combo, nil); err != nil {
		r.Reasons = append(r.Reasons, "combo: "+err.Error())
		return r
	}
	r.ComboResolved = true
	// Simula falha do primário — fallback.
	primary, _ := g.ResolveCombo(combo, nil)
	fb, err := g.ResolveCombo(combo, []string{primary})
	if err != nil {
		r.Reasons = append(r.Reasons, "fallback: "+err.Error())
		return r
	}
	if fb == primary {
		r.Reasons = append(r.Reasons, "fallback igual ao primário")
		return r
	}
	r.FallbackWorked = true
	r.Passed = true
	return r
}

func countModels(combos map[string][]string) int {
	seen := map[string]bool{}
	for _, ms := range combos {
		for _, m := range ms {
			seen[m] = true
		}
	}
	return len(seen)
}

func contains(slice []string, item string) bool {
	for _, x := range slice {
		if x == item {
			return true
		}
	}
	return false
}
