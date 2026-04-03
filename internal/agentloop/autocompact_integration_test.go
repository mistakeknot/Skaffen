package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// --- Integration tests: auto-compact through the full Loop ---

// smallWindowRouter returns a tiny context window to trigger compaction quickly.
type smallWindowRouter struct {
	NoOpRouter
	window int
}

func (r *smallWindowRouter) ContextWindow(_ string) int { return r.window }

// compactSession tracks messages and implements MessageReplacer.
type compactSession struct {
	NoOpSession
	turns           []Turn
	messages        []provider.Message
	replaceCalls    int
	lastReplaceSize int
}

func (s *compactSession) Save(t Turn) error {
	s.turns = append(s.turns, t)
	s.messages = append(s.messages, t.Messages...)
	return nil
}

func (s *compactSession) Messages() []provider.Message {
	return s.messages
}

func (s *compactSession) ReplaceMessages(msgs []provider.Message) {
	s.replaceCalls++
	s.lastReplaceSize = len(msgs)
	s.messages = make([]provider.Message, len(msgs))
	copy(s.messages, msgs)
}

// multiTurnProvider cycles through responses. Each response includes a tool call
// that generates a large result, except the final response which ends the turn.
type multiTurnProvider struct {
	turnCount  int
	maxTurns   int // after this many tool_use turns, return end_turn
	resultSize int // chars per tool result
}

func (p *multiTurnProvider) Name() string { return "multi-turn-mock" }

func (p *multiTurnProvider) Stream(_ context.Context, _ []provider.Message, _ []provider.ToolDef, _ provider.Config) (*provider.StreamResponse, error) {
	p.turnCount++

	if p.turnCount > p.maxTurns {
		// Final turn: end
		ch := make(chan provider.StreamEvent, 2)
		ch <- provider.StreamEvent{Type: provider.EventTextDelta, Text: "Done."}
		ch <- provider.StreamEvent{
			Type:       provider.EventDone,
			StopReason: "end_turn",
			Usage:      &provider.Usage{InputTokens: 100, OutputTokens: 10},
		}
		close(ch)
		return provider.NewStreamResponse(ch), nil
	}

	// Tool use turn: read a file → large result
	toolID := fmt.Sprintf("tool_%d", p.turnCount)
	ch := make(chan provider.StreamEvent, 4)
	ch <- provider.StreamEvent{Type: provider.EventTextDelta, Text: fmt.Sprintf("Reading file %d.", p.turnCount)}
	ch <- provider.StreamEvent{Type: provider.EventToolUseStart, ID: toolID, Name: "read"}
	ch <- provider.StreamEvent{Type: provider.EventToolUseDelta, Text: `{"file_path":"large.go"}`}
	ch <- provider.StreamEvent{
		Type:       provider.EventDone,
		StopReason: "tool_use",
		Usage:      &provider.Usage{InputTokens: 100, OutputTokens: 50},
	}
	close(ch)
	return provider.NewStreamResponse(ch), nil
}

// largeResultTool returns a large string to fill context quickly.
type largeResultTool struct {
	size int
}

func (t *largeResultTool) Name() string            { return "read" }
func (t *largeResultTool) Description() string     { return "read a file" }
func (t *largeResultTool) Schema() json.RawMessage { return json.RawMessage(`{}`) }
func (t *largeResultTool) Execute(_ context.Context, _ json.RawMessage) ToolResult {
	return ToolResult{Content: strings.Repeat("x", t.size)}
}

func TestAutoCompactTriggersInLoop(t *testing.T) {
	// Use a tiny context window so compaction triggers after a few turns.
	// window=5000, outputReserve=200 → effective=4800, threshold=4800-500=4300.
	// Each tool turn adds ~1012 tokens (4000 chars / 4 + overhead).
	// After ~4 turns: ~4048 tokens, triggers around turn 5.
	const windowSize = 5000
	const resultChars = 4000 // ~1000 tokens at 4 chars/token

	cfg := AutoCompactConfig{
		BufferTokens:           500,
		BlockingBufferTokens:   100,
		MaxConsecutiveFailures: 3,
		KeepRecent:             4,
		OutputReserve:          200,
		Enabled:                true,
	}

	p := &multiTurnProvider{maxTurns: 8, resultSize: resultChars}
	reg := NewRegistry()
	reg.Register(&largeResultTool{size: resultChars})

	session := &compactSession{}
	router := &smallWindowRouter{window: windowSize}

	var compactEvents []StreamEvent
	cb := func(ev StreamEvent) {
		if ev.Type == StreamCompact {
			compactEvents = append(compactEvents, ev)
		}
	}

	loop := New(p, reg,
		WithRouter(router),
		WithSession(session),
		WithMaxTurns(20),
		WithStreamCallback(cb),
		WithAutoCompact(cfg),
	)

	result, err := loop.Run(context.Background(), "fill context", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Verify the loop completed
	if result.Response != "Done." {
		t.Errorf("Response = %q, want 'Done.'", result.Response)
	}

	// Verify compaction happened at least once
	if len(compactEvents) == 0 {
		t.Fatal("expected at least one StreamCompact event")
	}

	// Verify StreamCompact event fields
	ev := compactEvents[0]
	if ev.TokensFreed <= 0 {
		t.Errorf("TokensFreed = %d, want > 0", ev.TokensFreed)
	}
	// MicroCompact preserves message count (elides content, not messages).
	// Snip reduces message count. Either is valid compaction.
	if ev.MessagesBefore < ev.MessagesAfter {
		t.Errorf("MessagesBefore (%d) should be >= MessagesAfter (%d)", ev.MessagesBefore, ev.MessagesAfter)
	}

	// Verify session was synced via ReplaceMessages
	if session.replaceCalls == 0 {
		t.Error("expected ReplaceMessages to be called at least once")
	}
}

func TestAutoCompactDisabledSkipsCompaction(t *testing.T) {
	cfg := AutoCompactConfig{
		Enabled: false, // disabled
	}

	// Even with a tiny window, compaction should not trigger
	p := &multiTurnProvider{maxTurns: 2, resultSize: 4000}
	reg := NewRegistry()
	reg.Register(&largeResultTool{size: 4000})

	session := &compactSession{}

	var compactEvents int
	cb := func(ev StreamEvent) {
		if ev.Type == StreamCompact {
			compactEvents++
		}
	}

	loop := New(p, reg,
		WithRouter(&smallWindowRouter{window: 25000}),
		WithSession(session),
		WithMaxTurns(10),
		WithStreamCallback(cb),
		WithAutoCompact(cfg),
	)

	_, err := loop.Run(context.Background(), "test", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if compactEvents > 0 {
		t.Errorf("got %d compact events with disabled config, want 0", compactEvents)
	}
	if session.replaceCalls > 0 {
		t.Errorf("ReplaceMessages called %d times with disabled config, want 0", session.replaceCalls)
	}
}

func TestAutoCompactCircuitBreakerInLoop(t *testing.T) {
	// Test that the circuit breaker stops compaction attempts after N failures.
	// Use a config where KeepRecent is larger than the message count, so
	// microCompact and snip are both no-ops → compaction "fails" (freed=0).
	cfg := AutoCompactConfig{
		BufferTokens:           10, // threshold = effective - 10, so almost any content triggers
		BlockingBufferTokens:   5,
		MaxConsecutiveFailures: 2,
		KeepRecent:             1000, // larger than any message count → microcompact+snip are no-ops
		OutputReserve:          10,
		Enabled:                true,
	}

	// Use large results so threshold is exceeded by turn 2, giving multiple
	// turns for the circuit breaker to accumulate failures.
	// 2000 chars = 500 tokens per tool result. threshold=480. Exceeds on turn 1.
	p := &multiTurnProvider{maxTurns: 6, resultSize: 2000}
	reg := NewRegistry()
	reg.Register(&largeResultTool{size: 2000})

	session := &compactSession{}

	var compactEvents int
	cb := func(ev StreamEvent) {
		if ev.Type == StreamCompact {
			compactEvents++
		}
	}

	loop := New(p, reg,
		WithRouter(&smallWindowRouter{window: 500}), // effective=490, threshold=480
		WithSession(session),
		WithMaxTurns(15),
		WithStreamCallback(cb),
		WithAutoCompact(cfg),
	)

	_, err := loop.Run(context.Background(), "test", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Should have 0 compact events (all attempts failed, breaker tripped)
	if compactEvents > 0 {
		t.Errorf("got %d compact events, want 0 (circuit breaker should prevent success)", compactEvents)
	}
	// Verify the circuit breaker tripped
	if !loop.autoCompactState.tripped(cfg.MaxConsecutiveFailures) {
		t.Error("circuit breaker should be tripped after consecutive failures")
	}
}

func TestAutoCompactMicroCompactElidesOldToolResultsInLoop(t *testing.T) {
	// Verify that after compaction, old tool results in the conversation
	// are actually elided (replaced with stubs).
	// window=5000, outputReserve=200 → effective=4800, threshold=4800-800=4000.
	// Each turn adds ~2000 tokens (8000 chars/4). Triggers after 2 turns.
	cfg := AutoCompactConfig{
		BufferTokens:           800,
		BlockingBufferTokens:   200,
		MaxConsecutiveFailures: 3,
		KeepRecent:             2, // keep only last 2 messages
		OutputReserve:          200,
		Enabled:                true,
	}

	// 4 tool turns with large results, then end
	p := &multiTurnProvider{maxTurns: 4, resultSize: 8000}
	reg := NewRegistry()
	reg.Register(&largeResultTool{size: 8000})

	session := &compactSession{}

	loop := New(p, reg,
		WithRouter(&smallWindowRouter{window: 5000}),
		WithSession(session),
		WithMaxTurns(15),
		WithAutoCompact(cfg),
	)

	_, err := loop.Run(context.Background(), "test", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// After compaction + completion, check that session messages
	// don't contain 8000-char tool results in older positions
	msgs := session.messages
	for i, msg := range msgs {
		for _, block := range msg.Content {
			if block.Type == "tool_result" && len(block.ResultContent) > 1000 {
				// Only recent messages should have full results
				if i < len(msgs)-cfg.KeepRecent {
					t.Errorf("message[%d] has unelided tool result (%d chars) — should have been compacted",
						i, len(block.ResultContent))
				}
			}
		}
	}
}

func TestAutoCompactSessionWithoutMessageReplacer(t *testing.T) {
	// Sessions that don't implement MessageReplacer should still compact
	// (the loop just skips the sync call).
	cfg := AutoCompactConfig{
		BufferTokens:           800,
		BlockingBufferTokens:   200,
		MaxConsecutiveFailures: 3,
		KeepRecent:             2,
		OutputReserve:          200,
		Enabled:                true,
	}

	p := &multiTurnProvider{maxTurns: 4, resultSize: 8000}
	reg := NewRegistry()
	reg.Register(&largeResultTool{size: 8000})

	// NoOpSession does NOT implement MessageReplacer
	session := &NoOpSession{}

	var compactEvents int
	cb := func(ev StreamEvent) {
		if ev.Type == StreamCompact {
			compactEvents++
		}
	}

	loop := New(p, reg,
		WithRouter(&smallWindowRouter{window: 5000}),
		WithSession(session),
		WithMaxTurns(15),
		WithStreamCallback(cb),
		WithAutoCompact(cfg),
	)

	_, err := loop.Run(context.Background(), "test", LoopConfig{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Compaction should still fire even without MessageReplacer
	if compactEvents == 0 {
		t.Error("expected compaction events even without MessageReplacer")
	}
}

func TestAutoCompactPostCompactHookInLoop(t *testing.T) {
	// Verify that PostCompactHook is called after compaction and its
	// returned messages replace the generic snip marker.
	cfg := AutoCompactConfig{
		BufferTokens:           800,
		BlockingBufferTokens:   200,
		MaxConsecutiveFailures: 3,
		KeepRecent:             2,
		OutputReserve:          200,
		Enabled:                true,
	}

	// Track hook invocations
	hookCalled := false
	var hookPreCompactLen int
	var hookPhase string

	cfg.PostCompactHook = func(preCompact []provider.Message, phase string) []provider.Message {
		hookCalled = true
		hookPreCompactLen = len(preCompact)
		hookPhase = phase
		return []provider.Message{{
			Role: provider.RoleUser,
			Content: []provider.ContentBlock{{
				Type: "text",
				Text: "[Context restored] Goal: test the hook. Phase: " + phase,
			}},
		}}
	}

	p := &multiTurnProvider{maxTurns: 4, resultSize: 8000}
	reg := NewRegistry()
	reg.Register(&largeResultTool{size: 8000})

	session := &compactSession{}

	loop := New(p, reg,
		WithRouter(&smallWindowRouter{window: 5000}),
		WithSession(session),
		WithMaxTurns(15),
		WithAutoCompact(cfg),
	)

	_, err := loop.Run(context.Background(), "test the hook", LoopConfig{
		Hints: SelectionHints{Phase: "act"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if !hookCalled {
		t.Fatal("PostCompactHook was not called")
	}

	// Hook should receive pre-compaction messages (more than keepRecent)
	if hookPreCompactLen <= cfg.KeepRecent {
		t.Errorf("hook received %d pre-compact messages, want > %d", hookPreCompactLen, cfg.KeepRecent)
	}

	// Hook should receive the phase
	if hookPhase != "act" {
		t.Errorf("hook phase = %q, want 'act'", hookPhase)
	}

	// The orientation message should be in the session (replacing the snip marker)
	found := false
	for _, msg := range session.messages {
		for _, block := range msg.Content {
			if block.Type == "text" && strings.Contains(block.Text, "[Context restored]") {
				found = true
			}
		}
	}
	if !found {
		t.Error("orientation message from PostCompactHook not found in session messages")
	}

	// The generic snip marker should NOT be present (replaced by hook)
	for _, msg := range session.messages {
		for _, block := range msg.Content {
			if block.Type == "text" && strings.Contains(block.Text, "automatically removed") {
				t.Error("generic snip marker should have been replaced by hook message")
			}
		}
	}
}
