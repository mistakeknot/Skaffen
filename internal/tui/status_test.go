package tui

import (
	"strings"
	"testing"
)

func TestStatusBarRenders(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "act", "claude", 0.25, 30, 3, false, "", experimentSlotData{}, nil, statusBarExtra{})
	view := sb.View()
	if view == "" {
		t.Fatal("status bar should not be empty")
	}
}

func TestStatusBarOmitsPhase(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "reflect", "claude", 0.0, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	view := sb.View()
	// Phase is shown in the breadcrumb, not the status bar
	if strings.Contains(view, "reflect") {
		t.Fatal("status bar should not contain phase name (shown in breadcrumb)")
	}
}

func TestStatusBarContainsModel(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "act", "opus", 0.0, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	view := sb.View()
	if !strings.Contains(view, "opus") {
		t.Fatal("status bar should contain model name")
	}
}

func TestStatusBarCostFormatting(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "act", "claude", 1.50, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	view := sb.View()
	if !strings.Contains(view, "$1.50") {
		t.Fatal("status bar should format cost as $X.XX")
	}
}

func TestStatusBarTurns(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "act", "claude", 0, 0, 7, false, "", experimentSlotData{}, nil, statusBarExtra{})
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
	updateStatusSlots(&sb, "act", "claude", 0, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	view := sb.View()
	if !strings.Contains(view, "│") {
		t.Fatal("status bar should contain separators")
	}
}

func TestStatusBarAllPhasesRender(t *testing.T) {
	// Status bar should render without error for all phases,
	// even though phase is no longer displayed (it's in breadcrumb).
	for _, phase := range phaseOrder {
		sb := newStatusBar(120)
		updateStatusSlots(&sb, phase, "claude", 0, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
		view := sb.View()
		if view == "" {
			t.Errorf("status bar should render for phase %q", phase)
		}
	}
}

func TestStatusBarCostThresholds(t *testing.T) {
	sb := newStatusBar(120)
	// These should not panic — we're testing that different cost ranges render
	updateStatusSlots(&sb, "act", "claude", 0.10, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	sb.View()
	updateStatusSlots(&sb, "act", "claude", 0.75, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	sb.View()
	updateStatusSlots(&sb, "act", "claude", 3.00, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	sb.View()
}

func TestStatusBarContextThresholds(t *testing.T) {
	sb := newStatusBar(120)
	// These should not panic — testing different context ranges render
	updateStatusSlots(&sb, "act", "claude", 0, 25, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	sb.View()
	updateStatusSlots(&sb, "act", "claude", 0, 65, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	sb.View()
	updateStatusSlots(&sb, "act", "claude", 0, 90, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	sb.View()
}

func TestStatusBarZeroWidth(t *testing.T) {
	sb := newStatusBar(0)
	updateStatusSlots(&sb, "act", "claude", 0, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	// Should not panic
	_ = sb.View()
}

func TestStatusBarPlanMode(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "act", "claude", 0, 0, 0, true, "", experimentSlotData{}, nil, statusBarExtra{})
	view := sb.View()
	if !strings.Contains(view, "PLAN") {
		t.Errorf("plan mode badge missing from status bar: %q", view)
	}
}

func TestStatusBarPlanModeOff(t *testing.T) {
	sb := newStatusBar(120)
	updateStatusSlots(&sb, "act", "claude", 0, 0, 0, false, "", experimentSlotData{}, nil, statusBarExtra{})
	view := sb.View()
	if strings.Contains(view, "PLAN") {
		t.Errorf("PLAN badge should not appear when plan mode off: %q", view)
	}
}

func TestStatusBarCustomItems(t *testing.T) {
	sb := newStatusBar(120)
	items := []StatusBarItem{ItemModel, ItemTurns} // cost hidden
	updateStatusSlots(&sb, "act", "opus", 1.50, 0, 3, false, "", experimentSlotData{}, items, statusBarExtra{})
	view := sb.View()
	if !strings.Contains(view, "opus") {
		t.Error("custom items: model should be visible")
	}
	if !strings.Contains(view, "3 turns") {
		t.Error("custom items: turns should be visible")
	}
	if strings.Contains(view, "$1.50") {
		t.Error("custom items: cost should be hidden")
	}
}

func TestStatusBarExtraSession(t *testing.T) {
	sb := newStatusBar(120)
	items := []StatusBarItem{ItemModel, ItemSession}
	extra := statusBarExtra{SessionName: "my-session"}
	updateStatusSlots(&sb, "act", "opus", 0, 0, 0, false, "", experimentSlotData{}, items, extra)
	view := sb.View()
	if !strings.Contains(view, "my-session") {
		t.Errorf("session name missing from status bar: %q", view)
	}
}

func TestStatusBarExtraBranch(t *testing.T) {
	sb := newStatusBar(120)
	items := []StatusBarItem{ItemModel, ItemBranch}
	extra := statusBarExtra{GitBranch: "feat/foo"}
	updateStatusSlots(&sb, "act", "opus", 0, 0, 0, false, "", experimentSlotData{}, items, extra)
	view := sb.View()
	if !strings.Contains(view, "feat/foo") {
		t.Errorf("branch name missing from status bar: %q", view)
	}
}

func TestStatusBarExtraFiles(t *testing.T) {
	sb := newStatusBar(120)
	items := []StatusBarItem{ItemModel, ItemFiles}
	extra := statusBarExtra{FilesChanged: 5}
	updateStatusSlots(&sb, "act", "opus", 0, 0, 0, false, "", experimentSlotData{}, items, extra)
	view := sb.View()
	if !strings.Contains(view, "5 files") {
		t.Errorf("file count missing from status bar: %q", view)
	}
}

func TestStatusBarDefaultItems(t *testing.T) {
	items := DefaultStatusBarItems()
	if len(items) != 3 {
		t.Fatalf("expected 3 default items, got %d", len(items))
	}
	if items[0] != ItemModel || items[1] != ItemCost || items[2] != ItemTurns {
		t.Errorf("unexpected defaults: %v", items)
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
