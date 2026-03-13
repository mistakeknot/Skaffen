package subagent

import (
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestScopedSession_SystemPrompt(t *testing.T) {
	s := NewScopedSession(ScopedSessionConfig{
		PromptTemplate:  "You are a researcher. {{.TaskPrompt}}",
		TaskPrompt:      "Find all Go files that import context",
		InjectedContext: "The project uses Go 1.22",
	})
	prompt := s.SystemPrompt(agentloop.PromptHints{})
	if !strings.Contains(prompt, "Find all Go files") {
		t.Error("prompt should contain task prompt")
	}
}

func TestScopedSession_TemplateWithBothPlaceholders(t *testing.T) {
	s := NewScopedSession(ScopedSessionConfig{
		PromptTemplate:  "Context: {{.InjectedContext}}\n\nTask: {{.TaskPrompt}}",
		TaskPrompt:      "find files",
		InjectedContext: "project uses Go 1.24",
	})
	prompt := s.SystemPrompt(agentloop.PromptHints{})
	if !strings.Contains(prompt, "find files") {
		t.Error("prompt should contain task prompt")
	}
	if !strings.Contains(prompt, "Go 1.24") {
		t.Error("prompt should contain injected context")
	}
}

func TestScopedSession_Isolation(t *testing.T) {
	s := NewScopedSession(ScopedSessionConfig{
		PromptTemplate: "system",
		TaskPrompt:     "task",
	})

	if len(s.Messages()) != 0 {
		t.Error("should start with no messages")
	}

	s.Save(agentloop.Turn{
		Messages: []provider.Message{{Role: provider.RoleAssistant}},
	})
	if len(s.Messages()) != 1 {
		t.Errorf("after save, got %d messages, want 1", len(s.Messages()))
	}
}

func TestScopedSession_BeadDescription(t *testing.T) {
	s := NewScopedSession(ScopedSessionConfig{
		PromptTemplate:  "Working on: {{.BeadDescription}}\n\n{{.TaskPrompt}}",
		TaskPrompt:      "fix the bug",
		BeadDescription: "Demarch-p23: ScopedSession context isolation",
	})
	prompt := s.SystemPrompt(agentloop.PromptHints{})
	if !strings.Contains(prompt, "Demarch-p23") {
		t.Error("prompt should contain bead description")
	}
	if !strings.Contains(prompt, "fix the bug") {
		t.Error("prompt should contain task prompt")
	}
}

func TestScopedSession_TokenCap_Truncates(t *testing.T) {
	// Create context that exceeds the default cap (4096 tokens * 4 chars = 16384 chars)
	bigContext := strings.Repeat("x", 20000) // ~5000 tokens, exceeds 4096 cap
	s := NewScopedSession(ScopedSessionConfig{
		PromptTemplate:  "{{.InjectedContext}}",
		TaskPrompt:      "test",
		InjectedContext: bigContext,
	})
	prompt := s.SystemPrompt(agentloop.PromptHints{})

	if !strings.Contains(prompt, "[...truncated") {
		t.Error("should contain truncation marker")
	}
	// The tail should be preserved
	if !strings.HasSuffix(prompt, strings.Repeat("x", 100)) {
		t.Error("should preserve tail of context")
	}
	// Should be approximately 16384 chars + marker
	if len(prompt) > 16500 {
		t.Errorf("truncated prompt too long: %d chars", len(prompt))
	}
}

func TestScopedSession_TokenCap_NoTruncation(t *testing.T) {
	shortContext := strings.Repeat("y", 100)
	s := NewScopedSession(ScopedSessionConfig{
		PromptTemplate:  "{{.InjectedContext}}",
		TaskPrompt:      "test",
		InjectedContext: shortContext,
	})
	prompt := s.SystemPrompt(agentloop.PromptHints{})

	if strings.Contains(prompt, "[...truncated") {
		t.Error("should not truncate short context")
	}
	if prompt != shortContext {
		t.Errorf("prompt = %q, want %q", prompt, shortContext)
	}
}

func TestScopedSession_TokenCap_Custom(t *testing.T) {
	// Custom cap of 10 tokens = 40 chars
	context := strings.Repeat("z", 100) // 25 tokens worth, exceeds cap of 10
	s := NewScopedSession(ScopedSessionConfig{
		PromptTemplate:  "{{.InjectedContext}}",
		TaskPrompt:      "test",
		InjectedContext: context,
		ContextTokenCap: 10,
	})
	prompt := s.SystemPrompt(agentloop.PromptHints{})

	if !strings.Contains(prompt, "[...truncated") {
		t.Error("should truncate with custom cap")
	}
	// Tail should be 40 chars of 'z'
	if !strings.HasSuffix(prompt, strings.Repeat("z", 40)) {
		t.Error("should preserve 40 chars (10 tokens * 4 chars)")
	}
}

func TestBuildInjectedContext(t *testing.T) {
	result := BuildInjectedContext([]ContextSource{
		{Label: "File: main.go", Content: "package main"},
		{Label: "Parent conversation", Content: "User asked about X"},
	})

	if !strings.Contains(result, "--- File: main.go ---") {
		t.Error("should contain file label")
	}
	if !strings.Contains(result, "package main") {
		t.Error("should contain file content")
	}
	if !strings.Contains(result, "--- Parent conversation ---") {
		t.Error("should contain conversation label")
	}
	if !strings.Contains(result, "User asked about X") {
		t.Error("should contain conversation content")
	}
}

func TestBuildInjectedContext_EmptyLabel(t *testing.T) {
	result := BuildInjectedContext([]ContextSource{
		{Content: "some content"},
	})
	if !strings.Contains(result, "--- Context ---") {
		t.Error("should use default label 'Context'")
	}
}

func TestBuildInjectedContext_SkipsEmpty(t *testing.T) {
	result := BuildInjectedContext([]ContextSource{
		{Label: "Keep", Content: "real content"},
		{Label: "Skip", Content: ""},
		{Label: "AlsoSkip", Content: "   "},
	})

	if !strings.Contains(result, "Keep") {
		t.Error("should include non-empty source")
	}
	if strings.Contains(result, "Skip") {
		t.Error("should skip empty-content sources")
	}
}

func TestBuildInjectedContext_Empty(t *testing.T) {
	result := BuildInjectedContext(nil)
	if result != "" {
		t.Errorf("nil sources should return empty string, got %q", result)
	}

	result = BuildInjectedContext([]ContextSource{})
	if result != "" {
		t.Errorf("empty sources should return empty string, got %q", result)
	}
}
