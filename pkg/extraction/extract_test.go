package extraction

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/pkg/prompts"
)

// mockStore implements EntityStore for testing.
type mockStore struct {
	entities    []Entity
	applied     []Entity
	dirtyUsers  []string
	feedbacks   []FeedbackEntity
	applyErr    error
}

func (m *mockStore) GetActive(_ context.Context, _ string, _ int) ([]Entity, error) {
	return m.entities, nil
}

func (m *mockStore) Apply(_ context.Context, _, _ string, entities []Entity, _ string) (int, error) {
	if m.applyErr != nil {
		return 0, m.applyErr
	}
	m.applied = append(m.applied, entities...)
	changes := 0
	for _, e := range entities {
		if e.Action != ActionNoop {
			changes++
		}
	}
	return changes, nil
}

func (m *mockStore) SetProfileDirty(_ context.Context, userID string) error {
	m.dirtyUsers = append(m.dirtyUsers, userID)
	return nil
}

func (m *mockStore) ApplyFeedback(_ context.Context, _ string, entities []FeedbackEntity) error {
	m.feedbacks = append(m.feedbacks, entities...)
	return nil
}

// mockProvider implements LLMProvider for testing.
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func TestExtractWithSignal(t *testing.T) {
	result := ExtractionResult{
		HasPreferenceSignal: true,
		Entities: []Entity{
			{Domain: "goals", Type: "career", Value: "wants to lead", Valence: ValencePositive, Origin: OriginStated, Confidence: 0.9, Action: ActionAdd},
		},
	}
	responseJSON, _ := json.Marshal(result)

	store := &mockStore{}
	provider := &mockProvider{response: string(responseJSON)}
	ext := NewExtractor(store, provider)

	turns := []prompts.Turn{
		{Role: "user", Content: "I want to lead a team someday"},
	}
	changes, err := ext.Extract(context.Background(), "user1", "sess1", turns)
	if err != nil {
		t.Fatal(err)
	}
	if changes != 1 {
		t.Errorf("changes = %d, want 1", changes)
	}
	if len(store.applied) != 1 {
		t.Fatalf("applied = %d, want 1", len(store.applied))
	}
	if store.applied[0].Value != "wants to lead" {
		t.Errorf("value = %q, want %q", store.applied[0].Value, "wants to lead")
	}
	if len(store.dirtyUsers) != 1 || store.dirtyUsers[0] != "user1" {
		t.Error("expected profile marked dirty")
	}
}

func TestExtractNoSignal(t *testing.T) {
	store := &mockStore{}
	provider := &mockProvider{response: `{"has_preference_signal": false, "entities": []}`}
	ext := NewExtractor(store, provider)

	changes, err := ext.Extract(context.Background(), "user1", "sess1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes != 0 {
		t.Errorf("changes = %d, want 0", changes)
	}
	if len(store.dirtyUsers) != 0 {
		t.Error("should not mark dirty when no changes")
	}
}

func TestExtractLLMError(t *testing.T) {
	store := &mockStore{}
	provider := &mockProvider{err: fmt.Errorf("timeout")}
	ext := NewExtractor(store, provider)

	_, err := ext.Extract(context.Background(), "user1", "sess1", nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestExtractFeedback(t *testing.T) {
	result := FeedbackResult{
		HasFeedback: true,
		Entities: []FeedbackEntity{
			{Type: "delivery", Value: "be more direct", Valence: ValenceNegative, Confidence: 0.85},
		},
	}
	responseJSON, _ := json.Marshal(result)

	store := &mockStore{}
	provider := &mockProvider{response: string(responseJSON)}
	ext := NewExtractor(store, provider)

	turns := []prompts.Turn{
		{Role: "user", Content: "stop being so vague, be more direct"},
	}
	err := ext.ExtractFeedback(context.Background(), "user1", "stop being vague", turns)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.feedbacks) != 1 {
		t.Fatalf("feedbacks = %d, want 1", len(store.feedbacks))
	}
	if store.feedbacks[0].Value != "be more direct" {
		t.Errorf("value = %q", store.feedbacks[0].Value)
	}
}

func TestScheduleExtractionDebounce(t *testing.T) {
	store := &mockStore{}
	provider := &mockProvider{response: `{"has_preference_signal": false, "entities": []}`}
	ext := NewExtractor(store, provider)
	ext.debounceWindow = 50 * time.Millisecond

	turns := []prompts.Turn{{Role: "user", Content: "msg1"}}

	// Schedule twice rapidly — first should be cancelled
	ext.ScheduleExtraction("user1", "sess1", turns)
	ext.ScheduleExtraction("user1", "sess1", turns)

	// Wait for debounce + execution
	time.Sleep(200 * time.Millisecond)

	// Timer cleanup
	ext.mu.Lock()
	_, pending := ext.pendingTimers["user1"]
	ext.mu.Unlock()
	if pending {
		t.Error("timer should be cleaned up after execution")
	}
}

func TestFormatConversation(t *testing.T) {
	turns := []prompts.Turn{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you"},
	}
	got := FormatConversation(turns, 2)
	if got != "Auraken: hi there\nUser: how are you" {
		t.Errorf("got %q", got)
	}
}

func TestFormatEntitiesEmpty(t *testing.T) {
	got := FormatEntities(nil)
	if got != "(No preferences tracked yet)" {
		t.Errorf("got %q", got)
	}
}

func TestFormatEntities(t *testing.T) {
	entities := []Entity{
		{Domain: "goals", Type: "career", Value: "lead a team", Valence: ValencePositive, Origin: OriginStated},
	}
	got := FormatEntities(entities)
	if got != "- goals/career: lead a team (positive, stated)" {
		t.Errorf("got %q", got)
	}
}
