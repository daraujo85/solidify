// Package coverage — importer de relatórios de coverage.
//
// SAI-032: parsing de LCOV (.info) e Cobertura XML. Output normalizado
// (Report) com percentuais agregados e por arquivo.
package coverage

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Report — coverage normalizado.
type Report struct {
	Format           string         `json:"format"`
	LinePct          float64        `json:"line_pct"`
	BranchPct        float64        `json:"branch_pct"`
	LinesTotal       int            `json:"lines_total"`
	LinesCovered     int            `json:"lines_covered"`
	BranchesTotal    int            `json:"branches_total"`
	BranchesCovered  int            `json:"branches_covered"`
	FunctionsTotal   int            `json:"functions_total"`
	FunctionsCovered int            `json:"functions_covered"`
	Files            []FileCoverage `json:"files"`
}

// FileCoverage — coverage por arquivo.
type FileCoverage struct {
	Path            string  `json:"path"`
	LinePct         float64 `json:"line_pct"`
	BranchPct       float64 `json:"branch_pct"`
	LinesTotal      int     `json:"lines_total"`
	LinesCovered    int     `json:"lines_covered"`
	BranchesTotal   int     `json:"branches_total"`
	BranchesCovered int     `json:"branches_covered"`
}

// Parse detecta formato pelo conteúdo (peek) e parseia.
func Parse(format string, r io.Reader) (Report, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Report{}, fmt.Errorf("coverage: read: %w", err)
	}
	switch strings.ToLower(format) {
	case "lcov":
		return ParseLCOV(bytes.NewReader(data))
	case "cobertura", "xml":
		return ParseCobertura(data)
	case "", "auto":
		return ParseAuto(data)
	default:
		return Report{}, fmt.Errorf("coverage: formato desconhecido: %s", format)
	}
}

// ParseAuto detecta pelo conteúdo.
func ParseAuto(data []byte) (Report, error) {
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("<?xml")) || bytes.HasPrefix(trimmed, []byte("<coverage")) {
		return ParseCobertura(data)
	}
	if bytes.HasPrefix(trimmed, []byte("TN:")) {
		return ParseLCOV(bytes.NewReader(data))
	}
	// Heurística: presença de "SF:" indica LCOV.
	if bytes.Contains(trimmed, []byte("SF:")) {
		return ParseLCOV(bytes.NewReader(data))
	}
	return Report{}, fmt.Errorf("coverage: não detectou formato")
}

// ParseLCOV parseia formato LCOV.
func ParseLCOV(r io.Reader) (Report, error) {
	rep := Report{Format: "lcov"}
	var current *FileCoverage
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "end_of_record" {
			if current != nil {
				if current.LinesTotal > 0 {
					current.LinePct = 100.0 * float64(current.LinesCovered) / float64(current.LinesTotal)
				}
				if current.BranchesTotal > 0 {
					current.BranchPct = 100.0 * 100.0 * float64(current.BranchesCovered) / float64(current.BranchesTotal) / 100.0
				}
				rep.Files = append(rep.Files, *current)
				rep.LinesTotal += current.LinesTotal
				rep.LinesCovered += current.LinesCovered
				rep.BranchesTotal += current.BranchesTotal
				rep.BranchesCovered += current.BranchesCovered
				rep.FunctionsTotal += 0
				rep.FunctionsCovered += 0
				current = nil
			}
			continue
		}
		key, val, ok := splitLCOVLine(line)
		if !ok {
			continue
		}
		switch key {
		case "SF":
			current = &FileCoverage{Path: val}
		case "LF":
			if current != nil {
				current.LinesTotal = atoiSafe(val)
			} else {
				rep.LinesTotal = atoiSafe(val)
			}
		case "LH":
			if current != nil {
				current.LinesCovered = atoiSafe(val)
			} else {
				rep.LinesCovered = atoiSafe(val)
			}
		case "BRF":
			if current != nil {
				current.BranchesTotal = atoiSafe(val)
			} else {
				rep.BranchesTotal = atoiSafe(val)
			}
		case "BRH":
			if current != nil {
				current.BranchesCovered = atoiSafe(val)
			} else {
				rep.BranchesCovered = atoiSafe(val)
			}
		case "FNF":
			if current == nil {
				rep.FunctionsTotal = atoiSafe(val)
			}
		case "FNH":
			if current == nil {
				rep.FunctionsCovered = atoiSafe(val)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return rep, fmt.Errorf("coverage: scan: %w", err)
	}
	if rep.LinesTotal > 0 {
		rep.LinePct = 100.0 * float64(rep.LinesCovered) / float64(rep.LinesTotal)
	}
	if rep.BranchesTotal > 0 {
		rep.BranchPct = 100.0 * float64(rep.BranchesCovered) / float64(rep.BranchesTotal)
	}
	return rep, nil
}

// splitLCOVLine divide "SF:path" em ("SF", "path").
func splitLCOVLine(line string) (string, string, bool) {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return "", "", false
	}
	return line[:idx], line[idx+1:], true
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// Cobertura XML.

type coberturaRoot struct {
	XMLName         xml.Name     `xml:"coverage"`
	LineRate        float64      `xml:"line-rate,attr"`
	BranchRate      float64      `xml:"branch-rate,attr"`
	LinesCovered    int          `xml:"lines-covered,attr"`
	LinesValid      int          `xml:"lines-valid,attr"`
	BranchesCovered int          `xml:"branches-covered,attr"`
	BranchesValid   int          `xml:"branches-valid,attr"`
	Packages        []coberturaP `xml:"packages>package"`
}

type coberturaP struct {
	Name    string       `xml:"name,attr"`
	Classes []coberturaC `xml:"classes>class"`
}

type coberturaC struct {
	Filename   string       `xml:"filename,attr"`
	LineRate   float64      `xml:"line-rate,attr"`
	BranchRate float64      `xml:"branch-rate,attr"`
	Lines      []coberturaL `xml:"lines>line"`
}

type coberturaL struct {
	Number int `xml:"number,attr"`
	Hits   int `xml:"hits,attr"`
}

// ParseCobertura parseia XML Cobertura.
func ParseCobertura(data []byte) (Report, error) {
	var root coberturaRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return Report{}, fmt.Errorf("coverage: xml: %w", err)
	}
	rep := Report{Format: "cobertura"}
	rep.LinesCovered = root.LinesCovered
	rep.LinesTotal = root.LinesValid
	rep.BranchesCovered = root.BranchesCovered
	rep.BranchesTotal = root.BranchesValid
	rep.LinePct = root.LineRate * 100
	rep.BranchPct = root.BranchRate * 100
	for _, p := range root.Packages {
		for _, c := range p.Classes {
			// Se a class tem lines detalhadas, calcula covered.
			total := len(c.Lines)
			covered := 0
			for _, l := range c.Lines {
				if l.Hits > 0 {
					covered++
				}
			}
			fc := FileCoverage{
				Path:         c.Filename,
				LinesTotal:   total,
				LinesCovered: covered,
			}
			if total > 0 {
				fc.LinePct = 100.0 * float64(covered) / float64(total)
			}
			if c.LineRate > 0 {
				fc.LinePct = c.LineRate * 100
			}
			rep.Files = append(rep.Files, fc)
		}
	}
	return rep, nil
}

// Merge combina múltiplos reports (soma counts).
func Merge(reports []Report) Report {
	out := Report{}
	for _, r := range reports {
		out.LinesTotal += r.LinesTotal
		out.LinesCovered += r.LinesCovered
		out.BranchesTotal += r.BranchesTotal
		out.BranchesCovered += r.BranchesCovered
		out.FunctionsTotal += r.FunctionsTotal
		out.FunctionsCovered += r.FunctionsCovered
		out.Files = append(out.Files, r.Files...)
	}
	if out.LinesTotal > 0 {
		out.LinePct = 100.0 * float64(out.LinesCovered) / float64(out.LinesTotal)
	}
	if out.BranchesTotal > 0 {
		out.BranchPct = 100.0 * float64(out.BranchesCovered) / float64(out.BranchesTotal)
	}
	return out
}

// IsGood devolve true se LinePct >= threshold.
func (r Report) IsGood(threshold float64) bool {
	return r.LinePct >= threshold
}

// WorstFiles devolve top N piores arquivos por LinePct.
func (r Report) WorstFiles(n int) []FileCoverage {
	if n <= 0 || len(r.Files) == 0 {
		return nil
	}
	files := make([]FileCoverage, len(r.Files))
	copy(files, r.Files)
	// Selection sort (small n).
	for i := 0; i < n && i < len(files); i++ {
		minIdx := i
		for j := i + 1; j < len(files); j++ {
			if files[j].LinePct < files[minIdx].LinePct {
				minIdx = j
			}
		}
		files[i], files[minIdx] = files[minIdx], files[i]
	}
	if n > len(files) {
		n = len(files)
	}
	return files[:n]
}
