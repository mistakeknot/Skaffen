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

func TestFormatWithIntentDebugging(t *testing.T) {
	cs := CompactionSummary{
		Goal:         "Fix login bug",
		Decisions:    []string{"Use bcrypt"},
		FilesRead:    []string{"auth.go"},
		FilesMutated: []string{"auth.go"},
		TestResults:  "1 failed",
		Errors:       []string{"TestLogin: wrong hash"},
	}

	result := cs.FormatWithIntent(IntentDebugging)

	// Errors should appear before files and decisions
	errIdx := strings.Index(result, "Errors encountered")
	fileIdx := strings.Index(result, "Files read")
	decIdx := strings.Index(result, "Decisions")
	if errIdx > fileIdx {
		t.Error("debugging intent should put errors before files")
	}
	if errIdx > decIdx {
		t.Error("debugging intent should put errors before decisions")
	}
}

func TestFormatWithIntentBuilding(t *testing.T) {
	cs := CompactionSummary{
		Goal:         "Add feature",
		Decisions:    []string{"Use pattern X"},
		FilesRead:    []string{"main.go"},
		FilesMutated: []string{"handler.go"},
		TestResults:  "all pass",
		Errors:       []string{"minor warning"},
	}

	result := cs.FormatWithIntent(IntentBuilding)

	// Files should appear before errors and test results
	fileIdx := strings.Index(result, "Files modified")
	errIdx := strings.Index(result, "Errors encountered")
	testIdx := strings.Index(result, "Tests")
	if fileIdx > errIdx {
		t.Error("building intent should put files before errors")
	}
	if fileIdx > testIdx {
		t.Error("building intent should put files before tests")
	}
}

func TestFormatWithIntentDefaultFallback(t *testing.T) {
	cs := CompactionSummary{
		Goal:    "test",
		Errors:  []string{"err"},
	}

	// Default intent should produce same output as Format()
	defaultResult := cs.FormatWithIntent(IntentDefault)
	formatResult := cs.Format()
	if defaultResult != formatResult {
		t.Errorf("default intent should match Format()\ngot:  %q\nwant: %q", defaultResult, formatResult)
	}
}

func TestFormatWithIntentEmpty(t *testing.T) {
	cs := CompactionSummary{}
	if cs.FormatWithIntent(IntentDebugging) != "" {
		t.Error("empty summary with intent should produce empty string")
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
