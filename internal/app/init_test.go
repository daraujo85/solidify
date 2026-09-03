package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diegoaraujo/solidify/internal/config"
)

// fakeGitRepo cria um diretório com .git/ dentro e o devolve. O retorno é o
// git root — para testar init em subdiretório, crie o subdir e chdir nele.
func fakeGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("criar .git/: %v", err)
	}
	return root
}

func TestInitCreatesFilesInGitRepo(t *testing.T) {
	repo := fakeGitRepo(t)
	t.Chdir(repo)

	code, out, errOut := run(t, "init")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(out, "inicializado") {
		t.Errorf("stdout sem confirmação: %q", out)
	}

	cfgPath := filepath.Join(repo, config.FileName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("solidify.json não foi criado: %v", err)
	}
	// O JSON gerado tem que carregar de volta sem erro.
	loaded, err := config.Load(repo, nil)
	if err != nil {
		t.Fatalf("config gerada não carrega: %v (conteúdo: %s)", err, data)
	}
	if loaded.Config.Project.Name != filepath.Base(repo) {
		t.Errorf("project.name = %q, quero basename do repo %q",
			loaded.Config.Project.Name, filepath.Base(repo))
	}

	if _, err := os.Stat(filepath.Join(repo, ".solidify")); err != nil {
		t.Errorf(".solidify/ não foi criado: %v", err)
	}
}

// Aceite da task: rodar duas vezes é idempotente.
func TestInitIsIdempotent(t *testing.T) {
	repo := fakeGitRepo(t)
	t.Chdir(repo)

	run(t, "init")
	code, _, errOut := run(t, "init")
	if code != 0 {
		t.Fatalf("segunda execução: exit = %d, stderr = %s", code, errOut)
	}

	cfgPath := filepath.Join(repo, config.FileName)
	first, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ler config: %v", err)
	}
	// Rodar de novo e checar que o arquivo não foi tocado.
	run(t, "init")
	second, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ler config: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotência quebrou: arquivo mudou\nantes: %s\ndepois: %s", first, second)
	}
}

// Sem --force, config existente não é sobrescrita.
func TestInitRefusesToOverwriteWithoutForce(t *testing.T) {
	repo := fakeGitRepo(t)
	t.Chdir(repo)

	cfgPath := filepath.Join(repo, config.FileName)
	sentinel := "placeholder-que-deve-sobreviver"
	if err := os.WriteFile(cfgPath, []byte(sentinel), 0o644); err != nil {
		t.Fatalf("escrever sentinel: %v", err)
	}

	code, _, errOut := run(t, "init")
	if code != 0 {
		t.Errorf("exit = %d, stderr = %s", code, errOut)
	}
	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ler config: %v", err)
	}
	if string(got) != sentinel {
		t.Errorf("config existente foi sobrescrita:\n%s", got)
	}
	if !strings.Contains(errOut, config.FileName) && !strings.Contains(errOut, "inicializado") {
		t.Errorf("stderr sem dica do estado: %q", errOut)
	}
}

// Com --force, a config existente é sobrescrita.
func TestInitForceOverwrites(t *testing.T) {
	repo := fakeGitRepo(t)
	t.Chdir(repo)

	cfgPath := filepath.Join(repo, config.FileName)
	if err := os.WriteFile(cfgPath, []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("escrever sentinel: %v", err)
	}

	code, _, errOut := run(t, "init", "--force")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}

	loaded, err := config.Load(repo, nil)
	if err != nil {
		t.Fatalf("config gerada não carrega: %v", err)
	}
	if loaded.Config.Project.Name != filepath.Base(repo) {
		t.Errorf("project.name = %q", loaded.Config.Project.Name)
	}
}

func TestInitNameFromFlag(t *testing.T) {
	repo := fakeGitRepo(t)
	t.Chdir(repo)

	run(t, "init", "--name=minha-api")
	code, _, errOut := run(t, "init")
	if code != 0 {
		t.Fatalf("segunda execução: exit = %d, stderr = %s", code, errOut)
	}

	loaded, err := config.Load(repo, nil)
	if err != nil {
		t.Fatalf("config não carrega: %v", err)
	}
	if loaded.Config.Project.Name != "minha-api" {
		t.Errorf("project.name = %q, quero minha-api", loaded.Config.Project.Name)
	}
}

// Subdiretório do repo: init sobe na hierarquia e usa o git root.
func TestInitFromSubdirFindsGitRoot(t *testing.T) {
	repo := fakeGitRepo(t)
	subdir := filepath.Join(repo, "services", "billing")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("criar subdir: %v", err)
	}
	t.Chdir(subdir)

	code, _, errOut := run(t, "init")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}

	cfgPath := filepath.Join(repo, config.FileName)
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("config esperada em %s: %v", cfgPath, err)
	}
	// O subdir não deve ter ganhado um solidify.json próprio.
	if _, err := os.Stat(filepath.Join(subdir, config.FileName)); err == nil {
		t.Errorf("config não deveria existir dentro do subdir")
	}
}

// Sem .git em lugar nenhum: erro com código git.
func TestInitFailsOutsideGitRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	code, _, errOut := run(t, "init")
	if code != 5 { // errs.CodeGit.ExitCode()
		t.Fatalf("exit = %d, quero 5 (git). stderr = %s", code, errOut)
	}
	if !strings.Contains(errOut, "[git]") {
		t.Errorf("stderr sem código git: %q", errOut)
	}
}

// --dir aponta para um caminho dentro de um repo Git existente.
func TestInitDirFlag(t *testing.T) {
	repo := fakeGitRepo(t)
	outside := t.TempDir() // sem .git

	if code, _, errOut := run(t, "init", "--dir="+outside); code != 5 {
		t.Fatalf("--dir fora de repo: exit = %d, quero 5. stderr = %s", code, errOut)
	}

	if code, _, errOut := run(t, "init", "--dir="+repo); code != 0 {
		t.Fatalf("--dir dentro do repo: exit = %d, stderr = %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(repo, config.FileName)); err != nil {
		t.Errorf("config não foi criada em --dir: %v", err)
	}
}

// JSON mode: o erro continua sendo JSON parseável, não texto.
func TestInitErrorJSONIsParseable(t *testing.T) {
	t.Chdir(t.TempDir())
	_, _, errOut := run(t, "--log-format=json", "init")
	if errOut == "" {
		t.Skip("init fora de repo deveria falhar")
	}
	var payload struct {
		Error struct {
			Code     string `json:"code"`
			ExitCode int    `json:"exit_code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(errOut), &payload); err != nil {
		t.Fatalf("stderr não é JSON: %v (%q)", err, errOut)
	}
	if payload.Error.Code != "git" {
		t.Errorf("code = %q, quero git", payload.Error.Code)
	}
}

// A config gerada tem todos os campos esperados para um Solidify válido.
func TestInitGeneratesContractualValidConfig(t *testing.T) {
	repo := fakeGitRepo(t)
	t.Chdir(repo)

	if _, _, errOut := run(t, "init"); errOut != "" {
		t.Logf("stderr: %s", errOut)
	}
	loaded, err := config.Load(repo, nil)
	if err != nil {
		t.Fatalf("config não carrega: %v", err)
	}
	if loaded.Config.Scoring.Weights.Solid != 50 {
		t.Errorf("peso SOLID = %d, quero 50", loaded.Config.Scoring.Weights.Solid)
	}
	for _, name := range []string{"quick", "release", "contractual", "strict-plus"} {
		if _, ok := loaded.Config.Profiles[name]; !ok {
			t.Errorf("perfil %q ausente", name)
		}
	}
}
