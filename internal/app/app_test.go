package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Run(args, Env{Stdout: &out, Stderr: &errb, OmitLogTime: true})
	return code, out.String(), errb.String()
}

func TestVersionHuman(t *testing.T) {
	code, out, _ := run(t, "version")
	if code != 0 {
		t.Fatalf("exit = %d, quero 0", code)
	}
	if !strings.HasPrefix(out, "solidify ") {
		t.Fatalf("saída = %q, quero prefixo %q", out, "solidify ")
	}
}

func TestVersionJSON(t *testing.T) {
	code, out, _ := run(t, "version", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, quero 0", code)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON inválido: %v (%q)", err, out)
	}
	for _, k := range []string{"version", "commit", "date", "go_version", "platform"} {
		if got[k] == "" {
			t.Errorf("campo %q vazio", k)
		}
	}
}

func TestUsageErrorsExitTwoWithUsageOnStderr(t *testing.T) {
	cases := map[string][]string{
		"sem args":              nil,
		"comando desconhecido":  {"naoexiste"},
		"flag global inválida":  {"--nope", "x", "version"},
		"nível de log inválido": {"--log-level", "trace", "version"},
		"formato inválido":      {"--log-format=xml", "version"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := run(t, args...)
			if code != 2 {
				t.Fatalf("exit = %d, quero 2 (stderr: %s)", code, errOut)
			}
			if !strings.Contains(errOut, "erro [usage]") {
				t.Errorf("stderr sem erro tipado: %q", errOut)
			}
			if !strings.Contains(errOut, "Uso:") {
				t.Errorf("stderr sem usage: %q", errOut)
			}
			if out != "" {
				t.Errorf("stdout deveria estar vazio em erro, tem %q", out)
			}
		})
	}
}

// Golden do erro humano na fronteira da CLI.
func TestUnknownCommandHumanGolden(t *testing.T) {
	_, _, errOut := run(t, "naoexiste")
	want := "erro [usage]: comando desconhecido: naoexiste\n"
	if !strings.HasPrefix(errOut, want) {
		t.Errorf("stderr = %q, quero prefixo %q", errOut, want)
	}
}

// Golden do erro em JSON: mesmo erro, formato de máquina.
func TestErrorRenderedAsJSONWhenLogFormatJSON(t *testing.T) {
	code, _, errOut := run(t, "--log-format", "json", "naoexiste")
	if code != 2 {
		t.Fatalf("exit = %d, quero 1", code)
	}
	var payload struct {
		Error struct {
			Code     string `json:"code"`
			Message  string `json:"message"`
			ExitCode int    `json:"exit_code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(errOut), &payload); err != nil {
		t.Fatalf("stderr não é JSON: %v (%q)", err, errOut)
	}
	if payload.Error.Code != "usage" {
		t.Errorf("code = %q, quero usage", payload.Error.Code)
	}
	if payload.Error.ExitCode != 2 {
		t.Errorf("exit_code = %d, quero 2", payload.Error.ExitCode)
	}
	if payload.Error.Message != "comando desconhecido: naoexiste" {
		t.Errorf("message = %q", payload.Error.Message)
	}
}

func TestHelpSucceeds(t *testing.T) {
	code, out, _ := run(t, "help")
	if code != 0 || !strings.Contains(out, "Comandos:") {
		t.Fatalf("exit = %d, out = %q", code, out)
	}
}

func TestGlobalFlagsAcceptBothSyntaxes(t *testing.T) {
	for _, args := range [][]string{
		{"--log-level", "debug", "version"},
		{"--log-level=debug", "version"},
		{"-log-level=debug", "version"},
	} {
		code, out, errOut := run(t, args...)
		if code != 0 {
			t.Errorf("args %v: exit = %d (stderr: %s)", args, code, errOut)
		}
		if !strings.HasPrefix(out, "solidify ") {
			t.Errorf("args %v: stdout = %q", args, out)
		}
	}
}

// Em debug o logger emite para stderr, nunca para stdout.
func TestLogsGoToStderrOnly(t *testing.T) {
	_, out, errOut := run(t, "--log-level", "debug", "version")
	if strings.Contains(out, "versão resolvida") {
		t.Errorf("log vazou para stdout: %q", out)
	}
	if !strings.Contains(errOut, "versão resolvida") {
		t.Errorf("log ausente no stderr: %q", errOut)
	}
}

func TestFlagWithoutValueFails(t *testing.T) {
	code, _, errOut := run(t, "--log-level")
	if code != 2 || !strings.Contains(errOut, "exige valor") {
		t.Errorf("exit = %d, stderr = %q", code, errOut)
	}
}

// SAI-124: doctor --canary e --only são mutuamente exclusivos.
func TestDoctorCanaryMutuallyExclusiveWithOnly(t *testing.T) {
	code, _, errOut := run(t, "doctor", "--canary", "--only=peer_review_canary")
	if code == 0 {
		t.Fatalf("esperado exit != 0 com flags conflitantes, got 0")
	}
	if !strings.Contains(errOut, "mutuamente exclusivos") {
		t.Errorf("stderr devia indicar conflito: %q", errOut)
	}
	if !strings.Contains(errOut, "Uso:") {
		t.Errorf("stderr devia mostrar usage em erro de usage: %q", errOut)
	}
}

// SAI-124: doctor --canary roda só os 2 checks v1 (peer_review_canary + peer_review_store_v1).
func TestDoctorCanaryRunsOnlyV1Checks(t *testing.T) {
	code, out, _ := run(t, "doctor", "--canary")
	// sem store/metrics, espera 2 OK ("sem submissões" + "store vazio")
	if code != 0 {
		t.Errorf("canary devia sair 0 (sem dados = OK), got %d: %s", code, out)
	}
	if !strings.Contains(out, "peer_review_canary") {
		t.Errorf("output devia incluir peer_review_canary: %s", out)
	}
	if !strings.Contains(out, "peer_review_store_v1") {
		t.Errorf("output devia incluir peer_review_store_v1: %s", out)
	}
	// não deve rodar outros checks
	for _, other := range []string{"docker", "disk_space", "9router"} {
		if strings.Contains(out, other) {
			t.Errorf("--canary NÃO devia rodar %s, mas output contém: %s", other, out)
		}
	}
}
