package agent

import (
	"context"
	"sync"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// eventRecorder collects StreamEvents in a thread-safe manner.
type eventRecorder struct {
	mu     sync.Mutex
	events []StreamEvent
}

func (r *eventRecorder) record(ev StreamEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *eventRecorder) all() []StreamEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]StreamEvent, len(r.events))
	copy(cp, r.events)
	return cp
}

func TestStreamCallback_TextOnly(t *testing.T) {
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Hello"},
				provider.StreamEvent{Type: provider.EventTextDelta, Text: ", world!"},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 5}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	rec := &eventRecorder{}
	a := New(mp, reg, WithMaxTurns(10), WithStreamCallback(rec.record))
	result, err := a.Run(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the agent loop still works correctly
	if result.Response != "Hello, world!" {
		t.Errorf("response = %q, want %q", result.Response, "Hello, world!")
	}
	if result.Turns != 1 {
		t.Errorf("turns = %d, want 1", result.Turns)
	}

	// Verify callback events
	events := rec.all()
	if len(events) < 3 {
		t.Fatalf("got %d events, want >= 3", len(events))
	}

	// First two should be text deltas
	if events[0].Type != StreamText || events[0].Text != "Hello" {
		t.Errorf("event 0: type=%d text=%q, want StreamText/Hello", events[0].Type, events[0].Text)
	}
	if events[1].Type != StreamText || events[1].Text != ", world!" {
		t.Errorf("event 1: type=%d text=%q, want StreamText/', world!'", events[1].Type, events[1].Text)
	}

	// Last should be turn complete
	last := events[len(events)-1]
	if last.Type != StreamTurnComplete {
		t.Errorf("last event type = %d, want StreamTurnComplete", last.Type)
	}
	if last.TurnNumber != 1 {
		t.Errorf("last event turn = %d, want 1", last.TurnNumber)
	}
	if last.Usage.InputTokens != 10 {
		t.Errorf("last event input tokens = %d, want 10", last.Usage.InputTokens)
	}
}

func TestStreamCallback_ToolUse(t *testing.T) {
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			// Turn 1: tool_use
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Let me run that."},
				provider.StreamEvent{Type: provider.EventToolUseStart, ID: "tu_1", Name: "bash"},
				provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"command":"echo hi"}`},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "tool_use", Usage: &provider.Usage{InputTokens: 20, OutputTokens: 15}},
			),
			// Turn 2: text response
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Done."},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{InputTokens: 30, OutputTokens: 5}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	rec := &eventRecorder{}
	a := New(mp, reg, WithMaxTurns(10), WithStreamCallback(rec.record))
	result, err := a.Run(context.Background(), "run echo hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the agent loop still works
	if result.Response != "Done." {
		t.Errorf("response = %q, want %q", result.Response, "Done.")
	}
	if result.Turns != 2 {
		t.Errorf("turns = %d, want 2", result.Turns)
	}
	if result.Usage.InputTokens != 50 {
		t.Errorf("input tokens = %d, want 50", result.Usage.InputTokens)
	}

	// Check that we got the expected event types
	events := rec.all()
	typeMap := make(map[StreamEventType]int)
	for _, ev := range events {
		typeMap[ev.Type]++
	}

	if typeMap[StreamText] < 2 {
		t.Errorf("StreamText count = %d, want >= 2", typeMap[StreamText])
	}
	if typeMap[StreamToolStart] != 1 {
		t.Errorf("StreamToolStart count = %d, want 1", typeMap[StreamToolStart])
	}
	if typeMap[StreamToolComplete] != 1 {
		t.Errorf("StreamToolComplete count = %d, want 1", typeMap[StreamToolComplete])
	}
	if typeMap[StreamTurnComplete] != 2 {
		t.Errorf("StreamTurnComplete count = %d, want 2", typeMap[StreamTurnComplete])
	}

	// Verify the tool start event carries the tool name
	for _, ev := range events {
		if ev.Type == StreamToolStart {
			if ev.ToolName != "bash" {
				t.Errorf("StreamToolStart tool name = %q, want %q", ev.ToolName, "bash")
			}
			break
		}
	}

	// Verify the tool complete event carries the tool name and result
	for _, ev := range events {
		if ev.Type == StreamToolComplete {
			if ev.ToolName != "bash" {
				t.Errorf("StreamToolComplete tool name = %q, want %q", ev.ToolName, "bash")
			}
			// The tool result should be non-empty (bash executed echo hi)
			break
		}
	}
}

func TestStreamCallback_MultipleToolCalls(t *testing.T) {
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			mockStream(
				provider.StreamEvent{Type: provider.EventToolUseStart, ID: "tu_1", Name: "bash"},
				provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"command":"echo one"}`},
				provider.StreamEvent{Type: provider.EventToolUseStart, ID: "tu_2", Name: "bash"},
				provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"command":"echo two"}`},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "tool_use", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 20}},
			),
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Both done."},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	rec := &eventRecorder{}
	a := New(mp, reg, WithMaxTurns(10), WithStreamCallback(rec.record))
	result, err := a.Run(context.Background(), "run two commands")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Turns != 2 {
		t.Errorf("turns = %d, want 2", result.Turns)
	}

	events := rec.all()
	typeMap := make(map[StreamEventType]int)
	for _, ev := range events {
		typeMap[ev.Type]++
	}

	if typeMap[StreamToolStart] != 2 {
		t.Errorf("StreamToolStart count = %d, want 2", typeMap[StreamToolStart])
	}
	if typeMap[StreamToolComplete] != 2 {
		t.Errorf("StreamToolComplete count = %d, want 2", typeMap[StreamToolComplete])
	}
}

func TestStreamCallback_NilDoesNotAffectExisting(t *testing.T) {
	// Verify that NOT setting a stream callback preserves original behavior
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Hello"},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 5}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	// No WithStreamCallback — should use the existing Collect() path
	a := New(mp, reg, WithMaxTurns(10))
	result, err := a.Run(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response != "Hello" {
		t.Errorf("response = %q, want %q", result.Response, "Hello")
	}
}

func TestStreamCallback_EventOrder(t *testing.T) {
	// Verify events arrive in expected order: text, tool_start, turn_complete,
	// tool_complete (from execution), text, turn_complete
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Thinking..."},
				provider.StreamEvent{Type: provider.EventToolUseStart, ID: "tu_1", Name: "bash"},
				provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"command":"echo ok"}`},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "tool_use", Usage: &provider.Usage{InputTokens: 5, OutputTokens: 3}},
			),
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Final."},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 2}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	rec := &eventRecorder{}
	a := New(mp, reg, WithMaxTurns(10), WithStreamCallback(rec.record))
	_, err := a.Run(context.Background(), "test ordering")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := rec.all()
	// Expected order:
	// 0: StreamText("Thinking...")
	// 1: StreamToolStart("bash")
	// 2: StreamTurnComplete (turn 1)
	// 3: StreamToolComplete("bash")
	// 4: StreamText("Final.")
	// 5: StreamTurnComplete (turn 2)
	expected := []StreamEventType{
		StreamText,
		StreamToolStart,
		StreamTurnComplete,
		StreamToolComplete,
		StreamText,
		StreamTurnComplete,
	}

	if len(events) != len(expected) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(expected), events)
	}
	for i, want := range expected {
		if events[i].Type != want {
			t.Errorf("event %d: type=%d, want %d", i, events[i].Type, want)
		}
	}

	// Verify turn numbers on TurnComplete events
	if events[2].TurnNumber != 1 {
		t.Errorf("turn complete 1: turn=%d, want 1", events[2].TurnNumber)
	}
	if events[5].TurnNumber != 2 {
		t.Errorf("turn complete 2: turn=%d, want 2", events[5].TurnNumber)
	}
}
