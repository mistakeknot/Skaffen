package mutations

import (
	"strings"
	"testing"
)

func TestSuggestNoHistory(t *testing.T) {
	store := NewStore(t.TempDir())
	suggestions, err := store.Suggest(TaskFeature)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 default suggestion, got %d", len(suggestions))
	}
	if !strings.Contains(suggestions[0].Approach, "No prior data") {
		t.Errorf("expected default suggestion, got: %s", suggestions[0].Approach)
	}
}

func TestSuggestWithHistory(t *testing.T) {
	store := NewStore(t.TempDir())

	// Write a successful signal
	store.WriteForType(QualitySignal{
		SessionID: "s1", TaskType: TaskBugFix,
		Hard: HardSignals{TokenEfficiency: 0.6, TurnCount: 8},
		Soft: SoftSignals{ToolErrorRate: 0.0},
		Human: HumanSignals{Outcome: "success"},
	})

	suggestions, err := store.Suggest(TaskBugFix)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Should include a reference to the best session
	found := false
	for _, s := range suggestions {
		if strings.Contains(s.Approach, "s1") {
			found = true
			break
		}
	}
	if !found {
		t.Error("suggestions should reference the best session")
	}
}

func TestSuggestHighTurns(t *testing.T) {
	store := NewStore(t.TempDir())
	store.WriteForType(QualitySignal{
		SessionID: "long", TaskType: TaskRefactor,
		Hard: HardSignals{TokenEfficiency: 0.5, TurnCount: 25},
		Human: HumanSignals{Outcome: "success"},
	})

	suggestions, err := store.Suggest(TaskRefactor)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	found := false
	for _, s := range suggestions {
		if strings.Contains(s.Approach, "smaller steps") {
			found = true
			break
		}
	}
	if !found {
		t.Error("high turn count should trigger 'smaller steps' suggestion")
	}
}

func TestFormatSuggestions(t *testing.T) {
	suggestions := []Suggestion{
		{TaskType: TaskBugFix, Approach: "Try X", Rationale: "Because Y"},
	}
	result := FormatSuggestions(suggestions)
	if !strings.Contains(result, "Mutation Suggestions") {
		t.Errorf("expected header, got: %s", result)
	}
	if !strings.Contains(result, "Try X") {
		t.Errorf("expected approach, got: %s", result)
	}
}
