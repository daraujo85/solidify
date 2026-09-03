// 9Router preset + network handling (SAI-060).
//
// Resolve BaseURL conforme contexto: host (localhost:20128) vs
// Docker (host.docker.internal:20128). Auth via token.
package ai

import (
	"errors"
	"os"
	"runtime"
	"strings"
)

// 9Router defaults.
const (
	NineRouterDefaultPort  = "20128"
	NineRouterDefaultHost  = "127.0.0.1"
	NineRouterDockerHost   = "host.docker.internal"
	NineRouterTokenEnv     = "ANTHROPIC_AUTH_TOKEN"
	NineRouterDefaultLabel = "9router"
)

// NetworkContext enum.
type NetworkContext string

const (
	NetworkHost   NetworkContext = "host"
	NetworkDocker NetworkContext = "docker"
	NetworkAuto   NetworkContext = "auto"
)

// DetectNetworkContext detecta contexto via env var ou runtime.
func DetectNetworkContext() NetworkContext {
	if v := os.Getenv("SOLIDIFY_NETWORK"); v != "" {
		switch strings.ToLower(v) {
		case "host", "native":
			return NetworkHost
		case "docker", "container":
			return NetworkDocker
		}
	}
	// /.dockerenv presente quando rodando dentro de container.
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return NetworkDocker
	}
	// macOS Docker Desktop expõe DOCKER_DESKTOP.
	if os.Getenv("DOCKER_DESKTOP") != "" {
		return NetworkDocker
	}
	return NetworkHost
}

// ResolveBaseURL monta URL conforme contexto. Inclui /v1 (OpenAI-compat path).
func ResolveBaseURL(ctx NetworkContext, port string) string {
	if port == "" {
		port = NineRouterDefaultPort
	}
	host := NineRouterDefaultHost
	if ctx == NetworkDocker {
		host = NineRouterDockerHost
	}
	return "http://" + host + ":" + port + "/v1"
}

// ResolveBaseURLWithHost customiza host (testes).
func ResolveBaseURLWithHost(ctx NetworkContext, host, port string) string {
	if port == "" {
		port = NineRouterDefaultPort
	}
	if host == "" {
		host = NineRouterDefaultHost
		if ctx == NetworkDocker {
			host = NineRouterDockerHost
		}
	}
	return "http://" + host + ":" + port + "/v1"
}

// ResolveToken lê token do env.
func ResolveToken() string {
	return os.Getenv(NineRouterTokenEnv)
}

// ResolvePreset monta preset para 9Router.
type NineRouterPreset struct {
	BaseURL     string         `json:"base_url"`
	Token       string         `json:"token,omitempty"`
	Network     NetworkContext `json:"network"`
	Host        string         `json:"host"`
	Port        string         `json:"port"`
	Label       string         `json:"label"`
	RequiresKey bool           `json:"requires_key"`
}

// BuildPreset monta preset com defaults.
func BuildPreset(opts PresetOptions) (*NineRouterPreset, error) {
	if opts.Port == "" {
		opts.Port = NineRouterDefaultPort
	}
	if opts.Host == "" {
		opts.Host = NineRouterDefaultHost
	}
	if opts.Network == "" || opts.Network == NetworkAuto {
		opts.Network = DetectNetworkContext()
	}
	if opts.Label == "" {
		opts.Label = NineRouterDefaultLabel
	}
	if opts.Network == NetworkDocker && opts.Host == NineRouterDefaultHost {
		opts.Host = NineRouterDockerHost
	}
	base := ResolveBaseURLWithHost(opts.Network, opts.Host, opts.Port)
	if base == "" {
		return nil, errors.New("9router: BaseURL vazia")
	}
	token := opts.Token
	if token == "" {
		token = ResolveToken()
	}
	return &NineRouterPreset{
		BaseURL:     base,
		Token:       token,
		Network:     opts.Network,
		Host:        opts.Host,
		Port:        opts.Port,
		Label:       opts.Label,
		RequiresKey: token != "",
	}, nil
}

// PresetOptions opções para preset.
type PresetOptions struct {
	Network NetworkContext `json:"network"`
	Host    string         `json:"host"`
	Port    string         `json:"port"`
	Token   string         `json:"token"`
	Label   string         `json:"label"`
}

// NewProviderFromPreset constrói OpenAIProvider a partir do preset.
func NewProviderFromPreset(preset *NineRouterPreset) *OpenAIProvider {
	if preset == nil {
		return nil
	}
	return NewOpenAIProvider(preset.BaseURL, preset.Token).
		WithProviderName(preset.Label)
}

// IsDockerNetwork helper.
func IsDockerNetwork() bool {
	return DetectNetworkContext() == NetworkDocker
}

// DefaultHostForContext helper.
func DefaultHostForContext(ctx NetworkContext) string {
	if ctx == NetworkDocker {
		return NineRouterDockerHost
	}
	return NineRouterDefaultHost
}

// FormatPreset renderiza para debug.
func FormatPreset(p *NineRouterPreset) string {
	if p == nil {
		return "<nil>"
	}
	var sb strings.Builder
	sb.WriteString("9Router[")
	sb.WriteString(string(p.Network))
	sb.WriteString("] ")
	sb.WriteString(p.BaseURL)
	if p.RequiresKey {
		sb.WriteString(" (auth)")
	}
	return sb.String()
}

// PlatformDockerHost retorna host.docker.internal-like conforme OS.
// Linux nativo (sem Docker Desktop) usa gateway address diferente;
// aqui só damos o "well-known" string que funciona na maioria dos
// setups Docker Desktop.
func PlatformDockerHost() string {
	switch runtime.GOOS {
	case "linux":
		// Em Linux, Docker bridge é 172.17.0.1 (legacy) ou gateway dinâmico.
		// Caller deve preferir host-gateway (Compose) ou override.
		return "172.17.0.1"
	default:
		// macOS / Windows: Docker Desktop expõe host.docker.internal.
		return NineRouterDockerHost
	}
}
