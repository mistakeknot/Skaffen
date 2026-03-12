package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestClaudeCodeProvider_Name(t *testing.T) {
	p := New(WithBinaryPath("/nonexistent"))
	if p.Name() != "claude-code" {
		t.Errorf("Name() = %q, want %q", p.Name(), "claude-code")
	}
}

func TestClaudeCodeProvider_BinaryNotFound(t *testing.T) {
	p := New(WithBinaryPath("/nonexistent/claude-does-not-exist"))
	_, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})

	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "claude binary not found") {
		t.Errorf("error = %v, want mention of binary not found", err)
	}
}

func TestClaudeCodeProvider_StreamText(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses bash script")
	}

	// Create a mock script that outputs the golden JSONL
	goldenData, err := os.ReadFile(filepath.Join("testdata", "stream_response.jsonl"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-claude")
	script := "#!/bin/sh\ncat <<'GOLDEN'\n" + string(goldenData) + "GOLDEN\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := New(WithBinaryPath(scriptPath))
	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	result, err := resp.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if result.Text != "Hello from Claude Code!" {
		t.Errorf("text = %q, want %q", result.Text, "Hello from Claude Code!")
	}
	if result.Usage.InputTokens != 15 {
		t.Errorf("input_tokens = %d, want 15", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 6 {
		t.Errorf("output_tokens = %d, want 6", result.Usage.OutputTokens)
	}
}

func TestClaudeCodeProvider_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses bash script")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-claude")
	script := "#!/bin/sh\necho 'something went wrong' >&2\nexit 1\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := New(WithBinaryPath(scriptPath))
	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Hello"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	_, err = resp.Collect()
	if err == nil {
		t.Fatal("expected error from non-zero exit")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("error = %v, want stderr content", err)
	}
}

func TestClaudeCodeProvider_NoUserMessage(t *testing.T) {
	p := New(WithBinaryPath("/bin/echo"))
	_, err := p.Stream(context.Background(), []provider.Message{}, nil, provider.Config{})
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

func TestClaudeCodeProvider_StreamToolUse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses bash script")
	}

	goldenData, err := os.ReadFile(filepath.Join("testdata", "stream_tool_use.jsonl"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-claude")
	script := "#!/bin/sh\ncat <<'GOLDEN'\n" + string(goldenData) + "GOLDEN\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := New(WithBinaryPath(scriptPath))
	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "Read /tmp/test.go"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Collect events manually to verify tool events
	var events []provider.StreamEvent
	for resp.Next() {
		events = append(events, resp.Event())
	}
	if resp.Err() != nil {
		t.Fatalf("stream error: %v", resp.Err())
	}

	// Expect: TextDelta, ToolUseStart, ToolResult, TextDelta, Done
	if len(events) < 5 {
		t.Fatalf("expected >= 5 events, got %d: %+v", len(events), events)
	}

	// First event: text "Let me read that file."
	if events[0].Type != provider.EventTextDelta || events[0].Text != "Let me read that file." {
		t.Errorf("event[0] = %+v, want TextDelta with text", events[0])
	}
	// Second: ToolUseStart for Read
	if events[1].Type != provider.EventToolUseStart {
		t.Errorf("event[1].Type = %d, want EventToolUseStart", events[1].Type)
	}
	if events[1].Name != "Read" {
		t.Errorf("event[1].Name = %q, want Read", events[1].Name)
	}
	if events[1].ID != "toolu_01abc" {
		t.Errorf("event[1].ID = %q", events[1].ID)
	}
	// Third: ToolResult
	if events[2].Type != provider.EventToolResult {
		t.Errorf("event[2].Type = %d, want EventToolResult", events[2].Type)
	}
	if events[2].ID != "toolu_01abc" {
		t.Errorf("event[2].ID = %q", events[2].ID)
	}
	if events[2].Err != nil {
		t.Errorf("event[2].Err = %v, want nil (success)", events[2].Err)
	}
	// Fourth: text from second assistant message
	if events[3].Type != provider.EventTextDelta {
		t.Errorf("event[3].Type = %d, want EventTextDelta", events[3].Type)
	}
	// Fifth: Done with usage
	if events[4].Type != provider.EventDone {
		t.Errorf("event[4].Type = %d, want EventDone", events[4].Type)
	}
	if events[4].Usage == nil || events[4].Usage.InputTokens != 300 {
		t.Errorf("event[4].Usage = %+v, want InputTokens=300", events[4].Usage)
	}
}

func TestClaudeCodeProvider_StreamToolError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses bash script")
	}

	goldenData, err := os.ReadFile(filepath.Join("testdata", "stream_tool_error.jsonl"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-claude")
	script := "#!/bin/sh\ncat <<'GOLDEN'\n" + string(goldenData) + "GOLDEN\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := New(WithBinaryPath(scriptPath))
	resp, err := p.Stream(context.Background(), []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "cat /nonexistent"}}},
	}, nil, provider.Config{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var events []provider.StreamEvent
	for resp.Next() {
		events = append(events, resp.Event())
	}
	if resp.Err() != nil {
		t.Fatalf("stream error: %v", resp.Err())
	}

	// Find the ToolResult event
	var toolResult *provider.StreamEvent
	for i, ev := range events {
		if ev.Type == provider.EventToolResult {
			toolResult = &events[i]
			break
		}
	}
	if toolResult == nil {
		t.Fatal("expected EventToolResult event")
	}
	if toolResult.Err == nil {
		t.Fatal("tool error event should have Err set")
	}
	if !strings.Contains(toolResult.Err.Error(), "No such file or directory") {
		t.Errorf("Err = %v, want 'No such file or directory'", toolResult.Err)
	}
}

// Unit tests for handleAssistantMessage and handleUserMessage

func collectEvents(events chan provider.StreamEvent) []provider.StreamEvent {
	var result []provider.StreamEvent
	for ev := range events {
		result = append(result, ev)
	}
	return result
}

func TestHandleAssistantMessage_TextOnly(t *testing.T) {
	p := &ClaudeCodeProvider{}
	events := make(chan provider.StreamEvent, 8)
	data := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello!"}]}}`)
	p.handleAssistantMessage(data, events)
	close(events)

	evs := collectEvents(events)
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	if evs[0].Type != provider.EventTextDelta || evs[0].Text != "Hello!" {
		t.Errorf("event = %+v, want TextDelta Hello!", evs[0])
	}
}

func TestHandleAssistantMessage_TextAndToolUse(t *testing.T) {
	p := &ClaudeCodeProvider{}
	events := make(chan provider.StreamEvent, 8)
	data := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Reading..."},{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"x.go"}}]}}`)
	p.handleAssistantMessage(data, events)
	close(events)

	evs := collectEvents(events)
	if len(evs) != 2 {
		t.Fatalf("expected 2 events, got %d", len(evs))
	}
	if evs[0].Type != provider.EventTextDelta {
		t.Errorf("event[0] type = %d, want TextDelta", evs[0].Type)
	}
	if evs[1].Type != provider.EventToolUseStart || evs[1].Name != "Read" {
		t.Errorf("event[1] = %+v, want ToolUseStart Read", evs[1])
	}
	if evs[1].ID != "t1" {
		t.Errorf("event[1].ID = %q, want t1", evs[1].ID)
	}
}

func TestHandleUserMessage_ToolResult(t *testing.T) {
	p := &ClaudeCodeProvider{}
	events := make(chan provider.StreamEvent, 8)
	data := []byte(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok","is_error":false}]}}`)
	p.handleUserMessage(data, events)
	close(events)

	evs := collectEvents(events)
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	if evs[0].Type != provider.EventToolResult || evs[0].ID != "t1" || evs[0].Err != nil {
		t.Errorf("event = %+v, want ToolResult t1 no error", evs[0])
	}
}

func TestHandleUserMessage_SkipsNonToolResult(t *testing.T) {
	p := &ClaudeCodeProvider{}
	events := make(chan provider.StreamEvent, 8)
	data := []byte(`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}`)
	p.handleUserMessage(data, events)
	close(events)

	evs := collectEvents(events)
	if len(evs) != 0 {
		t.Fatalf("expected 0 events, got %d", len(evs))
	}
}

func TestHandleAssistantMessage_InvalidJSON(t *testing.T) {
	p := &ClaudeCodeProvider{}
	events := make(chan provider.StreamEvent, 8)
	p.handleAssistantMessage([]byte(`{invalid`), events)
	close(events)

	evs := collectEvents(events)
	if len(evs) != 0 {
		t.Fatalf("expected 0 events for invalid JSON, got %d", len(evs))
	}
}

func TestHandleAssistantMessage_EmptyText(t *testing.T) {
	p := &ClaudeCodeProvider{}
	events := make(chan provider.StreamEvent, 8)
	data := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":""}]}}`)
	p.handleAssistantMessage(data, events)
	close(events)

	evs := collectEvents(events)
	if len(evs) != 0 {
		t.Fatalf("expected 0 events for empty text, got %d", len(evs))
	}
}
