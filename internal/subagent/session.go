package subagent

import (
	"strings"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// ScopedSession provides isolated conversation context for a subagent.
// It implements agentloop.Session with a fixed system prompt (template-expanded)
// and an independent message history.
type ScopedSession struct {
	systemPrompt string
	messages     []provider.Message
}

// NewScopedSession creates a session with a template-expanded system prompt.
// The template supports {{.TaskPrompt}} and {{.InjectedContext}} placeholders.
func NewScopedSession(promptTemplate, taskPrompt, injectedContext string) *ScopedSession {
	expanded := promptTemplate
	expanded = strings.ReplaceAll(expanded, "{{.TaskPrompt}}", taskPrompt)
	expanded = strings.ReplaceAll(expanded, "{{.InjectedContext}}", injectedContext)
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
