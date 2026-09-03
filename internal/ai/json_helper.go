package ai

import "encoding/json"

// jsonMarshal wrapper (testável).
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// jsonUnmarshal wrapper (testável).
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
