package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
)

// gatedStubTool implements agentloop.Tool for testing.
type gatedStubTool struct {
	name   string
	result string
}

func (s *gatedStubTool) Name() string              { return s.name }
func (s *gatedStubTool) Description() string        { return s.name + " tool" }
func (s *gatedStubTool) Schema() json.RawMessage    { return json.RawMessage(`{}`) }
func (s *gatedStubTool) Execute(_ context.Context, _ json.RawMessage) agentloop.ToolResult {
	return agentloop.ToolResult{Content: s.result}
}

func setupGatedRegistry() *GatedRegistry {
	reg := agentloop.NewRegistry()
	reg.Register(&gatedStubTool{name: "read", result: "file contents"})
	reg.Register(&gatedStubTool{name: "write", result: "written"})
	reg.Register(&gatedStubTool{name: "bash", result: "output"})
	reg.Register(&gatedStubTool{name: "glob", result: "files"})
	reg.Register(&gatedStubTool{name: "grep", result: "matches"})
	return NewGatedRegistry(reg, DefaultGates)
}

func TestGatedToolsOrientExcludesWrite(t *testing.T) {
	g := setupGatedRegistry()
	tools := g.Tools("orient")
	for _, d := range tools {
		if d.Name == "write" {
			t.Error("orient phase should not include 'write'")
		}
		if d.Name == "bash" {
			t.Error("orient phase should not include 'bash'")
		}
	}
}

func TestGatedToolsOrientIncludesRead(t *testing.T) {
	g := setupGatedRegistry()
	tools := g.Tools("orient")
	found := false
	for _, d := range tools {
		if d.Name == "read" {
			found = true
		}
	}
	if !found {
		t.Error("orient phase should include 'read'")
	}
}

func TestGatedToolsActIncludesAll(t *testing.T) {
	g := setupGatedRegistry()
	tools := g.Tools("act")
	names := make(map[string]bool)
	for _, d := range tools {
		names[d.Name] = true
	}
	for _, want := range []string{"read", "write", "bash", "glob", "grep"} {
		if !names[want] {
			t.Errorf("act phase should include %q", want)
		}
	}
}

func TestGatedExecuteOrientBlocksWrite(t *testing.T) {
	g := setupGatedRegistry()
	result := g.Execute(context.Background(), "orient", "write", json.RawMessage(`{}`))
	if !result.IsError {
		t.Error("expected error for write in orient")
	}
}

func TestGatedExecuteActAllowsWrite(t *testing.T) {
	g := setupGatedRegistry()
	result := g.Execute(context.Background(), "act", "write", json.RawMessage(`{}`))
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
	if result.Content != "written" {
		t.Errorf("Content = %q, want 'written'", result.Content)
	}
}

func TestGatedExecuteUnknownPhase(t *testing.T) {
	g := setupGatedRegistry()
	result := g.Execute(context.Background(), "unknown_phase", "read", json.RawMessage(`{}`))
	if !result.IsError {
		t.Error("expected error for unknown phase")
	}
}

func TestGatedToolsReflectIncludesBash(t *testing.T) {
	g := setupGatedRegistry()
	tools := g.Tools("reflect")
	found := false
	for _, d := range tools {
		if d.Name == "bash" {
			found = true
		}
	}
	if !found {
		t.Error("reflect phase should include 'bash'")
	}
}

func TestGatedToolsReflectExcludesWrite(t *testing.T) {
	g := setupGatedRegistry()
	tools := g.Tools("reflect")
	for _, d := range tools {
		if d.Name == "write" {
			t.Error("reflect phase should not include 'write'")
		}
	}
}

func TestGatedInner(t *testing.T) {
	g := setupGatedRegistry()
	inner := g.Inner()
	// Inner registry should have all tools regardless of phase
	all := inner.Tools()
	if len(all) != 5 {
		t.Errorf("inner registry has %d tools, want 5", len(all))
	}
}
