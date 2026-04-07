package openai

import (
	"encoding/json"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestTranslateMessages_TextOnly(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hello"}}},
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
	}

	result := translateMessages(msgs, "system prompt")

	if len(result) != 3 {
		t.Fatalf("len = %d, want 3 (system + user + assistant)", len(result))
	}
	if result[0].Role != "system" || result[0].Content != "system prompt" {
		t.Errorf("result[0] = %+v", result[0])
	}
	if result[1].Role != "user" || result[1].Content != "hello" {
		t.Errorf("result[1] = %+v", result[1])
	}
	if result[2].Role != "assistant" || result[2].Content != "hi" {
		t.Errorf("result[2] = %+v", result[2])
	}
}

func TestTranslateMessages_ToolUseAndResult(t *testing.T) {
	// Simulate: assistant calls a tool, user sends tool result.
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "read foo.go"}}},
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{
			{Type: "text", Text: "I'll read that file."},
			{Type: "tool_use", ID: "tu_1", Name: "Read", Input: json.RawMessage(`{"path":"foo.go"}`)},
		}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "tu_1", ResultContent: "package main\n"},
		}},
	}

	result := translateMessages(msgs, "")

	// Expect: user, assistant (with tool_calls), tool (result)
	if len(result) != 3 {
		t.Fatalf("len = %d, want 3", len(result))
	}

	// User message
	if result[0].Role != "user" || result[0].Content != "read foo.go" {
		t.Errorf("result[0] = %+v", result[0])
	}

	// Assistant with tool_calls
	assistant := result[1]
	if assistant.Role != "assistant" {
		t.Errorf("result[1].role = %q", assistant.Role)
	}
	if assistant.Content != "I'll read that file." {
		t.Errorf("result[1].content = %q", assistant.Content)
	}
	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(assistant.ToolCalls))
	}
	tc := assistant.ToolCalls[0]
	if tc.ID != "tu_1" || tc.Function.Name != "Read" {
		t.Errorf("tool_call = %+v", tc)
	}
	if tc.Function.Arguments != `{"path":"foo.go"}` {
		t.Errorf("arguments = %q", tc.Function.Arguments)
	}

	// Tool result
	toolMsg := result[2]
	if toolMsg.Role != "tool" {
		t.Errorf("result[2].role = %q", toolMsg.Role)
	}
	if toolMsg.ToolCallID != "tu_1" {
		t.Errorf("tool_call_id = %q", toolMsg.ToolCallID)
	}
	if toolMsg.Content != "package main\n" {
		t.Errorf("content = %q", toolMsg.Content)
	}
}

func TestTranslateMessages_ErrorResult(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "tu_2", ResultContent: "file not found", IsError: true},
		}},
	}

	result := translateMessages(msgs, "")
	if len(result) != 1 {
		t.Fatalf("len = %d", len(result))
	}
	if result[0].Content != "Error: file not found" {
		t.Errorf("content = %q", result[0].Content)
	}
}

func TestTranslateMessages_MultipleToolResults(t *testing.T) {
	// Parallel tool calls → multiple tool results in one user message.
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{
			{Type: "tool_result", ToolUseID: "tu_a", ResultContent: "result_a"},
			{Type: "tool_result", ToolUseID: "tu_b", ResultContent: "result_b"},
		}},
	}

	result := translateMessages(msgs, "")
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2 (one per tool result)", len(result))
	}
	if result[0].ToolCallID != "tu_a" {
		t.Errorf("result[0].tool_call_id = %q", result[0].ToolCallID)
	}
	if result[1].ToolCallID != "tu_b" {
		t.Errorf("result[1].tool_call_id = %q", result[1].ToolCallID)
	}
}

func TestTranslateMessages_NoSystem(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
	}

	result := translateMessages(msgs, "")
	if len(result) != 1 {
		t.Fatalf("len = %d, want 1 (no system message)", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("role = %q", result[0].Role)
	}
}
