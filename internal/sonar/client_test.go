package sonar

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// helper: cria test server que responde JSON por path.
func sonarTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := NewClient(ClientConfig{
		HostURL: srv.URL,
		Token:   "test-token",
		Login:   "test-user",
		Timeout: 5 * time.Second,
	})
	t.Cleanup(srv.Close)
	return srv, c
}

// Aceitação: NewClient com defaults.
func TestNewClientDefaults(t *testing.T) {
	c := NewClient(ClientConfig{HostURL: "https://x", Token: "t"})
	if c.HTTP.Timeout != 30*time.Second {
		t.Errorf("default timeout = %v", c.HTTP.Timeout)
	}
	if c.HostURL != "https://x" {
		t.Errorf("trim failed")
	}
}

// Aceitação: NewClient strip trailing slash.
func TestNewClientTrimSlash(t *testing.T) {
	c := NewClient(ClientConfig{HostURL: "https://x/"})
	if c.HostURL != "https://x" {
		t.Errorf("got %q", c.HostURL)
	}
}

// Aceitação: bearer token.
func TestApplyAuthBearer(t *testing.T) {
	c := NewClient(ClientConfig{HostURL: "x", Token: "abc"})
	req, _ := http.NewRequest("GET", "x", nil)
	c.applyAuth(req)
	if got := req.Header.Get("Authorization"); got != "Bearer abc" {
		t.Errorf("got %q", got)
	}
}

// Aceitação: basic auth (SonarQube ≤9.x).
func TestApplyAuthBasic(t *testing.T) {
	c := NewClient(ClientConfig{HostURL: "x", Token: "tok", Login: "admin"})
	req, _ := http.NewRequest("GET", "x", nil)
	c.applyAuth(req)
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:tok"))
	if got := req.Header.Get("Authorization"); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// Aceitação: applyAuth sem token.
func TestApplyAuthEmpty(t *testing.T) {
	c := &Client{}
	req, _ := http.NewRequest("GET", "x", nil)
	c.applyAuth(req)
	if req.Header.Get("Authorization") != "" {
		t.Errorf("devia estar vazio")
	}
}

// Aceitação: Ping OK.
func TestPingOK(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "system/status") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"status":"UP"}`))
	})
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("err: %v", err)
	}
}

// Aceitação: GetProject.
func TestGetProject(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project") != "myproj" {
			t.Errorf("query = %v", r.URL.Query())
		}
		w.Write([]byte(`{"components":[{"key":"myproj","name":"My Proj","qualifier":"TRK"}]}`))
	})
	p, err := c.GetProject(context.Background(), "myproj")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Key != "myproj" || p.Name != "My Proj" {
		t.Errorf("got %+v", p)
	}
}

// Aceitação: GetProject not found.
func TestGetProjectEmpty(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"components":[]}`))
	})
	if _, err := c.GetProject(context.Background(), "missing"); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: GetMeasures.
func TestGetMeasures(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		mk := r.URL.Query().Get("metricKeys")
		if !strings.Contains(mk, "coverage") {
			t.Errorf("metrics = %s", mk)
		}
		w.Write([]byte(`{
			"component": {
				"key":"myproj",
				"name":"My",
				"measures":[{"metric":"coverage","value":"85.5"}]
			}
		}`))
	})
	m, err := c.GetMeasures(context.Background(), "myproj", []string{"coverage", "bugs"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(m.Component.Measures) != 1 {
		t.Errorf("measures = %d", len(m.Component.Measures))
	}
	if m.Component.Measures[0].Value != "85.5" {
		t.Errorf("value = %s", m.Component.Measures[0].Value)
	}
}

// Aceitação: GetIssues.
func TestGetIssues(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"total":1,
			"p":1,
			"ps":100,
			"issues":[{"key":"i1","severity":"CRITICAL","rule":"java:S1234","line":42,"message":"bug","component":"myproj:src/Foo.java"}]
		}`))
	})
	iss, err := c.GetIssues(context.Background(), "myproj", []string{"CRITICAL"}, 100)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if iss.Total != 1 || len(iss.Issues) != 1 {
		t.Errorf("err: %+v", iss)
	}
	if iss.Issues[0].Severity != "CRITICAL" {
		t.Errorf("sev = %s", iss.Issues[0].Severity)
	}
}

// Aceitação: GetQualityGate.
func TestGetQualityGate(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"projectStatus":{
				"status":"ERROR",
				"conditions":[
					{"metric":"coverage","comparator":"LT","errorThreshold":"80","actualValue":"50","status":"ERROR"}
				]
			}
		}`))
	})
	qg, err := c.GetQualityGate(context.Background(), "myproj")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if qg.ProjectStatus.Status != "ERROR" {
		t.Errorf("status = %s", qg.ProjectStatus.Status)
	}
}

// Aceitação: GetServerVersion.
func TestGetServerVersion(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"10.4.0"}`))
	})
	v, err := c.GetServerVersion(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if v != "10.4.0" {
		t.Errorf("version = %s", v)
	}
	if c.Version != "10.4.0" {
		t.Errorf("client.Version = %s", c.Version)
	}
}

// Aceitação: 401 unauthorized.
func TestUnauthorized401(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"err":"unauthorized"}`))
	})
	if err := c.Ping(context.Background()); err == nil {
		t.Errorf("devia falhar")
	} else if !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %v", err)
	}
}

// Aceitação: 404 not found.
func TestNotFound404(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"err":"missing"}`))
	})
	if err := c.Ping(context.Background()); err == nil {
		t.Errorf("devia falhar")
	} else if !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v", err)
	}
}

// Aceitação: 500 server error.
func TestServerError500(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`oops`))
	})
	if err := c.Ping(context.Background()); err == nil {
		t.Errorf("devia falhar")
	} else if !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v", err)
	}
}

// Aceitação: 429 rate limit.
func TestRateLimit429(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"err":"too many requests"}`))
	})
	if err := c.Ping(context.Background()); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: 403 forbidden.
func TestForbidden403(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"err":"nope"}`))
	})
	if err := c.Ping(context.Background()); err == nil {
		t.Errorf("devia falhar")
	} else if !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v", err)
	}
}

// Aceitação: context cancel.
func TestContextCancel(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte(`{}`))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := c.Ping(ctx); err == nil {
		t.Errorf("devia falhar por timeout")
	}
}

// Aceitação: response com campos extras (API version tolerance).
func TestExtraFieldsIgnored(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"component":{
				"key":"p",
				"name":"P",
				"qualifier":"TRK",
				"newField":"future",
				"measures":[{"metric":"m","value":"1","future":"x"}]
			}
		}`))
	})
	m, err := c.GetMeasures(context.Background(), "p", []string{"m"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m.Component.Key != "p" {
		t.Errorf("err")
	}
}

// Aceitação: response JSON inválido.
func TestInvalidJSON(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{invalid`))
	})
	if _, err := c.GetServerVersion(context.Background()); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: User-Agent header.
func TestUserAgentHeader(t *testing.T) {
	var ua string
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Write([]byte(`{}`))
	})
	c.Ping(context.Background())
	if !strings.Contains(ua, "solidify") {
		t.Errorf("UA = %q", ua)
	}
}

// Aceitação: Accept JSON.
func TestAcceptHeader(t *testing.T) {
	var accept string
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		w.Write([]byte(`{}`))
	})
	c.Ping(context.Background())
	if !strings.Contains(accept, "json") {
		t.Errorf("accept = %q", accept)
	}
}

// Aceitação: get com query params.
func TestQueryParams(t *testing.T) {
	var gotQs string
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotQs = r.URL.RawQuery
		w.Write([]byte(`{"components":[{"key":"k"}]}`))
	})
	c.GetProject(context.Background(), "k")
	if !strings.Contains(gotQs, "project=k") {
		t.Errorf("qs = %s", gotQs)
	}
}

// Aceitação: body lido em stream.
func TestBodyRead(t *testing.T) {
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, strings.NewReader(`{"status":"UP"}`))
	})
	c.Ping(context.Background())
}

// Aceitação: APIVersionsSupported.
func TestAPIVersions(t *testing.T) {
	vs := APIVersionsSupported()
	if len(vs) < 2 {
		t.Errorf("vazio")
	}
	// retornar slice não deve mutar original.
	vs[0] = "hacked"
	if APIVersionsSupported()[0] == "hacked" {
		t.Errorf("vaza referência interna")
	}
}

// Aceitação: truncateMsg.
func TestTruncateMsg(t *testing.T) {
	short := "x"
	if truncateMsg(short) != "x" {
		t.Errorf("err")
	}
	long := strings.Repeat("a", 300)
	got := truncateMsg(long)
	if len(got) >= len(long) {
		t.Errorf("devia truncar (len=%d original=%d)", len(got), len(long))
	}
}

// Aceitação: GetIssues com ps default.
func TestGetIssuesDefaultPS(t *testing.T) {
	var gotPS string
	_, c := sonarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPS = r.URL.Query().Get("ps")
		w.Write([]byte(`{"total":0,"issues":[]}`))
	})
	c.GetIssues(context.Background(), "p", nil, 0)
	if gotPS != "100" {
		t.Errorf("ps = %s", gotPS)
	}
}

// Aceitação: QGCondition fields.
func TestQGConditionFields(t *testing.T) {
	data := `{"metric":"coverage","comparator":"LT","errorThreshold":"80","actualValue":"50","status":"ERROR"}`
	var c1 QGCondition
	if err := json.Unmarshal([]byte(data), &c1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if c1.Comparator != "LT" || c1.Status != "ERROR" {
		t.Errorf("err")
	}
}
