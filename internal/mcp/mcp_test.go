package mcp

import "testing"

// Aceitação: SDKVersion pinned.
func TestSDKVersion(t *testing.T) {
	if SDKVersion != "v1.0.0" {
		t.Errorf("expected v1.0.0 pin, got %s", SDKVersion)
	}
}

// Aceitação: ServerName.
func TestServerName(t *testing.T) {
	if ServerName != "solidify" {
		t.Errorf("server name = %s", ServerName)
	}
}

// Aceitação: ServerVersion.
func TestServerVersion(t *testing.T) {
	if ServerVersion == "" {
		t.Errorf("server version vazio")
	}
}

// Aceitação: NewServer.
func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Errorf("server nil")
	}
}

// Aceitação: CurrentVersion.
func TestCurrentVersion(t *testing.T) {
	v := CurrentVersion()
	if v.SDK != "v1.0.0" {
		t.Errorf("sdk = %s", v.SDK)
	}
	if v.Server == "" {
		t.Errorf("server vazio")
	}
}

// Aceitação: StdioTransport.
func TestStdioTransport(t *testing.T) {
	tr := StdioTransport()
	if tr == nil {
		t.Errorf("transport nil")
	}
}
