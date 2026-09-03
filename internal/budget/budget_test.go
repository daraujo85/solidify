package budget

import (
	"strings"
	"testing"
)

// Aceitação: Build inclui high priority antes de low.
func TestBuildPriorityOrder(t *testing.T) {
	b := NewBuilder(Budget{MaxChars: 0, MaxLines: 0})
	b.Add("low-1", PriorityLow, "r", "low content\n")
	b.Add("high-1", PriorityHigh, "r", "high content\n")
	b.Add("medium-1", PriorityMedium, "r", "medium content\n")
	b.Add("high-2", PriorityHigh, "r", "high 2 content\n")

	r := b.Build()
	if len(r.Items) != 4 {
		t.Errorf("items = %d", len(r.Items))
	}
	if r.Items[0].Source != "high-1" || r.Items[1].Source != "high-2" {
		t.Errorf("order: %v", r.Items)
	}
}

// Aceitação: low priority é dropado quando budget estourar.
func TestBuildLowDropsWhenBudgetExceeds(t *testing.T) {
	b := NewBuilder(Budget{MaxChars: 50})
	b.Add("low", PriorityLow, "r", strings.Repeat("x", 100))
	b.Add("high", PriorityHigh, "r", "high content")
	r := b.Build()
	if !strings.Contains(r.Content, "high content") {
		t.Errorf("high sumiu: %q", r.Content)
	}
	if strings.Contains(r.Content, "xxx") {
		t.Errorf("low não devia aparecer: %q", r.Content)
	}
	if len(r.Truncations) != 1 {
		t.Errorf("truncations = %d", len(r.Truncations))
	}
	if !r.Truncations[0].Truncated {
		t.Errorf("truncation não marcada")
	}
}

// Aceitação: high priority é truncao por linha quando budget pequeno.
func TestBuildHighTruncatedByLine(t *testing.T) {
	content := ""
	for i := 0; i < 100; i++ {
		content += "line\n"
	}
	b := NewBuilder(Budget{MaxLines: 10})
	b.Add("h", PriorityHigh, "r", content)
	r := b.Build()
	if r.TotalLines > 15 {
		t.Errorf("lines = %d, queria <=15", r.TotalLines)
	}
	if !strings.Contains(r.Content, "[... truncated") {
		t.Errorf("sem marker de trunc: %q", r.Content)
	}
}

// Aceitação: MaxChars respected.
func TestBuildMaxChars(t *testing.T) {
	b := NewBuilder(Budget{MaxChars: 100})
	b.Add("h", PriorityHigh, "r", strings.Repeat("a", 50))
	b.Add("h2", PriorityHigh, "r", strings.Repeat("b", 50))
	b.Add("h3", PriorityHigh, "r", strings.Repeat("c", 50))
	r := b.Build()
	if r.TotalChars > 100 {
		t.Errorf("chars = %d, quero <=100", r.TotalChars)
	}
}

// Aceitação: 10MB diff não explode memória.
func TestBuildLargeDiffBounded(t *testing.T) {
	// Diff artificial 10MB.
	big := strings.Repeat("diff line\n", 1024*1024) // 10MB ~ 10_485_760 chars
	b := NewBuilder(Budget{MaxChars: 10000, MaxLines: 100})
	b.Add("diff", PriorityLow, "git diff", big)
	r := b.Build()
	if r.TotalChars > 10000 {
		t.Errorf("chars = %d, devia estar budget", r.TotalChars)
	}
	// Truncation registrada.
	if len(r.Truncations) != 1 {
		t.Errorf("truncations = %d", len(r.Truncations))
	}
	if r.Truncations[0].OriginalChars < 10_000_000 {
		t.Errorf("original = %d, espero ~10MB", r.Truncations[0].OriginalChars)
	}
}

// Aceitação: empty builder.
func TestBuildEmpty(t *testing.T) {
	b := NewBuilder(Budget{MaxChars: 100})
	r := b.Build()
	if r.Content != "" {
		t.Errorf("content = %q", r.Content)
	}
	if len(r.Items) != 0 {
		t.Errorf("items = %d", len(r.Items))
	}
}

// Aceitação: zero budget = unlimited.
func TestBuildZeroBudget(t *testing.T) {
	b := NewBuilder(Budget{})
	b.Add("a", PriorityHigh, "r", strings.Repeat("x", 1000))
	r := b.Build()
	if r.TotalChars != 1000 {
		t.Errorf("chars = %d", r.TotalChars)
	}
}

// Aceitação: source order é alfabetico quando priority igual.
func TestBuildStableOrder(t *testing.T) {
	b := NewBuilder(Budget{})
	b.Add("z", PriorityHigh, "r", "z")
	b.Add("a", PriorityHigh, "r", "a")
	b.Add("m", PriorityHigh, "r", "m")
	r := b.Build()
	if r.Items[0].Source != "a" || r.Items[1].Source != "m" || r.Items[2].Source != "z" {
		t.Errorf("order: %v", r.Items)
	}
}

// Aceitação: read items de Reader.
func TestReadItems(t *testing.T) {
	input := `<<<source1|high|reason1>>>
content1 line1
content1 line2
<<<source2|low|reason2>>>
content2
`
	items, err := ReadItems(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadItems: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("items = %d", len(items))
	}
	if items[0].Source != "source1" || items[0].Priority != PriorityHigh {
		t.Errorf("err: %+v", items[0])
	}
	if !strings.Contains(items[0].Content, "content1 line1") {
		t.Errorf("content = %q", items[0].Content)
	}
}

// Aceitação: read items header inválido.
func TestReadItemsInvalidHeader(t *testing.T) {
	input := `<<<only_two|parts>>>
x
`
	if _, err := ReadItems(strings.NewReader(input)); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: read items priority inválida.
func TestReadItemsInvalidPriority(t *testing.T) {
	input := `<<<src|ultra|reason>>>
x
`
	if _, err := ReadItems(strings.NewReader(input)); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: NewBuilderFromReader.
func TestNewBuilderFromReader(t *testing.T) {
	input := `<<<s1|high|r>>>
x
`
	b, err := NewBuilderFromReader(strings.NewReader(input), Budget{})
	if err != nil {
		t.Fatalf("NewBuilderFromReader: %v", err)
	}
	if b.Len() != 1 {
		t.Errorf("len = %d", b.Len())
	}
}

// Aceitação: ContentHash da truncation.
func TestTruncationContentHash(t *testing.T) {
	c := "important content"
	b := NewBuilder(Budget{MaxChars: 5})
	b.Add("s", PriorityHigh, "r", c)
	r := b.Build()
	if r.Truncations[0].ContentHash == "" {
		t.Errorf("hash vazio")
	}
	if len(r.Truncations[0].ContentHash) != 64 {
		t.Errorf("hash não é SHA-256 hex: %d", len(r.Truncations[0].ContentHash))
	}
}

// Aceitação: PriorityString.
func TestPriorityString(t *testing.T) {
	cases := map[Priority]string{
		PriorityHigh:   "high",
		PriorityMedium: "medium",
		PriorityLow:    "low",
		Priority(99):   "low",
	}
	for p, want := range cases {
		if got := PriorityString(p); got != want {
			t.Errorf("got %s, quero %s", got, want)
		}
	}
}

// Aceitação: items não-truncados não têm truncation entry.
func TestNonTruncatedNoEntry(t *testing.T) {
	b := NewBuilder(Budget{})
	b.Add("s", PriorityHigh, "r", "small")
	r := b.Build()
	if len(r.Truncations) != 0 {
		t.Errorf("truncations = %d", len(r.Truncations))
	}
}

// Aceitação: high priority truncao por char quando muito grande.
func TestHighTruncatedByChar(t *testing.T) {
	c := strings.Repeat("a", 10000)
	b := NewBuilder(Budget{MaxChars: 100})
	b.Add("h", PriorityHigh, "r", c)
	r := b.Build()
	if r.TotalChars > 100 {
		t.Errorf("chars = %d", r.TotalChars)
	}
	if !r.Truncations[0].Truncated {
		t.Errorf("não truncado")
	}
}

// Aceitação: items low são dropados antes de high entrar.
func TestLowDroppedBeforeHighTrimmed(t *testing.T) {
	b := NewBuilder(Budget{MaxChars: 50})
	b.Add("h1", PriorityHigh, "r", strings.Repeat("H", 30))
	b.Add("l1", PriorityLow, "r", strings.Repeat("L", 100))
	b.Add("h2", PriorityHigh, "r", strings.Repeat("H", 30))
	r := b.Build()
	if !strings.Contains(r.Content, "HHHH") {
		t.Errorf("high items sumiram")
	}
	if strings.Contains(r.Content, "LLLL") {
		t.Errorf("low não devia aparecer")
	}
}

// Aceitação: items low mantidos se houver budget.
func TestLowKeptWhenBudgetAllows(t *testing.T) {
	b := NewBuilder(Budget{MaxChars: 1000})
	b.Add("l", PriorityLow, "r", "small low")
	r := b.Build()
	if !strings.Contains(r.Content, "small low") {
		t.Errorf("low devia ser mantido")
	}
}
