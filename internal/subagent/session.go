package subagent

import (
	"fmt"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// DefaultContextTokenCap is the default maximum tokens for injected context.
// Uses a 4-chars-per-token heuristic (conservative estimate).
const DefaultContextTokenCap = 4096

// ScopedSessionConfig configures a ScopedSession.
type ScopedSessionConfig struct {
	PromptTemplate  string // template with {{.TaskPrompt}}, {{.InjectedContext}}, {{.BeadDescription}}
	TaskPrompt      string
	InjectedContext string
	BeadDescription string
	ContextTokenCap int // 0 = DefaultContextTokenCap
}

// ScopedSession provides isolated conversation context for a subagent.
// It implements agentloop.Session with a fixed system prompt (template-expanded)
// and an independent message history.
type ScopedSession struct {
	systemPrompt string
	messages     []provider.Message
}

// NewScopedSession creates a session with a template-expanded system prompt.
// InjectedContext is truncated to ContextTokenCap before expansion.
func NewScopedSession(cfg ScopedSessionConfig) *ScopedSession {
	cap := cfg.ContextTokenCap
	if cap <= 0 {
		cap = DefaultContextTokenCap
	}

	injected := truncateToTokenCap(cfg.InjectedContext, cap)

	expanded := cfg.PromptTemplate
	expanded = strings.ReplaceAll(expanded, "{{.TaskPrompt}}", cfg.TaskPrompt)
	expanded = strings.ReplaceAll(expanded, "{{.InjectedContext}}", injected)
	expanded = strings.ReplaceAll(expanded, "{{.BeadDescription}}", cfg.BeadDescription)
	return &ScopedSession{
		systemPrompt: expanded,
	}
}

// SystemPrompt returns the expanded system prompt.
func (s *ScopedSession) SystemPrompt(_ agentloop.PromptHints) string {
	return s.systemPrompt
}

// Save appends turn messages to the isolated history.
func (s *ScopedSession) Save(turn agentloop.Turn) error {
	s.messages = append(s.messages, turn.Messages...)
	return nil
}

// Messages returns the isolated message history.
func (s *ScopedSession) Messages() []provider.Message {
	return s.messages
}

// ContextSource is an input for BuildInjectedContext.
type ContextSource struct {
	Label   string // e.g., "Parent conversation", "File: main.go"
	Content string
}

// BuildInjectedContext formats structured sources into a delimited string
// suitable for the {{.InjectedContext}} template placeholder.
// Empty-content sources are skipped.
func BuildInjectedContext(sources []ContextSource) string {
	var b strings.Builder
	for _, src := range sources {
		if strings.TrimSpace(src.Content) == "" {
			continue
		}
		label := src.Label
		if label == "" {
			label = "Context"
		}
		fmt.Fprintf(&b, "--- %s ---\n%s\n\n", label, src.Content)
	}
	return strings.TrimRight(b.String(), "\n")
}

// truncateToTokenCap truncates s to approximately cap tokens, keeping the tail
// (most recent content). Uses a 4-chars-per-token heuristic.
func truncateToTokenCap(s string, cap int) string {
	maxChars := cap * 4
	if len(s) <= maxChars {
		return s
	}
	truncatedChars := len(s) - maxChars
	truncatedTokens := truncatedChars / 4
	return fmt.Sprintf("[...truncated ~%d tokens...]\n%s", truncatedTokens, s[truncatedChars:])
}
