// Evidence retrieval handlers (SAI-052).
//
// Tools: manifest, diff chunk, context, symbol search,
// analyzer, release inventory. Budget-aware pagination.
package mcpserver

import (
	"context"
	"strings"

	"github.com/diegoaraujo/solidify/internal/artifacts"
)

// EvidenceKind constants.
const (
	KindManifest = "manifest"
	KindDiff     = "diff"
	KindContext  = "context"
	KindSymbols  = "symbols"
	KindAnalyzer = "analyzer"
	KindRelease  = "release"
)

// DefaultLimit default pagination.
const DefaultLimit = 50

// MaxLimit pagination cap.
const MaxLimit = 500

// ResolveStore cria Artifacts Store por runID.
func ResolveStore(baseDir, runID string) (*artifacts.Store, error) {
	if baseDir == "" {
		return nil, errEvidence("base_dir vazio")
	}
	if runID == "" {
		return nil, errEvidence("run_id vazio")
	}
	return artifacts.New(baseDir, runID)
}

// DefaultEvidenceHandler implementação default do handler.
func DefaultEvidenceHandler(baseDir string) EvidenceGetHandler {
	return func(ctx context.Context, in *EvidenceGetInput) (*EvidenceGetOutput, error) {
		if in == nil {
			return nil, errEvidence("input nil")
		}
		if in.RunID == "" {
			return nil, errEvidence("run_id vazio")
		}
		if in.Limit <= 0 {
			in.Limit = DefaultLimit
		}
		if in.Limit > MaxLimit {
			in.Limit = MaxLimit
		}
		if in.Offset < 0 {
			in.Offset = 0
		}
		store, err := ResolveStore(baseDir, in.RunID)
		if err != nil {
			return nil, err
		}
		switch in.Kind {
		case KindManifest:
			return getManifest(store, in)
		case KindDiff:
			return getDiffChunk(store, in)
		case KindContext:
			return getContext(store, in)
		case KindSymbols:
			return getSymbolSearch(store, in)
		case KindAnalyzer:
			return getAnalyzerResult(store, in)
		case KindRelease:
			return getReleaseInventory(store, in)
		default:
			return nil, errEvidence("kind unknown: %s", in.Kind)
		}
	}
}

func errEvidence(format string, args ...any) error {
	return &EvidenceError{Msg: sprintf(format, args...)}
}

type EvidenceError struct{ Msg string }

func (e *EvidenceError) Error() string { return "evidence: " + e.Msg }

func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	// minimal fmt.Sprintf sem importar.
	out := format
	for _, a := range args {
		out = strings.Replace(out, "%v", toStr(a), 1)
		out = strings.Replace(out, "%s", toStr(a), 1)
		out = strings.Replace(out, "%d", toStr(a), 1)
	}
	return out
}

func toStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(strings.Join(strings.Fields(stringify(v)), ""))
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	default:
		return ""
	}
}

func getManifest(store *artifacts.Store, in *EvidenceGetInput) (*EvidenceGetOutput, error) {
	data, err := store.Read("evidence.json")
	if err != nil {
		return nil, errEvidence("manifest missing: %v", err)
	}
	out := &EvidenceGetOutput{
		Kind:  KindManifest,
		RunID: in.RunID,
		Total: 1,
		Items: []any{rawJSON(data)},
	}
	return out, nil
}

func getDiffChunk(store *artifacts.Store, in *EvidenceGetInput) (*EvidenceGetOutput, error) {
	if in.Path == "" {
		return nil, errEvidence("path obrigatório")
	}
	data, err := store.Read("diff/" + in.Path)
	if err != nil {
		return nil, errEvidence("diff chunk missing: %v", err)
	}
	out := &EvidenceGetOutput{
		Kind:  KindDiff,
		RunID: in.RunID,
		Items: []any{rawJSON(data)},
	}
	// Pagination: chunk é arquivo único; truncated if > 100KB.
	if len(data) > 100*1024 {
		out.Truncated = true
		out.NextOffset = 100 * 1024
	}
	out.Total = 1
	return out, nil
}

func getContext(store *artifacts.Store, in *EvidenceGetInput) (*EvidenceGetOutput, error) {
	if in.Path == "" {
		return nil, errEvidence("path obrigatório")
	}
	data, err := store.Read("context/" + in.Path)
	if err != nil {
		return nil, errEvidence("context missing: %v", err)
	}
	lines := splitLines(string(data))
	total := len(lines)
	start := in.Offset
	if start >= total {
		start = total
	}
	end := start + in.Limit
	if end > total {
		end = total
	}
	page := lines[start:end]
	out := &EvidenceGetOutput{
		Kind:      KindContext,
		RunID:     in.RunID,
		Items:     anySlice(page),
		Total:     total,
		Truncated: end < total,
	}
	if end < total {
		out.NextOffset = end
	}
	return out, nil
}

func getSymbolSearch(store *artifacts.Store, in *EvidenceGetInput) (*EvidenceGetOutput, error) {
	if in.Query == "" {
		return nil, errEvidence("query obrigatório")
	}
	data, err := store.Read("symbols.json")
	if err != nil {
		return nil, errEvidence("symbols missing: %v", err)
	}
	// Trivial search: split por linha, match substring.
	lines := splitLines(string(data))
	var matches []any
	for i, line := range lines {
		if in.Offset > 0 && i < in.Offset {
			continue
		}
		if len(matches) >= in.Limit {
			break
		}
		if strings.Contains(line, in.Query) {
			matches = append(matches, line)
		}
	}
	out := &EvidenceGetOutput{
		Kind:      KindSymbols,
		RunID:     in.RunID,
		Items:     matches,
		Total:     len(matches),
		Truncated: len(matches) >= in.Limit,
	}
	return out, nil
}

func getAnalyzerResult(store *artifacts.Store, in *EvidenceGetInput) (*EvidenceGetOutput, error) {
	data, err := store.Read("analyzer/" + in.Path + ".json")
	if err != nil {
		return nil, errEvidence("analyzer result missing: %v", err)
	}
	out := &EvidenceGetOutput{
		Kind:  KindAnalyzer,
		RunID: in.RunID,
		Items: []any{rawJSON(data)},
		Total: 1,
	}
	return out, nil
}

func getReleaseInventory(store *artifacts.Store, in *EvidenceGetInput) (*EvidenceGetOutput, error) {
	data, err := store.Read("release-inventory.json")
	if err != nil {
		return nil, errEvidence("release inventory missing: %v", err)
	}
	out := &EvidenceGetOutput{
		Kind:  KindRelease,
		RunID: in.RunID,
		Items: []any{rawJSON(data)},
		Total: 1,
	}
	return out, nil
}

// rawJSON wrapper que retorna map[string]any se for JSON válido,
// senão string.
func rawJSON(data []byte) any {
	data = trim(data)
	if len(data) == 0 {
		return nil
	}
	c := data[0]
	if c == '{' || c == '[' {
		// best-effort parse via string; SDK não falha aqui.
		return map[string]any{"raw": string(data)}
	}
	return string(data)
}

func trim(b []byte) []byte {
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\t' || b[start] == '\n' || b[start] == '\r') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\n' || b[end-1] == '\r') {
		end--
	}
	return b[start:end]
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func anySlice(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}
