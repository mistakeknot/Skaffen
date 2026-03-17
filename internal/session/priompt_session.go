package session

import (
	"sync"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
	"github.com/mistakeknot/Masaq/priompt"
)

// PriomptSession wraps an agent.Session and uses priority-based prompt
// composition to assemble the system prompt within a token budget.
// It implements both agent.Session and agent.RenderReporter.
type PriomptSession struct {
	inner    agent.Session
	sections []priompt.Element

	mu         sync.Mutex
	lastRender priompt.RenderResult
}

// NewPriomptSession creates a PriomptSession wrapping the given session.
// The inner session handles Save/Messages; PriomptSession owns SystemPrompt
// rendering via priompt.Render. If sections is nil or empty, SystemPrompt
// returns an empty string.
func NewPriomptSession(inner agent.Session, sections []priompt.Element) *PriomptSession {
	return &PriomptSession{
		inner:    inner,
		sections: sections,
	}
}

// SystemPrompt renders the prompt elements within the given budget using
// priompt.Render, stores the result for RenderReporter access, and returns
// the assembled prompt string. Turn count is estimated from messages.
func (s *PriomptSession) SystemPrompt(phase tool.Phase, budget int) string {
	// Estimate turn count from message count (roughly 2 messages per turn).
	turnCount := len(s.inner.Messages()) / 2
	result := priompt.Render(s.sections, budget,
		priompt.WithPhase(string(phase)),
		priompt.WithTurnCount(turnCount),
	)

	s.mu.Lock()
	s.lastRender = result
	s.mu.Unlock()

	return result.Prompt
}

// Save delegates to the wrapped JSONLSession.
func (s *PriomptSession) Save(turn agent.Turn) error {
	return s.inner.Save(turn)
}

// Messages delegates to the wrapped JSONLSession.
func (s *PriomptSession) Messages() []provider.Message {
	return s.inner.Messages()
}

// --- RenderReporter interface ---

// ExcludedElements returns the names of dynamic elements excluded in the last render.
func (s *PriomptSession) ExcludedElements() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRender.Excluded
}

// ExcludedStableElements returns the names of stable elements excluded in the last render.
func (s *PriomptSession) ExcludedStableElements() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRender.ExcludedStable
}

// PromptTokens returns the estimated token count of the last rendered prompt.
func (s *PriomptSession) PromptTokens() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRender.TotalTokens
}

// RenderStableTokens returns the stable prefix token count from the last render.
// Returns 0 if any stable element was excluded (partial prefix invalidates cache).
func (s *PriomptSession) RenderStableTokens() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRender.StableTokens
}
