package agentloop

import (
	"context"
	"encoding/json"
	"testing"
)

// stubTool implements Tool for testing.
type stubTool struct {
	name   string
	desc   string
	result string
}

func (s *stubTool) Name() string                                                  { return s.name }
func (s *stubTool) Description() string                                           { return s.desc }
func (s *stubTool) Schema() json.RawMessage                                       { return json.RawMessage(`{}`) }
func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) ToolResult {
	return ToolResult{Content: s.result}
}

func TestRegistryToolsSorted(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "grep", desc: "search"})
	r.Register(&stubTool{name: "bash", desc: "shell"})
	r.Register(&stubTool{name: "read", desc: "read files"})

	defs := r.Tools()
	if len(defs) != 3 {
		t.Fatalf("len(Tools()) = %d, want 3", len(defs))
	}
	want := []string{"bash", "grep", "read"}
	for i, d := range defs {
		if d.Name != want[i] {
			t.Errorf("Tools()[%d].Name = %q, want %q", i, d.Name, want[i])
		}
	}
}

func TestRegistryExecuteSuccess(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "read", result: "file contents"})

	result := r.Execute(context.Background(), "read", json.RawMessage(`{}`))
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
	if result.Content != "file contents" {
		t.Errorf("Content = %q, want 'file contents'", result.Content)
	}
}

func TestRegistryExecuteUnknown(t *testing.T) {
	r := NewRegistry()
	result := r.Execute(context.Background(), "nonexistent", json.RawMessage(`{}`))
	if !result.IsError {
		t.Error("expected error for unknown tool")
	}
	if result.Content == "" {
		t.Error("expected error message")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "bash"})

	if _, ok := r.Get("bash"); !ok {
		t.Error("Get('bash') should return true")
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("Get('missing') should return false")
	}
}

func TestRegistryEmpty(t *testing.T) {
	r := NewRegistry()
	defs := r.Tools()
	if len(defs) != 0 {
		t.Errorf("empty registry has %d tools", len(defs))
	}
}

func TestRegistryOverwrite(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "read", result: "v1"})
	r.Register(&stubTool{name: "read", result: "v2"})

	result := r.Execute(context.Background(), "read", json.RawMessage(`{}`))
	if result.Content != "v2" {
		t.Errorf("Content = %q, want 'v2' (latest registration)", result.Content)
	}

	defs := r.Tools()
	if len(defs) != 1 {
		t.Errorf("len(Tools()) = %d, want 1 after overwrite", len(defs))
	}
}
