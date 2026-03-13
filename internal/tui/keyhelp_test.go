package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyHelpInitialState(t *testing.T) {
	kh := newKeyHelpModel()
	if !kh.visible {
		t.Fatal("key help should start visible")
	}
}

func TestKeyHelpDismissOnEsc(t *testing.T) {
	kh := newKeyHelpModel()
	kh, cmd := kh.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if kh.visible {
		t.Fatal("esc should hide key help")
	}
	if cmd == nil {
		t.Fatal("esc should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(keyHelpDismissMsg); !ok {
		t.Fatalf("expected keyHelpDismissMsg, got %T", msg)
	}
}

func TestKeyHelpDismissOnAnyKey(t *testing.T) {
	kh := newKeyHelpModel()
	kh, cmd := kh.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if kh.visible {
		t.Fatal("any key should hide key help")
	}
	if cmd == nil {
		t.Fatal("any key should produce a dismiss command")
	}
}

func TestKeyHelpViewNonEmpty(t *testing.T) {
	kh := newKeyHelpModel()
	view := kh.View(80)
	if view == "" {
		t.Fatal("view should not be empty")
	}
	if !strings.Contains(view, "Enter") {
		t.Fatal("view should mention Enter key")
	}
	if !strings.Contains(view, "Ctrl+R") {
		t.Fatal("view should mention Ctrl+R")
	}
}

func TestKeyHelpViewHidden(t *testing.T) {
	kh := newKeyHelpModel()
	kh.visible = false
	if kh.View(80) != "" {
		t.Fatal("hidden key help should return empty view")
	}
}

func TestKeyHelpViewContainsCategories(t *testing.T) {
	kh := newKeyHelpModel()
	view := kh.View(80)
	if !strings.Contains(view, "Input") {
		t.Fatal("view should have Input category")
	}
	if !strings.Contains(view, "Navigation") {
		t.Fatal("view should have Navigation category")
	}
}

func TestKeyHelpViewWidth(t *testing.T) {
	kh := newKeyHelpModel()
	view := kh.View(40)
	// Should still render without panic
	if view == "" {
		t.Fatal("narrow view should still render")
	}
}

// --- Prompt integration tests ---

func TestPromptQuestionMarkTriggersHelp(t *testing.T) {
	p := newPromptModel()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !p.helping {
		t.Fatal("? on empty prompt should activate key help")
	}
}

func TestPromptQuestionMarkMidTextNoTrigger(t *testing.T) {
	p := newPromptModel()
	p.input.SetValue("what")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if p.helping {
		t.Fatal("? mid-text should not activate key help")
	}
}

func TestPromptQuestionMarkWithLinesNoTrigger(t *testing.T) {
	p := newPromptModel()
	p.lines = []string{"first line"}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if p.helping {
		t.Fatal("? with accumulated lines should not activate key help")
	}
}

func TestPromptKeyHelpDismiss(t *testing.T) {
	p := newPromptModel()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	p, _ = p.Update(keyHelpDismissMsg{})
	if p.helping {
		t.Fatal("key help should close after dismiss")
	}
}

func TestKeyHelpContainsEsc(t *testing.T) {
	kh := newKeyHelpModel()
	view := kh.View(80)
	if !strings.Contains(view, "Esc") {
		t.Fatal("help should mention Esc for stopping agent")
	}
}

func TestKeyHelpContainsCtrlW(t *testing.T) {
	kh := newKeyHelpModel()
	view := kh.View(80)
	if !strings.Contains(view, "Ctrl+W") {
		t.Fatal("help should mention Ctrl+W for word delete")
	}
}

func TestKeyHelpContainsMouseWheel(t *testing.T) {
	kh := newKeyHelpModel()
	view := kh.View(80)
	if !strings.Contains(view, "Mouse") {
		t.Fatal("help should mention mouse wheel scrolling")
	}
}

func TestKeyHelpContainsShellEscape(t *testing.T) {
	kh := newKeyHelpModel()
	view := kh.View(80)
	if !strings.Contains(view, "!") {
		t.Fatal("help should mention ! for shell escape")
	}
}
