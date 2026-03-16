package session

import (
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestScoreMutationHighest(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "edit auth.go"}}},
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "tool_use", Name: "edit"}}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "tool_result", ResultContent: "File has been updated successfully"}}},
	}

	scored := ScoreMessages(msgs, nil)
	// Mutation result should have the highest score
	if scored[2].Score < scored[0].Score {
		t.Errorf("tool_result with mutation should score higher than user text: got %.1f vs %.1f", scored[2].Score, scored[0].Score)
	}
	if scored[1].Score < scored[0].Score {
		t.Errorf("tool_use edit should score higher than user text: got %.1f vs %.1f", scored[1].Score, scored[0].Score)
	}
}

func TestScoreActiveFileBoost(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "looking at auth.go"}}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "general question"}}},
	}

	scored := ScoreMessages(msgs, []string{"auth.go"})
	// First message references auth.go, should get +3 boost
	if scored[0].Score <= scored[1].Score {
		t.Errorf("message referencing active file should score higher: got %.1f vs %.1f", scored[0].Score, scored[1].Score)
	}
}

func TestScoreTestOutput(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "tool_result", ResultContent: "PASS: TestLogin (0.01s)"}}},
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "tool_result", ResultContent: "some other output"}}},
	}

	scored := ScoreMessages(msgs, nil)
	if scored[0].Score <= scored[1].Score {
		t.Errorf("test output should score higher: got %.1f vs %.1f", scored[0].Score, scored[1].Score)
	}
}

func TestTopKPreservesOrder(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "low"}}},                                    // score ~1
		{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "tool_use", Name: "edit"}}},                           // score ~8
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "another low"}}},                             // score ~1
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "tool_result", ResultContent: "File has been updated"}}},    // score ~10
	}

	scored := ScoreMessages(msgs, nil)
	indices := TopK(scored, 2)

	// Should return indices 1 and 3 (highest scores), in original order
	if len(indices) != 2 {
		t.Fatalf("expected 2 indices, got %d", len(indices))
	}
	if indices[0] != 1 || indices[1] != 3 {
		t.Errorf("expected [1, 3], got %v", indices)
	}
}

func TestTopKAllIfSmall(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "hi"}}},
	}

	scored := ScoreMessages(msgs, nil)
	indices := TopK(scored, 10)
	if len(indices) != 1 {
		t.Errorf("expected 1 index for 1 message, got %d", len(indices))
	}
}

func TestScoreEmpty(t *testing.T) {
	scored := ScoreMessages(nil, nil)
	if len(scored) != 0 {
		t.Errorf("expected 0 scored, got %d", len(scored))
	}
}
