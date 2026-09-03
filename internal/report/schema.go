// JSON Schema validation (SAI-076).
//
// Validador mínimo p/ `release-report.schema.json` em Draft
// 2020-12 subset. Sem dep externa — checa tipos, required,
// enum, pattern, range e additionalProperties.
//
// Cobertura intencional: modela só o que o report usa.
// Trade-off documentado em ADR 0070.
package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
)

// Validator valida report contra schema.
type Validator struct {
	schema map[string]any
	root   map[string]any
}

// NewValidator carrega schema.
func NewValidator(schemaJSON []byte) (*Validator, error) {
	var s map[string]any
	if err := json.Unmarshal(schemaJSON, &s); err != nil {
		return nil, fmt.Errorf("schema parse: %w", err)
	}
	return &Validator{schema: s, root: s}, nil
}

// Validate checa data vs schema.
func (v *Validator) Validate(data []byte) error {
	if v == nil || v.schema == nil {
		return errors.New("report: validator vazio")
	}
	var d any
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("data parse: %w", err)
	}
	return validateValue(d, v.schema, v.root, "$")
}

func validateValue(val any, sch any, root map[string]any, path string) error {
	sm, ok := sch.(map[string]any)
	if !ok {
		return errors.New("report: schema não-objeto em " + path)
	}
	// $ref simples — só intra-doc #/$defs/X.
	if ref, ok := sm["$ref"].(string); ok {
		resolved, err := resolveRef(ref, root)
		if err != nil {
			return err
		}
		return validateValue(val, resolved, root, path)
	}
	// type check.
	if t, ok := sm["type"].(string); ok {
		if err := checkType(val, t, path); err != nil {
			return err
		}
	}
	// enum.
	if enum, ok := sm["enum"].([]any); ok {
		if !inEnum(val, enum) {
			return fmt.Errorf("%s: valor fora do enum", path)
		}
	}
	// const.
	if c, ok := sm["const"]; ok {
		if !reflect.DeepEqual(val, c) {
			return fmt.Errorf("%s: valor diferente do const", path)
		}
	}
	// pattern (strings).
	if p, ok := sm["pattern"].(string); ok {
		s, isStr := val.(string)
		if !isStr {
			return fmt.Errorf("%s: pattern mas não-string", path)
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("%s: pattern inválido: %w", path, err)
		}
		if !re.MatchString(s) {
			return fmt.Errorf("%s: pattern não bate", path)
		}
	}
	// range (numbers).
	if min, ok := sm["minimum"].(float64); ok {
		f, isF := asFloat(val)
		if !isF || f < min {
			return fmt.Errorf("%s: abaixo do mínimo", path)
		}
	}
	if max, ok := sm["maximum"].(float64); ok {
		f, isF := asFloat(val)
		if !isF || f > max {
			return fmt.Errorf("%s: acima do máximo", path)
		}
	}
	// minLength / maxLength (strings).
	if mn, ok := sm["minLength"].(float64); ok {
		s, isStr := val.(string)
		if !isStr || float64(len(s)) < mn {
			return fmt.Errorf("%s: string curta demais", path)
		}
	}
	// object: required + additionalProperties + properties.
	if valObj, ok := val.(map[string]any); ok {
		if req, ok := sm["required"].([]any); ok {
			for _, r := range req {
				rs, isStr := r.(string)
				if !isStr {
					continue
				}
				if _, present := valObj[rs]; !present {
					return fmt.Errorf("%s: required faltando %q", path, rs)
				}
			}
		}
		ap, hasAP := sm["additionalProperties"]
		if hasAP && !isTrue(ap) {
			for k := range valObj {
				if _, hasProp := sm["properties"].(map[string]any); hasProp {
					props := sm["properties"].(map[string]any)
					if _, ok := props[k]; !ok {
						return fmt.Errorf("%s: prop extra %q", path, k)
					}
				}
			}
		}
		if props, ok := sm["properties"].(map[string]any); ok {
			for k, p := range props {
				if cv, present := valObj[k]; present {
					if err := validateValue(cv, p, root, path+"."+k); err != nil {
						return err
					}
				}
			}
		}
	}
	// array: items.
	if valArr, ok := val.([]any); ok {
		if items, ok := sm["items"]; ok {
			for i, item := range valArr {
				ip := path + "[" + strconv.Itoa(i) + "]"
				if err := validateValue(item, items, root, ip); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func resolveRef(ref string, root map[string]any) (map[string]any, error) {
	// only "#/$defs/Name".
	if len(ref) < 2 || ref[0] != '#' {
		return nil, errors.New("report: $ref não-local: " + ref)
	}
	path := ref[1:]
	if path[0] == '/' {
		path = path[1:]
	}
	segs := splitPath(path)
	cur := root
	for _, s := range segs {
		cm, ok := cur[s].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("report: $ref %s não-resolve", ref)
		}
		cur = cm
	}
	return cur, nil
}

// ResolveRef exposto p/ testes.
func ResolveRef(root map[string]any, ref string) (map[string]any, error) {
	return resolveRef(ref, root)
}

func splitPath(p string) []string {
	out := []string{}
	cur := ""
	for _, c := range p {
		if c == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func checkType(val any, t, path string) error {
	switch t {
	case "object":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("%s: esperava object", path)
		}
	case "array":
		if _, ok := val.([]any); !ok {
			return fmt.Errorf("%s: esperava array", path)
		}
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("%s: esperava string", path)
		}
	case "number":
		if _, ok := val.(float64); !ok {
			return fmt.Errorf("%s: esperava number", path)
		}
	case "integer":
		f, ok := val.(float64)
		if !ok || f != float64(int(f)) {
			return fmt.Errorf("%s: esperava integer", path)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("%s: esperava boolean", path)
		}
	case "null":
		if val != nil {
			return fmt.Errorf("%s: esperava null", path)
		}
	default:
		// tipo desconhecido — tolera.
	}
	return nil
}

func inEnum(v any, enum []any) bool {
	for _, e := range enum {
		if reflect.DeepEqual(v, e) {
			return true
		}
	}
	return false
}

func isTrue(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	}
	return 0, false
}
