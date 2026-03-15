package mutations

import (
	"strings"
	"testing"
)

func TestClassifyTask(t *testing.T) {
	tests := []struct {
		desc string
		want TaskType
	}{
		{"Fix the broken login flow", TaskBugFix},
		{"Bug in session handling", TaskBugFix},
		{"Refactor the mutations package", TaskRefactor},
		{"Extract helper function", TaskRefactor},
		{"Optimize query performance", TaskOptimization},
		{"Make rendering faster", TaskOptimization},
		{"Update README with new API", TaskDocs},
		{"Add documentation for mutations", TaskDocs},
		{"Add quality signal support", TaskFeature},
		{"Implement new Orient phase", TaskFeature},
		{"Random unclassifiable thing", TaskGeneral},
	}

	for _, tt := range tests {
		got := ClassifyTask(tt.desc)
		if got != tt.want {
			t.Errorf("ClassifyTask(%q) = %q, want %q", tt.desc, got, tt.want)
		}
	}
}

func TestInspireEmpty(t *testing.T) {
	store := NewStore(t.TempDir())
	insp := store.Inspire("Add a new feature")

	if insp.TaskType != TaskFeature {
		t.Errorf("task type = %q, want %q", insp.TaskType, TaskFeature)
	}
	// No history, should have default suggestion
	if len(insp.Suggestions) == 0 {
		t.Error("expected at least one default suggestion")
	}
}

func TestInspireWithHistory(t *testing.T) {
	store := NewStore(t.TempDir())

	// Seed with a feature signal
	store.WriteForType(QualitySignal{
		SessionID: "prev-feat",
		TaskType:  TaskFeature,
		Hard:      HardSignals{TokenEfficiency: 0.7, TurnCount: 8},
		Human:     HumanSignals{Outcome: "success"},
	})

	insp := store.Inspire("Implement new widget")
	if insp.BestHistory == "" {
		t.Error("expected best history from mutations store")
	}
	if !strings.Contains(insp.BestHistory, "prev-feat") {
		t.Errorf("best history should reference prev-feat, got: %s", insp.BestHistory)
	}
}

func TestFormatInspirationEmpty(t *testing.T) {
	result := FormatInspiration(Inspiration{TaskType: TaskGeneral})
	if result != "" {
		t.Errorf("empty inspiration should format to empty string, got: %q", result)
	}
}

func TestFormatInspirationWithData(t *testing.T) {
	insp := Inspiration{
		TaskType:    TaskBugFix,
		BestHistory: "Best: session s1, 5 turns",
		Suggestions: []Suggestion{{
			TaskType: TaskBugFix,
			Approach: "Try reproducing first",
		}},
	}

	result := FormatInspiration(insp)
	if !strings.Contains(result, "Orient Inspiration") {
		t.Error("should contain header")
	}
	if !strings.Contains(result, "bug-fix") {
		t.Error("should contain task type")
	}
	if !strings.Contains(result, "Try reproducing first") {
		t.Error("should contain suggestion")
	}
}
