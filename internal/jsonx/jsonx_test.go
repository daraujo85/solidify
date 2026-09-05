package jsonx

import "testing"

func TestParseContentClean(t *testing.T) {
	got, ok := ParseContent(`{"a":1,"b":"x"}`)
	if !ok {
		t.Fatal("esperava parse")
	}
	if got["a"].(float64) != 1 {
		t.Errorf("a = %v", got["a"])
	}
}

func TestParseContentFenced(t *testing.T) {
	s := "```json\n{\"k\":42}\n```"
	got, ok := ParseContent(s)
	if !ok || got["k"].(float64) != 42 {
		t.Errorf("fenced: got=%v ok=%v", got, ok)
	}
}

func TestParseContentThinkBlock(t *testing.T) {
	// mimo/MiniMax-M2.1/claude-coder costumam prefixar com <think>...</think>.
	s := "<think>let me reason</think>{\"decision\":\"ok\",\"reason\":\"fine\"}"
	got, ok := ParseContent(s)
	if !ok {
		t.Fatalf("think block: expected parse, got ok=false")
	}
	if got["decision"] != "ok" {
		t.Errorf("decision = %v", got["decision"])
	}
}

func TestParseContentProseBeforeJSON(t *testing.T) {
	s := "Sure, here you go:\n{\"k\":1}"
	got, ok := ParseContent(s)
	if !ok || got["k"].(float64) != 1 {
		t.Errorf("prose prefix: got=%v ok=%v", got, ok)
	}
}

func TestCandidatesIgnoresEscapedBraces(t *testing.T) {
	s := `{"k":"}{","v":1}`
	c := Candidates(s)
	if len(c) != 1 {
		t.Errorf("quer 1 candidato, got %d", len(c))
	}
}

func TestCandidatesSkipsNestedObjects(t *testing.T) {
	s := `{"a":{"b":1}}`
	c := Candidates(s)
	if len(c) != 1 || c[0][0] != 0 {
		t.Errorf("nested: %v", c)
	}
}

func TestFirstBounds(t *testing.T) {
	s := `{}{"a":1}`
	start, end := FirstBounds(s)
	if start != 0 || end != 1 {
		t.Errorf("first bounds: start=%d end=%d", start, end)
	}
}