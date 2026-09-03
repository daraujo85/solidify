// Package sonar — Web API adapter (client HTTP).
//
// SAI-034: client isolado com auth por env, timeout, JSON parser e
// tolerância a mudanças de versão da API SonarQube/SonarCloud.
package sonar

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client para API Sonar.
type Client struct {
	HostURL string
	Token   string
	Login   string // opcional; SonarQube ≤9.x usa basic auth
	HTTP    *http.Client
	Version string // retornado pelo server, opcional
}

// ClientConfig configura client.
type ClientConfig struct {
	HostURL  string
	Token    string
	Login    string
	Timeout  time.Duration
	Insecure bool
}

// NewClient cria client com defaults sensatos.
func NewClient(cfg ClientConfig) *Client {
	to := cfg.Timeout
	if to <= 0 {
		to = 30 * time.Second
	}
	return &Client{
		HostURL: strings.TrimRight(cfg.HostURL, "/"),
		Token:   cfg.Token,
		Login:   cfg.Login,
		HTTP: &http.Client{
			Timeout: to,
		},
	}
}

// apiVersions suportadas (informational). API atual usa paths estáveis.
var apiVersions = []string{"v9", "v10", "v2025"}

// Measure — measure retornada pelo Web API.
type Measure struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
}

// ComponentMeasures é o resultado de /measures/component.
type ComponentMeasures struct {
	Component struct {
		Key       string     `json:"key"`
		Name      string     `json:"name"`
		Qualifier string     `json:"qualifier"`
		Measures  []Measure  `json:"measures"`
		Issues    []IssueRef `json:"issues,omitempty"`
	} `json:"component"`
}

// IssueRef issue resumido (inline em measures).
type IssueRef struct {
	Key      string `json:"key"`
	Severity string `json:"severity"`
	Type     string `json:"type"`
	Rule     string `json:"rule"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Status   string `json:"status"`
	FilePath string `json:"component"`
}

// IssuesResponse — lista paginada de issues.
type IssuesResponse struct {
	Total  int        `json:"total"`
	P      int        `json:"p"`
	PSize  int        `json:"ps"`
	Issues []IssueRef `json:"issues"`
}

// QualityGateStatus — quality gate.
type QualityGateStatus struct {
	ProjectStatus struct {
		Status     string        `json:"status"` // "OK" | "WARN" | "ERROR"
		Conditions []QGCondition `json:"conditions"`
	} `json:"projectStatus"`
}

// QGCondition — condição do QG.
type QGCondition struct {
	Metric     string `json:"metric"`
	Comparator string `json:"comparator"`
	Error      string `json:"errorThreshold"`
	Actual     string `json:"actualValue"`
	Status     string `json:"status"` // "OK" | "WARN" | "ERROR"
}

// ProjectInfo — info básica do projeto.
type ProjectInfo struct {
	Key              string `json:"key"`
	Name             string `json:"name"`
	Qualifier        string `json:"qualifier"`
	Visibility       string `json:"visibility"`
	LastAnalysisDate string `json:"lastAnalysisDate"`
}

// PingResponse — health check.
type PingResponse struct {
	Status string `json:"status"`
}

// Ping testa conectividade e auth.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.get(ctx, "/api/system/status", nil)
	return err
}

// GetProject busca metadata do projeto.
func (c *Client) GetProject(ctx context.Context, projectKey string) (ProjectInfo, error) {
	v := url.Values{}
	v.Set("project", projectKey)
	body, err := c.get(ctx, "/api/projects/search", v)
	if err != nil {
		return ProjectInfo{}, err
	}
	var resp struct {
		Components []ProjectInfo `json:"components"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ProjectInfo{}, fmt.Errorf("sonar: parse project: %w", err)
	}
	if len(resp.Components) == 0 {
		return ProjectInfo{}, fmt.Errorf("sonar: project %q not found", projectKey)
	}
	return resp.Components[0], nil
}

// GetMeasures busca measures por metric keys.
func (c *Client) GetMeasures(ctx context.Context, projectKey string, metrics []string) (ComponentMeasures, error) {
	v := url.Values{}
	v.Set("component", projectKey)
	v.Set("metricKeys", strings.Join(metrics, ","))
	body, err := c.get(ctx, "/api/measures/component", v)
	if err != nil {
		return ComponentMeasures{}, err
	}
	var resp ComponentMeasures
	if err := json.Unmarshal(body, &resp); err != nil {
		return ComponentMeasures{}, fmt.Errorf("sonar: parse measures: %w", err)
	}
	return resp, nil
}

// GetIssues lista issues filtradas.
func (c *Client) GetIssues(ctx context.Context, projectKey string, severities []string, ps int) (IssuesResponse, error) {
	v := url.Values{}
	v.Set("componentKeys", projectKey)
	if len(severities) > 0 {
		v.Set("severities", strings.Join(severities, ","))
	}
	if ps <= 0 {
		ps = 100
	}
	v.Set("ps", fmt.Sprintf("%d", ps))
	v.Set("p", "1")
	body, err := c.get(ctx, "/api/issues/search", v)
	if err != nil {
		return IssuesResponse{}, err
	}
	var resp IssuesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return IssuesResponse{}, fmt.Errorf("sonar: parse issues: %w", err)
	}
	return resp, nil
}

// GetQualityGate busca status do QG.
func (c *Client) GetQualityGate(ctx context.Context, projectKey string) (QualityGateStatus, error) {
	v := url.Values{}
	v.Set("projectKey", projectKey)
	body, err := c.get(ctx, "/api/qualitygates/project_status", v)
	if err != nil {
		return QualityGateStatus{}, err
	}
	var resp QualityGateStatus
	if err := json.Unmarshal(body, &resp); err != nil {
		return QualityGateStatus{}, fmt.Errorf("sonar: parse qg: %w", err)
	}
	return resp, nil
}

// GetServerVersion tenta descobrir versão do server.
func (c *Client) GetServerVersion(ctx context.Context) (string, error) {
	body, err := c.get(ctx, "/api/server/version", nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("sonar: parse version: %w", err)
	}
	c.Version = resp.Version
	return resp.Version, nil
}

// get faz GET autenticado.
func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	u := c.HostURL + path
	if params != nil {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("sonar: build req: %w", err)
	}
	c.applyAuth(req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "solidify/0.1 (sonar)")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sonar: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("sonar: read: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, nil
	}

	return nil, c.apiError(resp.StatusCode, body)
}

// applyAuth injeta credenciais.
// SonarCloud moderno: Authorization: Bearer <token>
// SonarQube ≤9.x: Authorization: Basic base64(login:token)
func (c *Client) applyAuth(req *http.Request) {
	if c.Token == "" {
		return
	}
	if c.Login != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(c.Login + ":" + c.Token))
		req.Header.Set("Authorization", "Basic "+cred)
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
}

// apiError categoriza erro HTTP.
func (c *Client) apiError(code int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	switch code {
	case 401:
		return fmt.Errorf("sonar: 401 unauthorized (token inválido): %s", truncateMsg(msg))
	case 403:
		return fmt.Errorf("sonar: 403 forbidden (sem permissão no projeto): %s", truncateMsg(msg))
	case 404:
		return fmt.Errorf("sonar: 404 not found (projeto/recurso inexistente): %s", truncateMsg(msg))
	case 429:
		return fmt.Errorf("sonar: 429 rate limit: %s", truncateMsg(msg))
	}
	if code >= 500 {
		return fmt.Errorf("sonar: %d server error: %s", code, truncateMsg(msg))
	}
	return fmt.Errorf("sonar: http %d: %s", code, truncateMsg(msg))
}

// truncateMsg limita msg a 256 chars.
func truncateMsg(s string) string {
	const max = 256
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// APIVersionsSupported devolve versões conhecidas (informational).
func APIVersionsSupported() []string {
	out := make([]string, len(apiVersions))
	copy(out, apiVersions)
	return out
}
