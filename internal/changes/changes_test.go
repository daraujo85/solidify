package changes

import (
	"strings"
	"testing"
)

// Aceitação: Endpoint Fingerprint.
func TestEndpointFingerprint(t *testing.T) {
	a := Endpoint{Method: "GET", Path: "/users"}
	b := Endpoint{Method: "get", Path: "/users"}
	if a.Fingerprint() != b.Fingerprint() {
		t.Errorf("case-insensitive")
	}
	c := Endpoint{Method: "GET", Path: "//users"}
	if a.Fingerprint() != c.Fingerprint() {
		t.Errorf("slash collapse")
	}
}

// Aceitação: normalizeMethod/normalizePath.
func TestNormalize(t *testing.T) {
	if normalizeMethod("get") != "GET" {
		t.Errorf("method")
	}
	if normalizePath("") != "/" {
		t.Errorf("empty")
	}
	if normalizePath("users") != "/users" {
		t.Errorf("no leading")
	}
	if normalizePath("///users//") != "/users/" {
		t.Errorf("multi slash")
	}
}

// Aceitação: Diff empty.
func TestDiffEmpty(t *testing.T) {
	d := Diff(nil, nil)
	if !d.IsEmpty() {
		t.Errorf("empty devia IsEmpty")
	}
}

// Aceitação: Diff added.
func TestDiffAdded(t *testing.T) {
	old := []Endpoint{{Method: "GET", Path: "/a"}}
	new := []Endpoint{{Method: "GET", Path: "/a"}, {Method: "GET", Path: "/b"}}
	d := Diff(old, new)
	if len(d.Added) != 1 || d.Added[0].Path != "/b" {
		t.Errorf("added = %+v", d.Added)
	}
	if d.Unchanged != 1 {
		t.Errorf("unchanged = %d", d.Unchanged)
	}
}

// Aceitação: Diff removed.
func TestDiffRemoved(t *testing.T) {
	old := []Endpoint{{Method: "GET", Path: "/a"}, {Method: "GET", Path: "/b"}}
	new := []Endpoint{{Method: "GET", Path: "/a"}}
	d := Diff(old, new)
	if len(d.Removed) != 1 || d.Removed[0].Path != "/b" {
		t.Errorf("removed = %+v", d.Removed)
	}
}

// Aceitação: Diff HasChanges.
func TestDiffHasChanges(t *testing.T) {
	old := []Endpoint{{Method: "GET", Path: "/a"}}
	new := []Endpoint{{Method: "GET", Path: "/a"}, {Method: "GET", Path: "/b"}}
	d := Diff(old, new)
	if !d.HasChanges() {
		t.Errorf("added devia ser change")
	}
}

// Aceitação: Detector AddManual.
func TestDetectorAddManual(t *testing.T) {
	d := NewDetector()
	d.AddManual([]Endpoint{{Method: "GET", Path: "/x"}})
	d.AddManual([]Endpoint{{Method: "POST", Path: "/y"}})
	all := d.AllEndpoints()
	if len(all) != 2 {
		t.Errorf("got %d", len(all))
	}
	for _, e := range all {
		if e.Source != "manual" {
			t.Errorf("source = %s", e.Source)
		}
	}
}

// Aceitação: SnapshotFingerprint estável.
func TestSnapshotFingerprintStable(t *testing.T) {
	d1 := NewDetector()
	d1.AddManual([]Endpoint{{Method: "GET", Path: "/a"}, {Method: "POST", Path: "/b"}})
	d2 := NewDetector()
	d2.AddManual([]Endpoint{{Method: "POST", Path: "/b"}, {Method: "GET", Path: "/a"}})
	if d1.SnapshotFingerprint() != d2.SnapshotFingerprint() {
		t.Errorf("ordem não devia importar")
	}
}

// Aceitação: SnapshotFingerprint detecta mudança.
func TestSnapshotFingerprintChange(t *testing.T) {
	d1 := NewDetector()
	d1.AddManual([]Endpoint{{Method: "GET", Path: "/a"}})
	d2 := NewDetector()
	d2.AddManual([]Endpoint{{Method: "GET", Path: "/a"}, {Method: "GET", Path: "/b"}})
	if d1.SnapshotFingerprint() == d2.SnapshotFingerprint() {
		t.Errorf("mudança devia mudar hash")
	}
}

// Aceitação: ParseOpenAPIEndpoints 3.x.
func TestParseOpenAPIEndpoints3(t *testing.T) {
	data := []byte(`{"openapi":"3.0.0","paths":{"/a":{"get":{}},"/b":{"post":{}}}}`)
	eps, err := ParseOpenAPIEndpoints(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(eps) != 2 {
		t.Errorf("got %d", len(eps))
	}
}

// Aceitação: ParseOpenAPIEndpoints Swagger 2.0.
func TestParseOpenAPIEndpoints2(t *testing.T) {
	data := []byte(`{"swagger":"2.0","paths":{"/x":{"get":{}}}}`)
	eps, err := ParseOpenAPIEndpoints(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(eps) != 1 {
		t.Errorf("got %d", len(eps))
	}
}

// Aceitação: ParseOpenAPIEndpoints inválido.
func TestParseOpenAPIEndpointsInvalid(t *testing.T) {
	if _, err := ParseOpenAPIEndpoints([]byte("garbage")); err == nil {
		t.Errorf("devia falhar")
	}
	if _, err := ParseOpenAPIEndpoints(nil); err == nil {
		t.Errorf("nil devia falhar")
	}
}

// Aceitação: ParseOpenAPIEndpoints sem openapi/swagger.
func TestParseOpenAPIEndpointsUnknown(t *testing.T) {
	if _, err := ParseOpenAPIEndpoints([]byte(`{"foo":"bar"}`)); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ParseOpenAPIEndpoints filtra non-http methods.
func TestParseOpenAPIEndpointsFilters(t *testing.T) {
	data := []byte(`{"openapi":"3.0.0","paths":{"/a":{"get":{}}}}`)
	eps, _ := ParseOpenAPIEndpoints(data)
	if len(eps) != 1 {
		t.Errorf("got %d", len(eps))
	}
}

// Aceitação: DetectControllerEndpoints gin/echo.
func TestDetectControllerEndpoints(t *testing.T) {
	src := []byte(`
		router.GET("/users", listUsers)
		router.POST("/users", createUser)
		api.DELETE("/users/{id}", deleteUser)
		api.PUT("/users/{id}", updateUser)
	`)
	eps := DetectControllerEndpoints(src)
	if len(eps) != 4 {
		t.Errorf("got %d endpoints: %+v", len(eps), eps)
	}
}

// Aceitação: DetectControllerEndpoints sem matches.
func TestDetectControllerEndpointsNone(t *testing.T) {
	src := []byte(`package main; func main() {}`)
	eps := DetectControllerEndpoints(src)
	if len(eps) != 0 {
		t.Errorf("got %d", len(eps))
	}
}

// Aceitação: DetectControllerEndpoints source tag.
func TestDetectControllerEndpointsSource(t *testing.T) {
	src := []byte(`router.GET("/a", h)`)
	eps := DetectControllerEndpoints(src)
	if len(eps) != 1 || eps[0].Source != "controller" {
		t.Errorf("got %+v", eps)
	}
}

// Aceitação: HashToFingerprint / Fingerprint helpers.
func TestFingerprintHelpers(t *testing.T) {
	fp1 := Fingerprint("GET", "/a")
	fp2 := Fingerprint("GET", "/a")
	if fp1 != fp2 {
		t.Errorf("mesmo input mesmo fingerprint")
	}
	hash := HashToFingerprint("GET", "/a")
	if len(hash) != 64 {
		t.Errorf("sha256 hex = 64 chars")
	}
}

// Aceitação: DiffSet Summary.
func TestDiffSetSummary(t *testing.T) {
	d := DiffSet{}
	if !strings.Contains(d.Summary(), "no changes") {
		t.Errorf("empty = no changes")
	}
	d.Added = []Endpoint{{Method: "GET", Path: "/a"}}
	if !strings.Contains(d.Summary(), "+1") {
		t.Errorf("summary = %q", d.Summary())
	}
}

// Aceitação: toMap.
func TestToMap(t *testing.T) {
	m := toMap([]Endpoint{{Method: "GET", Path: "/a"}, {Method: "GET", Path: "/a"}})
	if len(m) != 1 {
		t.Errorf("dup devia colapsar: %d", len(m))
	}
}

// Aceitação: Detector vazio.
func TestDetectorEmpty(t *testing.T) {
	d := NewDetector()
	if len(d.AllEndpoints()) != 0 {
		t.Errorf("vazio")
	}
	if d.SnapshotFingerprint() == "" {
		t.Errorf("hash sempre presente")
	}
}

// Aceitação: Diff mixed.
func TestDiffMixed(t *testing.T) {
	old := []Endpoint{{Method: "GET", Path: "/a"}, {Method: "GET", Path: "/b"}}
	new := []Endpoint{{Method: "GET", Path: "/a"}, {Method: "GET", Path: "/c"}}
	d := Diff(old, new)
	if len(d.Added) != 1 || d.Added[0].Path != "/c" {
		t.Errorf("added = %+v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0].Path != "/b" {
		t.Errorf("removed = %+v", d.Removed)
	}
	if d.Unchanged != 1 {
		t.Errorf("unchanged = %d", d.Unchanged)
	}
}

// Aceitação: Source constants.
func TestEndpointSources(t *testing.T) {
	e := Endpoint{Source: "x"}
	if e.Source != "x" {
		t.Errorf("source")
	}
}
