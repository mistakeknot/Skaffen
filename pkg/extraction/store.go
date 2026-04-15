package extraction

import "context"

// EntityStore provides access to user preference entities.
// Concrete implementation deferred to the shared identity package (benl.10).
type EntityStore interface {
	// GetActive returns active (non-expired) entities for a user, ordered by
	// creation date descending, limited to the most recent entries.
	GetActive(ctx context.Context, userID string, limit int) ([]Entity, error)

	// Apply processes extracted entities against the database. For each entity:
	// ADD creates a new record, UPDATE expires the old and creates new,
	// EXPIRE marks existing records as expired. Returns the number of changes.
	// episodeText is stored as an immutable evidence record.
	Apply(ctx context.Context, userID, sessionID string, entities []Entity, episodeText string) (int, error)

	// SetProfileDirty marks the user's profile as needing regeneration.
	SetProfileDirty(ctx context.Context, userID string) error

	// ApplyFeedback stores meta-feedback entities with a TTL.
	ApplyFeedback(ctx context.Context, userID string, entities []FeedbackEntity) error
}

// LLMProvider abstracts the LLM call for extraction.
// Skaffen's provider.Provider can be adapted to this interface.
type LLMProvider interface {
	// Complete sends a system prompt + user prompt and returns the response text.
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
