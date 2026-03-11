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

// Registry holds tools and gates access by phase.
type Registry struct {
	tools map[string]Tool
	gates map[Phase]map[string]bool
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

// Tools returns tool definitions available for the given phase.
func (r *Registry) Tools(phase Phase) []ToolDef {
	allowed := r.gates[phase]
	var defs []ToolDef
	for name, t := range r.tools {
		if allowed[name] {
			defs = append(defs, ToolDef{
				Name:        t.Name(),
				Description: t.Description(),
				InputSchema: t.Schema(),
			})
		}
	}
	return defs
}

// Execute runs a tool by name if it's allowed in the given phase.
func (r *Registry) Execute(ctx context.Context, phase Phase, name string, params json.RawMessage) ToolResult {
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
