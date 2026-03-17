package repomap

import (
	"context"
	"encoding/json"
	"time"
)

// ToolCaller abstracts MCP tool invocation so repomap doesn't import
// the mcp package directly (avoiding circular imports).
type ToolCaller interface {
	CallTool(ctx context.Context, name string, args map[string]any) ([]byte, error)
}

// MCPEdgeFetcher implements EdgeFetcher by calling the intermap
// reference_edges MCP tool via a ToolCaller.
type MCPEdgeFetcher struct {
	Caller    ToolCaller
	MaxFiles  int
}

// FetchEdges calls the reference_edges MCP tool and parses the response.
// Returns nil, nil, nil if the caller is nil (graceful degradation).
func (f *MCPEdgeFetcher) FetchEdges(projectRoot string) ([]TagDef, []RefEdge, error) {
	if f.Caller == nil {
		return nil, nil, nil
	}

	maxFiles := f.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 500
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := f.Caller.CallTool(ctx, "reference_edges", map[string]any{
		"project":   projectRoot,
		"language":  "auto",
		"max_files": maxFiles,
	})
	if err != nil {
		return nil, nil, nil // degrade silently
	}

	return ParseMCPResponse(data)
}

// EdgeFetcher provides cross-file reference edges from an external source
// (e.g. intermap MCP's reference_edges tool). Implementations should return
// nil, nil, nil if the source is unavailable — this triggers graceful
// degradation to ExtractGoTags.
type EdgeFetcher interface {
	FetchEdges(projectRoot string) ([]TagDef, []RefEdge, error)
}

// MCPEdgeResponse matches the reference_edges MCP tool output schema.
type MCPEdgeResponse struct {
	Definitions []mcpDef  `json:"definitions"`
	Edges       []mcpEdge `json:"edges"`
	// Extra fields from the tool response (ignored but tolerated).
	FilesScanned int `json:"files_scanned"`
	EdgeCount    int `json:"edge_count"`
}

type mcpDef struct {
	File  string `json:"file"`
	Name  string `json:"name"`
	Line  int    `json:"line"`
	Kind  string `json:"kind"`
	Scope string `json:"scope"`
}

type mcpEdge struct {
	SrcFile   string `json:"src_file"`
	SrcSymbol string `json:"src_symbol"`
	DstFile   string `json:"dst_file"`
	DstSymbol string `json:"dst_symbol"`
}

// ParseMCPResponse converts the reference_edges MCP tool result JSON into
// TagDef and RefEdge slices. Returns an error if the JSON is malformed.
// Returns empty slices (not nil) if the response has no definitions/edges.
func ParseMCPResponse(data []byte) ([]TagDef, []RefEdge, error) {
	var resp MCPEdgeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, nil, err
	}

	defs := make([]TagDef, len(resp.Definitions))
	for i, d := range resp.Definitions {
		defs[i] = TagDef{
			File:  d.File,
			Name:  d.Name,
			Line:  d.Line,
			Kind:  d.Kind,
			Scope: d.Scope,
		}
	}

	edges := make([]RefEdge, len(resp.Edges))
	for i, e := range resp.Edges {
		edges[i] = RefEdge{
			SrcFile: e.SrcFile,
			DstFile: e.DstFile,
			Symbol:  e.DstSymbol,
		}
	}

	return defs, edges, nil
}
