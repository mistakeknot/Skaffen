package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSidebarModel_NewHasFourTabs(t *testing.T) {
	sb := newSidebarModel(40, 20)
	v := sb.View()
	for _, tab := range []string{"Files", "Git", "Tools", "Debug"} {
		if !strings.Contains(v, tab) {
			t.Errorf("sidebar view missing tab %q", tab)
		}
	}
}

func TestSidebarModel_TabCycling(t *testing.T) {
	sb := newSidebarModel(40, 20)
	if sb.activeTab != 0 {
		t.Fatalf("expected initial tab 0, got %d", sb.activeTab)
	}
	sb, _ = sb.Update(tea.KeyMsg{Type: tea.KeyTab})
	if sb.activeTab != 1 {
		t.Errorf("expected tab 1 after Tab, got %d", sb.activeTab)
	}
	sb, _ = sb.Update(tea.KeyMsg{Type: tea.KeyTab})
	if sb.activeTab != 2 {
		t.Errorf("expected tab 2 after second Tab, got %d", sb.activeTab)
	}
	// Wrap around
	sb, _ = sb.Update(tea.KeyMsg{Type: tea.KeyTab})
	sb, _ = sb.Update(tea.KeyMsg{Type: tea.KeyTab})
	if sb.activeTab != 0 {
		t.Errorf("expected tab 0 after wrap, got %d", sb.activeTab)
	}
}

func TestSidebarModel_TabCyclingReverse(t *testing.T) {
	sb := newSidebarModel(40, 20)
	sb, _ = sb.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if sb.activeTab != 3 {
		t.Errorf("expected tab 3 after reverse Tab, got %d", sb.activeTab)
	}
}

func TestSidebarModel_TrackFile(t *testing.T) {
	sb := newSidebarModel(40, 20)
	sb.TrackFile("src/main.go", true)
	sb.TrackFile("README.md", false)
	v := sb.View()
	if !strings.Contains(v, "main.go") {
		t.Error("expected tracked file in view")
	}
}

func TestSidebarModel_TrackFile_Dedup(t *testing.T) {
	sb := newSidebarModel(40, 20)
	sb.TrackFile("src/main.go", false)
	sb.TrackFile("src/main.go", true)
	if len(sb.files) != 1 {
		t.Errorf("expected 1 file after dedup, got %d", len(sb.files))
	}
	if !sb.files[0].Mutated {
		t.Error("expected file to be marked as mutated after upgrade")
	}
}

func TestSidebarModel_AddToolCall(t *testing.T) {
	sb := newSidebarModel(40, 20)
	sb.AddToolCall("read", "src/main.go", 150)
	sb.activeTab = 2 // Tools tab
	v := sb.View()
	if !strings.Contains(v, "read") {
		t.Error("expected tool call in Tools tab view")
	}
}

func TestSidebarModel_AddToolCall_Limit20(t *testing.T) {
	sb := newSidebarModel(40, 20)
	for i := 0; i < 25; i++ {
		sb.AddToolCall("read", "file.go", 100)
	}
	if len(sb.toolCalls) != 20 {
		t.Errorf("expected max 20 tool calls, got %d", len(sb.toolCalls))
	}
}

func TestSidebarModel_SetGitStatus(t *testing.T) {
	sb := newSidebarModel(40, 20)
	sb.SetGitStatus("M  src/main.go\n?? new.go")
	sb.activeTab = 1 // Git tab
	v := sb.View()
	if !strings.Contains(v, "main.go") {
		t.Error("expected git status in Git tab view")
	}
}

func TestSidebarModel_DebugTab(t *testing.T) {
	sb := newSidebarModel(40, 20)
	sb.SetDebugInfo("act", 5, 50000, 2)
	sb.activeTab = 3 // Debug tab
	v := sb.View()
	if !strings.Contains(v, "act") {
		t.Error("expected phase in Debug tab view")
	}
	if !strings.Contains(v, "50k") {
		t.Error("expected token count in Debug tab view")
	}
	if !strings.Contains(v, "2 active") {
		t.Error("expected subagent count in Debug tab view")
	}
}

func TestSidebarModel_EmptyFilesView(t *testing.T) {
	sb := newSidebarModel(40, 20)
	v := sb.View()
	if !strings.Contains(v, "No files touched") {
		t.Error("expected empty state message")
	}
}

func TestSidebarModel_SetSize(t *testing.T) {
	sb := newSidebarModel(40, 20)
	sb.SetSize(50, 30)
	if sb.width != 50 || sb.height != 30 {
		t.Errorf("expected 50x30, got %dx%d", sb.width, sb.height)
	}
}
