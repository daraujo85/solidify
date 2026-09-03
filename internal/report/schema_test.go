package report

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const miniSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["name", "score"],
  "properties": {
    "name": {"type": "string", "minLength": 1},
    "score": {"type": "number", "minimum": 0, "maximum": 100},
    "grade": {"type": "string", "enum": ["A", "B", "C"]},
    "sha": {"type": "string", "pattern": "^[a-f0-9]{64}$"},
    "count": {"type": "integer", "minimum": 0}
  }
}`

const refSchema = `{
  "type": "object",
  "required": ["item"],
  "properties": {
    "item": {"$ref": "#/$defs/item"}
  },
  "$defs": {
    "item": {
      "type": "object",
      "required": ["id"],
      "properties": {"id": {"type": "string"}, "val": {"type": "number"}}
    }
  }
}`

func TestValidatorBasicOK(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	data := []byte(`{"name": "x", "score": 85, "grade": "A"}`)
	if err := v.Validate(data); err != nil {
		t.Errorf("ok: %v", err)
	}
}

// Aceitação: required faltando.
func TestValidatorRequiredMissing(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	data := []byte(`{"name": "x"}`)
	if err := v.Validate(data); err == nil {
		t.Errorf("expected required")
	}
}

// Aceitação: tipo errado.
func TestValidatorTypeMismatch(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	data := []byte(`{"name": "x", "score": "high"}`)
	if err := v.Validate(data); err == nil {
		t.Errorf("expected type")
	}
}

// Aceitação: enum falha.
func TestValidatorEnumFail(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	data := []byte(`{"name": "x", "score": 50, "grade": "Z"}`)
	if err := v.Validate(data); err == nil {
		t.Errorf("expected enum")
	}
}

// Aceitação: pattern SHA256.
func TestValidatorPattern(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	data := []byte(`{"name": "x", "score": 50, "sha": "deadbeef"}`)
	if err := v.Validate(data); err == nil {
		t.Errorf("expected pattern")
	}
	good := []byte(`{"name": "x", "score": 50, "sha": "` + strings.Repeat("a", 64) + `"}`)
	if err := v.Validate(good); err != nil {
		t.Errorf("pattern ok: %v", err)
	}
}

// Aceitação: range number.
func TestValidatorRange(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	data := []byte(`{"name": "x", "score": 150}`)
	if err := v.Validate(data); err == nil {
		t.Errorf("expected range")
	}
}

// Aceitação: additionalProperties false.
func TestValidatorAdditional(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	data := []byte(`{"name": "x", "score": 50, "extra": "y"}`)
	if err := v.Validate(data); err == nil {
		t.Errorf("expected extra prop")
	}
}

// Aceitação: array items.
func TestValidatorArrayItems(t *testing.T) {
	sch := []byte(`{"type": "object", "properties": {"xs": {"type": "array", "items": {"type": "integer"}}}}`)
	v, _ := NewValidator(sch)
	if err := v.Validate([]byte(`{"xs": [1, 2, 3]}`)); err != nil {
		t.Errorf("ok: %v", err)
	}
	if err := v.Validate([]byte(`{"xs": [1, "x"]}`)); err == nil {
		t.Errorf("expected item type")
	}
}

// Aceitação: integer check.
func TestValidatorInteger(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	if err := v.Validate([]byte(`{"name": "x", "score": 50, "count": 5}`)); err != nil {
		t.Errorf("int ok: %v", err)
	}
	if err := v.Validate([]byte(`{"name": "x", "score": 50, "count": 5.5}`)); err == nil {
		t.Errorf("expected int")
	}
}

// Aceitação: $ref local.
func TestValidatorRef(t *testing.T) {
	v, _ := NewValidator([]byte(refSchema))
	if err := v.Validate([]byte(`{"item": {"id": "x"}}`)); err != nil {
		t.Errorf("ref: %v", err)
	}
	if err := v.Validate([]byte(`{"item": {}}`)); err == nil {
		t.Errorf("ref required")
	}
}

// Aceitação: schema parse fail.
func TestValidatorBadSchema(t *testing.T) {
	if _, err := NewValidator([]byte("not json")); err == nil {
		t.Errorf("expected parse")
	}
}

// Aceitação: data parse fail.
func TestValidatorBadData(t *testing.T) {
	v, _ := NewValidator([]byte(miniSchema))
	if err := v.Validate([]byte("not json")); err == nil {
		t.Errorf("expected data parse")
	}
}

// Aceitação: validator nil.
func TestValidatorNil(t *testing.T) {
	var v *Validator
	if err := v.Validate([]byte(`{}`)); err == nil {
		t.Errorf("nil")
	}
}

// Aceitação: schema real release-report.
func TestValidatorRealSchema(t *testing.T) {
	b, err := os.ReadFile("../../schemas/release-report.schema.json")
	if err != nil {
		t.Skip("schema ausente")
	}
	v, err := NewValidator(b)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	// report mínimo válido.
	r := map[string]any{
		"schema_version":  "1.1.0",
		"run":             map[string]any{"id": "r1", "profile": "release", "solidify_version": "1", "started_at": "2026-01-01T00:00:00Z", "finished_at": "2026-01-01T00:00:01Z", "config_hash": strings.Repeat("a", 64), "evidence_hash": strings.Repeat("b", 64)},
		"git":             map[string]any{"base_ref": "main", "base_sha": "abc", "head_ref": "feat", "head_sha": "def", "commits": []any{}, "changed_files": []any{}},
		"components":      []any{},
		"release_notes":   map[string]any{"executive_summary": "x", "groups": []any{}, "breaking_changes": []any{}},
		"migrations":      []any{},
		"env_changes":     []any{},
		"analyzers":       []any{},
		"solid":           map[string]any{"score": 80.0, "before_score": 70.0, "delta": 10.0, "principles": map[string]any{"S": fp(), "O": fp(), "L": fp(), "I": fp(), "D": fp()}},
		"ai_review":       map[string]any{"mode": "peer", "peer_reviewed": true, "actors": []any{}, "agreement": 0.9, "independence_degraded": false},
		"scores":          map[string]any{"quality": 80.0, "grade": "A", "confidence": 0.8, "confidence_level": "HIGH", "pillars": []any{}},
		"risk":            map[string]any{"level": "LOW", "factors": []any{}},
		"quality_gate":    map[string]any{"status": "PASS", "rules": []any{}},
		"recommendations": []any{},
		"limitations":     []any{},
		"artifacts":       []any{},
	}
	data, _ := json.Marshal(r)
	if err := v.Validate(data); err != nil {
		t.Errorf("real schema: %v", err)
	}
}

// Aceitação: real schema inválido.
func TestValidatorRealSchemaFail(t *testing.T) {
	b, err := os.ReadFile("../../schemas/release-report.schema.json")
	if err != nil {
		t.Skip("schema ausente")
	}
	v, _ := NewValidator(b)
	// status inválido.
	bad := []byte(`{"quality_gate": {"status": "PASS_WITH_WARNINGS", "rules": []}}`)
	if err := v.Validate(bad); err == nil {
		t.Errorf("real fail")
	}
}

// Aceitação: resolveRef helper.
func TestResolveRef(t *testing.T) {
	root := map[string]any{"$defs": map[string]any{"x": map[string]any{"type": "string"}}}
	r, err := ResolveRef(root, "#/$defs/x")
	if err != nil {
		t.Errorf("resolve: %v", err)
	}
	if r["type"] != "string" {
		t.Errorf("type")
	}
	if _, err := ResolveRef(root, "#/$defs/y"); err == nil {
		t.Errorf("missing")
	}
	if _, err := ResolveRef(root, "http://x"); err == nil {
		t.Errorf("external")
	}
}

// Aceitação: splitPath helper.
func TestSplitPath(t *testing.T) {
	if got := splitPath("$defs/x"); len(got) != 2 || got[1] != "x" {
		t.Errorf("path: %v", got)
	}
}

// Aceitação: const check.
func TestValidatorConst(t *testing.T) {
	sch := []byte(`{"type": "string", "const": "1.0.0"}`)
	v, _ := NewValidator(sch)
	if err := v.Validate([]byte(`"1.0.0"`)); err != nil {
		t.Errorf("const ok: %v", err)
	}
	if err := v.Validate([]byte(`"2.0.0"`)); err == nil {
		t.Errorf("const fail")
	}
}

// Aceitação: nested object validation.
func TestValidatorNested(t *testing.T) {
	sch := []byte(`{"type": "object", "properties": {"a": {"type": "object", "properties": {"b": {"type": "integer"}}}}}`)
	v, _ := NewValidator(sch)
	if err := v.Validate([]byte(`{"a": {"b": 5}}`)); err != nil {
		t.Errorf("nested: %v", err)
	}
	if err := v.Validate([]byte(`{"a": {"b": "x"}}`)); err == nil {
		t.Errorf("nested type")
	}
}

func fp() map[string]any {
	return map[string]any{
		"applicability": "APPLICABLE",
		"applicable":    true,
		"before_score":  70.0,
		"after_score":   80.0,
		"delta":         10.0,
		"confidence":    0.8,
		"reason":        "ok",
		"evidence_refs": []any{"file.go:1"},
		"summary":       "ok",
		"findings":      []any{},
	}
}
