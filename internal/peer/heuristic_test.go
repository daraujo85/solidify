// Testes de SAI-129B: heurística determinística de applicability.
package peer

import (
	"strings"
	"testing"

	"github.com/diegoaraujo/solidify/internal/gitx"
)

func hunk(path, header, content string) gitx.Hunk {
	return gitx.Hunk{FilePath: path, Header: header, Content: content}
}

func diffFile(path string, status gitx.ChangeKind, hunks ...gitx.Hunk) gitx.DiffFile {
	return gitx.DiffFile{Path: path, Status: status, Hunks: hunks}
}

// TestClassifyApplicability_GoldenRenameAllNotApplicable regride o caso de
// referência do plano (projeto real, HEAD~1..HEAD): rename de 1 linha
// (variável), sem mudança estrutural. Os 5 princípios devem vir
// CLEARLY_NOT_APPLICABLE, e AllNotApplicable() deve reportar skip total.
func TestClassifyApplicability_GoldenRenameAllNotApplicable(t *testing.T) {
	files := []gitx.DiffFile{
		diffFile("internal/service/tms.go", gitx.ChangeModified,
			hunk("internal/service/tms.go", "@@ -42,7 +42,7 @@ func Process() {",
				" func Process() {\n-\tresult := compute(oldName)\n+\tresult := compute(newName)\n \treturn result\n }"),
		),
	}
	hints := ClassifyApplicability(files)
	for _, p := range principleOrder {
		if hints[p].Hint != HeuristicNotApplicable {
			t.Errorf("principio %s: esperado CLEARLY_NOT_APPLICABLE, got %s (%s)", p, hints[p].Hint, hints[p].Reason)
		}
	}
	if !AllNotApplicable(hints) {
		t.Fatal("AllNotApplicable() deveria ser true pro golden case do rename")
	}
}

// TestClassifyApplicability_FalseNegativeGuard é o teste de falso-negativo
// exigido pelo usuário: um diff de 1 linha que MUDA COMPORTAMENTO (limite de
// condição, > vira >=) não pode ser classificado como trivial só por ter o
// mesmo tamanho/formato superficial de um rename.
func TestClassifyApplicability_FalseNegativeGuard(t *testing.T) {
	files := []gitx.DiffFile{
		diffFile("internal/service/tms.go", gitx.ChangeModified,
			hunk("internal/service/tms.go", "@@ -10,7 +10,7 @@ func Check(x int) bool {",
				" func Check(x int) bool {\n-\tif x > 0 {\n+\tif x >= 0 {\n \t\treturn true\n \t}"),
		),
	}
	hints := ClassifyApplicability(files)
	if AllNotApplicable(hints) {
		t.Fatal("mudança de operador (>  vira >=) não pode ser tratada como all-trivial — falso-negativo")
	}
	// O princípio mais diretamente afetado (comportamento condicional) não
	// pode vir CLEARLY_NOT_APPLICABLE.
	if hints["O"].Hint == HeuristicNotApplicable {
		t.Errorf("O não pode ser CLEARLY_NOT_APPLICABLE numa mudança de condição de contorno, got reason=%q", hints["O"].Reason)
	}
}

func TestSkeletonEqual_IdentifierRenameOnly(t *testing.T) {
	if !skeletonEqual("\tresult := compute(oldName)", "\tresult := compute(newName)") {
		t.Fatal("rename de identificador deveria produzir esqueletos iguais")
	}
}

func TestSkeletonEqual_OperatorChangeDiffers(t *testing.T) {
	if skeletonEqual("if x > 0 {", "if x >= 0 {") {
		t.Fatal("mudança de operador (> vs >=) não pode ter esqueleto igual")
	}
}

func TestClassifyApplicability_NewImportSignalsD(t *testing.T) {
	files := []gitx.DiffFile{
		diffFile("internal/service/tms.go", gitx.ChangeModified,
			hunk("internal/service/tms.go", "@@ -1,4 +1,5 @@",
				" import (\n \t\"fmt\"\n+\t\"github.com/diegoaraujo/solidify/internal/newthing\"\n )"),
		),
	}
	hints := ClassifyApplicability(files)
	if hints["D"].Hint != HeuristicApplicable {
		t.Errorf("D esperado CLEARLY_APPLICABLE com novo import, got %s", hints["D"].Hint)
	}
}

func TestClassifyApplicability_NewPublicSymbolSignalsIAndL(t *testing.T) {
	files := []gitx.DiffFile{
		diffFile("internal/service/tms.go", gitx.ChangeModified,
			hunk("internal/service/tms.go", "@@ -10,0 +11,3 @@",
				"+type Exporter interface {\n+\tExport() error\n+}"),
		),
	}
	hints := ClassifyApplicability(files)
	if hints["I"].Hint != HeuristicApplicable {
		t.Errorf("I esperado CLEARLY_APPLICABLE com novo símbolo público, got %s", hints["I"].Hint)
	}
	if hints["L"].Hint != HeuristicApplicable {
		t.Errorf("L esperado CLEARLY_APPLICABLE com novo símbolo público, got %s", hints["L"].Hint)
	}
}

func TestClassifyApplicability_NewConditionalSignalsO(t *testing.T) {
	files := []gitx.DiffFile{
		diffFile("internal/service/tms.go", gitx.ChangeModified,
			hunk("internal/service/tms.go", "@@ -20,3 +20,6 @@",
				" func Handle(kind string) {\n+\tif kind == \"special\" {\n+\t\tdoSpecial()\n+\t}\n }"),
		),
	}
	hints := ClassifyApplicability(files)
	if hints["O"].Hint != HeuristicApplicable {
		t.Errorf("O esperado CLEARLY_APPLICABLE com novo branch condicional, got %s", hints["O"].Hint)
	}
}

func TestClassifyApplicability_MultiDomainImportsSignalsS(t *testing.T) {
	files := []gitx.DiffFile{
		diffFile("internal/service/tms.go", gitx.ChangeModified,
			hunk("internal/service/tms.go", "@@ -1,4 +1,6 @@",
				" import (\n \t\"fmt\"\n+\t\"github.com/diegoaraujo/solidify/internal/billing\"\n+\t\"github.com/diegoaraujo/solidify/internal/notification\"\n )"),
		),
	}
	hints := ClassifyApplicability(files)
	if hints["S"].Hint != HeuristicApplicable {
		t.Errorf("S esperado CLEARLY_APPLICABLE com múltiplos domínios de import, got %s", hints["S"].Hint)
	}
}

func TestClassifyApplicability_CommentOnlyDiffIsTrivial(t *testing.T) {
	files := []gitx.DiffFile{
		diffFile("internal/service/tms.go", gitx.ChangeModified,
			hunk("internal/service/tms.go", "@@ -5,1 +5,1 @@",
				"-// old comment\n+// new comment explaining the same thing"),
		),
	}
	hints := ClassifyApplicability(files)
	if !AllNotApplicable(hints) {
		t.Fatal("diff só de comentário deveria ser all-trivial")
	}
}

func TestClassifyApplicability_TestOnlyFileMarksOLIButNotSD(t *testing.T) {
	files := []gitx.DiffFile{
		diffFile("internal/service/tms_test.go", gitx.ChangeModified,
			hunk("internal/service/tms_test.go", "@@ -1,3 +1,6 @@",
				" func TestFoo(t *testing.T) {\n+\tif true {\n+\t\tt.Fatal(\"x\")\n+\t}\n }"),
		),
	}
	hints := ClassifyApplicability(files)
	for _, p := range []string{"O", "L", "I"} {
		if hints[p].Hint != HeuristicNotApplicable {
			t.Errorf("%s esperado CLEARLY_NOT_APPLICABLE em diff restrito a teste, got %s", p, hints[p].Hint)
		}
	}
	if hints["S"].Hint == HeuristicNotApplicable || hints["D"].Hint == HeuristicNotApplicable {
		t.Error("S/D não devem ser forçados N/A pela regra de arquivo-de-teste (ficam ambíguos)")
	}
}

func TestClassifyApplicability_EmptyDiffAllAmbiguous(t *testing.T) {
	hints := ClassifyApplicability(nil)
	for _, p := range principleOrder {
		if hints[p].Hint != HeuristicAmbiguous {
			t.Errorf("%s: diff vazio deveria ser AMBIGUOUS (default seguro), got %s", p, hints[p].Hint)
		}
	}
	if AllNotApplicable(hints) {
		t.Fatal("diff vazio nunca deveria virar AllNotApplicable")
	}
}

func TestAllNotApplicable_PartialFalse(t *testing.T) {
	hints := defaultAmbiguousHints()
	hints["S"] = PrincipleHint{Principle: "S", Hint: HeuristicNotApplicable}
	if AllNotApplicable(hints) {
		t.Fatal("4/5 ambíguo + 1/5 N/A não pode ser AllNotApplicable")
	}
}

func TestSynthesizeHeuristicResult_ScoreStatusNotApplicable(t *testing.T) {
	hints := ClassifyApplicability([]gitx.DiffFile{
		diffFile("x.go", gitx.ChangeRenamed),
	})
	res := SynthesizeHeuristicResult("peer_a", "heuristic", "heuristic", hints)
	if res.ScoreStatus != SolidScoreNotApplicable {
		t.Errorf("ScoreStatus esperado %s, got %s", SolidScoreNotApplicable, res.ScoreStatus)
	}
	if res.QualityScore != 0 {
		t.Errorf("QualityScore esperado 0 (telemetria only), got %v", res.QualityScore)
	}
	solid, _ := res.ParsedContent["solid"].(map[string]any)
	for _, p := range principleOrder {
		node, _ := solid[p].(map[string]any)
		if node["applicability"] != ApplicabilityNotApplicable {
			t.Errorf("solid.%s.applicability esperado NOT_APPLICABLE, got %v", p, node["applicability"])
		}
		if node["score"] != nil {
			t.Errorf("solid.%s.score esperado nil, got %v", p, node["score"])
		}
	}
}

func TestFormatHeuristicHints_NonEmpty(t *testing.T) {
	hints := defaultAmbiguousHints()
	out := FormatHeuristicHints(hints)
	if !strings.Contains(out, "AMBIGUOUS") {
		t.Fatal("FormatHeuristicHints deveria mencionar os hints")
	}
}
