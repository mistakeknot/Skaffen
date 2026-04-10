package lens

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Depth controls verbosity of inter-lens transition text.
type Depth string

const (
	DepthDeepGold    Depth = "deep_gold"
	DepthShallowGold Depth = "shallow_gold"
	DepthWax         Depth = "wax"
)

// Transition templates indexed by depth.
// deep_gold is terse; shallow_gold names domains; wax is fully explicit.
var transitionTemplates = map[Depth]string{
	DepthDeepGold:    "Your answer just changed what this problem is about.",
	DepthShallowGold: "Notice the shift? A moment ago the question was about %s. Now it's about %s.",
	DepthWax:         "We've moved from %s to %s. The first lens addressed %s \u2014 now we're looking at %s.",
}

// ErrStackExhausted is returned when NextPhase is called after all lenses
// in the stack have been consumed.
var ErrStackExhausted = errors.New("lens: stack exhausted — all phases complete")

// PhaseResult describes the output of a single orchestrator phase advance.
type PhaseResult struct {
	Lens               string `json:"lens"`
	PhaseIndex         int    `json:"phase_index"`
	ProblemRedefinition string `json:"problem_redefinition"`
	TransitionText     string `json:"transition_text"`
	AnnealingSuggested bool   `json:"annealing_suggested"`
	AnnealingSeconds   int    `json:"annealing_seconds"`
}

// StackOrchestrator drives a sequence of lenses through a problem,
// generating explicit transition text at each phase boundary.
// All exported fields carry json tags for serialization.
type StackOrchestrator struct {
	Lenses           []string `json:"lenses"`
	DepthMode        Depth    `json:"depth"`
	AnnealingSeconds int      `json:"annealing_seconds"`
	PhaseIndex       int      `json:"phase_index"`

	mu sync.Mutex
}

// NewOrchestrator creates a StackOrchestrator for the given lens sequence
// and depth mode.
func NewOrchestrator(lenses []string, depth Depth) *StackOrchestrator {
	return &StackOrchestrator{
		Lenses:    lenses,
		DepthMode: depth,
	}
}

// WithAnnealing sets the annealing pause duration (seconds) suggested
// between phase transitions. Returns the orchestrator for chaining.
func (s *StackOrchestrator) WithAnnealing(seconds int) *StackOrchestrator {
	s.AnnealingSeconds = seconds
	return s
}

// NextPhase advances the orchestrator by one lens, returning a PhaseResult
// describing the current phase and its transition from the previous one.
// Returns ErrStackExhausted when all lenses have been consumed.
func (s *StackOrchestrator) NextPhase(userInput string) (PhaseResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.PhaseIndex
	if idx >= len(s.Lenses) {
		return PhaseResult{}, ErrStackExhausted
	}

	lens := s.Lenses[idx]

	result := PhaseResult{
		Lens:       lens,
		PhaseIndex: idx,
	}

	// Phases after the first get transition metadata.
	if idx > 0 {
		prevLens := s.Lenses[idx-1]
		result.ProblemRedefinition = userInput
		result.TransitionText = formatTransition(s.DepthMode, prevLens, lens)
		if s.AnnealingSeconds > 0 {
			result.AnnealingSuggested = true
			result.AnnealingSeconds = s.AnnealingSeconds
		}
	}

	s.PhaseIndex++
	return result, nil
}

// IsComplete reports whether all lenses in the stack have been consumed.
func (s *StackOrchestrator) IsComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.PhaseIndex >= len(s.Lenses)
}

// ToJSON serializes the orchestrator state to JSON.
func (s *StackOrchestrator) ToJSON() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Marshal(s)
}

// FromJSON deserializes a StackOrchestrator from JSON.
func FromJSON(data []byte) (*StackOrchestrator, error) {
	var so StackOrchestrator
	if err := json.Unmarshal(data, &so); err != nil {
		return nil, fmt.Errorf("lens: unmarshal stack orchestrator: %w", err)
	}
	return &so, nil
}

// formatTransition renders the transition text for a given depth,
// using the previous and next lens names.
func formatTransition(depth Depth, prevLens, nextLens string) string {
	tmpl, ok := transitionTemplates[depth]
	if !ok {
		// Fall back to shallow_gold for unknown depths.
		tmpl = transitionTemplates[DepthShallowGold]
		depth = DepthShallowGold
	}

	if depth == DepthDeepGold {
		return tmpl
	}

	prevDomain := strings.ToLower(prevLens)
	nextDomain := strings.ToLower(nextLens)

	if depth == DepthShallowGold {
		return fmt.Sprintf(tmpl, prevDomain, nextDomain)
	}

	// Wax: prev_lens (original case), next_lens (original case),
	// prev_domain (lowercase), next_domain (lowercase).
	return fmt.Sprintf(tmpl, prevLens, nextLens, prevDomain, nextDomain)
}
