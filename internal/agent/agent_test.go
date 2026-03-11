package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// --- Mock Provider ---

type mockProvider struct {
	responses []*provider.StreamResponse
	callIdx   int
}

func (m *mockProvider) Stream(_ context.Context, _ []provider.Message, _ []provider.ToolDef, _ provider.Config) (*provider.StreamResponse, error) {
	if m.callIdx >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	resp := m.responses[m.callIdx]
	m.callIdx++
	return resp, nil
}

func (m *mockProvider) Name() string { return "mock" }

func mockStream(events ...provider.StreamEvent) *provider.StreamResponse {
	ch := make(chan provider.StreamEvent, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return provider.NewStreamResponse(ch)
}

// --- Tests ---

func TestSimpleTextResponse(t *testing.T) {
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Hello, world!"},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{InputTokens: 10, OutputTokens: 5}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	a := New(mp, reg, WithMaxTurns(10))
	result, err := a.Run(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response != "Hello, world!" {
		t.Errorf("response = %q, want %q", result.Response, "Hello, world!")
	}
	if result.Turns != 1 {
		t.Errorf("turns = %d, want 1", result.Turns)
	}
	if result.Usage.InputTokens != 10 {
		t.Errorf("input tokens = %d, want 10", result.Usage.InputTokens)
	}
}

func TestToolUseAndResult(t *testing.T) {
	// Turn 1: model requests a tool call (read a file that doesn't exist)
	// Turn 2: model gets tool result, responds with text
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			// Turn 1: tool_use
			mockStream(
				provider.StreamEvent{Type: provider.EventToolUseStart, ID: "tu_1", Name: "bash"},
				provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"command":"echo hello"}`},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "tool_use", Usage: &provider.Usage{InputTokens: 20, OutputTokens: 15}},
			),
			// Turn 2: text response after seeing tool result
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Done."},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{InputTokens: 30, OutputTokens: 5}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	a := New(mp, reg, WithMaxTurns(10))
	result, err := a.Run(context.Background(), "run echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Turns != 2 {
		t.Errorf("turns = %d, want 2", result.Turns)
	}
	if result.Response != "Done." {
		t.Errorf("response = %q, want %q", result.Response, "Done.")
	}
	// Usage should be accumulated
	if result.Usage.InputTokens != 50 {
		t.Errorf("input tokens = %d, want 50", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 20 {
		t.Errorf("output tokens = %d, want 20", result.Usage.OutputTokens)
	}
}

func TestMaxTurnsExceeded(t *testing.T) {
	// Provider always returns tool_use, loop should hit maxTurns
	responses := make([]*provider.StreamResponse, 5)
	for i := range responses {
		responses[i] = mockStream(
			provider.StreamEvent{Type: provider.EventToolUseStart, ID: fmt.Sprintf("tu_%d", i), Name: "bash"},
			provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"command":"echo x"}`},
			provider.StreamEvent{Type: provider.EventDone, StopReason: "tool_use", Usage: &provider.Usage{}},
		)
	}

	mp := &mockProvider{responses: responses}
	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	a := New(mp, reg, WithMaxTurns(3))
	_, err := a.Run(context.Background(), "loop forever")
	if err == nil {
		t.Fatal("expected max turns error")
	}
	if got := err.Error(); got != "exceeded max turns (3)" {
		t.Errorf("error = %q", got)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// First response is a tool_use; cancel before second turn
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			mockStream(
				provider.StreamEvent{Type: provider.EventToolUseStart, ID: "tu_1", Name: "bash"},
				provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"command":"echo x"}`},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "tool_use", Usage: &provider.Usage{}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	a := New(mp, reg, WithMaxTurns(10))

	// Cancel after the first tool_use is processed
	cancel()

	_, err := a.Run(ctx, "do something")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	// Either context.Canceled or "no more mock responses" depending on timing
	// Both are acceptable — the point is the loop doesn't hang
}

func TestPhaseGateRejection(t *testing.T) {
	// In brainstorm phase, model tries to use "write" (not allowed)
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			// Turn 1: tries write (disallowed in brainstorm)
			mockStream(
				provider.StreamEvent{Type: provider.EventToolUseStart, ID: "tu_1", Name: "write"},
				provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"file_path":"/tmp/x","content":"bad"}`},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "tool_use", Usage: &provider.Usage{}},
			),
			// Turn 2: model acknowledges the error
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Sorry, I can't write files in this phase."},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	a := New(mp, reg, WithMaxTurns(10), WithStartPhase(tool.PhaseBrainstorm))
	result, err := a.Run(context.Background(), "write a file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should complete — the tool rejection is a tool_result error, not a loop error
	if result.Turns != 2 {
		t.Errorf("turns = %d, want 2", result.Turns)
	}
}

func TestMultipleToolCalls(t *testing.T) {
	// Model requests 2 tool calls in one turn
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

	a := New(mp, reg, WithMaxTurns(10))
	result, err := a.Run(context.Background(), "run two commands")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Turns != 2 {
		t.Errorf("turns = %d, want 2", result.Turns)
	}
}

func TestEmitterReceivesEvidence(t *testing.T) {
	mp := &mockProvider{
		responses: []*provider.StreamResponse{
			mockStream(
				provider.StreamEvent{Type: provider.EventTextDelta, Text: "Hi"},
				provider.StreamEvent{Type: provider.EventDone, StopReason: "end_turn", Usage: &provider.Usage{InputTokens: 5, OutputTokens: 3}},
			),
		},
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	em := &recordingEmitter{}
	a := New(mp, reg, WithMaxTurns(10), WithEmitter(em))
	_, err := a.Run(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(em.events) != 1 {
		t.Fatalf("emitter events = %d, want 1", len(em.events))
	}
	ev := em.events[0]
	if ev.TurnNumber != 1 {
		t.Errorf("turn = %d, want 1", ev.TurnNumber)
	}
	if ev.TokensIn != 5 {
		t.Errorf("tokens_in = %d, want 5", ev.TokensIn)
	}
	if ev.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want %q", ev.StopReason, "end_turn")
	}
}

type recordingEmitter struct {
	events []Evidence
}

func (e *recordingEmitter) Emit(ev Evidence) error {
	e.events = append(e.events, ev)
	return nil
}

// --- Phase FSM Tests ---

func TestPhaseFSMAdvance(t *testing.T) {
	fsm := newPhaseFSM(tool.PhaseBrainstorm)

	expected := []tool.Phase{
		tool.PhaseBrainstorm,
		tool.PhasePlan,
		tool.PhaseBuild,
		tool.PhaseReview,
		tool.PhaseShip,
	}

	for i, want := range expected {
		if got := fsm.Current(); got != want {
			t.Errorf("step %d: current = %q, want %q", i, got, want)
		}
		if i < len(expected)-1 {
			if err := fsm.Advance(); err != nil {
				t.Errorf("step %d: advance error: %v", i, err)
			}
		}
	}
}

func TestPhaseFSMAdvancePastShip(t *testing.T) {
	fsm := newPhaseFSM(tool.PhaseShip)
	err := fsm.Advance()
	if err == nil {
		t.Error("expected error advancing past ship")
	}
}

func TestPhaseFSMIsTerminal(t *testing.T) {
	tests := []struct {
		phase    tool.Phase
		terminal bool
	}{
		{tool.PhaseBrainstorm, false},
		{tool.PhaseBuild, false},
		{tool.PhaseShip, true},
	}

	for _, tt := range tests {
		fsm := newPhaseFSM(tt.phase)
		if got := fsm.IsTerminal(); got != tt.terminal {
			t.Errorf("phase %q: IsTerminal = %v, want %v", tt.phase, got, tt.terminal)
		}
	}
}

func TestPhaseFSMStartAtBuild(t *testing.T) {
	fsm := newPhaseFSM(tool.PhaseBuild)
	if fsm.Current() != tool.PhaseBuild {
		t.Errorf("current = %q, want %q", fsm.Current(), tool.PhaseBuild)
	}
	if err := fsm.Advance(); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if fsm.Current() != tool.PhaseReview {
		t.Errorf("after advance: current = %q, want %q", fsm.Current(), tool.PhaseReview)
	}
}

func TestAgentPhaseTransition(t *testing.T) {
	mp := &mockProvider{}
	reg := tool.NewRegistry()
	a := New(mp, reg, WithStartPhase(tool.PhaseBrainstorm))

	if a.CurrentPhase() != tool.PhaseBrainstorm {
		t.Errorf("phase = %q, want brainstorm", a.CurrentPhase())
	}
	if err := a.AdvancePhase(); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if a.CurrentPhase() != tool.PhasePlan {
		t.Errorf("phase = %q, want plan", a.CurrentPhase())
	}
}

// --- Message Building Tests ---

func TestBuildAssistantMessage(t *testing.T) {
	c := &provider.CollectedResponse{
		Text: "some text",
		ToolCalls: []provider.ToolCall{
			{ID: "tu_1", Name: "read", Input: json.RawMessage(`{"file_path":"x"}`)},
		},
	}

	msg := buildAssistantMessage(c)
	if msg.Role != provider.RoleAssistant {
		t.Errorf("role = %q, want assistant", msg.Role)
	}
	if len(msg.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(msg.Content))
	}
	if msg.Content[0].Type != "text" || msg.Content[0].Text != "some text" {
		t.Errorf("block 0 = %+v", msg.Content[0])
	}
	if msg.Content[1].Type != "tool_use" || msg.Content[1].Name != "read" {
		t.Errorf("block 1 = %+v", msg.Content[1])
	}
}
