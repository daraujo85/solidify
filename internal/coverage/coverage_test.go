package coverage

import (
	"strings"
	"testing"
)

// Aceitação: ParseLCOV básico.
func TestParseLCOVBasic(t *testing.T) {
	lcov := `TN:
SF:foo.go
LF:10
LH:7
BRF:4
BRH:2
end_of_record
SF:bar.go
LF:5
LH:5
BRF:0
BRH:0
end_of_record
`
	rep, err := ParseLCOV(strings.NewReader(lcov))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rep.LinesTotal != 15 || rep.LinesCovered != 12 {
		t.Errorf("lines = %d/%d", rep.LinesCovered, rep.LinesTotal)
	}
	if rep.LinePct != 80.0 {
		t.Errorf("LinePct = %v", rep.LinePct)
	}
	if len(rep.Files) != 2 {
		t.Errorf("files = %d", len(rep.Files))
	}
}

// Aceitação: ParseLCOV vazio.
func TestParseLCOVEmpty(t *testing.T) {
	rep, err := ParseLCOV(strings.NewReader(""))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rep.LinesTotal != 0 {
		t.Errorf("lines = %d", rep.LinesTotal)
	}
}

// Aceitação: ParseLCOV com branches.
func TestParseLCOVBranches(t *testing.T) {
	lcov := `TN:
SF:foo.go
LF:10
LH:10
BRF:4
BRH:3
end_of_record
`
	rep, _ := ParseLCOV(strings.NewReader(lcov))
	if rep.BranchPct != 75.0 {
		t.Errorf("BranchPct = %v", rep.BranchPct)
	}
}

// Aceitação: ParseLCOV file LinePct.
func TestParseLCOVFileLinePct(t *testing.T) {
	lcov := `SF:foo.go
LF:4
LH:3
end_of_record
`
	rep, _ := ParseLCOV(strings.NewReader(lcov))
	if rep.Files[0].LinePct != 75.0 {
		t.Errorf("file LinePct = %v", rep.Files[0].LinePct)
	}
}

// Aceitação: ParseCobertura básico.
func TestParseCoberturaBasic(t *testing.T) {
	xml := `<?xml version="1.0"?>
<coverage line-rate="0.85" branch-rate="0.7" lines-covered="100" lines-valid="120" branches-covered="20" branches-valid="30">
  <packages>
    <package name="x">
      <classes>
        <class filename="foo.go" line-rate="0.9" branch-rate="0.8">
          <lines>
            <line number="1" hits="1"/>
            <line number="2" hits="0"/>
            <line number="3" hits="1"/>
          </lines>
        </class>
      </classes>
    </package>
  </packages>
</coverage>`
	rep, err := ParseCobertura([]byte(xml))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rep.LinePct != 85.0 {
		t.Errorf("LinePct = %v", rep.LinePct)
	}
	if rep.LinesTotal != 120 || rep.LinesCovered != 100 {
		t.Errorf("lines = %d/%d", rep.LinesCovered, rep.LinesTotal)
	}
	if len(rep.Files) != 1 || rep.Files[0].Path != "foo.go" {
		t.Errorf("files = %+v", rep.Files)
	}
}

// Aceitação: ParseCobertura inválido.
func TestParseCoberturaInvalid(t *testing.T) {
	if _, err := ParseCobertura([]byte("not xml")); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: ParseAuto detecta LCOV.
func TestParseAutoLCOV(t *testing.T) {
	lcov := `TN:
SF:foo.go
LF:1
LH:1
end_of_record
`
	rep, err := ParseAuto([]byte(lcov))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rep.Format != "lcov" {
		t.Errorf("format = %s", rep.Format)
	}
}

// Aceitação: ParseAuto detecta Cobertura.
func TestParseAutoCobertura(t *testing.T) {
	xml := `<?xml version="1.0"?>
<coverage line-rate="0.5" branch-rate="0.5" lines-covered="1" lines-valid="2" branches-covered="0" branches-valid="0"/>`
	rep, _ := ParseAuto([]byte(xml))
	if rep.Format != "cobertura" {
		t.Errorf("format = %s", rep.Format)
	}
}

// Aceitação: ParseAuto desconhecido.
func TestParseAutoUnknown(t *testing.T) {
	if _, err := ParseAuto([]byte("garbage data")); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Parse por formato.
func TestParseFormat(t *testing.T) {
	cases := []struct {
		format, content string
		wantFormat      string
		wantErr         bool
	}{
		{"lcov", `TN:`, "lcov", false},
		{"cobertura", `<?xml version="1.0"?><coverage line-rate="0.5" branch-rate="0.5" lines-covered="1" lines-valid="2"/>`, "cobertura", false},
		{"xml", `<?xml version="1.0"?><coverage line-rate="0.5" branch-rate="0.5" lines-covered="1" lines-valid="2"/>`, "cobertura", false},
		{"unknown", "x", "", true},
		{"", `TN:`, "lcov", false},
	}
	for _, c := range cases {
		rep, err := Parse(c.format, strings.NewReader(c.content))
		if c.wantErr {
			if err == nil {
				t.Errorf("%s devia falhar", c.format)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s err: %v", c.format, err)
			continue
		}
		if rep.Format != c.wantFormat {
			t.Errorf("%s got format = %s", c.format, rep.Format)
		}
	}
}

// Aceitação: Merge.
func TestMerge(t *testing.T) {
	r1 := Report{LinesTotal: 10, LinesCovered: 8}
	r2 := Report{LinesTotal: 20, LinesCovered: 15}
	m := Merge([]Report{r1, r2})
	if m.LinesTotal != 30 || m.LinesCovered != 23 {
		t.Errorf("merge = %+v", m)
	}
	// 23/30 = 76.66%
	if m.LinePct < 76 || m.LinePct > 77 {
		t.Errorf("LinePct = %v", m.LinePct)
	}
}

// Aceitação: Merge empty.
func TestMergeEmpty(t *testing.T) {
	m := Merge(nil)
	if m.LinesTotal != 0 {
		t.Errorf("err: %+v", m)
	}
}

// Aceitação: IsGood.
func TestIsGood(t *testing.T) {
	if !(Report{LinePct: 80}).IsGood(70) {
		t.Errorf("devia ser true")
	}
	if (Report{LinePct: 60}).IsGood(70) {
		t.Errorf("devia ser false")
	}
	if !(Report{LinePct: 70}).IsGood(70) {
		t.Errorf("devia ser true (>=)")
	}
}

// Aceitação: WorstFiles.
func TestWorstFiles(t *testing.T) {
	r := Report{
		Files: []FileCoverage{
			{Path: "a.go", LinePct: 80},
			{Path: "b.go", LinePct: 30},
			{Path: "c.go", LinePct: 50},
			{Path: "d.go", LinePct: 10},
		},
	}
	worst := r.WorstFiles(2)
	if len(worst) != 2 {
		t.Fatalf("len = %d", len(worst))
	}
	if worst[0].Path != "d.go" || worst[1].Path != "b.go" {
		t.Errorf("order: %+v", worst)
	}
}

// Aceitação: WorstFiles n=0.
func TestWorstFilesNZero(t *testing.T) {
	r := Report{}
	if got := r.WorstFiles(0); got != nil {
		t.Errorf("err: %+v", got)
	}
}

// Aceitação: WorstFiles n > len.
func TestWorstFilesTooMany(t *testing.T) {
	r := Report{Files: []FileCoverage{{Path: "a", LinePct: 50}}}
	worst := r.WorstFiles(10)
	if len(worst) != 1 {
		t.Errorf("len = %d", len(worst))
	}
}

// Aceitação: splitLCOVLine.
func TestSplitLCOVLine(t *testing.T) {
	k, v, ok := splitLCOVLine("SF:foo.go")
	if !ok || k != "SF" || v != "foo.go" {
		t.Errorf("got = %q,%q,%v", k, v, ok)
	}
	_, _, ok = splitLCOVLine("invalid")
	if ok {
		t.Errorf("devia falhar")
	}
}

// Aceitação: atoiSafe.
func TestAtoiSafe(t *testing.T) {
	if atoiSafe("42") != 42 {
		t.Errorf("err")
	}
	if atoiSafe("abc") != 0 {
		t.Errorf("err")
	}
	if atoiSafe("  10  ") != 10 {
		t.Errorf("err")
	}
}

// Aceitação: ParseCobertura branch pct.
func TestParseCoberturaBranch(t *testing.T) {
	xml := `<?xml version="1.0"?>
<coverage line-rate="1.0" branch-rate="0.5" lines-covered="10" lines-valid="10" branches-covered="5" branches-valid="10"/>`
	rep, _ := ParseCobertura([]byte(xml))
	if rep.BranchPct != 50.0 {
		t.Errorf("BranchPct = %v", rep.BranchPct)
	}
}

// Aceitação: ParseLCOV sem end_of_record.
func TestParseLCOVNoEndOfRecord(t *testing.T) {
	lcov := `TN:
SF:foo.go
LF:10
LH:7
`
	rep, _ := ParseLCOV(strings.NewReader(lcov))
	// Sem end_of_record, file não é commitado.
	if rep.LinesTotal != 0 {
		t.Errorf("lines = %d (esperado 0 sem end_of_record)", rep.LinesTotal)
	}
}

// Aceitação: Merge com files.
func TestMergeFiles(t *testing.T) {
	r1 := Report{Files: []FileCoverage{{Path: "a.go", LinesTotal: 10, LinesCovered: 8}}}
	r2 := Report{Files: []FileCoverage{{Path: "b.go", LinesTotal: 20, LinesCovered: 15}}}
	m := Merge([]Report{r1, r2})
	if len(m.Files) != 2 {
		t.Errorf("files = %d", len(m.Files))
	}
}
