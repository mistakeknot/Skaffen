package mutations

import (
	"encoding/json"
	"testing"
)

func TestQualitySignalJSONRoundTrip(t *testing.T) {
	boolTrue := true
	sig := QualitySignal{
		SessionID: "test-123",
		Timestamp: "2026-03-14T12:00:00Z",
		Phase:     "compound",
		Hard: HardSignals{
			TestsPassed:     &boolTrue,
			TokenEfficiency: 0.52,
			TurnCount:       14,
		},
		Soft: SoftSignals{
			ComplexityTier: 3,
			ToolErrorRate:  0.1,
		},
		Human: HumanSignals{
			ApprovalRate: 0.95,
			Outcome:      "success",
		},
	}

	data, err := json.Marshal(sig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got QualitySignal
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SessionID != sig.SessionID {
		t.Errorf("session_id = %q, want %q", got.SessionID, sig.SessionID)
	}
	if got.Hard.TurnCount != 14 {
		t.Errorf("turn_count = %d, want 14", got.Hard.TurnCount)
	}
	if got.Hard.TestsPassed == nil || !*got.Hard.TestsPassed {
		t.Error("tests_passed should be true")
	}
	if got.Soft.ComplexityTier != 3 {
		t.Errorf("complexity_tier = %d, want 3", got.Soft.ComplexityTier)
	}
	if got.Human.Outcome != "success" {
		t.Errorf("outcome = %q, want %q", got.Human.Outcome, "success")
	}
}

func TestQualitySignalZeroValue(t *testing.T) {
	var sig QualitySignal
	data, err := json.Marshal(sig)
	if err != nil {
		t.Fatalf("marshal zero value: %v", err)
	}

	var got QualitySignal
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal zero value: %v", err)
	}

	if got.Hard.TestsPassed != nil {
		t.Error("tests_passed should be nil for zero value")
	}
	if got.Hard.TurnCount != 0 {
		t.Errorf("turn_count = %d, want 0", got.Hard.TurnCount)
	}
}

func TestDominates(t *testing.T) {
	// A is strictly better than B on all dimensions
	a := QualitySignal{
		Hard:  HardSignals{TokenEfficiency: 0.8, TurnCount: 5},
		Soft:  SoftSignals{ToolErrorRate: 0.0, ToolDenialRate: 0.0},
		Human: HumanSignals{ApprovalRate: 1.0, Outcome: "success"},
	}
	b := QualitySignal{
		Hard:  HardSignals{TokenEfficiency: 0.5, TurnCount: 10},
		Soft:  SoftSignals{ToolErrorRate: 0.2, ToolDenialRate: 0.1},
		Human: HumanSignals{ApprovalRate: 0.8, Outcome: "error"},
	}

	if !a.Dominates(&b) {
		t.Error("a should dominate b (better on all dimensions)")
	}
	if b.Dominates(&a) {
		t.Error("b should NOT dominate a")
	}
}

func TestDominatesPartialTrade(t *testing.T) {
	// A is better on some, B is better on others — neither dominates
	a := QualitySignal{
		Hard:  HardSignals{TokenEfficiency: 0.8, TurnCount: 20},
		Soft:  SoftSignals{ToolErrorRate: 0.0},
		Human: HumanSignals{ApprovalRate: 1.0, Outcome: "success"},
	}
	b := QualitySignal{
		Hard:  HardSignals{TokenEfficiency: 0.5, TurnCount: 5},
		Soft:  SoftSignals{ToolErrorRate: 0.0},
		Human: HumanSignals{ApprovalRate: 1.0, Outcome: "success"},
	}

	if a.Dominates(&b) {
		t.Error("a should NOT dominate b (b has fewer turns)")
	}
	if b.Dominates(&a) {
		t.Error("b should NOT dominate a (a has better efficiency)")
	}
}

func TestDominatesEqual(t *testing.T) {
	a := QualitySignal{
		Hard:  HardSignals{TokenEfficiency: 0.5, TurnCount: 10},
		Human: HumanSignals{Outcome: "success"},
	}
	b := a // identical

	if a.Dominates(&b) {
		t.Error("equal signals should NOT dominate each other")
	}
}

func TestParetoFront(t *testing.T) {
	dominated := QualitySignal{
		SessionID: "worst",
		Hard:      HardSignals{TokenEfficiency: 0.3, TurnCount: 20},
		Soft:      SoftSignals{ToolErrorRate: 0.5, ToolDenialRate: 0.3},
		Human:     HumanSignals{ApprovalRate: 0.5, Outcome: "error"},
	}
	frontA := QualitySignal{
		SessionID: "fast",
		Hard:      HardSignals{TokenEfficiency: 0.8, TurnCount: 5},
		Soft:      SoftSignals{ToolErrorRate: 0.0, ToolDenialRate: 0.0},
		Human:     HumanSignals{ApprovalRate: 1.0, Outcome: "success"},
	}
	frontB := QualitySignal{
		SessionID: "efficient",
		Hard:      HardSignals{TokenEfficiency: 0.9, TurnCount: 15},
		Soft:      SoftSignals{ToolErrorRate: 0.1, ToolDenialRate: 0.0},
		Human:     HumanSignals{ApprovalRate: 0.9, Outcome: "success"},
	}

	front := ParetoFront([]QualitySignal{dominated, frontA, frontB})

	if len(front) != 2 {
		t.Fatalf("Pareto front should have 2 elements, got %d", len(front))
	}

	// Check that dominated is excluded
	for _, s := range front {
		if s.SessionID == "worst" {
			t.Error("dominated signal should not be in Pareto front")
		}
	}
}

func TestParetoFrontEmpty(t *testing.T) {
	front := ParetoFront(nil)
	if front != nil {
		t.Errorf("empty input should return nil, got %v", front)
	}
}

func TestParetoFrontSingle(t *testing.T) {
	sig := QualitySignal{SessionID: "only"}
	front := ParetoFront([]QualitySignal{sig})
	if len(front) != 1 || front[0].SessionID != "only" {
		t.Error("single signal should be its own Pareto front")
	}
}
