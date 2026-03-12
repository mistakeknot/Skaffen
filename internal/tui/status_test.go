package tui

import (
	"strings"
	"testing"
)

func TestStatusBarRenders(t *testing.T) {
	s := statusModel{width: 120}
	view := s.View("build", "claude", 0.25, 30, 3)
	if view == "" {
		t.Fatal("status bar should not be empty")
	}
}

func TestStatusBarContainsPhase(t *testing.T) {
	s := statusModel{width: 120}
	view := s.View("review", "claude", 0.0, 0, 0)
	if !strings.Contains(view, "review") {
		t.Fatal("status bar should contain phase name")
	}
}

func TestStatusBarContainsModel(t *testing.T) {
	s := statusModel{width: 120}
	view := s.View("build", "opus", 0.0, 0, 0)
	if !strings.Contains(view, "opus") {
		t.Fatal("status bar should contain model name")
	}
}

func TestStatusBarCostFormatting(t *testing.T) {
	s := statusModel{width: 120}
	view := s.View("build", "claude", 1.50, 0, 0)
	if !strings.Contains(view, "$1.50") {
		t.Fatal("status bar should format cost as $X.XX")
	}
}

func TestStatusBarTurns(t *testing.T) {
	s := statusModel{width: 120}
	view := s.View("build", "claude", 0, 0, 7)
	if !strings.Contains(view, "7 turns") {
		t.Fatal("status bar should show turn count")
	}
}

func TestStatusBarContextPercent(t *testing.T) {
	s := statusModel{width: 120}
	view := s.View("build", "claude", 0, 75, 0)
	if !strings.Contains(view, "75%") {
		t.Fatal("status bar should show context percentage")
	}
}

func TestStatusBarSeparators(t *testing.T) {
	s := statusModel{width: 120}
	view := s.View("build", "claude", 0, 0, 0)
	if !strings.Contains(view, "|") {
		t.Fatal("status bar should contain separators")
	}
}

func TestStatusBarAllPhases(t *testing.T) {
	s := statusModel{width: 120}
	for _, phase := range phaseOrder {
		view := s.View(phase, "claude", 0, 0, 0)
		if !strings.Contains(view, phase) {
			t.Errorf("status bar should contain phase %q", phase)
		}
	}
}

func TestStatusBarCostThresholds(t *testing.T) {
	s := statusModel{width: 120}
	// These should not panic — we're testing that different cost ranges render
	s.View("build", "claude", 0.10, 0, 0) // low cost (green)
	s.View("build", "claude", 0.75, 0, 0) // medium cost (warning)
	s.View("build", "claude", 3.00, 0, 0) // high cost (error)
}

func TestStatusBarContextThresholds(t *testing.T) {
	s := statusModel{width: 120}
	// These should not panic — testing different context ranges render
	s.View("build", "claude", 0, 25, 0) // low (green)
	s.View("build", "claude", 0, 65, 0) // medium (warning)
	s.View("build", "claude", 0, 90, 0) // high (error)
}

func TestStatusBarZeroWidth(t *testing.T) {
	s := statusModel{width: 0}
	view := s.View("build", "claude", 0, 0, 0)
	// Should not panic
	if view == "" {
		t.Fatal("status bar should render even at zero width")
	}
}
