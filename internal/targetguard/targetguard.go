// Target guard (SAI-104).
//
// Impede ZAP active / k6 destructive de rodarem contra
// hosts fora da allowlist. Previne scan acidental de
// produção.
package targetguard

import (
	"errors"
	"strings"
)

// ErrNotAllowed host não está na allowlist.
var ErrNotAllowed = errors.New("targetguard: host não permitido")

// ErrProduction host parece produção (heurística).
var ErrProduction = errors.New("targetguard: host parece produção")

// ProductionSuffixes sufixos que sugerem produção.
var ProductionSuffixes = []string{
	".amazonaws.com",
	".azure.com",
	".googleapis.com",
	".cloudfront.net",
	".herokuapp.com",
	".prod.",
	".production.",
}

// Mode tipo de scan.
type Mode string

const (
	ModeZAPActive Mode = "zap_active"
	ModeK6        Mode = "k6"
	ModeDestruct  Mode = "destructive"
)

// DangerousModes que exigem allowlist estrita.
var DangerousModes = []Mode{ModeZAPActive, ModeK6, ModeDestruct}

// Guard validador.
type Guard struct {
	AllowHosts []string // ex: ["localhost", "127.0.0.1", "staging.internal"]
}

// NewGuard cria guard.
func NewGuard(allow []string) *Guard {
	return &Guard{AllowHosts: allow}
}

// Validate checa se target+mode é permitido.
func (g *Guard) Validate(host string, mode Mode) error {
	if !IsDangerous(mode) {
		return nil // modos não-perigosos passam
	}
	if host == "" {
		return ErrNotAllowed
	}
	// Rejeita heurística de produção.
	lower := strings.ToLower(host)
	for _, s := range ProductionSuffixes {
		if strings.Contains(lower, s) {
			return ErrProduction
		}
	}
	// Verifica allowlist.
	for _, allowed := range g.AllowHosts {
		if strings.EqualFold(allowed, host) {
			return nil
		}
	}
	return ErrNotAllowed
}

// IsDangerous retorna true se mode é destrutivo.
func IsDangerous(m Mode) bool {
	for _, d := range DangerousModes {
		if m == d {
			return true
		}
	}
	return false
}

// CheckProductionOnly checa só heurística produção.
func CheckProductionOnly(host string) bool {
	lower := strings.ToLower(host)
	for _, s := range ProductionSuffixes {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
