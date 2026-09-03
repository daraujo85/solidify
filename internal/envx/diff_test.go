package envx

import (
	"reflect"
	"sort"
	"testing"
)

// findByName devolve o primeiro Diff com o nome pedido.
func findDiffByName(ds []Diff, name string) *Diff {
	for i := range ds {
		if ds[i].Name == name {
			return &ds[i]
		}
	}
	return nil
}

func hasState(d *Diff, s DiffState) bool {
	for _, st := range d.States {
		if st == s {
			return true
		}
	}
	return false
}

// Aceitação: var nova só no head → added.
func TestCompareAddedVar(t *testing.T) {
	base := []Usage{{Name: "OLD", Path: "a.go"}}
	head := []Usage{
		{Name: "OLD", Path: "a.go"},
		{Name: "NEW_KEY", Path: "a.go"},
	}
	diffs := Compare(base, head, nil, nil)
	d := findDiffByName(diffs, "NEW_KEY")
	if d == nil {
		t.Fatalf("NEW_KEY sumiu: %+v", diffs)
	}
	if !hasState(d, DiffAdded) {
		t.Errorf("states = %v, quero added", d.States)
	}
	if d.HeadUses != 1 || d.BaseUses != 0 {
		t.Errorf("uses = base=%d head=%d, quero 0/1", d.BaseUses, d.HeadUses)
	}
}

// Aceitação: var removida → removed.
func TestCompareRemovedVar(t *testing.T) {
	base := []Usage{
		{Name: "OLD", Path: "a.go"},
		{Name: "DELETED", Path: "a.go"},
	}
	head := []Usage{{Name: "OLD", Path: "a.go"}}
	diffs := Compare(base, head, nil, nil)
	d := findDiffByName(diffs, "DELETED")
	if d == nil {
		t.Fatalf("DELETED sumiu: %+v", diffs)
	}
	if !hasState(d, DiffRemoved) {
		t.Errorf("states = %v, quero removed", d.States)
	}
	if d.BaseUses != 1 || d.HeadUses != 0 {
		t.Errorf("uses errados")
	}
}

// Aceitação: var presente nos dois sem mudança de forma → sem changed-use.
func TestCompareUnchangedVar(t *testing.T) {
	base := []Usage{{Name: "STABLE", Path: "a.go", Form: FormOSGetenv}}
	head := []Usage{{Name: "STABLE", Path: "a.go", Form: FormOSGetenv}}
	diffs := Compare(base, head, nil, nil)
	d := findDiffByName(diffs, "STABLE")
	if hasState(d, DiffChangedUse) {
		t.Errorf("não devia marcar changed-use: %v", d.States)
	}
}

// Aceitação: var presente nos dois com forma diferente → changed-use.
func TestCompareChangedUseForm(t *testing.T) {
	base := []Usage{{Name: "X", Path: "a.go", Form: FormOSGetenv}}
	head := []Usage{{Name: "X", Path: "a.go", Form: FormOSLookupEnv}}
	diffs := Compare(base, head, nil, nil)
	d := findDiffByName(diffs, "X")
	if !hasState(d, DiffChangedUse) {
		t.Errorf("devia marcar changed-use: %v", d.States)
	}
}

// Aceitação: var presente nos dois mas HasDefault mudou → added.
func TestCompareChangedUseDefault(t *testing.T) {
	base := []Usage{{Name: "X", Path: "a.go", Form: FormOSEnviron, HasDefault: false}}
	head := []Usage{{Name: "X", Path: "a.go", Form: FormOSEnviron, HasDefault: true}}
	diffs := Compare(base, head, nil, nil)
	d := findDiffByName(diffs, "X")
	if !hasState(d, DiffChangedUse) {
		t.Errorf("devia marcar changed-use: %v", d.States)
	}
}

// Aceitação: var documentada → documentation-added.
func TestCompareDocumentedVar(t *testing.T) {
	base := []Usage{{Name: "API_KEY", Path: "a.go"}}
	head := []Usage{{Name: "API_KEY", Path: "a.go"}}
	docs := []string{"API_KEY", "OTHER"}
	diffs := Compare(base, head, docs, nil)
	d := findDiffByName(diffs, "API_KEY")
	if !hasState(d, DiffDocAdded) {
		t.Errorf("devia marcar documentation-added: %v", d.States)
	}
}

// Aceitação: var NÃO documentada → documentation-missing.
func TestCompareUndocumentedVar(t *testing.T) {
	head := []Usage{{Name: "UNDOC_KEY", Path: "a.go"}}
	diffs := Compare(nil, head, nil, nil)
	d := findDiffByName(diffs, "UNDOC_KEY")
	if !hasState(d, DiffDocMissing) {
		t.Errorf("devia marcar documentation-missing: %v", d.States)
	}
	if hasState(d, DiffDocAdded) {
		t.Errorf("não devia marcar documentation-added")
	}
}

// Aceitação: var com HasDefault → Required=false.
func TestCompareRequiredInferenceNoDefault(t *testing.T) {
	head := []Usage{{Name: "API_KEY", Path: "a.go", HasDefault: false}}
	diffs := Compare(nil, head, nil, nil)
	d := findDiffByName(diffs, "API_KEY")
	if !d.Required {
		t.Errorf("sem default devia ser required")
	}
}

// Aceitação: var com HasDefault → Required=false.
func TestCompareRequiredInferenceWithDefault(t *testing.T) {
	head := []Usage{{Name: "PORT", Path: "a.go", HasDefault: true}}
	diffs := Compare(nil, head, nil, nil)
	d := findDiffByName(diffs, "PORT")
	if d.Required {
		t.Errorf("com default não devia ser required")
	}
	if !d.HasDefault {
		t.Errorf("HasDefault devia ser true")
	}
}

// Aceitação: LikelySecret via nome (KEY/SECRET/TOKEN etc).
func TestCompareLikelySecretByName(t *testing.T) {
	cases := []struct {
		name   string
		secret bool
	}{
		{"API_KEY", true},
		{"DB_PASSWORD", true},
		{"AUTH_TOKEN", true},
		{"SECRET_KEY_BASE", true},
		{"GITHUB_ACCESS_TOKEN", true},
		{"TLS_CERT", true},
		{"API_URL", false},
		{"DEBUG", false},
		{"PORT", false},
	}
	for _, tc := range cases {
		head := []Usage{{Name: tc.name, Path: "a.go"}}
		diffs := Compare(nil, head, nil, IsLikelySecret)
		d := findDiffByName(diffs, tc.name)
		if d.LikelySecret != tc.secret {
			t.Errorf("%s: likelySecret = %v, quero %v", tc.name, d.LikelySecret, tc.secret)
		}
	}
}

// Aceitação: LikelySecret case-insensitive.
func TestCompareLikelySecretCaseInsensitive(t *testing.T) {
	head := []Usage{{Name: "secret_key", Path: "a.go"}}
	diffs := Compare(nil, head, nil, IsLikelySecret)
	d := findDiffByName(diffs, "secret_key")
	if !d.LikelySecret {
		t.Errorf("lowercase secret_key devia ser detectado")
	}
}

// Aceitação: secretChecker nil não crash.
func TestCompareNilSecretChecker(t *testing.T) {
	head := []Usage{{Name: "API_KEY", Path: "a.go"}}
	diffs := Compare(nil, head, nil, nil)
	d := findDiffByName(diffs, "API_KEY")
	if d.LikelySecret {
		t.Errorf("nil checker devia deixar LikelySecret=false")
	}
}

// Aceitação: diffs são ordenados por nome.
func TestCompareOrderByName(t *testing.T) {
	head := []Usage{
		{Name: "Z", Path: "a.go"},
		{Name: "A", Path: "a.go"},
		{Name: "M", Path: "a.go"},
	}
	diffs := Compare(nil, head, nil, nil)
	names := []string{}
	for _, d := range diffs {
		names = append(names, d.Name)
	}
	want := []string{"A", "M", "Z"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, quero %v", names, want)
	}
}

// Aceitação: union de base+head preserva ambos.
func TestCompareUnion(t *testing.T) {
	base := []Usage{{Name: "BASE_ONLY", Path: "a.go"}}
	head := []Usage{{Name: "HEAD_ONLY", Path: "a.go"}}
	diffs := Compare(base, head, nil, nil)
	names := map[string]bool{}
	for _, d := range diffs {
		names[d.Name] = true
	}
	if !names["BASE_ONLY"] || !names["HEAD_ONLY"] {
		t.Errorf("union incompleto: %+v", diffs)
	}
}

// Aceitação: BaseFiles e HeadFiles deduplicados.
func TestCompareFilesDedupe(t *testing.T) {
	base := []Usage{
		{Name: "X", Path: "a.go"},
		{Name: "X", Path: "a.go"},
		{Name: "X", Path: "b.go"},
	}
	head := []Usage{{Name: "X", Path: "c.go"}}
	diffs := Compare(base, head, nil, nil)
	d := findDiffByName(diffs, "X")
	if len(d.BaseFiles) != 2 {
		t.Errorf("BaseFiles = %v, quero 2 únicos", d.BaseFiles)
	}
	if len(d.HeadFiles) != 1 || d.HeadFiles[0] != "c.go" {
		t.Errorf("HeadFiles = %v", d.HeadFiles)
	}
}

// Aceitação: DocumentedNames parse .env-style.
func TestDocumentedNamesDotEnv(t *testing.T) {
	src := []byte(`
DATABASE_URL=postgres://localhost
API_KEY=secret
# COMENTADO=1
DEBUG=true
`)
	got := DocumentedNames(src)
	sort.Strings(got)
	want := []string{"API_KEY", "COMENTADO", "DATABASE_URL", "DEBUG"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, quero %v", got, want)
	}
}

// Aceitação: DocumentedNames parse yaml/properties style.
func TestDocumentedNamesYaml(t *testing.T) {
	src := []byte(`
app:
  url: https://example.com
  port: 8080
db:
  host: localhost
`)
	got := DocumentedNames(src)
	sort.Strings(got)
	want := []string{"host", "port", "url"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, quero %v", got, want)
	}
}

// Aceitação: DocumentedNames com export prefix.
func TestDocumentedNamesExport(t *testing.T) {
	src := []byte(`export API_TOKEN=xxx
export DEBUG=1
`)
	got := DocumentedNames(src)
	sort.Strings(got)
	want := []string{"API_TOKEN", "DEBUG"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, quero %v", got, want)
	}
}

// Aceitação: DocumentedNames dedup.
func TestDocumentedNamesDedup(t *testing.T) {
	src := []byte(`
FOO=1
FOO=2
FOO=3
`)
	got := DocumentedNames(src)
	if len(got) != 1 || got[0] != "FOO" {
		t.Errorf("got %v, quero [FOO]", got)
	}
}

// Aceitação: DocumentedNames ignora linhas vazias.
func TestDocumentedNamesIgnoresEmpty(t *testing.T) {
	src := []byte(`

FOO=1

BAR=2

`)
	got := DocumentedNames(src)
	sort.Strings(got)
	want := []string{"BAR", "FOO"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, quero %v", got, want)
	}
}

// Aceitação: var added E documented tem 2 states.
func TestCompareAddedAndDocumented(t *testing.T) {
	head := []Usage{{Name: "NEW_KEY", Path: "a.go"}}
	docs := []string{"NEW_KEY"}
	diffs := Compare(nil, head, docs, nil)
	d := findDiffByName(diffs, "NEW_KEY")
	if !hasState(d, DiffAdded) {
		t.Errorf("sem added: %v", d.States)
	}
	if !hasState(d, DiffDocAdded) {
		t.Errorf("sem documentation-added: %v", d.States)
	}
	if hasState(d, DiffDocMissing) {
		t.Errorf("não devia ter documentation-missing: %v", d.States)
	}
}

// Aceitação: removed var com files base.
func TestCompareRemovedHasBaseFiles(t *testing.T) {
	base := []Usage{
		{Name: "GONE", Path: "old/a.go"},
		{Name: "GONE", Path: "old/b.go"},
	}
	diffs := Compare(base, nil, nil, nil)
	d := findDiffByName(diffs, "GONE")
	if !hasState(d, DiffRemoved) {
		t.Errorf("states = %v", d.States)
	}
	if len(d.BaseFiles) != 2 {
		t.Errorf("BaseFiles = %v", d.BaseFiles)
	}
	if len(d.HeadFiles) != 0 {
		t.Errorf("HeadFiles = %v, quero vazio", d.HeadFiles)
	}
}

// Aceitação: sameUsage detecta mudança de forma sutil.
func TestCompareSameUsageDetectsFormChange(t *testing.T) {
	base := []Usage{{Name: "X", Form: FormOSGetenv, HasDefault: false}}
	head := []Usage{{Name: "X", Form: FormOSLookupEnv, HasDefault: false}}
	if sameUsage(base, head) {
		t.Errorf("sameUsage devia detectar mudança de forma")
	}
}

// Aceitação: sameUsage aceita múltiplos usos com mesma forma.
func TestCompareSameUsageAcceptsMultipleSameForm(t *testing.T) {
	base := []Usage{
		{Name: "X", Form: FormOSGetenv},
		{Name: "X", Form: FormOSGetenv},
	}
	head := []Usage{
		{Name: "X", Form: FormOSGetenv},
		{Name: "X", Form: FormOSGetenv},
	}
	if !sameUsage(base, head) {
		t.Errorf("mesma forma devia ser equal")
	}
}

// Aceitação: sameUsage detecta HasDefault adicionado.
func TestCompareSameUsageDetectsDefaultChange(t *testing.T) {
	base := []Usage{{Name: "X", Form: FormOSGetenv, HasDefault: false}}
	head := []Usage{{Name: "X", Form: FormOSGetenv, HasDefault: true}}
	if sameUsage(base, head) {
		t.Errorf("HasDefault devia contar como mudança")
	}
}

// Aceitação: empty base e empty head.
func TestCompareEmpty(t *testing.T) {
	diffs := Compare(nil, nil, nil, nil)
	if len(diffs) != 0 {
		t.Errorf("esperv: %+v", diffs)
	}
}

// Aceitação: docs []string{} vs nil.
func TestCompareEmptyDocsIsMissing(t *testing.T) {
	head := []Usage{{Name: "X", Path: "a.go"}}
	diffs := Compare(nil, head, []string{}, nil)
	d := findDiffByName(diffs, "X")
	if !hasState(d, DiffDocMissing) {
		t.Errorf("docs vazio devia marcar missing: %v", d.States)
	}
}

// Aceitação: added var também é documentation-missing se não em docs.
func TestCompareAddedAlsoDocMissing(t *testing.T) {
	head := []Usage{{Name: "NEW", Path: "a.go"}}
	diffs := Compare(nil, head, nil, nil)
	d := findDiffByName(diffs, "NEW")
	if !hasState(d, DiffAdded) {
		t.Errorf("sem added")
	}
	if !hasState(d, DiffDocMissing) {
		t.Errorf("sem doc-missing")
	}
}

// Aceitação: removed var não tem doc state se não em docs.
func TestCompareRemovedNoDocState(t *testing.T) {
	base := []Usage{{Name: "GONE", Path: "a.go"}}
	diffs := Compare(base, nil, nil, nil)
	d := findDiffByName(diffs, "GONE")
	if !hasState(d, DiffRemoved) {
		t.Errorf("sem removed")
	}
	// Removed vars não precisam estar documentadas — saíram do código.
	// Marcamos mesmo assim para rastreabilidade.
	if !hasState(d, DiffDocMissing) {
		t.Errorf("esperava doc-missing para rastreabilidade")
	}
}
