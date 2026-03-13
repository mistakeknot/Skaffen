package tool

import (
	"context"
	"encoding/json"
)

// Tool is implemented by each built-in tool.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage // JSON Schema for input parameters
	Execute(ctx context.Context, params json.RawMessage) ToolResult
}

// ToolResult is the output of a tool execution.
type ToolResult struct {
	Content string // text content returned to the model
	IsError bool   // true if execution failed
}

// PhasedTool is optionally implemented by tools that need phase-aware execution.
// The registry checks for this interface and passes the current phase when available.
type PhasedTool interface {
	Tool
	ExecuteWithPhase(ctx context.Context, phase Phase, params json.RawMessage) ToolResult
}

// Phase represents an OODARC workflow phase.
type Phase string

const (
	PhaseBrainstorm Phase = "brainstorm"
	PhasePlan       Phase = "plan"
	PhaseBuild      Phase = "build"
	PhaseReview     Phase = "review"
	PhaseShip       Phase = "ship"
)
