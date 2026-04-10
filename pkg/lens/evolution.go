package lens

import (
	"context"
	"math"
	"strings"
	"sync"
)

// Store abstracts persistence for lens evolution state.
type Store interface {
	Load(ctx context.Context) error
	RecordEvent(ctx context.Context, lensID, userID, event string) error
	UsageCount(ctx context.Context, lensID string) (int, error)
	Effectiveness(ctx context.Context, lensID string) (float64, error)
	Flush(ctx context.Context) error
}

// Tracker provides in-memory lens evolution tracking with concurrent safety.
type Tracker interface {
	RecordEvent(lensID, userID, event string) error
	Effectiveness(lensID string) float64
	ClassifyEngagement(message string, pendingLensNames []string) string
}

// EMA weights for effectiveness scoring — matches Python exactly.
const (
	engagedDelta        = 0.1
	ignoredDelta        = -0.05
	pushedBackDelta     = -0.1
	confidenceFloor     = 0.1
	explorationBonus    = 0.15
	explorationThreshold = 3
)

// pushbackPhrases mirrors the Python classify_engagement pushback signals.
var pushbackPhrases = []string{
	"that's not it", "that doesn't apply", "not really",
	"i disagree", "that's not what", "wrong framework",
	"not helpful", "that misses", "off base",
}

// engagementSignals mirrors the Python classify_engagement engagement signals.
var engagementSignals = []string{
	"good point", "that makes sense", "interesting",
	"tell me more", "how would", "what if",
	"you're right", "i see what", "that explains",
	"so basically", "in other words",
}

// ComputeDelta returns the base effectiveness delta for an event type.
// Exported for testing.
func ComputeDelta(event string) float64 {
	switch event {
	case "engaged":
		return engagedDelta
	case "ignored":
		return ignoredDelta
	case "pushed_back":
		return pushedBackDelta
	default:
		return 0.0
	}
}

// ApplyEffectivenessUpdate applies an EMA update to an effectiveness score.
// New lenses (usageCount < explorationThreshold) receive an exploration
// bonus to prevent cold-start death spiral. Exported for testing.
func ApplyEffectivenessUpdate(current float64, event string, usageCount int) float64 {
	delta := ComputeDelta(event)

	// Exploration bonus for new lenses.
	if usageCount < explorationThreshold {
		delta += explorationBonus
	}

	newScore := current + delta
	return math.Max(confidenceFloor, math.Min(1.0, newScore))
}

// ClassifyEngagement classifies user engagement with previously applied lenses.
// Returns "pushed_back", "engaged", or "ignored".
func ClassifyEngagement(message string, pendingLensNames []string) string {
	msgLower := strings.ToLower(message)

	// Pushback signals.
	for _, phrase := range pushbackPhrases {
		if strings.Contains(msgLower, phrase) {
			return "pushed_back"
		}
	}

	// Engagement signals — user references lens concepts or asks to explore.
	lensRefs := false
	for _, name := range pendingLensNames {
		parts := strings.Fields(name)
		if len(parts) == 0 {
			continue
		}
		firstWord := strings.ToLower(parts[0])
		if len(firstWord) > 3 && strings.Contains(msgLower, firstWord) {
			lensRefs = true
			break
		}
	}

	if lensRefs {
		return "engaged"
	}
	for _, signal := range engagementSignals {
		if strings.Contains(msgLower, signal) {
			return "engaged"
		}
	}

	return "ignored"
}

// tracker is the concrete Tracker implementation backed by a Store.
type tracker struct {
	mu    sync.Mutex
	store Store
}

// NewTracker returns a Tracker that delegates persistence to the given Store.
func NewTracker(store Store) *tracker {
	return &tracker{store: store}
}

// RecordEvent records a lens usage event and updates effectiveness.
func (t *tracker) RecordEvent(lensID, userID, event string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ctx := context.Background()

	if err := t.store.RecordEvent(ctx, lensID, userID, event); err != nil {
		return err
	}
	return nil
}

// Effectiveness returns the current effectiveness score for a lens.
func (t *tracker) Effectiveness(lensID string) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	ctx := context.Background()
	eff, err := t.store.Effectiveness(ctx, lensID)
	if err != nil {
		return 0.5 // default score
	}
	return eff
}

// ClassifyEngagement delegates to the package-level function.
func (t *tracker) ClassifyEngagement(message string, pendingLensNames []string) string {
	return ClassifyEngagement(message, pendingLensNames)
}

// MemoryStore is an in-memory Store implementation for testing.
type MemoryStore struct {
	mu     sync.Mutex
	usage  map[string]int     // lensID -> count
	scores map[string]float64 // lensID -> effectiveness
}

// NewMemoryStore returns an initialized MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		usage:  make(map[string]int),
		scores: make(map[string]float64),
	}
}

// Load is a no-op for the in-memory store.
func (m *MemoryStore) Load(_ context.Context) error { return nil }

// RecordEvent records an event and applies the effectiveness update.
func (m *MemoryStore) RecordEvent(_ context.Context, lensID, _ string, event string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := m.usage[lensID]
	current, ok := m.scores[lensID]
	if !ok {
		current = 0.5 // default starting score
	}

	m.scores[lensID] = ApplyEffectivenessUpdate(current, event, count)
	m.usage[lensID] = count + 1
	return nil
}

// UsageCount returns the number of events recorded for a lens.
func (m *MemoryStore) UsageCount(_ context.Context, lensID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.usage[lensID], nil
}

// Effectiveness returns the current effectiveness score for a lens.
func (m *MemoryStore) Effectiveness(_ context.Context, lensID string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	score, ok := m.scores[lensID]
	if !ok {
		return 0.5, nil
	}
	return score, nil
}

// Flush is a no-op for the in-memory store.
func (m *MemoryStore) Flush(_ context.Context) error { return nil }
