package tool

import (
	"context"
	"encoding/json"
	"testing"
)

// stubTool is a minimal Tool for registry tests.
type stubTool struct {
	name string
}

func (s *stubTool) Name() string               { return s.name }
func (s *stubTool) Description() string         { return "stub: " + s.name }
func (s *stubTool) Schema() json.RawMessage     { return json.RawMessage(`{"type":"object"}`) }
func (s *stubTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	return ToolResult{Content: "executed " + s.name}
}

func allBuiltinNames() []string {
	return []string{"read", "write", "edit", "bash", "grep", "glob", "ls"}
}

func newRegistryWithStubs() *Registry {
	r := NewRegistry()
	for _, name := range allBuiltinNames() {
		r.Register(&stubTool{name: name})
	}
	return r
}

func toolNames(defs []ToolDef) map[string]bool {
	m := make(map[string]bool)
	for _, d := range defs {
		m[d.Name] = true
	}
	return m
}

func TestRegistry_BrainstormPhase(t *testing.T) {
	r := newRegistryWithStubs()
	names := toolNames(r.Tools(PhaseBrainstorm))

	want := map[string]bool{"read": true, "glob": true, "grep": true, "ls": true}
	notWant := []string{"write", "edit", "bash"}

	for name := range want {
		if !names[name] {
			t.Errorf("brainstorm: missing %q", name)
		}
	}
	for _, name := range notWant {
		if names[name] {
			t.Errorf("brainstorm: should not have %q", name)
		}
	}
}

func TestRegistry_BuildPhase(t *testing.T) {
	r := newRegistryWithStubs()
	names := toolNames(r.Tools(PhaseBuild))

	for _, name := range allBuiltinNames() {
		if !names[name] {
			t.Errorf("build: missing %q", name)
		}
	}
	if len(names) != 7 {
		t.Errorf("build: got %d tools, want 7", len(names))
	}
}

func TestRegistry_ReviewPhase(t *testing.T) {
	r := newRegistryWithStubs()
	names := toolNames(r.Tools(PhaseReview))

	want := []string{"read", "glob", "grep", "ls", "bash"}
	notWant := []string{"write", "edit"}

	for _, name := range want {
		if !names[name] {
			t.Errorf("review: missing %q", name)
		}
	}
	for _, name := range notWant {
		if names[name] {
			t.Errorf("review: should not have %q", name)
		}
	}
}

func TestRegistry_ShipPhase(t *testing.T) {
	r := newRegistryWithStubs()
	names := toolNames(r.Tools(PhaseShip))

	want := []string{"read", "glob", "ls", "bash"}
	notWant := []string{"write", "edit", "grep"}

	for _, name := range want {
		if !names[name] {
			t.Errorf("ship: missing %q", name)
		}
	}
	for _, name := range notWant {
		if names[name] {
			t.Errorf("ship: should not have %q", name)
		}
	}
}

func TestRegistry_ExecuteDisallowed(t *testing.T) {
	r := newRegistryWithStubs()
	result := r.Execute(context.Background(), PhaseBrainstorm, "write", nil)
	if !result.IsError {
		t.Error("expected error for disallowed tool")
	}
	if result.Content == "" {
		t.Error("expected error message")
	}
}

func TestRegistry_ExecuteUnknown(t *testing.T) {
	r := newRegistryWithStubs()
	result := r.Execute(context.Background(), PhaseBuild, "nonexistent", nil)
	if !result.IsError {
		t.Error("expected error for unknown tool")
	}
}

func TestRegistry_ExecuteAllowed(t *testing.T) {
	r := newRegistryWithStubs()
	result := r.Execute(context.Background(), PhaseBuild, "read", nil)
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
	if result.Content != "executed read" {
		t.Errorf("content = %q", result.Content)
	}
}

func TestRegistry_RuntimeRegistration(t *testing.T) {
	r := NewRegistry()
	custom := &stubTool{name: "custom_mcp"}
	r.Register(custom)

	// Custom tool should be available in build phase
	names := toolNames(r.Tools(PhaseBuild))
	if !names["custom_mcp"] {
		t.Error("custom tool not in build phase")
	}

	// But not in brainstorm
	names = toolNames(r.Tools(PhaseBrainstorm))
	if names["custom_mcp"] {
		t.Error("custom tool should not be in brainstorm phase")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := newRegistryWithStubs()
	tool, ok := r.Get("read")
	if !ok {
		t.Fatal("Get('read') returned false")
	}
	if tool.Name() != "read" {
		t.Errorf("Name() = %q", tool.Name())
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Get('nonexistent') should return false")
	}
}
