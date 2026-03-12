package tui

import (
	"strings"
	"testing"
)

func TestStatusBarRenders(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0.25, 30, 3)
	view := sb.View()
	if view == "" {
		t.Fatal("status bar should not be empty")
	}
}

func TestStatusBarContainsPhase(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "review", "claude", 0.0, 0, 0)
	view := sb.View()
	if !strings.Contains(view, "review") {
		t.Fatal("status bar should contain phase name")
	}
}

func TestStatusBarContainsModel(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "opus", 0.0, 0, 0)
	view := sb.View()
	if !strings.Contains(view, "opus") {
		t.Fatal("status bar should contain model name")
	}
}

func TestStatusBarCostFormatting(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 1.50, 0, 0)
	view := sb.View()
	if !strings.Contains(view, "$1.50") {
		t.Fatal("status bar should format cost as $X.XX")
	}
}

func TestStatusBarTurns(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0, 0, 7)
	view := sb.View()
	if !strings.Contains(view, "7 turns") {
		t.Fatal("status bar should show turn count")
	}
}

func TestStatusBarContextPercent(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0, 75, 0)
	view := sb.View()
	if !strings.Contains(view, "75%") {
		t.Fatal("status bar should show context percentage")
	}
}

func TestStatusBarSeparators(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0, 0, 0)
	view := sb.View()
	if !strings.Contains(view, "│") {
		t.Fatal("status bar should contain separators")
	}
}

func TestStatusBarAllPhases(t *testing.T) {
	for _, phase := range phaseOrder {
		sb := newStatusBar(120)
		updateStatusSlots(&sb, phase, "claude", 0, 0, 0)
		view := sb.View()
		if !strings.Contains(view, phase) {
			t.Errorf("status bar should contain phase %q", phase)
		}
	}
}

func TestStatusBarCostThresholds(t *testing.T) {
	sb := newStatusBar(120)
	// These should not panic — we're testing that different cost ranges render
	updateStatusSlots(&sb, "build", "claude", 0.10, 0, 0)
	sb.View()
	updateStatusSlots(&sb, "build", "claude", 0.75, 0, 0)
	sb.View()
	updateStatusSlots(&sb, "build", "claude", 3.00, 0, 0)
	sb.View()
}

func TestStatusBarContextThresholds(t *testing.T) {
	sb := newStatusBar(120)
	// These should not panic — testing different context ranges render
	updateStatusSlots(&sb, "build", "claude", 0, 25, 0)
	sb.View()
	updateStatusSlots(&sb, "build", "claude", 0, 65, 0)
	sb.View()
	updateStatusSlots(&sb, "build", "claude", 0, 90, 0)
	sb.View()
}

func TestStatusBarZeroWidth(t *testing.T) {
	sb := newStatusBar(0)
	updateStatusSlots(&sb, "build", "claude", 0, 0, 0)
	// Should not panic
	_ = sb.View()
}
