package errs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// golden compara saída com testdata/<nome>; -update regrava o arquivo.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("gravar golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler golden %s: %v (rode com UPDATE_GOLDEN=1)", path, err)
	}
	if got != string(want) {
		t.Errorf("saída difere do golden %s\n--- quero ---\n%s\n--- tenho ---\n%s", path, want, got)
	}
}

func sampleError() error {
	return Wrap(CodeConfig, "campo obrigatório ausente", errors.New("chave \"ai.provider\" não encontrada")).
		WithField("ai.provider.base_url").
		WithHint("rode `solidify init` para gerar um solidify.json válido")
}

func TestRenderHumanGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderHuman(&buf, sampleError()); err != nil {
		t.Fatalf("RenderHuman: %v", err)
	}
	golden(t, "error_human.txt", buf.String())
}

func TestRenderJSONGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderJSON(&buf, sampleError()); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	golden(t, "error.json", buf.String())
}

func TestRenderMinimalErrorOmitsEmptyFields(t *testing.T) {
	var human, jsonBuf bytes.Buffer
	err := New(CodeGit, "range inválido")
	if rerr := RenderHuman(&human, err); rerr != nil {
		t.Fatalf("RenderHuman: %v", rerr)
	}
	if got, want := human.String(), "erro [git]: range inválido\n"; got != want {
		t.Errorf("humano = %q, quero %q", got, want)
	}
	if rerr := RenderJSON(&jsonBuf, err); rerr != nil {
		t.Fatalf("RenderJSON: %v", rerr)
	}
	for _, absent := range []string{"field", "cause", "hint"} {
		if bytes.Contains(jsonBuf.Bytes(), []byte(`"`+absent+`"`)) {
			t.Errorf("JSON não deveria conter %q: %s", absent, jsonBuf.String())
		}
	}
}

func TestUntypedErrorBecomesInternal(t *testing.T) {
	plain := errors.New("boom")
	if got := CodeOf(plain); got != CodeInternal {
		t.Errorf("CodeOf = %q, quero %q", got, CodeInternal)
	}
	if got := ExitCodeOf(plain); got != 1 {
		t.Errorf("ExitCodeOf = %d, quero 1", got)
	}
	var buf bytes.Buffer
	if err := RenderHuman(&buf, plain); err != nil {
		t.Fatalf("RenderHuman: %v", err)
	}
	if got, want := buf.String(), "erro [internal]: boom\n"; got != want {
		t.Errorf("humano = %q, quero %q", got, want)
	}
}

func TestNilErrorRendersNothing(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderHuman(&buf, nil); err != nil || buf.Len() != 0 {
		t.Errorf("RenderHuman(nil) escreveu %q, err=%v", buf.String(), err)
	}
	if err := RenderJSON(&buf, nil); err != nil || buf.Len() != 0 {
		t.Errorf("RenderJSON(nil) escreveu %q, err=%v", buf.String(), err)
	}
	if got := ExitCodeOf(nil); got != 0 {
		t.Errorf("ExitCodeOf(nil) = %d, quero 0", got)
	}
}

// Os exit codes fazem parte do contrato observável: mudanças aqui quebram
// scripts de usuários e devem ser deliberadas.
func TestExitCodesAreStable(t *testing.T) {
	want := map[Code]int{
		CodeOK: 0, CodeInternal: 1, CodeUsage: 2, CodeConfig: 3,
		CodeNotFound: 4, CodeGit: 5, CodeIO: 6, CodeStorage: 7,
		CodeSecurity: 8, CodeSchema: 9, CodeAnalyzer: 20,
		CodeProvider: 21, CodeTimeout: 22, CodeCanceled: 23,
	}
	for code, exit := range want {
		if got := code.ExitCode(); got != exit {
			t.Errorf("%s.ExitCode() = %d, quero %d", code, got, exit)
		}
	}
	if got := Code("inexistente").ExitCode(); got != 1 {
		t.Errorf("código desconhecido = %d, quero 1", got)
	}
}

func TestUnwrapPreservesCause(t *testing.T) {
	cause := errors.New("causa raiz")
	err := Wrap(CodeStorage, "falha ao abrir DB", cause)
	if !errors.Is(err, cause) {
		t.Error("errors.Is não encontrou a causa")
	}
}

// WithHint/WithField devolvem cópias: mutar a cópia não afeta o original.
func TestWithHelpersDoNotMutateOriginal(t *testing.T) {
	base := New(CodeUsage, "x")
	_ = base.WithHint("h").WithField("f")
	if base.Hint != "" || base.Field != "" {
		t.Errorf("original mutado: %+v", base)
	}
}
