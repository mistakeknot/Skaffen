package tui

import (
	"strings"
	"testing"
	"time"
)

func TestRenderMessageUser(t *testing.T) {
	msg := ChatMessage{
		Role:      RoleUser,
		Content:   "Hello world",
		Timestamp: time.Now(),
	}
	rendered := RenderMessage(msg, 80)
	if !strings.Contains(rendered, "You") {
		t.Fatal("user message should contain 'You' header")
	}
	if !strings.Contains(rendered, "Hello world") {
		t.Fatal("user message should contain content")
	}
}

func TestRenderMessageAssistant(t *testing.T) {
	msg := ChatMessage{
		Role:    RoleAssistant,
		Content: "I can help with that",
	}
	rendered := RenderMessage(msg, 80)
	if !strings.Contains(rendered, "Skaffen") {
		t.Fatal("assistant message should contain 'Skaffen' header")
	}
	if !strings.Contains(rendered, "I can help with that") {
		t.Fatal("assistant message should contain content")
	}
}

func TestRenderMessageSystem(t *testing.T) {
	msg := ChatMessage{
		Role:    RoleSystem,
		Content: "Session started",
	}
	rendered := RenderMessage(msg, 80)
	if !strings.Contains(rendered, "Session started") {
		t.Fatal("system message should contain content")
	}
	if !strings.Contains(rendered, "---") {
		t.Fatal("system message should have dashes")
	}
}

func TestRenderMessageToolSuccess(t *testing.T) {
	msg := ChatMessage{
		Role:     RoleTool,
		ToolName: "Read",
		Content:  "file contents",
		IsError:  false,
	}
	rendered := RenderMessage(msg, 80)
	if !strings.Contains(rendered, "file contents") {
		t.Fatal("tool message should contain content")
	}
}

func TestRenderMessageToolError(t *testing.T) {
	msg := ChatMessage{
		Role:     RoleTool,
		ToolName: "Bash",
		Content:  "exit code 1",
		IsError:  true,
	}
	rendered := RenderMessage(msg, 80)
	if !strings.Contains(rendered, "Bash") {
		t.Fatal("tool error should contain tool name")
	}
	if !strings.Contains(rendered, "exit code 1") {
		t.Fatal("tool error should contain error content")
	}
}

func TestRenderMessageDefaultRole(t *testing.T) {
	msg := ChatMessage{
		Role:    MessageRole(99),
		Content: "raw content",
	}
	rendered := RenderMessage(msg, 80)
	if rendered != "raw content" {
		t.Errorf("default role rendering = %q, want 'raw content'", rendered)
	}
}
