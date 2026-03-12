package agent

import (
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// Router selects which model to use per turn and tracks token budget.
type Router interface {
	SelectModel(phase tool.Phase) (model string, reason string)
	RecordUsage(usage provider.Usage)
	BudgetState() (spent, max int, pct float64)
	ContextWindow(model string) int
}

// RenderReporter provides prompt composition metadata for evidence emission.
// Implemented by PriomptSession; checked via type assertion in the agent loop.
type RenderReporter interface {
	ExcludedElements() []string
	ExcludedStableElements() []string
	PromptTokens() int
	RenderStableTokens() int
}

// Session persists conversation state.
// Stubbed here — real implementation comes in F5.
type Session interface {
	// SystemPrompt returns the system prompt for the current phase.
	// The budget parameter is the estimated token budget available for the prompt.
	// Implementations that don't use priority packing can ignore the budget.
	SystemPrompt(phase tool.Phase, budget int) string
	// Save persists a turn to the session log.
	Save(turn Turn) error
	// Messages returns the conversation history (for session resume).
	Messages() []provider.Message
}

// Emitter receives structured evidence per turn.
// Stubbed here — real implementation comes in F6.
type Emitter interface {
	Emit(event Evidence) error
}

// Turn captures one loop iteration for session persistence.
type Turn struct {
	Phase     tool.Phase
	Messages  []provider.Message
	Usage     provider.Usage
	ToolCalls int
}

// Evidence captures one turn's structured data for the reflect step.
type Evidence struct {
	Timestamp  string     `json:"timestamp"`
	SessionID  string     `json:"session_id,omitempty"`
	Phase      tool.Phase `json:"phase"`
	TurnNumber int        `json:"turn"`
	ToolCalls  []string   `json:"tool_calls,omitempty"`
	TokensIn   int        `json:"tokens_in"`
	TokensOut  int        `json:"tokens_out"`
	StopReason string     `json:"stop_reason"`
	DurationMs         int64      `json:"duration_ms,omitempty"`
	Outcome            string     `json:"outcome,omitempty"` // success, failure, timeout
	BudgetSpent        int        `json:"budget_spent,omitempty"`
	BudgetMax          int        `json:"budget_max,omitempty"`
	BudgetPercentage   float64    `json:"budget_pct,omitempty"`
	ComplexityTier     int        `json:"complexity_tier,omitempty"`
	ComplexityOverride bool       `json:"complexity_override,omitempty"`
	PromptTokens       int        `json:"prompt_tokens,omitempty"`
	StableTokens       int        `json:"stable_tokens,omitempty"`
	ExcludedElements   []string   `json:"excluded_elements,omitempty"`
	ExcludedStable     []string   `json:"excluded_stable,omitempty"`
	Model              string     `json:"model,omitempty"`
	ModelReason        string     `json:"model_reason,omitempty"`
}

// NoOpRouter always returns the default model.
type NoOpRouter struct{ Model string }

func (r *NoOpRouter) SelectModel(_ tool.Phase) (string, string) {
	if r.Model == "" {
		return "claude-sonnet-4-20250514", "default"
	}
	return r.Model, "configured"
}

func (r *NoOpRouter) RecordUsage(_ provider.Usage) {}

func (r *NoOpRouter) BudgetState() (int, int, float64) { return 0, 0, 0 }

func (r *NoOpRouter) ContextWindow(_ string) int { return 200000 }

// NoOpSession discards all state.
type NoOpSession struct{ Prompt string }

func (s *NoOpSession) SystemPrompt(_ tool.Phase, _ int) string { return s.Prompt }
func (s *NoOpSession) Save(_ Turn) error                 { return nil }
func (s *NoOpSession) Messages() []provider.Message      { return nil }

// NoOpEmitter discards all evidence.
type NoOpEmitter struct{}

func (e *NoOpEmitter) Emit(_ Evidence) error { return nil }
