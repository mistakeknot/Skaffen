package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/mutations"
)

type mockSignalReader struct {
	signals []mutations.QualitySignal
	err     error
}

func (m *mockSignalReader) ReadRecent(n int) ([]mutations.QualitySignal, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.signals) <= n {
		return m.signals, nil
	}
	return m.signals[len(m.signals)-n:], nil
}

func TestQualityHistoryToolEmpty(t *testing.T) {
	tool := NewQualityHistoryTool(&mockSignalReader{})

	result := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "No quality signals") {
		t.Errorf("expected empty message, got: %s", result.Content)
	}
}

func TestQualityHistoryToolWithSignals(t *testing.T) {
	store := &mockSignalReader{
		signals: []mutations.QualitySignal{
			{SessionID: "s1", Phase: "compound", Hard: mutations.HardSignals{TurnCount: 10}},
			{SessionID: "s2", Phase: "compound", Hard: mutations.HardSignals{TurnCount: 14}},
		},
	}
	tool := NewQualityHistoryTool(store)

	result := tool.Execute(context.Background(), json.RawMessage(`{"count": 5}`))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "s1") || !strings.Contains(result.Content, "s2") {
		t.Errorf("expected both sessions in output, got: %s", result.Content)
	}
}

func TestQualityHistoryToolDefaultCount(t *testing.T) {
	store := &mockSignalReader{
		signals: []mutations.QualitySignal{
			{SessionID: "s1", Phase: "compound"},
		},
	}
	tool := NewQualityHistoryTool(store)

	// Empty params should default to 5
	result := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "s1") {
		t.Errorf("expected session in output, got: %s", result.Content)
	}
}

func TestQualityHistoryToolPhaseGating(t *testing.T) {
	store := &mockSignalReader{}
	reg := NewRegistry()
	RegisterQualityHistory(reg, store)

	// Should be available in Orient
	orientTools := reg.Tools(PhaseOrient)
	found := false
	for _, td := range orientTools {
		if td.Name == "quality_history" {
			found = true
			break
		}
	}
	if !found {
		t.Error("quality_history should be available in Orient phase")
	}

	// Should NOT be available in Act
	actTools := reg.Tools(PhaseAct)
	for _, td := range actTools {
		if td.Name == "quality_history" {
			t.Error("quality_history should NOT be available in Act phase")
		}
	}
}
