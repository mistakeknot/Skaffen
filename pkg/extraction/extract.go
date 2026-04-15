package extraction

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mistakeknot/Skaffen/pkg/prompts"
)

// Extractor orchestrates the preference extraction pipeline.
type Extractor struct {
	store    EntityStore
	provider LLMProvider

	mu             sync.Mutex
	pendingTimers  map[string]*time.Timer
	debounceWindow time.Duration
}

// NewExtractor creates an Extractor with the given store and LLM provider.
func NewExtractor(store EntityStore, provider LLMProvider) *Extractor {
	return &Extractor{
		store:          store,
		provider:       provider,
		pendingTimers:  make(map[string]*time.Timer),
		debounceWindow: 3 * time.Second,
	}
}

// Extract runs the full extraction pipeline for a user's recent messages.
// Returns the number of entity changes applied.
func (e *Extractor) Extract(ctx context.Context, userID, sessionID string, messages []prompts.Turn) (int, error) {
	conversationText := FormatConversation(messages, 10)

	existing, err := e.store.GetActive(ctx, userID, 30)
	if err != nil {
		return 0, fmt.Errorf("get active entities: %w", err)
	}
	existingText := FormatEntities(existing)

	prompt := fmt.Sprintf(ExtractionPrompt, existingText, conversationText)
	raw, err := e.provider.Complete(ctx, ExtractionSystemPrompt, prompt)
	if err != nil {
		return 0, fmt.Errorf("llm extraction: %w", err)
	}

	result, err := ParseExtractionResponse(raw)
	if err != nil {
		return 0, fmt.Errorf("parse extraction: %w", err)
	}

	if !result.HasPreferenceSignal || len(result.Entities) == 0 {
		return 0, nil
	}

	changes, err := e.store.Apply(ctx, userID, sessionID, result.Entities, conversationText)
	if err != nil {
		return 0, fmt.Errorf("apply entities: %w", err)
	}

	if changes > 0 {
		if err := e.store.SetProfileDirty(ctx, userID); err != nil {
			return changes, fmt.Errorf("set profile dirty: %w", err)
		}
	}

	return changes, nil
}

// ExtractFeedback extracts meta-feedback about the bot and stores it.
func (e *Extractor) ExtractFeedback(ctx context.Context, userID string, text string, history []prompts.Turn) error {
	conversationText := FormatConversation(history, 6)

	prompt := fmt.Sprintf(FeedbackExtractionPrompt, conversationText)
	raw, err := e.provider.Complete(ctx, ExtractionSystemPrompt, prompt)
	if err != nil {
		return fmt.Errorf("llm feedback extraction: %w", err)
	}

	result, err := ParseFeedbackResponse(raw)
	if err != nil {
		return fmt.Errorf("parse feedback: %w", err)
	}

	if !result.HasFeedback || len(result.Entities) == 0 {
		return nil
	}

	return e.store.ApplyFeedback(ctx, userID, result.Entities)
}

// ScheduleExtraction schedules an extraction with burst deduplication.
// If a new message arrives within the debounce window, the previous
// extraction is cancelled and rescheduled with the updated messages.
func (e *Extractor) ScheduleExtraction(userID, sessionID string, messages []prompts.Turn) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if existing, ok := e.pendingTimers[userID]; ok {
		existing.Stop()
	}

	e.pendingTimers[userID] = time.AfterFunc(e.debounceWindow, func() {
		e.mu.Lock()
		delete(e.pendingTimers, userID)
		e.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = e.Extract(ctx, userID, sessionID, messages)
	})
}
