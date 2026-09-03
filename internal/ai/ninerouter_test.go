package ai

import (
	"os"
	"strings"
	"testing"
)

// Aceitação: ResolveBaseURL host.
func TestResolveBaseURLHost(t *testing.T) {
	u := ResolveBaseURL(NetworkHost, "20128")
	if u != "http://127.0.0.1:20128/v1" {
		t.Errorf("host: %s", u)
	}
}

// Aceitação: ResolveBaseURL docker.
func TestResolveBaseURLDocker(t *testing.T) {
	u := ResolveBaseURL(NetworkDocker, "20128")
	if u != "http://host.docker.internal:20128/v1" {
		t.Errorf("docker: %s", u)
	}
}

// Aceitação: ResolveBaseURL port default.
func TestResolveBaseURLDefaultPort(t *testing.T) {
	u := ResolveBaseURL(NetworkHost, "")
	if !strings.Contains(u, ":20128/") {
		t.Errorf("default port: %s", u)
	}
}

// Aceitação: ResolveBaseURLWithHost custom.
func TestResolveBaseURLCustom(t *testing.T) {
	u := ResolveBaseURLWithHost(NetworkHost, "10.0.0.1", "8080")
	if u != "http://10.0.0.1:8080/v1" {
		t.Errorf("custom: %s", u)
	}
}

// Aceitação: ResolveBaseURLWithHost docker sem host → docker default.
func TestResolveBaseURLDockerDefault(t *testing.T) {
	u := ResolveBaseURLWithHost(NetworkDocker, "", "20128")
	if !strings.Contains(u, "host.docker.internal") {
		t.Errorf("docker default: %s", u)
	}
}

// Aceitação: ResolveToken via env.
func TestResolveTokenEnv(t *testing.T) {
	os.Setenv(NineRouterTokenEnv, "secret123")
	defer os.Unsetenv(NineRouterTokenEnv)
	if ResolveToken() != "secret123" {
		t.Errorf("token env")
	}
}

// Aceitação: ResolveToken sem env.
func TestResolveTokenEmpty(t *testing.T) {
	os.Unsetenv(NineRouterTokenEnv)
	if ResolveToken() != "" {
		t.Errorf("token vazio")
	}
}

// Aceitação: DetectNetworkContext env override.
func TestDetectNetworkEnv(t *testing.T) {
	os.Setenv("SOLIDIFY_NETWORK", "docker")
	defer os.Unsetenv("SOLIDIFY_NETWORK")
	if DetectNetworkContext() != NetworkDocker {
		t.Errorf("env")
	}
}

// Aceitação: DetectNetworkContext env host.
func TestDetectNetworkEnvHost(t *testing.T) {
	os.Setenv("SOLIDIFY_NETWORK", "host")
	defer os.Unsetenv("SOLIDIFY_NETWORK")
	if DetectNetworkContext() != NetworkHost {
		t.Errorf("env host")
	}
}

// Aceitação: DetectNetworkContext env garbage.
func TestDetectNetworkEnvGarbage(t *testing.T) {
	os.Setenv("SOLIDIFY_NETWORK", "weird")
	defer os.Unsetenv("SOLIDIFY_NETWORK")
	// Cai no fallback. Em CI rodando em container, /.dockerenv existe
	// e cai em docker — esse teste só é confiável em host nativo.
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Skip("rodando em container, fallback é docker")
	}
	if DetectNetworkContext() == NetworkDocker {
		t.Errorf("garbage → docker?")
	}
}

// Aceitação: BuildPreset basic.
func TestBuildPresetBasic(t *testing.T) {
	p, err := BuildPreset(PresetOptions{Network: NetworkHost, Token: "abc"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.BaseURL != "http://127.0.0.1:20128/v1" {
		t.Errorf("base: %s", p.BaseURL)
	}
	if p.Token != "abc" {
		t.Errorf("token")
	}
	if !p.RequiresKey {
		t.Errorf("requires_key")
	}
}

// Aceitação: BuildPreset docker + auto-detect.
func TestBuildPresetDocker(t *testing.T) {
	p, err := BuildPreset(PresetOptions{Network: NetworkDocker})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(p.BaseURL, "host.docker.internal") {
		t.Errorf("docker: %s", p.BaseURL)
	}
}

// Aceitação: BuildPreset auto-detect.
func TestBuildPresetAuto(t *testing.T) {
	p, err := BuildPreset(PresetOptions{Network: NetworkAuto})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Network == "" {
		t.Errorf("network vazio")
	}
}

// Aceitação: BuildPreset env token fallback.
func TestBuildPresetEnvToken(t *testing.T) {
	os.Setenv(NineRouterTokenEnv, "envtoken")
	defer os.Unsetenv(NineRouterTokenEnv)
	p, err := BuildPreset(PresetOptions{Network: NetworkHost})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.Token != "envtoken" {
		t.Errorf("env token: %s", p.Token)
	}
}

// Aceitação: BuildPreset label default.
func TestBuildPresetLabelDefault(t *testing.T) {
	p, _ := BuildPreset(PresetOptions{Network: NetworkHost})
	if p.Label != NineRouterDefaultLabel {
		t.Errorf("label default")
	}
}

// Aceitação: BuildPreset custom port.
func TestBuildPresetCustomPort(t *testing.T) {
	p, _ := BuildPreset(PresetOptions{Network: NetworkHost, Port: "9999"})
	if !strings.Contains(p.BaseURL, ":9999/") {
		t.Errorf("custom port: %s", p.BaseURL)
	}
}

// Aceitação: NewProviderFromPreset.
func TestNewProviderFromPreset(t *testing.T) {
	preset := &NineRouterPreset{BaseURL: "http://x:20128", Token: "t", Label: "9router"}
	p := NewProviderFromPreset(preset)
	if p == nil {
		t.Fatal("nil")
	}
	if p.Name() != "9router" {
		t.Errorf("name")
	}
	if p.BaseURL != "http://x:20128" {
		t.Errorf("base")
	}
}

// Aceitação: NewProviderFromPreset nil.
func TestNewProviderFromPresetNil(t *testing.T) {
	if NewProviderFromPreset(nil) != nil {
		t.Errorf("nil preset devia dar nil")
	}
}

// Aceitação: IsDockerNetwork com env override.
func TestIsDockerNetwork(t *testing.T) {
	os.Setenv("SOLIDIFY_NETWORK", "docker")
	defer os.Unsetenv("SOLIDIFY_NETWORK")
	if !IsDockerNetwork() {
		t.Errorf("docker")
	}
}

// Aceitação: DefaultHostForContext.
func TestDefaultHost(t *testing.T) {
	if DefaultHostForContext(NetworkHost) != NineRouterDefaultHost {
		t.Errorf("host")
	}
	if DefaultHostForContext(NetworkDocker) != NineRouterDockerHost {
		t.Errorf("docker host")
	}
}

// Aceitação: FormatPreset.
func TestFormatPreset(t *testing.T) {
	p := &NineRouterPreset{Network: NetworkHost, BaseURL: "http://127.0.0.1:20128", RequiresKey: true}
	out := FormatPreset(p)
	if !strings.Contains(out, "host") {
		t.Errorf("format: %s", out)
	}
	if !strings.Contains(out, "auth") {
		t.Errorf("auth: %s", out)
	}
	if FormatPreset(nil) != "<nil>" {
		t.Errorf("nil")
	}
}

// Aceitação: FormatPreset sem auth.
func TestFormatPresetNoAuth(t *testing.T) {
	p := &NineRouterPreset{Network: NetworkHost, BaseURL: "http://x"}
	out := FormatPreset(p)
	if strings.Contains(out, "auth") {
		t.Errorf("no auth: %s", out)
	}
}

// Aceitação: PlatformDockerHost.
func TestPlatformDockerHost(t *testing.T) {
	h := PlatformDockerHost()
	if h == "" {
		t.Errorf("vazio")
	}
}

// Aceitação: BuildPreset docker auto-rewrite host.
func TestBuildPresetDockerAutoHost(t *testing.T) {
	p, err := BuildPreset(PresetOptions{
		Network: NetworkDocker,
		Host:    NineRouterDefaultHost, // deve virar docker
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(p.BaseURL, NineRouterDockerHost) {
		t.Errorf("docker host auto: %s", p.BaseURL)
	}
}

// Aceitação: BuildPreset docker explicit host preserva.
func TestBuildPresetDockerCustomHost(t *testing.T) {
	p, err := BuildPreset(PresetOptions{
		Network: NetworkDocker,
		Host:    "172.17.0.1",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(p.BaseURL, "172.17.0.1") {
		t.Errorf("custom docker: %s", p.BaseURL)
	}
}

// Aceitação: BuildPreset custom label.
func TestBuildPresetCustomLabel(t *testing.T) {
	p, _ := BuildPreset(PresetOptions{Network: NetworkHost, Label: "router"})
	if p.Label != "router" {
		t.Errorf("label custom")
	}
}
