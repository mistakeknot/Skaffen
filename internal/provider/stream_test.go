package provider

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestStreamResponse_NextAndEvent(t *testing.T) {
	ch := make(chan StreamEvent, 3)
	ch <- StreamEvent{Type: EventTextDelta, Text: "hello"}
	ch <- StreamEvent{Type: EventTextDelta, Text: " world"}
	ch <- StreamEvent{Type: EventDone, Usage: &Usage{InputTokens: 10, OutputTokens: 5}, StopReason: "end_turn"}
	close(ch)

	s := NewStreamResponse(ch)
	var events []StreamEvent
	for s.Next() {
		events = append(events, s.Event())
	}
	if s.Err() != nil {
		t.Fatalf("unexpected error: %v", s.Err())
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	if events[0].Text != "hello" {
		t.Errorf("event[0].Text = %q", events[0].Text)
	}
	if events[2].StopReason != "end_turn" {
		t.Errorf("event[2].StopReason = %q", events[2].StopReason)
	}
}

func TestStreamResponse_ErrorStopsIteration(t *testing.T) {
	ch := make(chan StreamEvent, 2)
	ch <- StreamEvent{Type: EventTextDelta, Text: "partial"}
	ch <- StreamEvent{Type: EventError, Err: fmt.Errorf("test error")}
	close(ch)

	s := NewStreamResponse(ch)

	if !s.Next() {
		t.Fatal("expected first Next() to return true")
	}
	if s.Event().Text != "partial" {
		t.Errorf("event text = %q", s.Event().Text)
	}

	if s.Next() {
		t.Fatal("expected Next() to return false after error")
	}
	if s.Err() == nil {
		t.Fatal("expected error")
	}
	if s.Err().Error() != "test error" {
		t.Errorf("err = %v", s.Err())
	}
}

func TestStreamResponse_CollectText(t *testing.T) {
	ch := make(chan StreamEvent, 3)
	ch <- StreamEvent{Type: EventTextDelta, Text: "Hello"}
	ch <- StreamEvent{Type: EventTextDelta, Text: ", world!"}
	ch <- StreamEvent{Type: EventDone, Usage: &Usage{InputTokens: 25, OutputTokens: 8}, StopReason: "end_turn"}
	close(ch)

	s := NewStreamResponse(ch)
	result, err := s.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if result.Text != "Hello, world!" {
		t.Errorf("text = %q", result.Text)
	}
	if result.Usage.InputTokens != 25 {
		t.Errorf("input_tokens = %d", result.Usage.InputTokens)
	}
	if result.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q", result.StopReason)
	}
}

func TestStreamResponse_CollectToolCalls(t *testing.T) {
	ch := make(chan StreamEvent, 6)
	ch <- StreamEvent{Type: EventTextDelta, Text: "Reading file."}
	ch <- StreamEvent{Type: EventToolUseStart, ID: "toolu_01", Name: "read"}
	ch <- StreamEvent{Type: EventToolUseDelta, Text: `{"file_path":`}
	ch <- StreamEvent{Type: EventToolUseDelta, Text: `"/tmp/test.go"}`}
	ch <- StreamEvent{Type: EventDone, Usage: &Usage{InputTokens: 50, OutputTokens: 20}, StopReason: "tool_use"}
	close(ch)

	s := NewStreamResponse(ch)
	result, err := s.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if result.Text != "Reading file." {
		t.Errorf("text = %q", result.Text)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(result.ToolCalls))
	}

	tc := result.ToolCalls[0]
	if tc.ID != "toolu_01" {
		t.Errorf("tool ID = %q", tc.ID)
	}
	if tc.Name != "read" {
		t.Errorf("tool name = %q", tc.Name)
	}

	var input map[string]string
	if err := json.Unmarshal(tc.Input, &input); err != nil {
		t.Fatalf("unmarshal tool input: %v", err)
	}
	if input["file_path"] != "/tmp/test.go" {
		t.Errorf("file_path = %q", input["file_path"])
	}
}

func TestStreamResponse_CollectMultipleTools(t *testing.T) {
	ch := make(chan StreamEvent, 8)
	ch <- StreamEvent{Type: EventToolUseStart, ID: "t1", Name: "read"}
	ch <- StreamEvent{Type: EventToolUseDelta, Text: `{"path":"a.go"}`}
	ch <- StreamEvent{Type: EventToolUseStart, ID: "t2", Name: "read"}
	ch <- StreamEvent{Type: EventToolUseDelta, Text: `{"path":"b.go"}`}
	ch <- StreamEvent{Type: EventDone, Usage: &Usage{}, StopReason: "tool_use"}
	close(ch)

	s := NewStreamResponse(ch)
	result, err := s.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("tool_calls = %d, want 2", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ID != "t1" || result.ToolCalls[1].ID != "t2" {
		t.Error("tool IDs don't match")
	}
}

func TestStreamResponse_CollectWithError(t *testing.T) {
	ch := make(chan StreamEvent, 3)
	ch <- StreamEvent{Type: EventTextDelta, Text: "partial"}
	ch <- StreamEvent{Type: EventError, Err: fmt.Errorf("stream failed")}
	close(ch)

	s := NewStreamResponse(ch)
	result, err := s.Collect()
	if err == nil {
		t.Fatal("expected error")
	}
	// Partial result should still be available
	if result.Text != "partial" {
		t.Errorf("text = %q, want partial content before error", result.Text)
	}
}

func TestStreamResponse_EmptyStream(t *testing.T) {
	ch := make(chan StreamEvent)
	close(ch)

	s := NewStreamResponse(ch)
	if s.Next() {
		t.Fatal("expected Next() to return false on empty stream")
	}
	if s.Err() != nil {
		t.Errorf("unexpected error: %v", s.Err())
	}
}
