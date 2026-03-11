package provider

import "encoding/json"

// Role identifies the message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single conversation turn.
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock is a polymorphic content element.
type ContentBlock struct {
	Type  string          `json:"type"`            // "text", "tool_use", "tool_result"
	Text  string          `json:"text,omitempty"`  // text content
	ID    string          `json:"id,omitempty"`    // tool_use ID
	Name  string          `json:"name,omitempty"`  // tool name
	Input json.RawMessage `json:"input,omitempty"` // tool_use input (raw JSON)

	// tool_result fields
	ToolUseID      string `json:"tool_use_id,omitempty"`
	ResultContent  string `json:"content,omitempty"` // tool result text
	IsError        bool   `json:"is_error,omitempty"`
}

// ToolDef describes a tool available to the model.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Config holds per-request settings.
type Config struct {
	Model       string
	MaxTokens   int
	Temperature float64 // -1 means use provider default
	System      string  // system prompt
}

// Usage tracks token consumption for a single response.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}
