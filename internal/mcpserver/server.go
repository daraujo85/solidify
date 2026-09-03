// Package mcpserver — MCP server Solidify.
//
// SAI-051/052/053: server MCP com tools de evidence retrieval
// e peer review. Usa SDK MCP (internal/mcp) em transporte stdio.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	mcpsdk "github.com/diegoaraujo/solidify/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wrapper com registro de tools.
type Server struct {
	sdk *mcp.Server
}

// New cria server MCP Solidify.
func New() *Server {
	return &Server{sdk: mcpsdk.NewServer()}
}

// SDK devolve underlying SDK server.
func (s *Server) SDK() *mcp.Server { return s.sdk }

// Run roda server stdio. Blocks até EOF.
func (s *Server) Run(ctx context.Context) error {
	return s.sdk.Run(ctx, &mcp.StdioTransport{})
}

// BeginReviewInput args de solidify_begin_review.
type BeginReviewInput struct {
	RunID    string `json:"run_id,omitempty"`    // opcional: reusar
	BaseRef  string `json:"base_ref,omitempty"`  // git base ref
	HeadRef  string `json:"head_ref,omitempty"`  // git head ref
	RepoPath string `json:"repo_path,omitempty"` // repo path
	Resume   bool   `json:"resume,omitempty"`    // tentar reusar run_id
}

// BeginReviewOutput contract refs.
type BeginReviewOutput struct {
	RunID        string   `json:"run_id"`
	CreatedAt    string   `json:"created_at"`
	Reused       bool     `json:"reused"`
	Schema       string   `json:"schema"`
	AnalyzerIDs  []string `json:"analyzer_ids"`
	HasEvidence  bool     `json:"has_evidence"`
	Limits       []string `json:"limits,omitempty"`
	Instructions string   `json:"instructions"`
}

// BeginReviewHandler é o handler do tool.
type BeginReviewHandler func(ctx context.Context, in *BeginReviewInput) (*BeginReviewOutput, error)

// RegisterBeginReview tool solidify_begin_review.
func (s *Server) RegisterBeginReview(h BeginReviewHandler) {
	tool := &mcp.Tool{
		Name:        "solidify_begin_review",
		Description: "Criar ou reusar run de peer review; prepara analyzers/evidence e devolve contract refs.",
	}
	mcp.AddTool(s.sdk, tool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, map[string]any, error) {
		var in BeginReviewInput
		if args != nil {
			raw, _ := json.Marshal(args)
			_ = json.Unmarshal(raw, &in)
		}
		out, err := h(ctx, &in)
		if err != nil {
			return nil, nil, err
		}
		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: mustJSON(out)},
			},
		}
		return result, nil, nil
	})
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("err: %v", err)
	}
	return string(b)
}

// SubmitPeerReviewInput args de submit tool.
// SAI-120: CanonicalPayload é o peer review canônico (SAI-116)
// como objeto nativo. v1 (Notes JSON-string) continua aceito
// pra leitura mas não é mais first-class.
type SubmitPeerReviewInput struct {
	RunID            string         `json:"run_id"`
	Actor            string         `json:"actor"`
	Schema           string         `json:"schema"`
	EvidenceHash     string         `json:"evidence_hash"`
	Verdict          string         `json:"verdict"`
	Findings         map[string]any `json:"findings,omitempty"`
	Notes            string         `json:"notes,omitempty"`
	CanonicalPayload map[string]any `json:"canonical_payload,omitempty"`
}

// SubmitPeerReviewOutput.
type SubmitPeerReviewOutput struct {
	Accepted     bool     `json:"accepted"`
	ReviewID     string   `json:"review_id"`
	SavedAt      string   `json:"saved_at"`
	ValidationOK bool     `json:"validation_ok"`
	Errors       []string `json:"errors,omitempty"`
	Warnings     []string `json:"warnings,omitempty"` // SAI-120: deprecation hints
}

// SubmitPeerReviewHandler. Em SAI-053 terá schema validation + atomic save.
type SubmitPeerReviewHandler func(ctx context.Context, in *SubmitPeerReviewInput) (*SubmitPeerReviewOutput, error)

// RegisterSubmitPeerReview tool solidify_submit_peer_review.
func (s *Server) RegisterSubmitPeerReview(h SubmitPeerReviewHandler) {
	tool := &mcp.Tool{
		Name:        "solidify_submit_peer_review",
		Description: "Submete peer review (validada, atomically saved).",
	}
	mcp.AddTool(s.sdk, tool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, map[string]any, error) {
		var in SubmitPeerReviewInput
		if args != nil {
			raw, _ := json.Marshal(args)
			_ = json.Unmarshal(raw, &in)
		}
		out, err := h(ctx, &in)
		if err != nil {
			return nil, nil, err
		}
		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: mustJSON(out)},
			},
		}
		return result, nil, nil
	})
}

// EvidenceGetInput args de evidence tools (SAI-052).
type EvidenceGetInput struct {
	RunID  string `json:"run_id"`
	Kind   string `json:"kind"` // "manifest" | "diff" | "context" | "symbols" | "analyzer" | "release"
	Path   string `json:"path,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Query  string `json:"query,omitempty"`
}

// EvidenceGetOutput unified.
type EvidenceGetOutput struct {
	Kind       string         `json:"kind"`
	RunID      string         `json:"run_id"`
	Items      []any          `json:"items,omitempty"`
	Total      int            `json:"total"`
	Truncated  bool           `json:"truncated"`
	NextOffset int            `json:"next_offset,omitempty"`
	Budget     map[string]any `json:"budget,omitempty"`
}

// EvidenceGetHandler handler genérico de evidence retrieval.
type EvidenceGetHandler func(ctx context.Context, in *EvidenceGetInput) (*EvidenceGetOutput, error)

// RegisterEvidenceGet tool solidify_evidence_get.
func (s *Server) RegisterEvidenceGet(h EvidenceGetHandler) {
	tool := &mcp.Tool{
		Name:        "solidify_evidence_get",
		Description: "Retrieves evidence: manifest, diff chunks, context, symbol search, analyzer results, release inventory.",
	}
	mcp.AddTool(s.sdk, tool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, map[string]any, error) {
		var in EvidenceGetInput
		if args != nil {
			raw, _ := json.Marshal(args)
			_ = json.Unmarshal(raw, &in)
		}
		out, err := h(ctx, &in)
		if err != nil {
			return nil, nil, err
		}
		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: mustJSON(out)},
			},
		}
		return result, nil, nil
	})
}
