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
