package subagent

import (
	"sync"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// AggregatingEmitter buffers evidence from a subagent, tags events with
// subagent metadata, and flushes to a parent emitter on completion.
type AggregatingEmitter struct {
	subagentID   string
	subagentType string
	parent       agentloop.Emitter

	mu     sync.Mutex
	events []agentloop.Evidence
	usage  provider.Usage
}

// NewAggregatingEmitter creates an emitter that buffers events for a subagent.
func NewAggregatingEmitter(subagentID, subagentType string, parent agentloop.Emitter) *AggregatingEmitter {
	return &AggregatingEmitter{
		subagentID:   subagentID,
		subagentType: subagentType,
		parent:       parent,
	}
}

// Emit buffers an evidence event, tagging it with subagent metadata.
func (e *AggregatingEmitter) Emit(ev agentloop.Evidence) error {
	ev.SessionID = e.subagentID // tag with subagent ID for attribution

	e.mu.Lock()
	defer e.mu.Unlock()

	e.events = append(e.events, ev)
	e.usage.InputTokens += ev.TokensIn
	e.usage.OutputTokens += ev.TokensOut
	e.usage.CacheCreationInputTokens += ev.CacheCreationTokens
	e.usage.CacheReadInputTokens += ev.CacheReadTokens
	return nil
}

// Flush sends all buffered events to the parent emitter.
func (e *AggregatingEmitter) Flush() {
	e.mu.Lock()
	events := make([]agentloop.Evidence, len(e.events))
	copy(events, e.events)
	parent := e.parent
	e.mu.Unlock()

	if parent == nil {
		return
	}

	for _, ev := range events {
		_ = parent.Emit(ev) // best-effort forwarding
	}
}

// Events returns a copy of buffered evidence events.
func (e *AggregatingEmitter) Events() []agentloop.Evidence {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]agentloop.Evidence, len(e.events))
	copy(out, e.events)
	return out
}

// TotalUsage returns aggregated token usage across all turns.
func (e *AggregatingEmitter) TotalUsage() provider.Usage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.usage
}
