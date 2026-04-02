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

// ConcurrencyClassifier is optionally implemented by tools that can declare
// whether a specific invocation is safe for concurrent execution.
// Tools that do not implement this interface are assumed unsafe (serial).
// ConcurrencySafe must be safe to call from any goroutine.
type ConcurrencyClassifier interface {
	ConcurrencySafe(params json.RawMessage) bool
}

// ErrorPropagator is optionally implemented by tools whose execution errors
// should cancel sibling goroutines in a concurrent batch. Without this,
// a tool error only affects that tool's result — siblings complete normally.
type ErrorPropagator interface {
	PropagatesErrorToSiblings() bool
}

// Phase represents an OODARC workflow phase.
type Phase string

const (
	PhaseObserve  Phase = "observe"
	PhaseOrient   Phase = "orient"
	PhaseDecide   Phase = "decide"
	PhaseAct      Phase = "act"
	PhaseReflect  Phase = "reflect"
	PhaseCompound Phase = "compound"

	// Deprecated aliases — remove after all consumers migrate.
	PhaseBrainstorm = PhaseOrient
	PhasePlan       = PhaseDecide
	PhaseBuild      = PhaseAct
	PhaseReview     = PhaseReflect
	PhaseShip       = PhaseCompound
)
