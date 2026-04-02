package agentloop

import (
	"context"
	"encoding/json"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// SelectionHints carries optional context for model routing.
// Phase is an opaque string — empty for non-phased consumers.
type SelectionHints struct {
	Phase    string // optional — empty for non-phased consumers
	Urgency  string // "interactive", "batch", "background"
	TaskType string // "code", "chat", "analysis"
}

// PromptHints carries optional context for system prompt generation.
type PromptHints struct {
	Phase     string // optional
	Budget    int
	Model     string
	PlanMode  bool
	TurnCount int // current turn number (1-indexed)
}

// Router selects which model to use per turn and tracks token budget.
type Router interface {
	SelectModel(hints SelectionHints) (model string, reason string)
	RecordUsage(usage provider.Usage)
	BudgetState() BudgetState
	ContextWindow(model string) int
}

// BudgetState holds budget tracking information.
type BudgetState struct {
	Spent      int
	Max        int
	Percentage float64
}

// Session persists conversation state.
type Session interface {
	SystemPrompt(hints PromptHints) string
	Save(turn Turn) error
	Messages() []provider.Message
}

// Emitter receives structured evidence per turn.
type Emitter interface {
	Emit(event Evidence) error
}

// MessageReplacer is an optional Session interface for sessions that support
// direct message replacement. After auto-compaction modifies the in-loop
// message slice, the loop syncs the session via this interface.
type MessageReplacer interface {
	ReplaceMessages(messages []provider.Message)
}

// RenderReporter provides prompt composition metadata for evidence emission.
// Implemented by PriomptSession; checked via type assertion in the agent loop.
type RenderReporter interface {
	ExcludedElements() []string
	ExcludedStableElements() []string
	PromptTokens() int
	RenderStableTokens() int
}

// Turn captures one loop iteration for session persistence.
type Turn struct {
	Phase     string // opaque — set by agent layer for OODARC, empty for non-phased
	Messages  []provider.Message
	Usage     provider.Usage
	ToolCalls int
}

// FileActivity records a file operation observed during a turn.
type FileActivity struct {
	Path      string `json:"path"`
	Operation string `json:"op"` // "read", "write", "edit"
}

// FailureType classifies the kind of failure in a turn.
// Enables typed failure learning in the Reflect phase — the agent can
// distinguish "wrong plan" from "wrong tool usage" from "hallucinated path"
// and adjust its strategy accordingly.
type FailureType string

const (
	FailNone          FailureType = ""              // success — no failure
	FailToolError     FailureType = "tool_error"    // tool execution failed (timeout, permission, crash)
	FailTestFailure   FailureType = "test_failure"  // tests failed after edit (nonzero exit from test runner)
	FailHallucination FailureType = "hallucination" // referenced nonexistent file, function, or API
	FailPlanError     FailureType = "plan_error"    // correct execution of wrong plan (tests pass but wrong behavior)
	FailSyntaxError   FailureType = "syntax_error"  // produced invalid code (compile error, parse error)
)

// Evidence captures one turn's structured data for the reflect step.
type Evidence struct {
	Timestamp           string         `json:"timestamp"`
	SessionID           string         `json:"session_id,omitempty"`
	Phase               string         `json:"phase"`
	TurnNumber          int            `json:"turn"`
	ToolCalls           []string       `json:"tool_calls,omitempty"`
	FileActivity        []FileActivity `json:"file_activity,omitempty"`
	TokensIn            int            `json:"tokens_in"`
	TokensOut           int            `json:"tokens_out"`
	CacheCreationTokens int            `json:"cache_creation_tokens,omitempty"`
	CacheReadTokens     int            `json:"cache_read_tokens,omitempty"`
	StopReason          string         `json:"stop_reason"`
	DurationMs          int64          `json:"duration_ms,omitempty"`
	Outcome             string         `json:"outcome,omitempty"`
	Failure             FailureType    `json:"failure_type,omitempty"`
	BudgetSpent         int            `json:"budget_spent,omitempty"`
	BudgetMax           int            `json:"budget_max,omitempty"`
	BudgetPercentage    float64        `json:"budget_pct,omitempty"`
	ComplexityTier      int            `json:"complexity_tier,omitempty"`
	ComplexityOverride  bool           `json:"complexity_override,omitempty"`
	PromptTokens        int            `json:"prompt_tokens,omitempty"`
	StableTokens        int            `json:"stable_tokens,omitempty"`
	ExcludedElements    []string       `json:"excluded_elements,omitempty"`
	ExcludedStable      []string       `json:"excluded_stable,omitempty"`
	Model               string         `json:"model,omitempty"`
	ModelReason         string         `json:"model_reason,omitempty"`
}

// ToolApprover is called before executing a tool call. It blocks until
// the caller (typically the TUI) returns an approval decision. Returning
// false skips execution and feeds an error result back to the model.
type ToolApprover func(toolName string, input json.RawMessage) (allow bool)

// StreamEventType identifies the kind of stream event.
type StreamEventType int

const (
	StreamText         StreamEventType = iota // Partial text from the model
	StreamToolStart                           // A tool call has begun
	StreamToolComplete                        // A tool call has finished executing
	StreamTurnComplete                        // The turn is complete (usage available)
	StreamPhaseChange                         // The OODARC phase has changed
	StreamCompact                             // Auto-compaction was applied
)

// StreamEvent carries real-time data from the agent loop to the TUI.
type StreamEvent struct {
	Type       StreamEventType
	Text       string
	ToolName   string
	ToolParams string
	ToolResult string
	IsError    bool
	Phase      string
	Model      string // model name used for this turn/phase
	Usage      provider.Usage
	TurnNumber int

	// Compaction fields (StreamCompact only)
	TokensFreed    int // tokens recovered by compaction
	MessagesBefore int // message count before compaction
	MessagesAfter  int // message count after compaction
	PercentUsed    int // context utilization after compaction (0–100)
}

// StreamCallback receives events during the agent loop.
type StreamCallback func(StreamEvent)

// RunResult holds the outcome of a completed agent run.
type RunResult struct {
	Response string
	Usage    provider.Usage
	Turns    int
	Phase    string // opaque — set by agent layer
}

// HookRunner executes lifecycle hooks. The agentloop only uses
// PreToolUse and PostToolUse; SessionStart and Notify are called
// directly from main.go.
//
// PreToolUse returns a string decision: "allow", "deny", or "ask".
// Using string (not a typed enum) keeps agentloop decoupled from
// the hooks package — the agent layer adapts between the two.
type HookRunner interface {
	PreToolUse(ctx context.Context, toolName string, input json.RawMessage) (string, error)
	PostToolUse(ctx context.Context, toolName string, input json.RawMessage, result string, isError bool)
}

// --- NoOp implementations ---

// NoOpRouter always returns the default model.
type NoOpRouter struct{ Model string }

func (r *NoOpRouter) SelectModel(_ SelectionHints) (string, string) {
	if r.Model == "" {
		return "claude-sonnet-4-20250514", "default"
	}
	return r.Model, "configured"
}

func (r *NoOpRouter) RecordUsage(_ provider.Usage) {}

func (r *NoOpRouter) BudgetState() BudgetState { return BudgetState{} }

func (r *NoOpRouter) ContextWindow(_ string) int { return 200000 }

// NoOpSession discards all state.
type NoOpSession struct{ Prompt string }

func (s *NoOpSession) SystemPrompt(_ PromptHints) string { return s.Prompt }
func (s *NoOpSession) Save(_ Turn) error                 { return nil }
func (s *NoOpSession) Messages() []provider.Message      { return nil }

// NoOpEmitter discards all evidence.
type NoOpEmitter struct{}

func (e *NoOpEmitter) Emit(_ Evidence) error { return nil }
