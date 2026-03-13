package subagent

import (
	"sync"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
)

type collectEmitter struct {
	mu     sync.Mutex
	events []agentloop.Evidence
}

func (c *collectEmitter) Emit(ev agentloop.Evidence) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
	return nil
}

func TestAggregatingEmitter_TagsEvents(t *testing.T) {
	parent := &collectEmitter{}
	agg := NewAggregatingEmitter("sub-1", "explore", parent)

	if err := agg.Emit(agentloop.Evidence{TurnNumber: 1, TokensIn: 100}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if err := agg.Emit(agentloop.Evidence{TurnNumber: 2, TokensIn: 200}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	agg.Flush()

	if len(parent.events) != 2 {
		t.Fatalf("parent got %d events, want 2", len(parent.events))
	}
	for _, ev := range parent.events {
		if ev.SessionID != "sub-1" {
			t.Errorf("SessionID = %q, want sub-1", ev.SessionID)
		}
	}
}

func TestAggregatingEmitter_TotalUsage(t *testing.T) {
	parent := &collectEmitter{}
	agg := NewAggregatingEmitter("sub-1", "explore", parent)

	if err := agg.Emit(agentloop.Evidence{TokensIn: 100, TokensOut: 50}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if err := agg.Emit(agentloop.Evidence{TokensIn: 200, TokensOut: 75}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	total := agg.TotalUsage()
	if total.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", total.InputTokens)
	}
	if total.OutputTokens != 125 {
		t.Errorf("OutputTokens = %d, want 125", total.OutputTokens)
	}
}

func TestAggregatingEmitter_Events(t *testing.T) {
	parent := &collectEmitter{}
	agg := NewAggregatingEmitter("sub-1", "explore", parent)

	if err := agg.Emit(agentloop.Evidence{TurnNumber: 1}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if err := agg.Emit(agentloop.Evidence{TurnNumber: 2}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	events := agg.Events()
	if len(events) != 2 {
		t.Fatalf("Events() = %d, want 2", len(events))
	}
}
