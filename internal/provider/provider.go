package provider

import "context"

// EventType identifies a streaming event.
type EventType int

const (
	EventTextDelta    EventType = iota // Partial text content
	EventToolUseStart                  // Tool call begins (has ID, name)
	EventToolUseDelta                  // Partial tool input JSON
	EventDone                          // Stream complete, Usage populated
	EventError                         // Stream error
	EventToolResult                    // Tool result observed (subprocess providers)
)

// StreamEvent is a single event from a streaming response.
type StreamEvent struct {
	Type       EventType
	Text       string // for TextDelta and ToolUseDelta
	ID         string // for ToolUseStart (tool_use ID)
	Name       string // for ToolUseStart (tool name)
	Usage      *Usage // for EventDone
	Err        error  // for EventError
	StopReason string // for EventDone: "end_turn", "tool_use", "max_tokens"
}

// Provider is the LLM inference interface.
type Provider interface {
	// Stream sends a request and returns a streaming response.
	Stream(ctx context.Context, messages []Message, tools []ToolDef, config Config) (*StreamResponse, error)

	// Name returns the provider identifier (e.g., "anthropic", "claude-code").
	Name() string
}
