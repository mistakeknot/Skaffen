package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/mistakeknot/Skaffen/internal/sandbox"
)

// GateConstraint defines how a tool is gated in a specific phase.
// A nil constraint means the tool is fully allowed (no restrictions).
type GateConstraint struct {
	// AllowedGlobs restricts the tool to files matching these patterns.
	// Empty means all files allowed. Matched against the basename.
	AllowedGlobs []string

	// RateLimit is the max number of calls allowed per phase session.
	// 0 means unlimited.
	RateLimit int

	// RequirePrompt forces the trust evaluator to always prompt for this tool.
	RequirePrompt bool
}

// MatchesPath checks if a file path is allowed by the constraint's glob patterns.
// Returns true if no globs are set (unconstrained) or if any glob matches.
func (gc *GateConstraint) MatchesPath(filePath string) bool {
	if gc == nil || len(gc.AllowedGlobs) == 0 {
		return true
	}
	base := filepath.Base(filePath)
	for _, pattern := range gc.AllowedGlobs {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// manifestGlobs are file patterns allowed for edit/write during Ship phase.
var manifestGlobs = []string{
	"*.md", "CHANGELOG*", "VERSION*", "*.json", "*.yaml", "*.yml", "*.toml", "*.txt",
}

// defaultGates defines which tools are available per phase with constraints.
var defaultGates = map[Phase]map[string]*GateConstraint{
	PhaseBrainstorm: {
		"read": nil, "glob": nil, "grep": nil, "ls": nil,
	},
	PhasePlan: {
		"read": nil, "glob": nil, "grep": nil, "ls": nil,
	},
	PhaseBuild: {
		"read": nil, "write": nil, "edit": nil, "bash": nil,
		"grep": nil, "glob": nil, "ls": nil,
	},
	PhaseReview: {
		"read": nil, "glob": nil, "grep": nil, "ls": nil, "bash": nil,
		"edit": {RateLimit: 3, RequirePrompt: true},
	},
	PhaseShip: {
		"read": nil, "glob": nil, "ls": nil, "bash": nil,
		"edit":  {AllowedGlobs: manifestGlobs},
		"write": {AllowedGlobs: manifestGlobs},
	},
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
	tools      map[string]Tool
	gates      map[Phase]map[string]*GateConstraint
	planMode   bool
	sandbox    *sandbox.Sandbox
	rateCounts map[Phase]map[string]int // per-phase rate limit counters
	mu         sync.Mutex              // protects rateCounts
}

// SetSandbox configures the sandbox used for in-process tool path validation.
func (r *Registry) SetSandbox(s *sandbox.Sandbox) { r.sandbox = s }

// NewRegistry creates a registry with the default phase gate matrix.
func NewRegistry() *Registry {
	r := &Registry{
		tools:      make(map[string]Tool),
		gates:      make(map[Phase]map[string]*GateConstraint),
		rateCounts: make(map[Phase]map[string]int),
	}
	for phase, constraints := range defaultGates {
		m := make(map[string]*GateConstraint, len(constraints))
		for name, gc := range constraints {
			m[name] = gc
		}
		r.gates[phase] = m
	}
	return r
}

// ResetRateCounts clears rate limit counters (call on phase transition).
func (r *Registry) ResetRateCounts() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rateCounts = make(map[Phase]map[string]int)
}

// Register adds a tool to the registry. It is automatically allowed in
// the build phase (extension point for MCP tools in v0.2).
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
	// Ensure dynamically registered tools are available in build phase
	if r.gates[PhaseBuild] == nil {
		r.gates[PhaseBuild] = make(map[string]*GateConstraint)
	}
	r.gates[PhaseBuild][t.Name()] = nil // unconstrained
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
			r.gates[phase] = make(map[string]*GateConstraint)
		}
		r.gates[phase][t.Name()] = nil // unconstrained
	}
}

// SetPlanMode enables or disables plan mode. When active, only read-only
// tools (read, glob, grep, ls) are available regardless of phase.
// Must not be called while Agent.Run is executing (guarded by TUI's !m.running).
func (r *Registry) SetPlanMode(on bool) { r.planMode = on }

// PlanMode returns whether plan mode is active.
func (r *Registry) PlanMode() bool { return r.planMode }

// Tools returns tool definitions available for the given phase.
// In plan mode, returns only tools that are both read-only AND phase-allowed.
func (r *Registry) Tools(phase Phase) []ToolDef {
	allowed := r.gates[phase]
	var defs []ToolDef
	for name, t := range r.tools {
		_, gated := allowed[name]
		if r.planMode {
			if !readOnlyTools[name] || !gated {
				continue
			}
		} else if !gated {
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

// Constraint returns the gate constraint for a tool in a phase.
// Returns nil, false if the tool is not gated for that phase.
func (r *Registry) Constraint(phase Phase, toolName string) (*GateConstraint, bool) {
	allowed := r.gates[phase]
	gc, ok := allowed[toolName]
	return gc, ok
}

// Execute runs a tool by name if it's allowed in the given phase.
// In plan mode, the tool must be both read-only AND phase-allowed.
// The plan mode check is defence-in-depth: Tools() already filters,
// but Execute() enforces independently for direct callers.
// Phase softening: constrained tools enforce file-glob and rate-limit checks.
func (r *Registry) Execute(ctx context.Context, phase Phase, name string, params json.RawMessage) ToolResult {
	if r.planMode && !readOnlyTools[name] {
		return ToolResult{
			Content: fmt.Sprintf("tool %q not available in plan mode (read-only)", name),
			IsError: true,
		}
	}
	allowed := r.gates[phase]
	gc, gated := allowed[name]
	if !gated {
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

	// Phase softening: enforce gate constraints (file globs, rate limits).
	if gc != nil {
		// File-glob constraint: check if the target file matches allowed patterns.
		if len(gc.AllowedGlobs) > 0 {
			filePath := extractFilePath(name, params)
			if filePath == "" {
				return ToolResult{
					Content: fmt.Sprintf("tool %q in phase %q requires a file path matching %v", name, phase, gc.AllowedGlobs),
					IsError: true,
				}
			}
			if !gc.MatchesPath(filePath) {
				return ToolResult{
					Content: fmt.Sprintf("tool %q blocked in phase %q: file %q does not match allowed patterns %v (manifests only)", name, phase, filepath.Base(filePath), gc.AllowedGlobs),
					IsError: true,
				}
			}
		}

		// Rate-limit constraint: check call count.
		if gc.RateLimit > 0 {
			r.mu.Lock()
			if r.rateCounts[phase] == nil {
				r.rateCounts[phase] = make(map[string]int)
			}
			count := r.rateCounts[phase][name]
			if count >= gc.RateLimit {
				r.mu.Unlock()
				return ToolResult{
					Content: fmt.Sprintf("tool %q rate limit reached in phase %q (%d/%d calls)", name, phase, count, gc.RateLimit),
					IsError: true,
				}
			}
			r.rateCounts[phase][name] = count + 1
			r.mu.Unlock()
		}
	}

	// Sandbox path validation for file-accessing tools.
	// Bash tool handles its own sandboxing via WrapArgs.
	if r.sandbox != nil && !r.sandbox.Disabled() && name != "bash" {
		if filePath := extractFilePath(name, params); filePath != "" {
			if err := r.sandbox.CheckPath(filePath, isWriteTool(name)); err != nil {
				return ToolResult{
					Content: fmt.Sprintf("sandbox: %v", err),
					IsError: true,
				}
			}
		}
	}

	// If the tool implements PhasedTool, pass the phase for phase-aware behavior.
	if pt, ok := t.(PhasedTool); ok {
		return pt.ExecuteWithPhase(ctx, phase, params)
	}
	return t.Execute(ctx, params)
}

// Get returns a tool by name (ignoring phase).
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// writeTools are tools that modify files.
var writeTools = map[string]bool{"write": true, "edit": true}

func isWriteTool(name string) bool { return writeTools[name] }

// extractFilePath pulls the file_path or path param from tool input JSON.
// Returns empty string if not a file-accessing tool or no path found.
func extractFilePath(toolName string, params json.RawMessage) string {
	switch toolName {
	case "read", "write", "edit":
		var p struct {
			FilePath string `json:"file_path"`
		}
		json.Unmarshal(params, &p)
		return p.FilePath
	case "grep", "glob":
		var p struct {
			Path string `json:"path"`
		}
		json.Unmarshal(params, &p)
		return p.Path
	default:
		return ""
	}
}
