package analyzer

import (
	"context"
	"errors"
	"testing"
)

// stubAnalyzer — analyzer usado em testes.
type stubAnalyzer struct {
	name    string
	version string
	class   Class
	detect  bool
	derr    error
	rres    Result
	rerr    error
}

func (s *stubAnalyzer) Name() string    { return s.name }
func (s *stubAnalyzer) Version() string { return s.version }
func (s *stubAnalyzer) Class() Class    { return s.class }
func (s *stubAnalyzer) Detect(_ context.Context, _ Context) (bool, error) {
	return s.detect, s.derr
}
func (s *stubAnalyzer) Run(_ context.Context, _ Context) (Result, error) {
	return s.rres, s.rerr
}

// Aceitação: registry Register/Get.
func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	a := &stubAnalyzer{name: "x", version: "1", class: ClassLight}
	if err := r.Register(a); err != nil {
		t.Fatalf("err: %v", err)
	}
	got, ok := r.Get("x")
	if !ok || got != a {
		t.Errorf("not found")
	}
}

// Aceitação: registry rejeita duplicado.
func TestRegistryDuplicate(t *testing.T) {
	r := NewRegistry()
	a := &stubAnalyzer{name: "x", class: ClassLight}
	_ = r.Register(a)
	if err := r.Register(a); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: registry rejeita name vazio.
func TestRegistryEmptyName(t *testing.T) {
	r := NewRegistry()
	a := &stubAnalyzer{name: "", class: ClassLight}
	if err := r.Register(a); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: registry rejeita nil.
func TestRegistryNil(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: registry Names ordena.
func TestRegistryNames(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&stubAnalyzer{name: "z", class: ClassLight})
	_ = r.Register(&stubAnalyzer{name: "a", class: ClassLight})
	names := r.Names()
	if names[0] != "a" || names[1] != "z" {
		t.Errorf("names = %v", names)
	}
}

// Aceitação: registry All ordem determinística.
func TestRegistryAllOrder(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&stubAnalyzer{name: "z", class: ClassLight})
	_ = r.Register(&stubAnalyzer{name: "a", class: ClassLight})
	all := r.All()
	if all[0].Name() != "a" || all[1].Name() != "z" {
		t.Errorf("order")
	}
}

// Aceitação: MustRegister panic em erro.
func TestRegistryMustRegisterPanic(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&stubAnalyzer{name: "x", class: ClassLight})
	defer func() {
		if recover() == nil {
			t.Errorf("devia panic")
		}
	}()
	r.MustRegister(&stubAnalyzer{name: "x", class: ClassLight})
}

// Aceitação: Len.
func TestRegistryLen(t *testing.T) {
	r := NewRegistry()
	if r.Len() != 0 {
		t.Errorf("len = %d", r.Len())
	}
	_ = r.Register(&stubAnalyzer{name: "x", class: ClassLight})
	if r.Len() != 1 {
		t.Errorf("len = %d", r.Len())
	}
}

// Aceitação: Run OK com detect=true.
func TestRunOK(t *testing.T) {
	a := &stubAnalyzer{
		name: "x", version: "1", class: ClassLight, detect: true,
		rres: Result{Findings: []Finding{{Code: "c", Severity: SeverityLow}}},
	}
	res, err := Run(context.Background(), a, Context{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Detected || res.Status != StatusOK {
		t.Errorf("res = %+v", res)
	}
	if len(res.Findings) != 1 {
		t.Errorf("findings = %d", len(res.Findings))
	}
}

// Aceitação: Run com detect=false → skip.
func TestRunSkip(t *testing.T) {
	a := &stubAnalyzer{
		name: "x", version: "1", class: ClassLight, detect: false,
	}
	res, err := Run(context.Background(), a, Context{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Skipped || res.Status != StatusSkip {
		t.Errorf("res = %+v", res)
	}
	if res.SkipReason == "" {
		t.Errorf("skip reason vazio")
	}
}

// Aceitação: Run com Detect error.
func TestRunDetectError(t *testing.T) {
	a := &stubAnalyzer{
		name: "x", class: ClassLight, derr: errors.New("detect fail"),
	}
	_, err := Run(context.Background(), a, Context{})
	if err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Run com Run error.
func TestRunRunError(t *testing.T) {
	a := &stubAnalyzer{
		name: "x", class: ClassLight, detect: true, rerr: errors.New("run fail"),
	}
	res, err := Run(context.Background(), a, Context{})
	if err == nil {
		t.Errorf("devia falhar")
	}
	if res.Status != StatusError {
		t.Errorf("status = %s", res.Status)
	}
}

// Aceitação: Run preenche metadata ausente.
func TestRunFillsMetadata(t *testing.T) {
	a := &stubAnalyzer{
		name: "x", version: "1", class: ClassLight, detect: true,
		rres: Result{},
	}
	res, err := Run(context.Background(), a, Context{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Analyzer != "x" || res.Version != "1" || res.Class != ClassLight {
		t.Errorf("metadata não preenchida: %+v", res)
	}
}

// Aceitação: Run respeita metadata do analyzer.
func TestRunPreservesMetadata(t *testing.T) {
	a := &stubAnalyzer{
		name: "x", version: "1", class: ClassLight, detect: true,
		rres: Result{Analyzer: "override", Version: "9", Class: ClassCPU, Status: StatusWarn},
	}
	res, _ := Run(context.Background(), a, Context{})
	if res.Analyzer != "override" || res.Version != "9" || res.Class != ClassCPU || res.Status != StatusWarn {
		t.Errorf("metadata sobrescrita: %+v", res)
	}
}

// Aceitação: DetectAll filtra não-detectados.
func TestDetectAll(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&stubAnalyzer{name: "yes", class: ClassLight, detect: true})
	_ = r.Register(&stubAnalyzer{name: "no", class: ClassLight, detect: false})
	det := DetectAll(context.Background(), r, Context{})
	if len(det) != 1 || det[0].Name() != "yes" {
		t.Errorf("det = %v", det)
	}
}

// Aceitação: ByClass filtra.
func TestByClass(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&stubAnalyzer{name: "a", class: ClassLight})
	_ = r.Register(&stubAnalyzer{name: "b", class: ClassBrowser})
	_ = r.Register(&stubAnalyzer{name: "c", class: ClassLight})
	out := ByClass(r, ClassLight)
	if len(out) != 2 {
		t.Errorf("len = %d", len(out))
	}
}

// Aceitação: IsValidClass.
func TestIsValidClass(t *testing.T) {
	cases := map[Class]bool{
		ClassLight:       true,
		ClassBrowser:     true,
		ClassActiveNet:   true,
		Class("unknown"): false,
		Class(""):        false,
	}
	for c, want := range cases {
		if got := IsValidClass(c); got != want {
			t.Errorf("IsValidClass(%s) = %v", c, got)
		}
	}
}

// Aceitação: AllClasses cobre todas.
func TestAllClasses(t *testing.T) {
	all := AllClasses()
	if len(all) != 5 {
		t.Errorf("len = %d", len(all))
	}
}

// Aceitação: Summary conta por severity.
func TestResultSummary(t *testing.T) {
	r := Result{
		Findings: []Finding{
			{Severity: SeverityLow},
			{Severity: SeverityLow},
			{Severity: SeverityHigh},
			{Severity: SeverityCritical},
		},
	}
	s := r.Summary()
	if s[SeverityLow] != 2 || s[SeverityHigh] != 1 || s[SeverityCritical] != 1 {
		t.Errorf("summary = %v", s)
	}
}

// Aceitação: HasCritical.
func TestResultHasCritical(t *testing.T) {
	r1 := Result{Findings: []Finding{{Severity: SeverityHigh}}}
	if r1.HasCritical() {
		t.Errorf("devia ser false")
	}
	r2 := Result{Findings: []Finding{{Severity: SeverityCritical}}}
	if !r2.HasCritical() {
		t.Errorf("devia ser true")
	}
}

// Aceitação: HighestSeverity.
func TestResultHighestSeverity(t *testing.T) {
	cases := []struct {
		findings []Finding
		want     Severity
	}{
		{[]Finding{}, ""},
		{[]Finding{{Severity: SeverityLow}}, SeverityLow},
		{[]Finding{{Severity: SeverityLow}, {Severity: SeverityCritical}}, SeverityCritical},
		{[]Finding{{Severity: SeverityHigh}, {Severity: SeverityMedium}}, SeverityHigh},
		{[]Finding{{Severity: SeverityInfo}, {Severity: SeverityLow}, {Severity: SeverityMedium}}, SeverityMedium},
	}
	for _, c := range cases {
		r := Result{Findings: c.findings}
		if got := r.HighestSeverity(); got != c.want {
			t.Errorf("got %s, quero %s", got, c.want)
		}
	}
}

// Aceitação: Context.MetadataGet.
func TestContextMetadataGet(t *testing.T) {
	c := Context{}
	if c.MetadataGet("x") != "" {
		t.Errorf("nil map devia dar vazio")
	}
	c.Metadata = map[string]string{"x": "y"}
	if c.MetadataGet("x") != "y" {
		t.Errorf("got = %s", c.MetadataGet("x"))
	}
	if c.MetadataGet("z") != "" {
		t.Errorf("missing devia dar vazio")
	}
}
