// JSON Schema for Peer Review (SAI-055).
//
// Schema version "1" — embedded as JSON literal. Validator
// minimalista sem deps externas (subset JSON Schema Draft 7
// suficiente: required, type, enum, pattern, minLength, maxLength).
package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// PeerReviewJSONSchema schema literal v1.
const PeerReviewJSONSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "id": "https://solidify/peer-review.schema.json",
  "title": "Solidify Peer Review",
  "type": "object",
  "additionalProperties": false,
  "required": ["run_id", "actor", "schema", "evidence_hash", "verdict"],
  "properties": {
    "run_id": {"type": "string", "minLength": 1, "maxLength": 200},
    "actor": {"type": "string", "minLength": 1, "maxLength": 200},
    "schema": {"type": "string", "enum": ["1"]},
    "evidence_hash": {"type": "string", "pattern": "^[0-9a-f]{64}$"},
    "verdict": {"type": "string", "enum": ["approve", "request_changes", "comment"]},
    "findings": {"type": "object"},
    "notes": {"type": "string", "maxLength": 10000}
  }
}`

// SchemaValidator valida via JSON Schema subset.
type SchemaValidator struct {
	schema string
}

// NewSchemaValidator parseia schema literal.
func NewSchemaValidator(schema string) (*SchemaValidator, error) {
	if schema == "" {
		return nil, errors.New("schema: vazio")
	}
	var v any
	if err := json.Unmarshal([]byte(schema), &v); err != nil {
		return nil, fmt.Errorf("schema: parse: %w", err)
	}
	return &SchemaValidator{schema: schema}, nil
}

// NewPeerReviewSchemaValidator carrega schema padrão.
func NewPeerReviewSchemaValidator() (*SchemaValidator, error) {
	return NewSchemaValidator(PeerReviewJSONSchema)
}

// SchemaNode parsed schema.
type SchemaNode struct {
	Type                 string                 `json:"type,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Properties           map[string]*SchemaNode `json:"properties,omitempty"`
	Enum                 []any                  `json:"enum,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
	MinLength            int                    `json:"minLength,omitempty"`
	MaxLength            int                    `json:"maxLength,omitempty"`
	AdditionalProperties bool                   `json:"additionalProperties,omitempty"`
}

// ParsedSchema parsed schema.
type ParsedSchema struct {
	Root *SchemaNode
}

// Parse schema to node tree.
func (v *SchemaValidator) Parse() (*ParsedSchema, error) {
	var root SchemaNode
	if err := json.Unmarshal([]byte(v.schema), &root); err != nil {
		return nil, err
	}
	return &ParsedSchema{Root: &root}, nil
}

// SchemaError item.
type SchemaError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SchemaErrors slice.
type SchemaErrors []SchemaError

func (e SchemaErrors) Error() string {
	if len(e) == 0 {
		return "no schema errors"
	}
	parts := make([]string, len(e))
	for i, x := range e {
		parts[i] = x.Path + ": " + x.Message
	}
	return strings.Join(parts, "; ")
}

// Validate roda subset JSON Schema.
func (v *SchemaValidator) Validate(data []byte) SchemaErrors {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return SchemaErrors{{Path: "$", Message: "parse: " + err.Error()}}
	}
	parsed, err := v.Parse()
	if err != nil {
		return SchemaErrors{{Path: "$", Message: err.Error()}}
	}
	var errs SchemaErrors
	validateNode(parsed.Root, doc, "$", &errs)
	return errs
}

// ValidateMap valida map[string]any.
func (v *SchemaValidator) ValidateMap(doc map[string]any) SchemaErrors {
	parsed, err := v.Parse()
	if err != nil {
		return SchemaErrors{{Path: "$", Message: err.Error()}}
	}
	var errs SchemaErrors
	validateNode(parsed.Root, doc, "$", &errs)
	return errs
}

func validateNode(node *SchemaNode, value any, path string, errs *SchemaErrors) {
	if node == nil {
		return
	}
	if node.Type != "" {
		actual := jsonTypeOf(value)
		if actual != node.Type {
			*errs = append(*errs, SchemaError{Path: path, Message: fmt.Sprintf("expected %s, got %s", node.Type, actual)})
			return
		}
	}
	if node.Type == "object" {
		m, ok := value.(map[string]any)
		if !ok {
			return
		}
		for _, req := range node.Required {
			if _, present := m[req]; !present {
				*errs = append(*errs, SchemaError{Path: path + "." + req, Message: "required"})
			}
		}
		for k, v := range m {
			child, hasProp := node.Properties[k]
			if !hasProp {
				if !node.AdditionalProperties {
					*errs = append(*errs, SchemaError{Path: path + "." + k, Message: "additional property not allowed"})
				}
				continue
			}
			validateNode(child, v, path+"."+k, errs)
		}
	}
	if node.Type == "string" {
		s, ok := value.(string)
		if !ok {
			return
		}
		if node.MinLength > 0 && len(s) < node.MinLength {
			*errs = append(*errs, SchemaError{Path: path, Message: fmt.Sprintf("minLength %d", node.MinLength)})
		}
		if node.MaxLength > 0 && len(s) > node.MaxLength {
			*errs = append(*errs, SchemaError{Path: path, Message: fmt.Sprintf("maxLength %d", node.MaxLength)})
		}
		if node.Pattern != "" {
			matched, err := regexp.MatchString(node.Pattern, s)
			if err != nil {
				*errs = append(*errs, SchemaError{Path: path, Message: "pattern invalid: " + err.Error()})
			} else if !matched {
				*errs = append(*errs, SchemaError{Path: path, Message: "pattern mismatch"})
			}
		}
	}
	if len(node.Enum) > 0 {
		matched := false
		for _, e := range node.Enum {
			if e == value {
				matched = true
				break
			}
		}
		if !matched {
			*errs = append(*errs, SchemaError{Path: path, Message: "enum mismatch"})
		}
	}
}

func jsonTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, int, int32, int64:
		return "number"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return "unknown"
	}
}

// IsValid helper rápido.
func (e SchemaErrors) IsValid() bool { return len(e) == 0 }

// ValidPeerReviewFixture JSON válido.
const ValidPeerReviewFixture = `{
  "run_id": "r1",
  "actor": "alice",
  "schema": "1",
  "evidence_hash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
  "verdict": "approve",
  "findings": {},
  "notes": "looks good"
}`

// InvalidPeerReviewFixtures fixtures inválidos.
var InvalidPeerReviewFixtures = map[string]string{
	"missing_run_id": `{
		"actor": "alice",
		"schema": "1",
		"evidence_hash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"verdict": "approve"
	}`,
	"unknown_schema": `{
		"run_id": "r1",
		"actor": "alice",
		"schema": "99",
		"evidence_hash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"verdict": "approve"
	}`,
	"bad_hash": `{
		"run_id": "r1",
		"actor": "alice",
		"schema": "1",
		"evidence_hash": "not-hex",
		"verdict": "approve"
	}`,
	"bad_verdict": `{
		"run_id": "r1",
		"actor": "alice",
		"schema": "1",
		"evidence_hash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"verdict": "wat"
	}`,
	"additional_field": `{
		"run_id": "r1",
		"actor": "alice",
		"schema": "1",
		"evidence_hash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"verdict": "approve",
		"unexpected": "field"
	}`,
	"actor_too_long": `{
		"run_id": "r1",
		"actor": "` + strings.Repeat("a", 201) + `",
		"schema": "1",
		"evidence_hash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"verdict": "approve"
	}`,
	"notes_too_long": `{
		"run_id": "r1",
		"actor": "alice",
		"schema": "1",
		"evidence_hash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"verdict": "approve",
		"notes": "` + strings.Repeat("x", 10001) + `"
	}`,
	"wrong_type": `{
		"run_id": "r1",
		"actor": "alice",
		"schema": "1",
		"evidence_hash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"verdict": 123
	}`,
}

// RunSchemaFixtures roda todas fixtures e retorna map result.
func RunSchemaFixtures(v *SchemaValidator, valid string, invalids map[string]string) map[string]bool {
	out := make(map[string]bool)
	if errs := v.Validate([]byte(valid)); errs.IsValid() {
		out["valid"] = true
	} else {
		out["valid"] = false
	}
	for name, fixture := range invalids {
		errs := v.Validate([]byte(fixture))
		out[name] = !errs.IsValid()
	}
	return out
}
