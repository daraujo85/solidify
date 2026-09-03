// Package mcp — wrapper do SDK MCP oficial Go.
//
// SAI-050: pin de versão + transporte stdio. Wrapper thin
// re-exporta tipos/funções do SDK e adiciona helpers de
// versão/identidade.
package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SDKVersion versão pinada do SDK MCP.
const SDKVersion = "v1.0.0"

// ServerName identidade do MCP server Solidify.
const ServerName = "solidify"

// ServerVersion versão do servidor (semver sem prefixo v).
const ServerVersion = "0.1.0"

// Re-exports do SDK para callers não importarem diretamente.
type (
	// Server MCP server.
	Server = mcp.Server
	// ServerOptions opções de construção.
	ServerOptions = mcp.ServerOptions
	// Transport para stdio transport.
	Transport = mcp.Transport
	// ToolHandler função de tool.
	ToolHandler = mcp.ToolHandler
	// Tool anotação metadata.
	Tool = mcp.Tool
	// Session contexto de sessão.
	Session = mcp.Session
)

// NewServer cria MCP server com identidade Solidify.
func NewServer() *Server {
	return mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, nil)
}

// StdioTransport transport via stdin/stdout.
func StdioTransport() mcp.Transport {
	return &mcp.StdioTransport{}
}

// AddTool wrapper em torno de s.AddTool com tipos do SDK.
func AddTool(s *Server, name, description string, handler mcp.ToolHandler) {
	tool := &mcp.Tool{
		Name:        name,
		Description: description,
	}
	s.AddTool(tool, handler)
}

// VersionInfo snapshot de identidade.
type VersionInfo struct {
	SDK    string `json:"sdk"`
	Server string `json:"server"`
	Build  string `json:"build,omitempty"`
}

// CurrentVersion devolve version info atual.
func CurrentVersion() VersionInfo {
	return VersionInfo{
		SDK:    SDKVersion,
		Server: ServerVersion,
	}
}
