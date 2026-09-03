// Package changes — endpoint change detector.
//
// SAI-046: detecta endpoints novos/removidos/alterados para
// priorizar load test (k6). Fontes: OpenAPI/Swagger diff, route/
// controller scan em código, lista manual em config.
package changes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Endpoint representa um endpoint detectado.
type Endpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Source string `json:"source,omitempty"` // openapi|controller|manual
}

// Fingerprint hash estável do endpoint (method+path normalizado).
func (e Endpoint) Fingerprint() string {
	return normalizeMethod(e.Method) + " " + normalizePath(e.Path)
}

// normalizeMethod uppercase.
func normalizeMethod(m string) string {
	return strings.ToUpper(strings.TrimSpace(m))
}

// normalizePath colapsa múltiplos slashes, lowercase.
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// colapsa múltiplos slashes.
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

// DiffSet representa diff entre dois snapshots.
type DiffSet struct {
	Added     []Endpoint `json:"added,omitempty"`
	Removed   []Endpoint `json:"removed,omitempty"`
	Changed   []Endpoint `json:"changed,omitempty"` // path mudou mas method+base é "equivalente"
	Unchanged int        `json:"unchanged"`
}

// HasChanges devolve true se tem add/remove/change.
func (d *DiffSet) HasChanges() bool {
	return len(d.Added) > 0 || len(d.Removed) > 0 || len(d.Changed) > 0
}

// IsEmpty devolve true se diff vazio.
func (d *DiffSet) IsEmpty() bool {
	return !d.HasChanges() && d.Unchanged == 0
}

// Diff compara old vs new; retorna DiffSet.
func Diff(old, new []Endpoint) DiffSet {
	oldMap := toMap(old)
	newMap := toMap(new)
	out := DiffSet{}
	for fp, e := range newMap {
		if _, ok := oldMap[fp]; !ok {
			out.Added = append(out.Added, e)
		} else {
			out.Unchanged++
		}
	}
	for fp, e := range oldMap {
		if _, ok := newMap[fp]; !ok {
			out.Removed = append(out.Removed, e)
		}
	}
	sort.Slice(out.Added, func(i, j int) bool { return out.Added[i].Fingerprint() < out.Added[j].Fingerprint() })
	sort.Slice(out.Removed, func(i, j int) bool { return out.Removed[i].Fingerprint() < out.Removed[j].Fingerprint() })
	return out
}

func toMap(es []Endpoint) map[string]Endpoint {
	out := make(map[string]Endpoint, len(es))
	for _, e := range es {
		out[e.Fingerprint()] = e
	}
	return out
}

// Detector agrega fontes.
type Detector struct {
	manualEndpoints []Endpoint
}

// NewDetector cria detector vazio.
func NewDetector() *Detector {
	return &Detector{}
}

// AddManual adiciona endpoints manuais (de config).
func (d *Detector) AddManual(eps []Endpoint) {
	for _, e := range eps {
		e.Source = "manual"
		d.manualEndpoints = append(d.manualEndpoints, e)
	}
}

// AllEndpoints retorna todos os endpoints conhecidos.
func (d *Detector) AllEndpoints() []Endpoint {
	return append([]Endpoint{}, d.manualEndpoints...)
}

// SnapshotFingerprint hash de todos os endpoints atuais.
// Usado pra detectar mudança entre runs.
func (d *Detector) SnapshotFingerprint() string {
	all := d.AllEndpoints()
	sort.Slice(all, func(i, j int) bool { return all[i].Fingerprint() < all[j].Fingerprint() })
	h := sha256.New()
	for _, e := range all {
		h.Write([]byte(e.Fingerprint()))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ParseOpenAPIEndpoints extrai endpoints de OpenAPI/Swagger JSON.
// Reusa schema subset do internal/zap.
func ParseOpenAPIEndpoints(data []byte) ([]Endpoint, error) {
	if len(data) == 0 {
		return nil, errors.New("changes: spec vazio")
	}
	// Detectar OpenAPI 3.x.
	var v3 struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &v3); err == nil && strings.HasPrefix(v3.OpenAPI, "3.") {
		return extractFromPaths(v3.Paths, "openapi"), nil
	}
	// Swagger 2.0.
	var v2 struct {
		Swagger string                                `json:"swagger"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &v2); err == nil && strings.HasPrefix(v2.Swagger, "2.") {
		return extractFromPaths(v2.Paths, "openapi"), nil
	}
	return nil, errors.New("changes: spec não é OpenAPI nem Swagger")
}

func extractFromPaths(paths map[string]map[string]json.RawMessage, source string) []Endpoint {
	var out []Endpoint
	for path, methods := range paths {
		for method := range methods {
			m := normalizeMethod(method)
			switch m {
			case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
				out = append(out, Endpoint{Method: m, Path: normalizePath(path), Source: source})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Fingerprint() < out[j].Fingerprint() })
	return out
}

// DetectControllerEndpoints — extrai endpoints de source code patterns.
// Patterns reconhecidos: gin/router.GET/POST, echo.GET/POST,
// express app.get/post, fastapi @app.get/post.
//
// Apenas string scan, sem parsing AST — usado pra heurística
// barata em monorepos.
func DetectControllerEndpoints(content []byte) []Endpoint {
	patterns := []struct {
		method string
		regex  string // simplificado — não regex completa aqui
	}{
		// patterns são pseudo-anchor; busca por substring.
		{"GET", ".GET(\""},
		{"POST", ".POST(\""},
		{"PUT", ".PUT(\""},
		{"DELETE", ".DELETE(\""},
		{"PATCH", ".PATCH(\""},
	}
	src := string(content)
	var out []Endpoint
	for _, p := range patterns {
		idx := 0
		for {
			at := strings.Index(src[idx:], p.regex)
			if at < 0 {
				break
			}
			idx += at + len(p.regex)
			// extrai path até próxima aspa.
			end := strings.Index(src[idx:], "\"")
			if end < 0 {
				continue
			}
			path := src[idx : idx+end]
			out = append(out, Endpoint{Method: p.method, Path: normalizePath(path), Source: "controller"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Fingerprint() < out[j].Fingerprint() })
	return out
}

// Fingerprint struct devolve hash do endpoint.
func Fingerprint(method, path string) string {
	return Endpoint{Method: method, Path: path}.Fingerprint()
}

// HashToFingerprint devolve hash hex de fingerprint.
func HashToFingerprint(method, path string) string {
	h := sha256.Sum256([]byte(Fingerprint(method, path)))
	return hex.EncodeToString(h[:])
}

// Summary devolve string human-readable do DiffSet.
func (d DiffSet) Summary() string {
	if d.IsEmpty() {
		return "no changes"
	}
	return fmt.Sprintf("+%d -%d ~%d =%d", len(d.Added), len(d.Removed), len(d.Changed), d.Unchanged)
}
