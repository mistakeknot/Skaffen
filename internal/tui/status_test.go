package tui

import (
	"strings"
	"testing"
)

func TestStatusBarRenders(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0.25, 30, 3, false, "")
	view := sb.View()
	if view == "" {
		t.Fatal("status bar should not be empty")
	}
}

func TestStatusBarContainsPhase(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "review", "claude", 0.0, 0, 0, false, "")
	view := sb.View()
	if !strings.Contains(view, "review") {
		t.Fatal("status bar should contain phase name")
	}
}

func TestStatusBarContainsModel(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "opus", 0.0, 0, 0, false, "")
	view := sb.View()
	if !strings.Contains(view, "opus") {
		t.Fatal("status bar should contain model name")
	}
}

func TestStatusBarCostFormatting(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 1.50, 0, 0, false, "")
	view := sb.View()
	if !strings.Contains(view, "$1.50") {
		t.Fatal("status bar should format cost as $X.XX")
	}
}

func TestStatusBarTurns(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0, 0, 7, false, "")
	view := sb.View()
	if !strings.Contains(view, "7 turns") {
		t.Fatal("status bar should show turn count")
	}
}

func TestContextMeterShowsPercentage(t *testing.T) {
	m := newContextMeter(80)
	m.SetValue(float64(contextMaxTokens)*0.75, contextMaxTokens)
	view := m.View()
	if !strings.Contains(view, "75%") {
		t.Fatalf("context meter should show 75%%, got %q", view)
	}
}

func TestStatusBarSeparators(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0, 0, 0, false, "")
	view := sb.View()
	if !strings.Contains(view, "│") {
		t.Fatal("status bar should contain separators")
	}
}

func TestStatusBarAllPhases(t *testing.T) {
	for _, phase := range phaseOrder {
		sb := newStatusBar(120)
		updateStatusSlots(&sb, phase, "claude", 0, 0, 0, false, "")
		view := sb.View()
		if !strings.Contains(view, phase) {
			t.Errorf("status bar should contain phase %q", phase)
		}
	}
}

func TestStatusBarCostThresholds(t *testing.T) {
	sb := newStatusBar(120)
	// These should not panic — we're testing that different cost ranges render
	updateStatusSlots(&sb, "build", "claude", 0.10, 0, 0, false, "")
	sb.View()
	updateStatusSlots(&sb, "build", "claude", 0.75, 0, 0, false, "")
	sb.View()
	updateStatusSlots(&sb, "build", "claude", 3.00, 0, 0, false, "")
	sb.View()
}

func TestStatusBarContextThresholds(t *testing.T) {
	sb := newStatusBar(120)
	// These should not panic — testing different context ranges render
	updateStatusSlots(&sb, "build", "claude", 0, 25, 0, false, "")
	sb.View()
	updateStatusSlots(&sb, "build", "claude", 0, 65, 0, false, "")
	sb.View()
	updateStatusSlots(&sb, "build", "claude", 0, 90, 0, false, "")
	sb.View()
}

func TestStatusBarZeroWidth(t *testing.T) {
	sb := newStatusBar(0)
	updateStatusSlots(&sb, "build", "claude", 0, 0, 0, false, "")
	// Should not panic
	_ = sb.View()
}

func TestStatusBarPlanMode(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0, 0, 0, true, "")
	view := sb.View()
	if !strings.Contains(view, "PLAN") {
		t.Errorf("plan mode badge missing from status bar: %q", view)
	}
	if !strings.Contains(view, "build") {
		t.Errorf("phase should still show after PLAN badge: %q", view)
	}
}

func TestStatusBarPlanModeOff(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "build", "claude", 0, 0, 0, false, "")
	view := sb.View()
	if strings.Contains(view, "PLAN") {
		t.Errorf("PLAN badge should not appear when plan mode off: %q", view)
	}
}

func TestContextMeterForecast(t *testing.T) {
	m := newContextMeter(60)
	m.SetValue(0, contextMaxTokens)
	view := m.View()
	if view == "" {
		t.Fatal("meter should render")
	}
	// Forecast marker at 80% should be present (│ character)
	if !strings.Contains(view, "│") {
		t.Fatal("meter should show forecast marker")
	}
}

func TestContextMeterLabel(t *testing.T) {
	m := newContextMeter(60)
	m.SetValue(50000, contextMaxTokens)
	view := m.View()
	if !strings.Contains(view, "ctx") {
		t.Fatalf("meter should show ctx label, got %q", view)
	}
}

func TestTokenSparklineRenders(t *testing.T) {
	s := newTokenSparkline(80)
	s.Push(10000)
	s.Push(15000)
	s.Push(12000)
	view := s.View()
	if view == "" {
		t.Fatal("sparkline should render after pushes")
	}
}

func TestMeterRowCombined(t *testing.T) {
	m := newContextMeter(80)
	m.SetValue(100000, contextMaxTokens)
	s := newTokenSparkline(80)
	s.Push(5000)
	row := renderMeterRow(m, s, 80)
	if row == "" {
		t.Fatal("meter row should render")
	}
	if !strings.Contains(row, "│") {
		t.Fatal("meter row should contain separator between meter and sparkline")
	}
}
