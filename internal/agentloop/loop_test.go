package agentloop

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// mockProvider returns a single end_turn response.
type mockProvider struct {
	text       string
	toolCalls  []provider.ToolCall
	stopReason string
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Stream(_ context.Context, _ []provider.Message, _ []provider.ToolDef, _ provider.Config) (*provider.StreamResponse, error) {
	// Buffer must hold all events: optional text + 2 per tool call + done
	bufSize := 1 + len(m.toolCalls)*2
	if m.text != "" {
		bufSize++
	}
	ch := make(chan provider.StreamEvent, bufSize)
	if m.text != "" {
		ch <- provider.StreamEvent{Type: provider.EventTextDelta, Text: m.text}
	}
	for _, tc := range m.toolCalls {
		ch <- provider.StreamEvent{Type: provider.EventToolUseStart, ID: tc.ID, Name: tc.Name}
		ch <- provider.StreamEvent{Type: provider.EventToolUseDelta, Text: string(tc.Input)}
	}
	stop := m.stopReason
	if stop == "" {
		stop = "end_turn"
	}
	ch <- provider.StreamEvent{
		Type:       provider.EventDone,
		StopReason: stop,
		Usage:      &provider.Usage{InputTokens: 100, OutputTokens: 50},
	}
	close(ch)
	return provider.NewStreamResponse(ch), nil
}

func TestLoopRunEndTurn(t *testing.T) {
	p := &mockProvider{text: "Hello, world!"}
	reg := NewRegistry()
	loop := New(p, reg)

	result, err := loop.Run(context.Background(), "Say hello", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Response != "Hello, world!" {
		t.Errorf("Response = %q, want 'Hello, world!'", result.Response)
	}
	if result.Turns != 1 {
		t.Errorf("Turns = %d, want 1", result.Turns)
	}
	if result.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.Usage.InputTokens)
	}
}

func TestLoopRunWithHints(t *testing.T) {
	p := &mockProvider{text: "done"}
	reg := NewRegistry()

	var gotHints SelectionHints
	router := &capturingRouter{inner: &NoOpRouter{}, captureHints: &gotHints}
	loop := New(p, reg, WithRouter(router))

	config := LoopConfig{Hints: SelectionHints{Phase: "build", Urgency: "interactive"}}
	_, err := loop.Run(context.Background(), "test", config)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if gotHints.Phase != "build" {
		t.Errorf("Phase = %q, want 'build'", gotHints.Phase)
	}
	if gotHints.Urgency != "interactive" {
		t.Errorf("Urgency = %q, want 'interactive'", gotHints.Urgency)
	}
}

func TestLoopRunEmptyPhase(t *testing.T) {
	p := &mockProvider{text: "done"}
	reg := NewRegistry()
	loop := New(p, reg)

	result, err := loop.Run(context.Background(), "test", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Phase != "" {
		t.Errorf("Phase = %q, want empty", result.Phase)
	}
}

func TestLoopRunMaxTurns(t *testing.T) {
	// Provider that never returns end_turn
	p := &mockProvider{
		text:       "thinking...",
		toolCalls:  []provider.ToolCall{{ID: "1", Name: "read", Input: json.RawMessage(`{}`)}},
		stopReason: "tool_use",
	}
	reg := NewRegistry()
	reg.Register(&stubTool{name: "read", result: "file"})

	loop := New(p, reg, WithMaxTurns(3))
	_, err := loop.Run(context.Background(), "test", LoopConfig{})
	if err == nil {
		t.Fatal("expected max turns error")
	}
}

func TestLoopRunWithToolApprover(t *testing.T) {
	p := &mockProvider{
		toolCalls:  []provider.ToolCall{{ID: "1", Name: "bash", Input: json.RawMessage(`{"cmd":"rm -rf /"}`)}},
		stopReason: "tool_use",
	}
	// Second call returns end_turn
	callCount := 0
	dualProvider := &callCountProvider{
		first: p,
		second: &mockProvider{text: "ok"},
		count:  &callCount,
	}
	reg := NewRegistry()
	reg.Register(&stubTool{name: "bash", result: "done"})

	denied := false
	loop := New(dualProvider, reg, WithMaxTurns(5))
	loop.SetToolApprover(func(name string, _ json.RawMessage) bool {
		if name == "bash" {
			denied = true
			return false
		}
		return true
	})

	_, err := loop.Run(context.Background(), "test", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !denied {
		t.Error("approver was not called")
	}
}

func TestLoopRunContextCancelled(t *testing.T) {
	p := &mockProvider{
		toolCalls:  []provider.ToolCall{{ID: "1", Name: "read", Input: json.RawMessage(`{}`)}},
		stopReason: "tool_use",
	}
	reg := NewRegistry()
	reg.Register(&stubTool{name: "read", result: "file"})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after first tool execution
	callCount := 0
	cancellingProvider := &cancelOnCallProvider{
		inner:  p,
		cancel: cancel,
		count:  &callCount,
	}

	loop := New(cancellingProvider, reg, WithMaxTurns(10))
	_, err := loop.Run(ctx, "test", LoopConfig{})
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestLoopRunWithStreamCallback(t *testing.T) {
	p := &mockProvider{text: "hello"}
	reg := NewRegistry()

	var events []StreamEventType
	cb := func(ev StreamEvent) {
		events = append(events, ev.Type)
	}
	loop := New(p, reg, WithStreamCallback(cb))

	_, err := loop.Run(context.Background(), "test", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	// Should get at least StreamText and StreamTurnComplete
	if len(events) < 2 {
		t.Fatalf("got %d events, want at least 2", len(events))
	}
	if events[0] != StreamText {
		t.Errorf("events[0] = %d, want StreamText", events[0])
	}
	if events[len(events)-1] != StreamTurnComplete {
		t.Errorf("last event = %d, want StreamTurnComplete", events[len(events)-1])
	}
}

func TestLoopRunEvidenceEmitted(t *testing.T) {
	p := &mockProvider{text: "done"}
	reg := NewRegistry()

	emitter := &capturingEmitter{}
	loop := New(p, reg, WithEmitter(emitter), WithSessionID("test-session"))

	config := LoopConfig{Hints: SelectionHints{Phase: "review"}}
	_, err := loop.Run(context.Background(), "test", config)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(emitter.events) != 1 {
		t.Fatalf("emitted %d events, want 1", len(emitter.events))
	}
	ev := emitter.events[0]
	if ev.Phase != "review" {
		t.Errorf("Phase = %q, want 'review'", ev.Phase)
	}
	if ev.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want 'test-session'", ev.SessionID)
	}
	if ev.TokensIn != 100 {
		t.Errorf("TokensIn = %d, want 100", ev.TokensIn)
	}
}

func TestLoopRunSessionSaved(t *testing.T) {
	p := &mockProvider{text: "response"}
	reg := NewRegistry()

	session := &capturingSession{}
	loop := New(p, reg, WithSession(session))

	config := LoopConfig{Hints: SelectionHints{Phase: "build"}}
	_, err := loop.Run(context.Background(), "test", config)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(session.turns) != 1 {
		t.Fatalf("saved %d turns, want 1", len(session.turns))
	}
	if session.turns[0].Phase != "build" {
		t.Errorf("turn.Phase = %q, want 'build'", session.turns[0].Phase)
	}
}

// --- Test helpers ---

type capturingRouter struct {
	inner        *NoOpRouter
	captureHints *SelectionHints
}

func (r *capturingRouter) SelectModel(hints SelectionHints) (string, string) {
	*r.captureHints = hints
	return r.inner.SelectModel(hints)
}
func (r *capturingRouter) RecordUsage(u provider.Usage)  { r.inner.RecordUsage(u) }
func (r *capturingRouter) BudgetState() BudgetState       { return r.inner.BudgetState() }
func (r *capturingRouter) ContextWindow(m string) int      { return r.inner.ContextWindow(m) }

type capturingEmitter struct {
	events []Evidence
}

func (e *capturingEmitter) Emit(ev Evidence) error {
	e.events = append(e.events, ev)
	return nil
}

type capturingSession struct {
	NoOpSession
	turns []Turn
}

func (s *capturingSession) Save(t Turn) error {
	s.turns = append(s.turns, t)
	return nil
}

// callCountProvider dispatches to first on call 0, second on subsequent calls.
type callCountProvider struct {
	first  *mockProvider
	second *mockProvider
	count  *int
}

func (p *callCountProvider) Name() string { return "dual-mock" }
func (p *callCountProvider) Stream(ctx context.Context, msgs []provider.Message, tools []provider.ToolDef, cfg provider.Config) (*provider.StreamResponse, error) {
	*p.count++
	if *p.count == 1 {
		return p.first.Stream(ctx, msgs, tools, cfg)
	}
	return p.second.Stream(ctx, msgs, tools, cfg)
}

// cancelOnCallProvider cancels context after first Stream call.
type cancelOnCallProvider struct {
	inner  *mockProvider
	cancel context.CancelFunc
	count  *int
}

func (p *cancelOnCallProvider) Name() string { return "cancel-mock" }
func (p *cancelOnCallProvider) Stream(ctx context.Context, msgs []provider.Message, tools []provider.ToolDef, cfg provider.Config) (*provider.StreamResponse, error) {
	*p.count++
	resp, err := p.inner.Stream(ctx, msgs, tools, cfg)
	if *p.count >= 1 {
		p.cancel()
	}
	return resp, err
}
