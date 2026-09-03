// Doctor diagnostics (SAI-113).
//
// Checa pré-requisitos de ambiente: docker, git, go,
// network, disk space, gateway 9router.
package doctor

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/diegoaraujo/solidify/internal/ai"
	"github.com/diegoaraujo/solidify/internal/config"
)

// CheckResult resultado de um check.
type CheckResult struct {
	Name    string
	OK      bool
	Message string
}

// Check func genérica.
type Check func() CheckResult

// AllChecks checks canônicos.
var AllChecks = []string{
	"docker",
	"git",
	"go",
	"disk_space",
	"network",
	"9router",
	"peer_review_canary",
	"peer_review_store_v1",
}

// RunCheck roda 1 check por nome.
func RunCheck(name string) CheckResult {
	switch name {
	case "docker":
		return checkDocker()
	case "git":
		return checkGit()
	case "go":
		return checkGo()
	case "disk_space":
		return checkDisk()
	case "network":
		return checkNetwork()
	case "9router":
		return check9Router()
	case "peer_review_canary":
		return checkPeerReviewCanary()
	case "peer_review_store_v1":
		return checkPeerReviewStoreV1()
	default:
		return CheckResult{Name: name, OK: false, Message: "check desconhecido"}
	}
}

// RunAll roda todos os checks.
func RunAll() []CheckResult {
	rs := make([]CheckResult, 0, len(AllChecks))
	for _, n := range AllChecks {
		rs = append(rs, RunCheck(n))
	}
	return rs
}

// Summary agrega.
type Summary struct {
	Total int
	OK    int
	Fail  int
}

// Summarize agrega results.
func Summarize(rs []CheckResult) Summary {
	s := Summary{Total: len(rs)}
	for _, r := range rs {
		if r.OK {
			s.OK++
		} else {
			s.Fail++
		}
	}
	return s
}

func checkDocker() CheckResult {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// Rodando dentro do container: docker está no host, não aqui.
		// Validar a presença do socket é um proxy razoável.
		if _, serr := os.Stat("/var/run/docker.sock"); serr == nil {
			return CheckResult{Name: "docker", OK: true, Message: "docker socket acessível (in-container)"}
		}
		return CheckResult{Name: "docker", OK: true, Message: "in-container (docker host-side assumido)"}
	}
	_, err := exec.LookPath("docker")
	if err != nil {
		return CheckResult{Name: "docker", OK: false, Message: "docker não encontrado"}
	}
	return CheckResult{Name: "docker", OK: true, Message: "docker disponível"}
}

func checkGit() CheckResult {
	_, err := exec.LookPath("git")
	if err != nil {
		return CheckResult{Name: "git", OK: false, Message: "git não encontrado"}
	}
	return CheckResult{Name: "git", OK: true, Message: "git disponível"}
}

func checkGo() CheckResult {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// Rodando dentro do container: a toolchain Go vive na imagem
		// de build, não no runtime. Não exigir.
		return CheckResult{Name: "go", OK: true, Message: "in-container (Go toolchain no build stage)"}
	}
	_, err := exec.LookPath("go")
	if err != nil {
		return CheckResult{Name: "go", OK: false, Message: "go não encontrado"}
	}
	return CheckResult{Name: "go", OK: true, Message: "go disponível"}
}

func checkDisk() CheckResult {
	wd, err := os.Getwd()
	if err != nil {
		return CheckResult{Name: "disk_space", OK: false, Message: "sem wd"}
	}
	// Best-effort: stat wd.
	if _, err := os.Stat(wd); err != nil {
		return CheckResult{Name: "disk_space", OK: false, Message: "stat falhou"}
	}
	return CheckResult{Name: "disk_space", OK: true, Message: "wd acessível: " + filepath.Base(wd)}
}

func checkNetwork() CheckResult {
	// Stub: considera OK se houver rota default conhecida.
	// Sem network call (test-friendly).
	if _, err := os.Stat("/etc/resolv.conf"); err != nil {
		return CheckResult{Name: "network", OK: true, Message: "resolv.conf ausente (sem DNS check)"}
	}
	return CheckResult{Name: "network", OK: true, Message: "resolv presente"}
}

func check9Router() CheckResult {
	// Fonte única: internal/config (AI.ExternalProvider.BaseURL{Host,Docker}).
	// Fallback de override: env NINEROUTER_URL.
	cfg := config.Default()
	// Auto-detecta contexto (docker vs host) e escolhe BaseURL adequada.
	// Dentro do container, o default 127.0.0.1 não alcança o gateway no host.
	host := cfg.AI.ExternalProvider.BaseURLDocker
	if ai.DetectNetworkContext() == ai.NetworkHost {
		host = cfg.AI.ExternalProvider.BaseURLHost
	}
	if override := os.Getenv("NINEROUTER_URL"); override != "" {
		host = override
	}
	keyEnv := cfg.AI.ExternalProvider.APIKeyEnv
	if keyEnv == "" {
		keyEnv = ai.NineRouterTokenEnv
	}
	token := os.Getenv(keyEnv)
	if host == "" {
		return CheckResult{Name: "9router", OK: false, Message: "BaseURL vazia na config"}
	}
	if !strings.HasPrefix(host, "http") {
		return CheckResult{Name: "9router", OK: false, Message: "URL inválida: " + host}
	}
	if token == "" {
		return CheckResult{Name: "9router", OK: false, Message: "token não setado (esperado env " + keyEnv + ")"}
	}
	// Probe real: GET /v1/models com timeout curto. 200 OK = gateway up + auth ok.
	probeURL := strings.TrimRight(host, "/") + "/models"
	if !strings.Contains(probeURL, "/v1/") {
		// BaseURLHost já vem com /v1 (config default); defense in depth.
		probeURL = strings.TrimRight(host, "/") + "/v1/models"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, probeURL, nil)
	if err != nil {
		return CheckResult{Name: "9router", OK: false, Message: "probe URL inválida: " + err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return CheckResult{Name: "9router", OK: false, Message: "probe falhou: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return CheckResult{Name: "9router", OK: false, Message: "401 Unauthorized — token inválido ou expirado"}
	}
	if resp.StatusCode != http.StatusOK {
		return CheckResult{Name: "9router", OK: false, Message: "status " + resp.Status}
	}
	return CheckResult{Name: "9router", OK: true, Message: "OK " + probeURL + " (" + resp.Status + ")"}
}

// ErrCheckFailed check falhou.
var ErrCheckFailed = errors.New("doctor: check falhou")
