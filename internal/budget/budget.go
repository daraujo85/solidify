// Package budget — bounded context windows para LLMs.
//
// SAI-024: char/line budgets, priorities, reason-for-context, truncation
// metadata. Diff >10MB não explode memória — entrada é streamed via
// Reader e mantemos apenas o que cabe.
//
// Estratégia: incluímos tudo até bater budget, priorizando high antes
// de medium antes de low. Items são truncaveis por linha (head + tail)
// ou por char (mid).
package budget

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Priority de um item. High = sempre incluído. Low = primeiro a truncar.
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityMedium Priority = 1
	PriorityHigh   Priority = 2
)

// Item é uma evidência a incluir (ou truncar) no output.
type Item struct {
	Source   string // "git.diff", "envx", "migrations", etc
	Priority Priority
	Reason   string // motivo para incluir (audit/traceability)
	Content  string // conteúdo
}

// Budget limita o output total.
type Budget struct {
	MaxChars int // limite máximo de chars totais (0 = ilimitado)
	MaxLines int // limite máximo de linhas (0 = ilimitado)
}

// Truncation registra como um item foi cortado.
type Truncation struct {
	Source        string `json:"source"`
	Reason        string `json:"reason"`
	Priority      int    `json:"priority"`
	OriginalChars int    `json:"original_chars"`
	KeptChars     int    `json:"kept_chars"`
	OriginalLines int    `json:"original_lines"`
	KeptLines     int    `json:"kept_lines"`
	Truncated     bool   `json:"truncated"`
	ContentHash   string `json:"content_hash"` // SHA-256 do original
}

// Result é o resultado do build.
type Result struct {
	Content     string       // saída final
	Items       []Item       // itens incluídos (na ordem)
	Truncations []Truncation // metadata de truncations
	TotalChars  int
	TotalLines  int
}

// Builder agrega itens e monta o resultado respeitando budget.
type Builder struct {
	budget    Budget
	items     []Item
	truncated []Truncation
}

// NewBuilder cria um builder.
func NewBuilder(b Budget) *Builder {
	return &Builder{budget: b}
}

// Add adiciona um item com source priority reason content.
func (b *Builder) Add(source string, priority Priority, reason, content string) {
	b.items = append(b.items, Item{
		Source:   source,
		Priority: priority,
		Reason:   reason,
		Content:  content,
	})
}

// AddItem adiciona um Item já construído.
func (b *Builder) AddItem(it Item) {
	b.items = append(b.items, it)
}

// Len devolve nº de items.
func (b *Builder) Len() int { return len(b.items) }

// Build monta o resultado. Items high-priority são garantidos; low são
// truncados se necessário.
//
// Algoritmo:
//  1. Sort por priority desc, depois source asc (estabilidade).
//  2. Itera; tenta incluir item inteiro. Se não cabe, low é dropado,
//     high/medium é truncado via head+tail.
//  3. Marca truncation metadata.
func (b *Builder) Build() Result {
	// Sort estável.
	sorted := make([]Item, len(b.items))
	copy(sorted, b.items)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		return sorted[i].Source < sorted[j].Source
	})

	usedChars := 0
	usedLines := 0
	var out []string
	var kept []Item
	var truncs []Truncation

	hasCharLimit := b.budget.MaxChars > 0
	hasLineLimit := b.budget.MaxLines > 0

	for _, it := range sorted {
		origChars := len(it.Content)
		origLines := strings.Count(it.Content, "\n") + 1
		if origLines == 0 {
			origLines = 1
		}
		contentHash := hashString(it.Content)

		// Cabe inteiro?
		canFitChars := !hasCharLimit || usedChars+origChars <= b.budget.MaxChars
		canFitLines := !hasLineLimit || usedLines+origLines <= b.budget.MaxLines
		if canFitChars && canFitLines {
			kept = append(kept, it)
			out = append(out, it.Content)
			usedChars += origChars
			usedLines += origLines
			continue
		}

		// Não cabe. Low priority é dropado.
		if it.Priority == PriorityLow {
			truncs = append(truncs, Truncation{
				Source:        it.Source,
				Reason:        it.Reason,
				Priority:      int(it.Priority),
				OriginalChars: origChars,
				KeptChars:     0,
				OriginalLines: origLines,
				KeptLines:     0,
				Truncated:     true,
				ContentHash:   contentHash,
			})
			continue
		}

		// High/medium: trunca por linha e char.
		keepLines := 0
		if hasLineLimit {
			keepLines = b.budget.MaxLines - usedLines
			if keepLines < 0 {
				keepLines = 0
			}
		}
		keepChars := 0
		if hasCharLimit {
			keepChars = b.budget.MaxChars - usedChars
			if keepChars < 0 {
				keepChars = 0
			}
		}
		truncated := truncate(it.Content, keepLines, keepChars)
		wasTrunc := truncated != it.Content
		keptChars := len(truncated)
		keptLines := strings.Count(truncated, "\n") + 1
		if keptLines == 0 {
			keptLines = 1
		}
		kept = append(kept, it)
		out = append(out, truncated)
		usedChars += keptChars
		usedLines += keptLines
		truncs = append(truncs, Truncation{
			Source:        it.Source,
			Reason:        it.Reason,
			Priority:      int(it.Priority),
			OriginalChars: origChars,
			KeptChars:     keptChars,
			OriginalLines: origLines,
			KeptLines:     keptLines,
			Truncated:     wasTrunc,
			ContentHash:   contentHash,
		})
	}

	return Result{
		Content:     strings.Join(out, "\n"),
		Items:       kept,
		Truncations: truncs,
		TotalChars:  usedChars,
		TotalLines:  usedLines,
	}
}

// ReadItems lê Evidence Items de Reader linha-a-linha (formato:
// `<<<source|priority|reason>>>`\ncontent). Útil para inputs grandes.
func ReadItems(r io.Reader) ([]Item, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var items []Item
	var current *Item
	var content []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "<<<") && strings.HasSuffix(line, ">>>") {
			// Salva item anterior.
			if current != nil {
				current.Content = strings.Join(content, "\n")
				items = append(items, *current)
			}
			header := line[3 : len(line)-3]
			parts := strings.SplitN(header, "|", 3)
			if len(parts) != 3 {
				return nil, fmt.Errorf("budget: header inválido: %s", line)
			}
			pri := PriorityLow
			switch parts[1] {
			case "low":
				pri = PriorityLow
			case "medium":
				pri = PriorityMedium
			case "high":
				pri = PriorityHigh
			default:
				return nil, fmt.Errorf("budget: priority inválida: %s", parts[1])
			}
			current = &Item{Source: parts[0], Priority: pri, Reason: parts[2]}
			content = content[:0]
			continue
		}
		if current != nil {
			content = append(content, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("budget: scan: %w", err)
	}
	if current != nil {
		current.Content = strings.Join(content, "\n")
		items = append(items, *current)
	}
	return items, nil
}

// NewBuilderFromReader lê items do reader e constrói builder.
func NewBuilderFromReader(r io.Reader, b Budget) (*Builder, error) {
	items, err := ReadItems(r)
	if err != nil {
		return nil, err
	}
	bb := NewBuilder(b)
	for _, it := range items {
		bb.AddItem(it)
	}
	return bb, nil
}

// truncate devolve content cortado a head + tail respeitando limites.
// keepLines=0 ou keepChars=0 corta agressivamente.
func truncate(content string, keepLines, keepChars int) string {
	if keepLines <= 0 && keepChars <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	// Por linha: manter head + tail de keepLines.
	if keepLines > 0 && len(lines) > keepLines {
		half := keepLines / 2
		head := lines[:half]
		tail := lines[len(lines)-half:]
		marker := fmt.Sprintf("[... truncated %d lines ...]", len(lines)-2*half)
		combined := append([]string{}, head...)
		combined = append(combined, marker)
		combined = append(combined, tail...)
		content = strings.Join(combined, "\n")
	}
	// Por char: se ainda passa, manter head + tail + marker.
	if keepChars > 0 && len(content) > keepChars {
		marker := fmt.Sprintf("[... truncated %d chars ...]", len(content))
		markerLen := len(marker)
		if keepChars <= markerLen {
			return ""
		}
		contentChars := keepChars - markerLen
		head := contentChars / 2
		tail := contentChars - head
		if head+tail+markerLen > len(content) {
			return content
		}
		content = content[:head] + marker + content[len(content)-tail:]
	}
	return content
}

// hashString devolve SHA-256 hex do conteúdo.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// PriorityString devolve nome textual da priority.
func PriorityString(p Priority) string {
	switch p {
	case PriorityHigh:
		return "high"
	case PriorityMedium:
		return "medium"
	default:
		return "low"
	}
}
