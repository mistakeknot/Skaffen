package tmuxagent

// AgentAdapter defines how to spawn and interact with a specific CLI agent
// (Claude Code, Codex, Gemini, AMP, etc.) inside a tmux session.
type AgentAdapter interface {
	// Name returns the adapter identifier (e.g., "claude-code", "codex").
	Name() string

	// SpawnCmd returns the command and args to start the agent.
	// workDir is the project root the agent should operate in.
	SpawnCmd(workDir string, cfg AgentConfig) (bin string, args []string)

	// ResumeCmd returns the command and args to resume an existing session.
	// Returns empty bin if resume is not supported.
	ResumeCmd(sessionID string, workDir string, cfg AgentConfig) (bin string, args []string)

	// FormatPrompt prepares a prompt string for send-keys.
	// Some agents need escaping or special framing.
	FormatPrompt(prompt string) string

	// SessionDir returns where this agent stores session files,
	// so the CASS observer can tail the right JSONL.
	// Returns empty string if unknown (falls back to CASS index lookup).
	SessionDir() string

	// CassConnector returns the CASS connector name for this agent
	// (e.g., "claude_code", "codex", "gemini").
	// Returns empty string if no CASS connector exists (use screen scraping).
	CassConnector() string

	// SupportsResume reports whether this agent can resume sessions.
	SupportsResume() bool
}

// AgentConfig holds per-invocation settings for an agent.
type AgentConfig struct {
	Model          string // model override
	PermissionMode string // e.g., "bypassPermissions" for Claude Code
	ExtraArgs      []string
	SessionName    string // tmux session name
}

// Registry maps adapter names to AgentAdapter instances.
var adapters = map[string]AgentAdapter{}

// RegisterAdapter adds an adapter to the global registry.
func RegisterAdapter(a AgentAdapter) {
	adapters[a.Name()] = a
}

// GetAdapter returns the named adapter, or nil if not found.
func GetAdapter(name string) AgentAdapter {
	return adapters[name]
}

// ListAdapters returns all registered adapter names.
func ListAdapters() []string {
	names := make([]string, 0, len(adapters))
	for k := range adapters {
		names = append(names, k)
	}
	return names
}
