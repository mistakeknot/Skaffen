package mutations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAggregate(t *testing.T) {
	dir := t.TempDir()
	sessionID := "test-session"

	// Write 3 evidence turns
	records := []evidenceRecord{
		{SessionID: sessionID, Phase: "act", TurnNumber: 1, TokensIn: 1000, TokensOut: 500, Outcome: "success", ComplexityTier: 2},
		{SessionID: sessionID, Phase: "act", TurnNumber: 2, TokensIn: 800, TokensOut: 600, Outcome: "error", ComplexityTier: 3},
		{SessionID: sessionID, Phase: "reflect", TurnNumber: 3, TokensIn: 200, TokensOut: 100, Outcome: "success", ComplexityTier: 3},
	}

	path := filepath.Join(dir, sessionID+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}
	for _, r := range records {
		data, _ := json.Marshal(r)
		f.Write(data)
		f.Write([]byte("\n"))
	}
	f.Close()

	sig, err := Aggregate(dir, sessionID)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	// Hard signals
	if sig.Hard.TurnCount != 3 {
		t.Errorf("turn_count = %d, want 3", sig.Hard.TurnCount)
	}
	// TokenEfficiency = totalOut/totalIn = (500+600+100)/(1000+800+200) = 1200/2000 = 0.6
	expectedEff := 0.6
	if sig.Hard.TokenEfficiency < expectedEff-0.01 || sig.Hard.TokenEfficiency > expectedEff+0.01 {
		t.Errorf("token_efficiency = %f, want ~%f", sig.Hard.TokenEfficiency, expectedEff)
	}

	// Soft signals
	if sig.Soft.ComplexityTier != 3 {
		t.Errorf("complexity_tier = %d, want 3", sig.Soft.ComplexityTier)
	}
	// ToolErrorRate = 1 error / 3 turns ≈ 0.333
	if sig.Soft.ToolErrorRate < 0.3 || sig.Soft.ToolErrorRate > 0.4 {
		t.Errorf("tool_error_rate = %f, want ~0.333", sig.Soft.ToolErrorRate)
	}

	// Human signals
	if sig.Human.Outcome != "success" {
		t.Errorf("outcome = %q, want success", sig.Human.Outcome)
	}

	if sig.SessionID != sessionID {
		t.Errorf("session_id = %q, want %q", sig.SessionID, sessionID)
	}
}

func TestAggregateEmptyEvidence(t *testing.T) {
	dir := t.TempDir()
	sessionID := "empty-session"

	// Create empty file
	path := filepath.Join(dir, sessionID+".jsonl")
	os.WriteFile(path, []byte{}, 0644)

	sig, err := Aggregate(dir, sessionID)
	if err != nil {
		t.Fatalf("aggregate empty: %v", err)
	}

	if sig.Hard.TurnCount != 0 {
		t.Errorf("turn_count = %d, want 0", sig.Hard.TurnCount)
	}
}

func TestAggregateMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Aggregate(dir, "nonexistent")
	if err == nil {
		t.Error("expected error for missing evidence file")
	}
}
