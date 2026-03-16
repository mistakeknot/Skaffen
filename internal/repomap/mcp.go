package repomap

import (
	"encoding/json"
)

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
