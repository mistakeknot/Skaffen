package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// defaultGates defines which tools are available per phase.
var defaultGates = map[Phase][]string{
	PhaseBrainstorm: {"read", "glob", "grep", "ls"},
	PhasePlan:       {"read", "glob", "grep", "ls"},
	PhaseBuild:      {"read", "write", "edit", "bash", "grep", "glob", "ls"},
	PhaseReview:     {"read", "glob", "grep", "ls", "bash"},
	PhaseShip:       {"read", "glob", "ls", "bash"},
}

// ToolDef describes a tool for the LLM (matches provider.ToolDef shape).
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// readOnlyTools defines the tool set available in plan mode.
var readOnlyTools = map[string]bool{
	"read": true, "glob": true, "grep": true, "ls": true,
}

// Registry holds tools and gates access by phase.
type Registry struct {
	tools    map[string]Tool
	gates    map[Phase]map[string]bool
	planMode bool
}

// NewRegistry creates a registry with the default phase gate matrix.
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
		gates: make(map[Phase]map[string]bool),
	}
	for phase, names := range defaultGates {
		m := make(map[string]bool, len(names))
		for _, name := range names {
			m[name] = true
		}
		r.gates[phase] = m
	}
	return r
}

// Register adds a tool to the registry. It is automatically allowed in
// the build phase (extension point for MCP tools in v0.2).
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
	// Ensure dynamically registered tools are available in build phase
	if r.gates[PhaseBuild] == nil {
		r.gates[PhaseBuild] = make(map[string]bool)
	}
	r.gates[PhaseBuild][t.Name()] = true
}

// RegisterForPhases adds a tool to the registry, gated to specific phases.
// If phases is nil or empty, defaults to build-only (same as Register).
func (r *Registry) RegisterForPhases(t Tool, phases []Phase) {
	r.tools[t.Name()] = t
	if len(phases) == 0 {
		phases = []Phase{PhaseBuild}
	}
	for _, phase := range phases {
		if r.gates[phase] == nil {
			r.gates[phase] = make(map[string]bool)
		}
		r.gates[phase][t.Name()] = true
	}
}

// SetPlanMode enables or disables plan mode. When active, only read-only
// tools (read, glob, grep, ls) are available regardless of phase.
// Must not be called while Agent.Run is executing (guarded by TUI's !m.running).
func (r *Registry) SetPlanMode(on bool) { r.planMode = on }

// PlanMode returns whether plan mode is active.
func (r *Registry) PlanMode() bool { return r.planMode }

// Tools returns tool definitions available for the given phase.
// In plan mode, returns only read-only tools regardless of phase gates.
func (r *Registry) Tools(phase Phase) []ToolDef {
	allowed := r.gates[phase]
	var defs []ToolDef
	for name, t := range r.tools {
		if r.planMode {
			if !readOnlyTools[name] {
				continue
			}
		} else if !allowed[name] {
			continue
		}
		defs = append(defs, ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.Schema(),
		})
	}
	return defs
}

// Execute runs a tool by name if it's allowed in the given phase.
// In plan mode, only read-only tools may execute.
func (r *Registry) Execute(ctx context.Context, phase Phase, name string, params json.RawMessage) ToolResult {
	if r.planMode && !readOnlyTools[name] {
		return ToolResult{
			Content: fmt.Sprintf("tool %q not available in plan mode (read-only)", name),
			IsError: true,
		}
	}
	allowed := r.gates[phase]
	if !allowed[name] {
		return ToolResult{
			Content: fmt.Sprintf("tool %q not available in phase %q", name, phase),
			IsError: true,
		}
	}

	t, ok := r.tools[name]
	if !ok {
		return ToolResult{
			Content: fmt.Sprintf("unknown tool %q", name),
			IsError: true,
		}
	}

	return t.Execute(ctx, params)
}

// Get returns a tool by name (ignoring phase).
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}
