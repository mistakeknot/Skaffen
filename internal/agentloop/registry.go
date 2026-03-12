package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Tool is implemented by each built-in tool.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Execute(ctx context.Context, params json.RawMessage) ToolResult
}

// ToolResult is the output of a tool execution.
type ToolResult struct {
	Content string
	IsError bool
}

// ToolDef describes a tool for the LLM.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Registry holds tools with flat (ungated) access.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Tools returns all registered tool definitions, sorted by name for determinism.
func (r *Registry) Tools() []ToolDef {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]ToolDef, len(names))
	for i, name := range names {
		t := r.tools[name]
		defs[i] = ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.Schema(),
		}
	}
	return defs
}

// Execute runs a tool by name. Returns an error ToolResult if not found.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) ToolResult {
	t, ok := r.tools[name]
	if !ok {
		return ToolResult{
			Content: fmt.Sprintf("unknown tool %q", name),
			IsError: true,
		}
	}
	return t.Execute(ctx, input)
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}
