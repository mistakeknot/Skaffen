package agent

import (
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// Router selects which model to use per turn.
// Stubbed here — real implementation comes in F4.
type Router interface {
	SelectModel(phase tool.Phase) (model string, reason string)
}

// Session persists conversation state.
// Stubbed here — real implementation comes in F5.
type Session interface {
	// SystemPrompt returns the system prompt for the current phase.
	SystemPrompt(phase tool.Phase) string
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
	DurationMs int64      `json:"duration_ms,omitempty"`
	Outcome    string     `json:"outcome,omitempty"` // success, failure, timeout
}

// NoOpRouter always returns the default model.
type NoOpRouter struct{ Model string }

func (r *NoOpRouter) SelectModel(_ tool.Phase) (string, string) {
	if r.Model == "" {
		return "claude-sonnet-4-20250514", "default"
	}
	return r.Model, "configured"
}

// NoOpSession discards all state.
type NoOpSession struct{ Prompt string }

func (s *NoOpSession) SystemPrompt(_ tool.Phase) string  { return s.Prompt }
func (s *NoOpSession) Save(_ Turn) error                 { return nil }
func (s *NoOpSession) Messages() []provider.Message      { return nil }

// NoOpEmitter discards all evidence.
type NoOpEmitter struct{}

func (e *NoOpEmitter) Emit(_ Evidence) error { return nil }
