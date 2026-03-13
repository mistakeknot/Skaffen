package tui

import (
	"strings"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/subagent"
)

func TestSubagentBlock_CollapsedView(t *testing.T) {
	b := newSubagentBlock("sub-1", "find all Go files")
	b.update(subagent.StatusUpdate{
		ID:          "sub-1",
		Description: "find all Go files",
		Status:      subagent.StatusDone,
		Turn:        3,
		MaxTurns:    10,
		TokensUsed:  1234,
	})
	b.response = "Found 42 Go files in the project."

	view := b.View(80, false) // collapsed
	if !strings.Contains(view, "find all Go files") {
		t.Error("collapsed view should contain description")
	}
	if !strings.Contains(view, "done") {
		t.Error("collapsed view should show status")
	}
	if !strings.Contains(view, "1.2k") {
		t.Error("collapsed view should show token count")
	}
}

func TestSubagentBlock_ExpandedView(t *testing.T) {
	b := newSubagentBlock("sub-1", "find files")
	b.response = "Found files:\n- main.go\n- types.go"
	b.update(subagent.StatusUpdate{Status: subagent.StatusDone})

	view := b.View(80, true) // expanded
	if !strings.Contains(view, "main.go") {
		t.Error("expanded view should contain response")
	}
}

func TestSubagentBlock_RunningSpinner(t *testing.T) {
	b := newSubagentBlock("sub-1", "searching")
	b.update(subagent.StatusUpdate{
		Status:   subagent.StatusRunning,
		Turn:     2,
		MaxTurns: 10,
	})

	view := b.View(80, false)
	if !strings.Contains(view, "searching") {
		t.Error("running view should contain description")
	}
	if !strings.Contains(view, "turn 2/10") {
		t.Error("running view should show turn progress")
	}
}

func TestSubagentTracker_MultipleBlocks(t *testing.T) {
	tracker := newSubagentTracker()
	tracker.update(subagent.StatusUpdate{ID: "sub-1", Description: "task 1", Status: subagent.StatusDone, TokensUsed: 500})
	tracker.update(subagent.StatusUpdate{ID: "sub-2", Description: "task 2", Status: subagent.StatusRunning, Turn: 1, MaxTurns: 10})

	view := tracker.View(80)
	if !strings.Contains(view, "task 1") {
		t.Error("should contain first block")
	}
	if !strings.Contains(view, "task 2") {
		t.Error("should contain second block")
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{500, "500"},
		{1234, "1.2k"},
		{15000, "15k"},
	}
	for _, tt := range tests {
		got := formatTokens(tt.n)
		if got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
