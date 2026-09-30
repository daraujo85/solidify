package applicability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diegoaraujo/solidify/internal/component"
	"github.com/diegoaraujo/solidify/internal/jev"
)

// buildJEVServer devolve um servidor que responde todos os 8 gates
// com o verdict dado.
func buildJEVServer(verdict string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		answers := map[string]any{}
		for _, g := range allGates() {
			answers[string(g)] = map[string]any{"type": "choice", "choice": verdict, "confidence": 0.95}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-test","answers":` + jsonRaw(answers) + `}`))
	}))
}

func jsonRaw(v any) string {
	// helper local simples — sem import extra de encoding/json no test setup
	switch m := v.(type) {
	case map[string]any:
		s := "{"
		first := true
		for k, val := range m {
			if !first {
				s += ","
			}
			first = false
			s += `"` + k + `":` + jsonRaw(val)
		}
		return s + "}"
	case string:
		return `"` + m + `"`
	}
	return "null"
}

func profileForJEV() Profile {
	return Profile{
		Components:      []component.Component{component.BackendAPI},
		ChangedPaths:    []string{"apps/api/server.go"},
		SonarConfigured: true,
	}
}

func TestJEVDecider_NilClientFallsBackToHeuristic(t *testing.T) {
	d := NewJEVDecider(nil)
	got := d.DecideWithContext(context.Background(), profileForJEV())
	if len(got) != 8 {
		t.Fatalf("decisões = %d, quero 8", len(got))
	}
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Source != SourceHeuristic {
		t.Errorf("Source = %s, quero heuristic", sonar.Source)
	}
	if sonar.JEVAssessed {
		t.Errorf("JEVAssessed = true, quero false (sem client)")
	}
}

func TestJEVDecider_ClientWithoutKeyFallsBack(t *testing.T) {
	t.Setenv("JEV_API_KEY", "")
	t.Setenv("TYPESAFE_API_KEY", "")
	c := jev.NewClient("http://unused", "")
	d := NewJEVDecider(c)
	got := d.DecideWithContext(context.Background(), profileForJEV())
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Source != SourceHeuristic {
		t.Errorf("Source = %s, quero heuristic (sem chave)", sonar.Source)
	}
}

func TestJEVDecider_ServerErrorFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := jev.NewClient(srv.URL, "k")
	d := NewJEVDecider(c)
	got := d.DecideWithContext(context.Background(), profileForJEV())
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Source != SourceJEVFallback {
		t.Errorf("Source = %s, quero jev-fallback", sonar.Source)
	}
	if sonar.JEVAssessed {
		t.Errorf("JEVAssessed = true, quero false (fallback)")
	}
	if sonar.Verdict != Applicable {
		t.Errorf("Verdict = %s, quero Applicable (heurística)", sonar.Verdict)
	}
}

func TestJEVDecider_AdvisoryPreservesHeuristicVerdict(t *testing.T) {
	srv := buildJEVServer(jevVerdictApplicable)
	defer srv.Close()

	c := jev.NewClient(srv.URL, "k")
	d := NewJEVDecider(c)
	d.Mode = ModeAdvisory

	// Backend-only sem Sonar config: heurística = Conditional.
	profile := Profile{
		Components:   []component.Component{component.BackendAPI},
		ChangedPaths: []string{"apps/api/server.go"},
	}
	got := d.DecideWithContext(context.Background(), profile)
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Verdict != Conditional {
		t.Errorf("advisory: Verdict = %s, quero Conditional (heurística)", sonar.Verdict)
	}
	if sonar.JEVAssessed != true {
		t.Errorf("advisory: JEVAssessed = %v, quero true", sonar.JEVAssessed)
	}
	if !contains(sonar.Reason, "JEV:") {
		t.Errorf("advisory: Reason não-enriquecido: %q", sonar.Reason)
	}
}

func TestJEVDecider_EnforceAllowsOverride(t *testing.T) {
	srv := buildJEVServer(jevVerdictApplicable)
	defer srv.Close()

	c := jev.NewClient(srv.URL, "k")
	d := NewJEVDecider(c)
	d.Mode = ModeEnforce
	profile := profileForJEV()
	got := d.DecideWithContext(context.Background(), profile)
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Verdict != Applicable {
		t.Errorf("enforce: Verdict = %s, quero Applicable (JEV override)", sonar.Verdict)
	}
	if sonar.Source != SourceJEV {
		t.Errorf("enforce: Source = %s, quero jev", sonar.Source)
	}
}

func TestJEVDecider_ServerPartialAnswerFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// só 1 gate respondido — resposta inválida ⇒ fallback
		_, _ = w.Write([]byte(`{"answers":{"sonar":{"type":"choice","choice":"APPLICABLE"}}}`))
	}))
	defer srv.Close()

	c := jev.NewClient(srv.URL, "k")
	d := NewJEVDecider(c)
	got := d.DecideWithContext(context.Background(), profileForJEV())
	sonar := decisionByGate(t, got, GateSonar)
	if sonar.Source != SourceJEVFallback {
		t.Errorf("Source = %s, quero jev-fallback (parcial)", sonar.Source)
	}
}
