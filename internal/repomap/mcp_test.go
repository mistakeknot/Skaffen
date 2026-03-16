package repomap

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mistakeknot/Masaq/priompt"
)

func TestParseMCPResponse_ValidJSON(t *testing.T) {
	data := []byte(`{
		"definitions": [
			{"file": "main.go", "name": "Run", "line": 10, "kind": "func", "scope": ""},
			{"file": "server.go", "name": "Server", "line": 5, "kind": "type", "scope": ""},
			{"file": "server.go", "name": "Start", "line": 15, "kind": "method", "scope": "*Server"}
		],
		"edges": [
			{"src_file": "cmd.go", "src_symbol": "exec", "dst_file": "main.go", "dst_symbol": "Run"},
			{"src_file": "main.go", "src_symbol": "init", "dst_file": "server.go", "dst_symbol": "Start"}
		],
		"files_scanned": 3,
		"edge_count": 2
	}`)

	defs, edges, err := ParseMCPResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defs) != 3 {
		t.Fatalf("expected 3 defs, got %d", len(defs))
	}
	if defs[0].Name != "Run" || defs[0].Kind != "func" {
		t.Errorf("defs[0] = %+v, want Run/func", defs[0])
	}
	if defs[2].Scope != "*Server" {
		t.Errorf("defs[2].Scope = %q, want *Server", defs[2].Scope)
	}

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
	if edges[0].Symbol != "Run" {
		t.Errorf("edges[0].Symbol = %q, want Run", edges[0].Symbol)
	}
	if edges[0].SrcFile != "cmd.go" || edges[0].DstFile != "main.go" {
		t.Errorf("edges[0] src/dst = %s->%s, want cmd.go->main.go", edges[0].SrcFile, edges[0].DstFile)
	}
}

func TestParseMCPResponse_EmptyResponse(t *testing.T) {
	data := []byte(`{"definitions": [], "edges": []}`)

	defs, edges, err := ParseMCPResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Errorf("expected 0 defs, got %d", len(defs))
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestParseMCPResponse_MalformedJSON(t *testing.T) {
	data := []byte(`{invalid json`)

	_, _, err := ParseMCPResponse(data)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseMCPResponse_MissingFields(t *testing.T) {
	// JSON with only definitions, no edges key — should unmarshal with zero edges
	data := []byte(`{"definitions": [{"file": "a.go", "name": "Foo", "line": 1, "kind": "func"}]}`)

	defs, edges, err := ParseMCPResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 {
		t.Errorf("expected 1 def, got %d", len(defs))
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestParseMCPResponse_ExtraFields(t *testing.T) {
	// Ensure unknown fields don't cause errors
	data := []byte(`{
		"definitions": [{"file": "x.go", "name": "X", "line": 1, "kind": "type"}],
		"edges": [],
		"files_scanned": 42,
		"edge_count": 0,
		"some_future_field": true
	}`)

	defs, _, err := ParseMCPResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "X" {
		t.Errorf("unexpected defs: %+v", defs)
	}
}

// mockFetcher implements EdgeFetcher for testing.
type mockFetcher struct {
	defs  []TagDef
	edges []RefEdge
	err   error
}

func (m *mockFetcher) FetchEdges(_ string) ([]TagDef, []RefEdge, error) {
	return m.defs, m.edges, m.err
}

func TestNewElement_WithEdgeFetcher_UsesMCPData(t *testing.T) {
	dir := t.TempDir()
	// Write a Go file so ExtractGoTags would find something if called
	writeFile(t, filepath.Join(dir, "main.go"), `package main
type Fallback struct{}
func FallbackFunc() {}
`)

	// MCP fetcher returns different data than what ExtractGoTags would find
	fetcher := &mockFetcher{
		defs: []TagDef{
			{File: "mcp_file.go", Name: "MCPSymbol", Line: 1, Kind: "func"},
		},
		edges: []RefEdge{
			{SrcFile: "caller.go", DstFile: "mcp_file.go", Symbol: "MCPSymbol"},
		},
	}

	elem := NewElement(dir, WithEdgeFetcher(fetcher))
	ctx := priompt.RenderContext{Budget: 100000, Phase: "orient"}
	content := elem.Render(ctx)

	if content == "" {
		t.Fatal("expected non-empty content")
	}
	if !strings.Contains(content, "MCPSymbol") {
		t.Errorf("expected MCP data to be used, got:\n%s", content)
	}
}

func TestNewElement_WithEdgeFetcher_FallsBackOnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), `package main
type GoFallback struct{}
func GoFunc() {}
`)

	// MCP fetcher returns an error — should fall back to ExtractGoTags
	fetcher := &mockFetcher{
		err: errors.New("intermap unavailable"),
	}

	elem := NewElement(dir, WithEdgeFetcher(fetcher))
	ctx := priompt.RenderContext{Budget: 100000, Phase: "orient"}
	content := elem.Render(ctx)

	if content == "" {
		t.Fatal("expected non-empty content from Go fallback")
	}
	if !strings.Contains(content, "GoFallback") {
		t.Errorf("expected Go fallback data, got:\n%s", content)
	}
}

func TestNewElement_WithEdgeFetcher_FallsBackOnNilData(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), `package main
type GoType struct{}
`)

	// MCP fetcher returns nil data (no error, but no data either)
	fetcher := &mockFetcher{
		defs:  nil,
		edges: nil,
		err:   nil,
	}

	elem := NewElement(dir, WithEdgeFetcher(fetcher))
	ctx := priompt.RenderContext{Budget: 100000, Phase: "orient"}
	content := elem.Render(ctx)

	if content == "" {
		t.Fatal("expected non-empty content from Go fallback")
	}
	if !strings.Contains(content, "GoType") {
		t.Errorf("expected Go fallback data, got:\n%s", content)
	}
}

func TestNewElement_WithoutEdgeFetcher_StillWorks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), `package main
type OriginalAPI struct{}
`)

	// No fetcher — original API, should use ExtractGoTags directly
	elem := NewElement(dir)
	ctx := priompt.RenderContext{Budget: 100000, Phase: "orient"}
	content := elem.Render(ctx)

	if content == "" {
		t.Fatal("expected non-empty content")
	}
	if !strings.Contains(content, "OriginalAPI") {
		t.Errorf("expected Go data, got:\n%s", content)
	}
}
