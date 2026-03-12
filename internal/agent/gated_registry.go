package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// DefaultGates defines which tools are available per OODARC phase.
var DefaultGates = map[string]map[string]bool{
	string(tool.PhaseBrainstorm): {"read": true, "glob": true, "grep": true, "ls": true},
	string(tool.PhasePlan):       {"read": true, "glob": true, "grep": true, "ls": true},
	string(tool.PhaseBuild):      {"read": true, "write": true, "edit": true, "bash": true, "grep": true, "glob": true, "ls": true},
	string(tool.PhaseReview):     {"read": true, "glob": true, "grep": true, "ls": true, "bash": true},
	string(tool.PhaseShip):       {"read": true, "glob": true, "ls": true, "bash": true},
}

// GatedRegistry wraps an agentloop.Registry with phase-based access gating.
type GatedRegistry struct {
	inner *agentloop.Registry
	gates map[string]map[string]bool // phase → {tool name → allowed}
}

// NewGatedRegistry creates a phase-gated registry wrapping a flat registry.
func NewGatedRegistry(inner *agentloop.Registry, gates map[string]map[string]bool) *GatedRegistry {
	return &GatedRegistry{inner: inner, gates: gates}
}

// Tools returns tool definitions filtered by phase.
func (g *GatedRegistry) Tools(phase string) []agentloop.ToolDef {
	allowed := g.gates[phase]
	all := g.inner.Tools()
	var filtered []agentloop.ToolDef
	for _, d := range all {
		if allowed[d.Name] {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// Execute runs a tool if it's allowed in the given phase.
func (g *GatedRegistry) Execute(ctx context.Context, phase, name string, input json.RawMessage) agentloop.ToolResult {
	allowed := g.gates[phase]
	if !allowed[name] {
		return agentloop.ToolResult{
			Content: fmt.Sprintf("tool %q not available in phase %q", name, phase),
			IsError: true,
		}
	}
	return g.inner.Execute(ctx, name, input)
}

// Inner returns the underlying flat registry.
func (g *GatedRegistry) Inner() *agentloop.Registry {
	return g.inner
}
