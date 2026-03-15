package mutations

import (
	"strings"
	"testing"
)

func TestBestApproachEmpty(t *testing.T) {
	store := NewStore(t.TempDir())
	front, err := store.BestApproach(TaskFeature)
	if err != nil {
		t.Fatalf("BestApproach: %v", err)
	}
	if front != nil {
		t.Errorf("expected nil for empty store, got %v", front)
	}
}

func TestBestApproachWithData(t *testing.T) {
	store := NewStore(t.TempDir())

	// Write 3 signals: one dominated, two on Pareto front
	store.WriteForType(QualitySignal{
		SessionID: "bad", TaskType: TaskBugFix,
		Hard: HardSignals{TokenEfficiency: 0.3, TurnCount: 20},
		Soft: SoftSignals{ToolErrorRate: 0.5},
		Human: HumanSignals{Outcome: "error"},
	})
	store.WriteForType(QualitySignal{
		SessionID: "good-fast", TaskType: TaskBugFix,
		Hard: HardSignals{TokenEfficiency: 0.7, TurnCount: 5},
		Soft: SoftSignals{ToolErrorRate: 0.0},
		Human: HumanSignals{Outcome: "success"},
	})
	store.WriteForType(QualitySignal{
		SessionID: "good-efficient", TaskType: TaskBugFix,
		Hard: HardSignals{TokenEfficiency: 0.9, TurnCount: 12},
		Soft: SoftSignals{ToolErrorRate: 0.0},
		Human: HumanSignals{Outcome: "success"},
	})

	front, err := store.BestApproach(TaskBugFix)
	if err != nil {
		t.Fatalf("BestApproach: %v", err)
	}
	if len(front) != 2 {
		t.Fatalf("expected 2 on Pareto front, got %d", len(front))
	}
	for _, s := range front {
		if s.SessionID == "bad" {
			t.Error("dominated signal should not be in Pareto front")
		}
	}
}

func TestBestSummary(t *testing.T) {
	store := NewStore(t.TempDir())
	store.WriteForType(QualitySignal{
		SessionID: "s1", TaskType: TaskFeature,
		Hard: HardSignals{TokenEfficiency: 0.6, TurnCount: 8},
		Human: HumanSignals{Outcome: "success"},
	})

	summary, err := store.BestSummary(TaskFeature)
	if err != nil {
		t.Fatalf("BestSummary: %v", err)
	}
	if !strings.Contains(summary, "s1") {
		t.Errorf("summary should mention session, got: %s", summary)
	}
	if !strings.Contains(summary, "feature") {
		t.Errorf("summary should mention task type, got: %s", summary)
	}
}
