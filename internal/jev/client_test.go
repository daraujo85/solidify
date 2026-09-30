package jev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_DefaultsFromEnv(t *testing.T) {
	t.Setenv(EnvKeyAPIKey, "env-key")
	c := NewClient("", "")
	if c.Endpoint != DefaultEndpoint {
		t.Errorf("endpoint = %s, quero default", c.Endpoint)
	}
	if c.APIKey != "env-key" {
		t.Errorf("apiKey = %q, quero env-key", c.APIKey)
	}
	if !c.Enabled() {
		t.Error("Enabled = false com chave presente")
	}
}

func TestNewClient_EmptyKeyDisabled(t *testing.T) {
	t.Setenv(EnvKeyAPIKey, "")
	t.Setenv("TYPESAFE_API_KEY", "")
	c := NewClient("", "")
	if c.Enabled() {
		t.Error("Enabled = true sem chave")
	}
}

func TestEvaluate_ChoiceNoulScore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"jev-test","latency_ms":42,
			"answers":{
				"k1":{"type":"choice","choice":"b","confidence":0.9,"probabilities":{"a":0.1,"b":0.9}},
				"k2":{"type":"noul","noul":true,"probability":0.95},
				"k3":{"type":"score","score":8.5,"confidence":0.8}
			}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	res, err := c.Evaluate(context.Background(), "state", map[string]Question{
		"k1": {Type: TypeChoice, Instructions: "pick"},
		"k2": {Type: TypeNoul, Instructions: "bool"},
		"k3": {Type: TypeScore, Instructions: "score", Min: fptr(1), Max: fptr(10)},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Model != "jev-test" || res.LatencyMS != 42 {
		t.Errorf("model/latency = %s/%d", res.Model, res.LatencyMS)
	}
	if res.Answers["k1"].Choice == nil || res.Answers["k1"].Choice.Choice != "b" {
		t.Errorf("k1 = %+v", res.Answers["k1"])
	}
	if res.Answers["k2"].Noul == nil || !res.Answers["k2"].Noul.Noul {
		t.Errorf("k2 = %+v", res.Answers["k2"])
	}
	if res.Answers["k3"].Score == nil || res.Answers["k3"].Score.Score != 8.5 {
		t.Errorf("k3 = %+v", res.Answers["k3"])
	}
}

func TestEvaluate_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	_, err := c.Evaluate(context.Background(), "s", map[string]Question{"k": {Type: TypeNoul, Instructions: "x"}})
	if err == nil {
		t.Fatal("esperava erro de status")
	}
}

func TestEvaluate_NoKey(t *testing.T) {
	c := NewClient("http://x", "")
	if _, err := c.Evaluate(context.Background(), "s", map[string]Question{"k": {Type: TypeNoul, Instructions: "x"}}); err == nil {
		t.Fatal("esperava erro sem chave")
	}
}

func TestEvaluate_UnknownTypeInferred(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"answers":{"k":{"noul":true,"probability":0.9}}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	res, err := c.Evaluate(context.Background(), "s", map[string]Question{"k": {Type: TypeNoul, Instructions: "x"}})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Answers["k"].Noul == nil || !res.Answers["k"].Noul.Noul {
		t.Errorf("k = %+v (inferência falhou)", res.Answers["k"])
	}
}

func TestRequestPayload_Marshals(t *testing.T) {
	p := requestPayload{Model: "jev-latest", State: "s", Questions: map[string]Question{
		"k": {Type: TypeChoice, Instructions: "i", Criteria: map[string]string{"a": "A"}},
	}}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("payload vazio")
	}
}

// A API JEV real devolve noul como número 0–1. flexBool deve aceitar
// boolean explícito, 1/0, e número; preserva V como probabilidade.
func TestFlexBool_Numeric(t *testing.T) {
	cases := []struct {
		raw   string
		want  bool
		wantV float64
	}{
		{`true`, true, 1},
		{`false`, false, 0},
		{`1`, true, 1},
		{`0`, false, 0},
		{`0.51`, true, 0.51},
		{`0.05`, false, 0.05},
	}
	for _, c := range cases {
		var f flexBool
		if err := f.UnmarshalJSON([]byte(c.raw)); err != nil {
			t.Fatalf("%s: %v", c.raw, err)
		}
		if f.Bool() != c.want || f.V != c.wantV {
			t.Errorf("%s: bool=%v V=%v, quero bool=%v V=%v", c.raw, f.Bool(), f.V, c.want, c.wantV)
		}
	}
}

// noul como número 0.51 ⇒ Answer true com Probability 0.51 (não 0).
func TestEvaluate_NoulNumericProbability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"answers":{"k":{"type":"noul","noul":0.51}}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	res, err := c.Evaluate(context.Background(), "s", map[string]Question{"k": {Type: TypeNoul, Instructions: "x"}})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Answers["k"].Noul == nil || !res.Answers["k"].Noul.Noul {
		t.Errorf("k = %+v, quero noul=true", res.Answers["k"])
	}
	if res.Answers["k"].Noul.Probability != 0.51 {
		t.Errorf("probability = %v, quero 0.51", res.Answers["k"].Noul.Probability)
	}
}

func fptr(f float64) *float64 { return &f }
