package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

// mockCaller implements ToolCaller for testing.
type mockCaller struct {
	callResult CallResult
	callErr    error
}

func (m *mockCaller) CallTool(ctx context.Context, name string, arguments map[string]any) (CallResult, error) {
	return m.callResult, m.callErr
}

func TestMCPTool_ImplementsInterface(t *testing.T) {
	ti := ToolInfo{
		Name:        "search",
		Description: "Search for things",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
	}
	mt := NewMCPTool("myplugin", "myserver", ti, &mockCaller{
		callResult: CallResult{Content: "found it"},
	})

	// Verify it satisfies tool.Tool
	var _ tool.Tool = mt

	if mt.Name() != "myplugin_myserver_search" {
		t.Errorf("Name() = %q", mt.Name())
	}
	if mt.Description() != "Search for things" {
		t.Errorf("Description() = %q", mt.Description())
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(mt.Schema(), &schema); err != nil {
		t.Fatalf("Schema() not valid JSON: %v", err)
	}
}

func TestMCPTool_Execute(t *testing.T) {
	ti := ToolInfo{
		Name:        "echo",
		Description: "Echo back",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}
	mc := &mockCaller{
		callResult: CallResult{Content: "echo: hello"},
	}
	mt := NewMCPTool("test", "srv", ti, mc)

	params := json.RawMessage(`{"text":"hello"}`)
	result := mt.Execute(context.Background(), params)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content != "echo: hello" {
		t.Errorf("Content = %q", result.Content)
	}
}

func TestMCPTool_Execute_Error(t *testing.T) {
	ti := ToolInfo{Name: "fail", Description: "Fails", InputSchema: json.RawMessage(`{}`)}
	mc := &mockCaller{
		callErr: context.DeadlineExceeded,
	}
	mt := NewMCPTool("test", "srv", ti, mc)

	result := mt.Execute(context.Background(), nil)
	if !result.IsError {
		t.Error("expected IsError=true")
	}
}
