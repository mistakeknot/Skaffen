package extraction

import "time"

// Valence describes the polarity of a preference signal.
type Valence string

const (
	ValencePositive Valence = "positive"
	ValenceNegative Valence = "negative"
	ValenceNeutral  Valence = "neutral"
)

// Origin describes how a preference was expressed.
type Origin string

const (
	OriginStated    Origin = "stated"    // User explicitly said it
	OriginRevealed  Origin = "revealed"  // Inferred from behavior/choices
	OriginSubmerged Origin = "submerged" // Surfaced unexpectedly through conversation
)

// Action describes what to do with an extracted entity.
type Action string

const (
	ActionAdd    Action = "ADD"
	ActionUpdate Action = "UPDATE"
	ActionExpire Action = "EXPIRE"
	ActionNoop   Action = "NOOP"
)

// Entity represents a preference or context signal about a user.
type Entity struct {
	ID                string     `json:"id,omitempty"`
	Domain            string     `json:"domain"`
	Type              string     `json:"type"`
	Value             string     `json:"value"`
	Valence           Valence    `json:"valence"`
	Origin            Origin     `json:"origin"`
	Confidence        float64    `json:"confidence"`
	Action            Action     `json:"action"`
	ValidUntil        *time.Time `json:"valid_until,omitempty"`
	SourceEpisodeIDs  []string   `json:"source_episode_ids,omitempty"`
	SourceContradicts string     `json:"source_contradicts,omitempty"`
	Updates           string     `json:"updates,omitempty"` // Old value when action=UPDATE
}

// Episode represents an immutable evidence record of a conversation chunk.
type Episode struct {
	ID        string `json:"id,omitempty"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	ChunkText string `json:"chunk_text"`
	ChunkType string `json:"chunk_type"` // "exchange"
}

// ExtractionResult holds the parsed output from the extraction LLM call.
type ExtractionResult struct {
	HasPreferenceSignal bool     `json:"has_preference_signal"`
	Entities            []Entity `json:"entities"`
}

// FeedbackEntity represents a meta-feedback signal about the bot's behavior.
type FeedbackEntity struct {
	Type       string  `json:"type"`       // "lens_fit", "delivery", "outcome"
	Value      string  `json:"value"`
	Valence    Valence `json:"valence"`
	Confidence float64 `json:"confidence"`
}

// FeedbackResult holds the parsed output from the feedback extraction LLM call.
type FeedbackResult struct {
	HasFeedback bool             `json:"has_feedback"`
	Entities    []FeedbackEntity `json:"entities"`
}
