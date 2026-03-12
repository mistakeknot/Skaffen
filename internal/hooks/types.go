package hooks

import "encoding/json"

// Event identifies a hook lifecycle event.
type Event string

const (
	EventSessionStart Event = "SessionStart"
	EventPreToolUse   Event = "PreToolUse"
	EventPostToolUse  Event = "PostToolUse"
	EventNotification Event = "Notification"
)

// Decision is the result of a PreToolUse hook.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
	DecisionAsk   Decision = "ask"
)

// Config is the top-level hooks configuration.
type Config struct {
	Hooks map[Event][]HookGroup `json:"hooks"`
}

// HookGroup matches a tool name pattern and runs its hooks.
type HookGroup struct {
	Matcher string    `json:"matcher"`
	Hooks   []HookDef `json:"hooks"`
}

// HookDef defines a single hook command.
type HookDef struct {
	Type    string `json:"type"`     // "command"
	Command string `json:"command"`  // shell command to execute
	Timeout int    `json:"timeout"`  // seconds, 0 = use default
	OnError string `json:"on_error"` // "allow" (default) or "deny" — what to do when hook fails/times out
}

// HookResult holds the outcome of a hook execution.
type HookResult struct {
	Decision Decision `json:"decision,omitempty"` // only for PreToolUse
	Output   string   `json:"output,omitempty"`   // stdout capture
	Error    string   `json:"error,omitempty"`     // stderr or error message
}

// PreToolUsePayload is the JSON sent to PreToolUse hooks on stdin.
type PreToolUsePayload struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// PostToolUsePayload is the JSON sent to PostToolUse hooks on stdin.
type PostToolUsePayload struct {
	ToolName   string          `json:"tool_name"`
	ToolInput  json.RawMessage `json:"tool_input"`
	ToolResult string          `json:"tool_result"`
	IsError    bool            `json:"is_error"`
}

// SessionStartPayload is the JSON sent to SessionStart hooks on stdin.
type SessionStartPayload struct {
	SessionID string `json:"session_id"`
	WorkDir   string `json:"work_dir"`
	Mode      string `json:"mode"` // "tui" or "print"
}

// NotificationPayload is the JSON sent to Notification hooks on stdin.
type NotificationPayload struct {
	EventType string `json:"event_type"`
	Message   string `json:"message"`
	Severity  string `json:"severity"` // "info", "warning", "error"
}
