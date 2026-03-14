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

	// Phase softening: Review includes edit (rate-limited to 3 calls)
	want := []string{"read", "glob", "grep", "ls", "bash", "edit"}
	notWant := []string{"write"}

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

	// Verify constraint properties
	gc, ok := r.Constraint(PhaseReview, "edit")
	if !ok {
		t.Fatal("review: edit should be gated")
	}
	if gc == nil {
		t.Fatal("review: edit should have constraints")
	}
	if gc.RateLimit != 3 {
		t.Errorf("review edit rate limit = %d, want 3", gc.RateLimit)
	}
	if !gc.RequirePrompt {
		t.Error("review edit should require prompt")
	}
}

func TestRegistry_ShipPhase(t *testing.T) {
	r := newRegistryWithStubs()
	names := toolNames(r.Tools(PhaseShip))

	// Phase softening: Ship includes edit/write (manifest globs only)
	want := []string{"read", "glob", "ls", "bash", "edit", "write"}
	notWant := []string{"grep"}

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

	// Verify constraints
	for _, tool := range []string{"edit", "write"} {
		gc, ok := r.Constraint(PhaseShip, tool)
		if !ok {
			t.Errorf("ship: %s should be gated", tool)
			continue
		}
		if gc == nil || len(gc.AllowedGlobs) == 0 {
			t.Errorf("ship: %s should have file glob constraints", tool)
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

func TestRegistry_RegisterForPhases(t *testing.T) {
	r := NewRegistry()
	custom := &stubTool{name: "mcp_search"}
	r.RegisterForPhases(custom, []Phase{PhaseBrainstorm, PhaseBuild})

	// Available in brainstorm
	names := toolNames(r.Tools(PhaseBrainstorm))
	if !names["mcp_search"] {
		t.Error("mcp_search should be in brainstorm")
	}

	// Available in build
	names = toolNames(r.Tools(PhaseBuild))
	if !names["mcp_search"] {
		t.Error("mcp_search should be in build")
	}

	// NOT available in review
	names = toolNames(r.Tools(PhaseReview))
	if names["mcp_search"] {
		t.Error("mcp_search should not be in review")
	}

	// NOT available in ship
	names = toolNames(r.Tools(PhaseShip))
	if names["mcp_search"] {
		t.Error("mcp_search should not be in ship")
	}
}

func TestRegistry_RegisterForPhases_Empty(t *testing.T) {
	r := NewRegistry()
	custom := &stubTool{name: "mcp_nophase"}
	r.RegisterForPhases(custom, nil)

	// Default: build only
	names := toolNames(r.Tools(PhaseBuild))
	if !names["mcp_nophase"] {
		t.Error("nil phases should default to build")
	}

	names = toolNames(r.Tools(PhaseBrainstorm))
	if names["mcp_nophase"] {
		t.Error("should not be in brainstorm with nil phases")
	}
}

// --- Plan mode tests ---

func TestPlanMode_ToolsFiltered(t *testing.T) {
	r := newRegistryWithStubs()
	r.SetPlanMode(true)

	// Phases that include all four read-only tools in their gates
	for _, phase := range []Phase{PhaseBrainstorm, PhasePlan, PhaseBuild, PhaseReview} {
		names := toolNames(r.Tools(phase))
		for _, want := range []string{"read", "glob", "grep", "ls"} {
			if !names[want] {
				t.Errorf("plan mode %s: missing %q", phase, want)
			}
		}
		for _, block := range []string{"write", "edit", "bash"} {
			if names[block] {
				t.Errorf("plan mode %s: should not have %q", phase, block)
			}
		}
	}
}

func TestPlanMode_RespectsPhaseGates(t *testing.T) {
	r := newRegistryWithStubs()
	r.SetPlanMode(true)

	// PhaseShip excludes "grep" — plan mode must not re-enable it
	names := toolNames(r.Tools(PhaseShip))
	if names["grep"] {
		t.Error("plan mode in ship phase should not include grep (excluded by phase gate)")
	}
	// But "read", "glob", "ls" are both read-only AND phase-allowed
	for _, want := range []string{"read", "glob", "ls"} {
		if !names[want] {
			t.Errorf("plan mode ship: missing %q", want)
		}
	}
	// Write tools still blocked
	for _, block := range []string{"write", "edit", "bash"} {
		if names[block] {
			t.Errorf("plan mode ship: should not have %q", block)
		}
	}
}

func TestPlanMode_ExecuteBlocked(t *testing.T) {
	r := newRegistryWithStubs()
	r.SetPlanMode(true)

	for _, tool := range []string{"write", "edit", "bash"} {
		result := r.Execute(context.Background(), PhaseBuild, tool, nil)
		if !result.IsError {
			t.Errorf("plan mode: %q should be blocked", tool)
		}
		if result.Content == "" {
			t.Errorf("plan mode: %q error should have message", tool)
		}
	}
}

func TestPlanMode_ExecuteAllowed(t *testing.T) {
	r := newRegistryWithStubs()
	r.SetPlanMode(true)

	for _, tool := range []string{"read", "glob", "grep", "ls"} {
		result := r.Execute(context.Background(), PhaseBuild, tool, nil)
		if result.IsError {
			t.Errorf("plan mode: %q should be allowed, got: %s", tool, result.Content)
		}
	}
}

func TestPlanMode_Toggle(t *testing.T) {
	r := newRegistryWithStubs()

	// Default: plan mode off
	if r.PlanMode() {
		t.Error("plan mode should be off by default")
	}
	names := toolNames(r.Tools(PhaseBuild))
	if !names["write"] {
		t.Error("write should be available with plan mode off")
	}

	// Enable plan mode
	r.SetPlanMode(true)
	if !r.PlanMode() {
		t.Error("plan mode should be on")
	}
	names = toolNames(r.Tools(PhaseBuild))
	if names["write"] {
		t.Error("write should not be available with plan mode on")
	}

	// Disable plan mode
	r.SetPlanMode(false)
	if r.PlanMode() {
		t.Error("plan mode should be off again")
	}
	names = toolNames(r.Tools(PhaseBuild))
	if !names["write"] {
		t.Error("write should be available after disabling plan mode")
	}
}

func TestPlanMode_MCPToolsBlocked(t *testing.T) {
	r := NewRegistry()
	// Register a read-only tool and a write-capable MCP tool
	r.Register(&stubTool{name: "read"})
	mcp := &stubTool{name: "mcp_deploy"}
	r.RegisterForPhases(mcp, []Phase{PhaseBuild})

	r.SetPlanMode(true)

	names := toolNames(r.Tools(PhaseBuild))
	if names["mcp_deploy"] {
		t.Error("MCP tool should be blocked in plan mode")
	}
	if !names["read"] {
		t.Error("read should be available in plan mode")
	}
}

// --- PhasedTool tests ---

// mockPhasedTool records the phase it was called with.
type mockPhasedTool struct {
	calledPhase Phase
}

func (m *mockPhasedTool) Name() string                                  { return "mock_phased" }
func (m *mockPhasedTool) Description() string                           { return "test phased tool" }
func (m *mockPhasedTool) Schema() json.RawMessage                       { return json.RawMessage(`{}`) }
func (m *mockPhasedTool) Execute(_ context.Context, _ json.RawMessage) ToolResult {
	return ToolResult{Content: "non-phased"}
}
func (m *mockPhasedTool) ExecuteWithPhase(_ context.Context, phase Phase, _ json.RawMessage) ToolResult {
	m.calledPhase = phase
	return ToolResult{Content: "phased:" + string(phase)}
}

// mockPlainTool is a non-phased tool for comparison.
type mockPlainTool struct{}

func (m *mockPlainTool) Name() string                            { return "mock_plain" }
func (m *mockPlainTool) Description() string                     { return "test plain tool" }
func (m *mockPlainTool) Schema() json.RawMessage                 { return json.RawMessage(`{}`) }
func (m *mockPlainTool) Execute(_ context.Context, _ json.RawMessage) ToolResult {
	return ToolResult{Content: "plain"}
}

func TestRegistryCallsPhasedTool(t *testing.T) {
	r := NewRegistry()
	phased := &mockPhasedTool{}
	r.RegisterForPhases(phased, []Phase{PhaseBrainstorm, PhaseBuild})

	result := r.Execute(context.Background(), PhaseBrainstorm, "mock_phased", json.RawMessage(`{}`))
	if result.Content != "phased:brainstorm" {
		t.Errorf("expected 'phased:brainstorm', got %q", result.Content)
	}
	if phased.calledPhase != PhaseBrainstorm {
		t.Errorf("expected phase brainstorm, got %q", phased.calledPhase)
	}
}

func TestRegistryCallsPhasedToolBuildPhase(t *testing.T) {
	r := NewRegistry()
	phased := &mockPhasedTool{}
	r.RegisterForPhases(phased, []Phase{PhaseBrainstorm, PhaseBuild})

	result := r.Execute(context.Background(), PhaseBuild, "mock_phased", json.RawMessage(`{}`))
	if result.Content != "phased:build" {
		t.Errorf("expected 'phased:build', got %q", result.Content)
	}
}

func TestRegistryCallsPlainToolUnchanged(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlainTool{})

	result := r.Execute(context.Background(), PhaseBuild, "mock_plain", json.RawMessage(`{}`))
	if result.Content != "plain" {
		t.Errorf("expected 'plain', got %q", result.Content)
	}
}

func TestPlanMode_ErrorMessage(t *testing.T) {
	r := newRegistryWithStubs()
	r.SetPlanMode(true)

	result := r.Execute(context.Background(), PhaseBuild, "write", nil)
	if !result.IsError {
		t.Fatal("expected error")
	}
	want := `tool "write" not available in plan mode (read-only)`
	if result.Content != want {
		t.Errorf("error message = %q, want %q", result.Content, want)
	}
}

// --- Phase softening tests ---

func TestSoftening_ReviewEditRateLimit(t *testing.T) {
	r := newRegistryWithStubs()

	// First 3 calls should succeed
	for i := 0; i < 3; i++ {
		result := r.Execute(context.Background(), PhaseReview, "edit", json.RawMessage(`{"file_path":"/tmp/test.go","old_string":"a","new_string":"b"}`))
		if result.IsError {
			t.Errorf("call %d: unexpected error: %s", i+1, result.Content)
		}
	}

	// 4th call should be rate-limited
	result := r.Execute(context.Background(), PhaseReview, "edit", json.RawMessage(`{"file_path":"/tmp/test.go","old_string":"a","new_string":"b"}`))
	if !result.IsError {
		t.Error("4th edit call should be rate-limited")
	}
	if result.Content == "" || result.Content == "executed edit" {
		t.Error("expected rate limit error message")
	}
}

func TestSoftening_ReviewEditRateLimitReset(t *testing.T) {
	r := newRegistryWithStubs()

	// Use 3 calls
	for i := 0; i < 3; i++ {
		r.Execute(context.Background(), PhaseReview, "edit", json.RawMessage(`{"file_path":"/tmp/test.go","old_string":"a","new_string":"b"}`))
	}

	// Reset counters (simulates phase transition)
	r.ResetRateCounts()

	// Should work again
	result := r.Execute(context.Background(), PhaseReview, "edit", json.RawMessage(`{"file_path":"/tmp/test.go","old_string":"a","new_string":"b"}`))
	if result.IsError {
		t.Errorf("after reset, edit should work: %s", result.Content)
	}
}

func TestSoftening_ShipEditManifestAllowed(t *testing.T) {
	r := newRegistryWithStubs()

	// Editing a markdown file should work in Ship phase
	result := r.Execute(context.Background(), PhaseShip, "edit", json.RawMessage(`{"file_path":"/project/README.md","old_string":"old","new_string":"new"}`))
	if result.IsError {
		t.Errorf("ship edit *.md should be allowed: %s", result.Content)
	}

	// Editing a CHANGELOG should work
	result = r.Execute(context.Background(), PhaseShip, "write", json.RawMessage(`{"file_path":"/project/CHANGELOG.md","content":"new"}`))
	if result.IsError {
		t.Errorf("ship write CHANGELOG should be allowed: %s", result.Content)
	}

	// Editing a JSON file should work
	result = r.Execute(context.Background(), PhaseShip, "edit", json.RawMessage(`{"file_path":"/project/package.json","old_string":"1.0","new_string":"1.1"}`))
	if result.IsError {
		t.Errorf("ship edit *.json should be allowed: %s", result.Content)
	}
}

func TestSoftening_ShipEditCodeBlocked(t *testing.T) {
	r := newRegistryWithStubs()

	// Editing a Go file should be blocked in Ship phase
	result := r.Execute(context.Background(), PhaseShip, "edit", json.RawMessage(`{"file_path":"/project/main.go","old_string":"old","new_string":"new"}`))
	if !result.IsError {
		t.Error("ship edit *.go should be blocked")
	}

	// Editing a Python file should be blocked
	result = r.Execute(context.Background(), PhaseShip, "write", json.RawMessage(`{"file_path":"/project/app.py","content":"code"}`))
	if !result.IsError {
		t.Error("ship write *.py should be blocked")
	}

	// Editing a TypeScript file should be blocked
	result = r.Execute(context.Background(), PhaseShip, "edit", json.RawMessage(`{"file_path":"/project/index.ts","old_string":"a","new_string":"b"}`))
	if !result.IsError {
		t.Error("ship edit *.ts should be blocked")
	}
}

func TestSoftening_BuildPhaseUnconstrained(t *testing.T) {
	r := newRegistryWithStubs()

	// Build phase should have no constraints on edit/write
	result := r.Execute(context.Background(), PhaseBuild, "edit", json.RawMessage(`{"file_path":"/project/main.go","old_string":"a","new_string":"b"}`))
	if result.IsError {
		t.Errorf("build edit should be unconstrained: %s", result.Content)
	}

	result = r.Execute(context.Background(), PhaseBuild, "write", json.RawMessage(`{"file_path":"/project/main.go","content":"code"}`))
	if result.IsError {
		t.Errorf("build write should be unconstrained: %s", result.Content)
	}
}

func TestGateConstraint_MatchesPath(t *testing.T) {
	tests := []struct {
		name  string
		gc    *GateConstraint
		path  string
		match bool
	}{
		{"nil constraint", nil, "/any/path.go", true},
		{"empty globs", &GateConstraint{}, "/any/path.go", true},
		{"md match", &GateConstraint{AllowedGlobs: []string{"*.md"}}, "/project/README.md", true},
		{"md no match", &GateConstraint{AllowedGlobs: []string{"*.md"}}, "/project/main.go", false},
		{"json match", &GateConstraint{AllowedGlobs: []string{"*.json"}}, "/project/package.json", true},
		{"changelog match", &GateConstraint{AllowedGlobs: []string{"CHANGELOG*"}}, "/project/CHANGELOG.md", true},
		{"version match", &GateConstraint{AllowedGlobs: []string{"VERSION*"}}, "/project/VERSION", true},
		{"multi glob", &GateConstraint{AllowedGlobs: []string{"*.md", "*.json"}}, "/project/config.json", true},
		{"multi glob no match", &GateConstraint{AllowedGlobs: []string{"*.md", "*.json"}}, "/project/main.rs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.gc.MatchesPath(tt.path)
			if got != tt.match {
				t.Errorf("MatchesPath(%q) = %v, want %v", tt.path, got, tt.match)
			}
		})
	}
}
