// SOLID Score engine (SAI-071).
//
// Média das letras aplicáveis; before/after/delta; N/A correto.
package score

import (
	"errors"
	"sort"
	"strings"
)

// Principles canônicos (ordem fixa S→O→L→I→D).
var Principles = []string{"SRP", "OCP", "LSP", "ISP", "DIP"}

// LetterScore uma letra SOLID.
type LetterScore struct {
	Principle  string  `json:"principle"`
	Score      float64 `json:"score"` // 0..100
	Applicable bool    `json:"applicable"`
	Reasoning  string  `json:"reasoning,omitempty"`
}

// Snapshot conjunto de scores (before/after).
type Snapshot struct {
	RunID   string        `json:"run_id"`
	Label   string        `json:"label"` // "before"|"after"
	Letters []LetterScore `json:"letters"`
}

// Delta before → after.
type Delta struct {
	RunID            string  `json:"run_id"`
	BeforeScore      float64 `json:"before_score"`
	AfterScore       float64 `json:"after_score"`
	ScoreDelta       float64 `json:"score_delta"`
	Improved         bool    `json:"improved"`
	Regressed        bool    `json:"regressed"`
	Unchanged        bool    `json:"unchanged"`
	ApplicableBefore int     `json:"applicable_before"`
	ApplicableAfter  int     `json:"applicable_after"`
	Notes            string  `json:"notes,omitempty"`
}

// ComputeScore média das letras aplicáveis.
// Letras N/A (Applicable=false) são excluídas do cálculo.
// Se nenhuma aplicável → score = 0 (sem divisão por zero).
func ComputeScore(letters []LetterScore) float64 {
	sum := 0.0
	count := 0
	for _, l := range letters {
		if !l.Applicable {
			continue
		}
		if l.Score < 0 {
			continue
		}
		sum += l.Score
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// ComputeDelta before → after.
func ComputeDelta(before, after Snapshot) (*Delta, error) {
	if before.RunID == "" || after.RunID == "" {
		return nil, errors.New("score: RunID vazio")
	}
	if before.RunID != after.RunID {
		return nil, errors.New("score: RunIDs diferentes")
	}
	bScore := ComputeScore(before.Letters)
	aScore := ComputeScore(after.Letters)
	d := &Delta{
		RunID:            before.RunID,
		BeforeScore:      bScore,
		AfterScore:       aScore,
		ScoreDelta:       aScore - bScore,
		ApplicableBefore: countApplicable(before.Letters),
		ApplicableAfter:  countApplicable(after.Letters),
	}
	switch {
	case d.ScoreDelta > 0:
		d.Improved = true
	case d.ScoreDelta < 0:
		d.Regressed = true
	default:
		d.Unchanged = true
	}
	return d, nil
}

// countApplicable número de letras aplicáveis.
func countApplicable(letters []LetterScore) int {
	n := 0
	for _, l := range letters {
		if l.Applicable {
			n++
		}
	}
	return n
}

// ValidateLetters checa consistência.
func ValidateLetters(letters []LetterScore) error {
	seen := make(map[string]bool)
	for _, l := range letters {
		if l.Principle == "" {
			return errors.New("score: principle vazio")
		}
		if seen[l.Principle] {
			return errors.New("score: principle duplicado: " + l.Principle)
		}
		seen[l.Principle] = true
		if l.Applicable && (l.Score < 0 || l.Score > 100) {
			return errors.New("score: " + l.Principle + " fora de [0,100]")
		}
	}
	return nil
}

// CanonicalLetters ordena por Principles canônico.
func CanonicalLetters(letters []LetterScore) []LetterScore {
	out := make([]LetterScore, len(letters))
	copy(out, letters)
	sort.Slice(out, func(i, j int) bool {
		return principleIndex(out[i].Principle) < principleIndex(out[j].Principle)
	})
	return out
}

func principleIndex(p string) int {
	for i, k := range Principles {
		if k == p {
			return i
		}
	}
	return len(Principles) + 1
}

// LetterByPrinciple lookup.
func LetterByPrinciple(letters []LetterScore, p string) (LetterScore, bool) {
	for _, l := range letters {
		if l.Principle == p {
			return l, true
		}
	}
	return LetterScore{}, false
}

// IsPrincipleValid check.
func IsPrincipleValid(p string) bool {
	for _, k := range Principles {
		if k == p {
			return true
		}
	}
	return false
}

// RenderSnapshot textual.
func RenderSnapshot(s Snapshot) string {
	var sb strings.Builder
	sb.WriteString("Snapshot[")
	sb.WriteString(s.RunID)
	sb.WriteString(" ")
	sb.WriteString(s.Label)
	sb.WriteString(" score=")
	sb.WriteString(ftoa2(s.Letters))
	sb.WriteString("]\n")
	for _, l := range s.Letters {
		marker := "✓"
		if !l.Applicable {
			marker = "N/A"
		}
		sb.WriteString("  ")
		sb.WriteString(l.Principle)
		sb.WriteString(": ")
		sb.WriteString(marker)
		if l.Applicable {
			sb.WriteString(" ")
			sb.WriteString(ftoa1(l.Score))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// RenderDelta textual.
func RenderDelta(d *Delta) string {
	if d == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("Delta[")
	sb.WriteString(d.RunID)
	sb.WriteString("] before=")
	sb.WriteString(ftoa1(d.BeforeScore))
	sb.WriteString(" after=")
	sb.WriteString(ftoa1(d.AfterScore))
	sb.WriteString(" Δ=")
	sb.WriteString(ftoa1(d.ScoreDelta))
	sb.WriteString("\n")
	switch {
	case d.Improved:
		sb.WriteString("  improved ↑\n")
	case d.Regressed:
		sb.WriteString("  regressed ↓\n")
	default:
		sb.WriteString("  unchanged =\n")
	}
	return sb.String()
}

// ftoa1 1 casa decimal.
func ftoa1(f float64) string {
	intPart := int(f)
	frac := int((f - float64(intPart)) * 10)
	if frac < 0 {
		frac = -frac
	}
	out := itoa2(intPart) + "." + itoa2(frac)
	return out
}

// ftoa2 score = média 2 casas.
func ftoa2(letters []LetterScore) string {
	return ftoa1(ComputeScore(letters))
}

func itoa2(n int) string {
	if n == 0 {
		return "0"
	}
	out := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	if neg {
		out = append([]byte{'-'}, out...)
	}
	return string(out)
}
