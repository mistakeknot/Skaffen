package subagent

import (
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestScopedSession_SystemPrompt(t *testing.T) {
	s := NewScopedSession(
		"You are a researcher. {{.TaskPrompt}}",
		"Find all Go files that import context",
		"The project uses Go 1.22",
	)
	prompt := s.SystemPrompt(agentloop.PromptHints{})
	if !strings.Contains(prompt, "Find all Go files") {
		t.Error("prompt should contain task prompt")
	}
	// InjectedContext is only inserted if {{.InjectedContext}} placeholder is in template
}

func TestScopedSession_TemplateWithBothPlaceholders(t *testing.T) {
	s := NewScopedSession(
		"Context: {{.InjectedContext}}\n\nTask: {{.TaskPrompt}}",
		"find files",
		"project uses Go 1.24",
	)
	prompt := s.SystemPrompt(agentloop.PromptHints{})
	if !strings.Contains(prompt, "find files") {
		t.Error("prompt should contain task prompt")
	}
	if !strings.Contains(prompt, "Go 1.24") {
		t.Error("prompt should contain injected context")
	}
}

func TestScopedSession_Isolation(t *testing.T) {
	s := NewScopedSession("system", "task", "")

	// Initially empty
	if len(s.Messages()) != 0 {
		t.Error("should start with no messages")
	}

	// Save a turn
	s.Save(agentloop.Turn{
		Messages: []provider.Message{{Role: provider.RoleAssistant}},
	})
	if len(s.Messages()) != 1 {
		t.Errorf("after save, got %d messages, want 1", len(s.Messages()))
	}
}
