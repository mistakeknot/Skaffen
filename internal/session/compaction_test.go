package session

import (
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/agent"
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

func TestCompactionSummaryFormat(t *testing.T) {
	cs := CompactionSummary{
		Goal:         "Fix login bug",
		Phase:        "act",
		Decisions:    []string{"Use bcrypt instead of SHA256"},
		FilesRead:    []string{"auth.go", "auth_test.go"},
		FilesMutated: []string{"auth.go"},
		TestResults:  "3 passed, 1 failed",
		Errors:       []string{"TestLogin: wrong hash algorithm"},
	}

	result := cs.Format()

	if !strings.Contains(result, "**Goal:** Fix login bug") {
		t.Error("missing goal")
	}
	if !strings.Contains(result, "**Phase:** act") {
		t.Error("missing phase")
	}
	if !strings.Contains(result, "Use bcrypt") {
		t.Error("missing decision")
	}
	if !strings.Contains(result, "auth.go, auth_test.go") {
		t.Error("missing files read")
	}
	if !strings.Contains(result, "**Files modified:** auth.go") {
		t.Error("missing files mutated")
	}
	if !strings.Contains(result, "3 passed, 1 failed") {
		t.Error("missing test results")
	}
}

func TestCompactionSummaryFormatEmpty(t *testing.T) {
	cs := CompactionSummary{}
	if cs.Format() != "" {
		t.Error("empty summary should produce empty string")
	}
}

func TestCompactionSummaryDedup(t *testing.T) {
	cs := CompactionSummary{
		Goal:      "test",
		FilesRead: []string{"a.go", "b.go", "a.go", "c.go", "b.go"},
	}
	result := cs.Format()
	if !strings.Contains(result, "a.go, b.go, c.go") {
		t.Errorf("dedup failed, got: %s", result)
	}
}

func TestCompactStructured(t *testing.T) {
	dir := t.TempDir()
	s := New("structured", dir, "sys", 100)

	// Add 20 messages (10 turns)
	for i := 0; i < 10; i++ {
		s.Save(agent.Turn{
			Phase: tool.PhaseAct,
			Messages: []provider.Message{
				{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "q"}}},
				{Role: provider.RoleAssistant, Content: []provider.ContentBlock{{Type: "text", Text: "a"}}},
			},
		})
	}

	summary := CompactionSummary{
		Goal:         "Implement feature X",
		FilesRead:    []string{"main.go"},
		FilesMutated: []string{"handler.go"},
	}

	before, after := s.CompactStructured(summary, 4)
	if before != 20 {
		t.Errorf("before: got %d, want 20", before)
	}
	if after != 5 { // 1 summary + 4 recent
		t.Errorf("after: got %d, want 5", after)
	}

	msgs := s.Messages()
	if !strings.Contains(msgs[0].Content[0].Text, "Structured context") {
		t.Error("first message should be structured summary")
	}
	if !strings.Contains(msgs[0].Content[0].Text, "Implement feature X") {
		t.Error("summary should contain goal")
	}
}

func TestCompactStructuredEmptySummaryNoOp(t *testing.T) {
	dir := t.TempDir()
	s := New("noop", dir, "sys", 100)

	s.Save(agent.Turn{
		Phase: tool.PhaseAct,
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: []provider.ContentBlock{{Type: "text", Text: "q"}}},
		},
	})

	before, after := s.CompactStructured(CompactionSummary{}, 4)
	if before != after {
		t.Errorf("empty summary should be no-op: before=%d, after=%d", before, after)
	}
}
