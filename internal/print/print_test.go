package print

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Aceitação: CommonCandidates não-vazio.
func TestCommonCandidates(t *testing.T) {
	cands := CommonCandidates()
	if len(cands) == 0 {
		t.Errorf("vazio")
	}
	for _, c := range cands {
		if c.Path == "" || c.Name == "" {
			t.Errorf("candidate inválido: %+v", c)
		}
		if len(c.Args) == 0 {
			t.Errorf("args vazio: %s", c.Name)
		}
	}
}

// Aceitação: HasBrowser sem panic.
func TestHasBrowser(t *testing.T) {
	_ = HasBrowser()
}

// Aceitação: ExtractPDFText em não-PDF.
func TestExtractPDFTextNotPDF(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fake.pdf")
	os.WriteFile(p, []byte("not a pdf"), 0644)
	if _, err := ExtractPDFText(p); err == nil {
		t.Errorf("esperado erro")
	}
}

// Aceitação: ExtractPDFText path vazio.
func TestExtractPDFTextEmptyPath(t *testing.T) {
	if _, err := ExtractPDFText(""); err == nil {
		t.Errorf("path vazio")
	}
}

// Aceitação: ExtractPDFText básico em PDF fake.
func TestExtractPDFTextBasic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fake.pdf")
	// conteúdo simulado: PDF header + texto entre ().
	content := "%PDF-1.4\n(Hello World)Tj\n(Second Line)Tj\n%%EOF"
	os.WriteFile(p, []byte(content), 0644)

	// Fixture simples não é PDF válido pro Poppler; garante fallback zero-deps.
	original := runPDFToText
	runPDFToText = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { runPDFToText = original })

	txt, err := ExtractPDFText(p)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !strings.Contains(txt, "Hello World") {
		t.Errorf("texto: %s", txt)
	}
}

// Aceitação: extrator prefere pdftotext, que decodifica /ToUnicode CMap
// de PDFs Chrome com fontes subsetizadas; o scanner zero-deps não consegue.
func TestExtractPDFTextUsesPDFToText(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "chrome.pdf")
	os.WriteFile(p, []byte("%PDF-1.7\n"), 0644)

	original := runPDFToText
	runPDFToText = func(got string) ([]byte, error) {
		if got != p {
			t.Fatalf("path: %q", got)
		}
		return []byte("Quality gate: PASS\nrun-123\n"), nil
	}
	t.Cleanup(func() { runPDFToText = original })

	txt, err := ExtractPDFText(p)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !strings.Contains(txt, "Quality gate: PASS") {
		t.Errorf("texto: %q", txt)
	}
}

// Aceitação: ValidatePDF detecta seções faltando.
func TestValidatePDFMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fake.pdf")
	content := "%PDF-1.4\n(Hello)Tj\n%%EOF"
	os.WriteFile(p, []byte(content), 0644)
	res, err := ValidatePDF(p, DefaultRequiredSections)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if res.OK {
		t.Errorf("esperado missing")
	}
	if len(res.Missing) == 0 {
		t.Errorf("sem missing list")
	}
}

// Aceitação: ValidatePDF passa c/ markers.
func TestValidatePDFOK(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fake.pdf")
	content := "%PDF-1.4\n(Quality gate: PASS)Tj\n(Score: 85)Tj\n(SOLID ok)Tj\n(Risk LOW)Tj\n(run r1)Tj\n%%EOF"
	os.WriteFile(p, []byte(content), 0644)
	res, err := ValidatePDF(p, DefaultRequiredSections)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !res.OK {
		t.Errorf("esperado ok: %+v", res)
	}
}

// Aceitação: GeneratePDF sem source.
func TestGeneratePDFNoSource(t *testing.T) {
	if err := GeneratePDF(PDFOptions{Output: "/tmp/x.pdf"}); err == nil {
		t.Errorf("sem source")
	}
}

// Aceitação: GeneratePDF sem output.
func TestGeneratePDFNoOutput(t *testing.T) {
	if err := GeneratePDF(PDFOptions{URL: "http://x"}); err == nil {
		t.Errorf("sem output")
	}
}

// Aceitação: IsPDF detecta magic bytes.
func TestIsPDFMagic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.pdf")
	os.WriteFile(p, []byte("%PDF-1.4"), 0644)
	ok, err := IsPDF(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Errorf("magic")
	}
}

// Aceitação: IsPDF path vazio.
func TestIsPDFEmpty(t *testing.T) {
	if _, err := IsPDF(""); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: IsPDF file missing.
func TestIsPDFMissing(t *testing.T) {
	if _, err := IsPDF("/nonexistent/x.pdf"); err == nil {
		t.Errorf("missing")
	}
}

// Aceitação: PDFSize.
func TestPDFSize(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.pdf")
	os.WriteFile(p, []byte("hello"), 0644)
	sz, err := PDFSize(p)
	if err != nil {
		t.Fatalf("size: %v", err)
	}
	if sz != 5 {
		t.Errorf("size: %d", sz)
	}
	if _, err := PDFSize(""); err == nil {
		t.Errorf("vazio")
	}
}

// Aceitação: printable helper.
func TestPrintable(t *testing.T) {
	if !printable("hello") {
		t.Errorf("hello")
	}
	if printable("\x00\x01\x02") {
		t.Errorf("binary")
	}
	if printable("") {
		t.Errorf("vazio")
	}
}

// Aceitação: WithTimeout helper.
func TestWithTimeout(t *testing.T) {
	ctx, cancel := WithTimeout(nil, 0)
	defer cancel()
	if ctx == nil {
		t.Errorf("nil ctx")
	}
}
