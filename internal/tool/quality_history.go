package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mistakeknot/Skaffen/internal/mutations"
)

// SignalReader reads quality signals. Defined here to avoid importing agent
// (which would create an import cycle since agent imports tool).
type SignalReader interface {
	ReadRecent(n int) ([]mutations.QualitySignal, error)
}

// QualityHistoryTool exposes quality signal history during Orient phase.
type QualityHistoryTool struct {
	store SignalReader
}

// NewQualityHistoryTool creates a quality history tool backed by the given store.
func NewQualityHistoryTool(store SignalReader) *QualityHistoryTool {
	return &QualityHistoryTool{store: store}
}

func (t *QualityHistoryTool) Name() string { return "quality_history" }

func (t *QualityHistoryTool) Description() string {
	return "View quality signals from recent Skaffen sessions. Shows token efficiency, tool error rates, complexity tiers, and outcomes to help Orient phase planning."
}

func (t *QualityHistoryTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"count": {
				"type": "integer",
				"description": "Number of recent sessions to show (default 5, max 20)"
			}
		}
	}`)
}

func (t *QualityHistoryTool) Execute(_ context.Context, params json.RawMessage) ToolResult {
	var input struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		input.Count = 5
	}
	if input.Count <= 0 {
		input.Count = 5
	}
	if input.Count > 20 {
		input.Count = 20
	}

	signals, err := t.store.ReadRecent(input.Count)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("error reading quality signals: %v", err), IsError: true}
	}

	if len(signals) == 0 {
		return ToolResult{Content: "No quality signals recorded yet. This is the first session or no Compound phase has run."}
	}

	data, err := json.MarshalIndent(signals, "", "  ")
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("error formatting signals: %v", err), IsError: true}
	}

	return ToolResult{Content: fmt.Sprintf("Quality signals from last %d session(s):\n%s", len(signals), string(data))}
}
