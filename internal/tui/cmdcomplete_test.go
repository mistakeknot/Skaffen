package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewCmdCompleter(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	if !cc.visible {
		t.Fatal("completer should start visible")
	}
	if len(cc.commands) == 0 {
		t.Fatal("completer should have commands")
	}
	if len(cc.filtered) != len(cc.commands) {
		t.Error("filtered should equal all commands initially")
	}
	// Should be sorted
	for i := 1; i < len(cc.commands); i++ {
		if cc.commands[i].name < cc.commands[i-1].name {
			t.Errorf("commands not sorted: %q before %q", cc.commands[i-1].name, cc.commands[i].name)
		}
	}
}

func TestCmdCompleterFilter(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	// Type "he" — should match "help"
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	found := false
	for _, e := range cc.filtered {
		if e.name == "help" {
			found = true
			break
		}
	}
	if !found {
		t.Error("filtering 'he' should include 'help'")
	}
	if len(cc.filtered) >= len(cc.commands) {
		t.Error("filtering should reduce the list")
	}
}

func TestCmdCompleterFilterNoMatch(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if len(cc.filtered) != 0 {
		t.Errorf("filtering 'zzz' should have 0 matches, got %d", len(cc.filtered))
	}
}

func TestCmdCompleterArrowKeys(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	if cc.cursor != 0 {
		t.Fatal("cursor should start at 0")
	}
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cc.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", cc.cursor)
	}
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cc.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", cc.cursor)
	}
	// Up at 0 should stay at 0
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cc.cursor != 0 {
		t.Errorf("cursor should not go negative: %d", cc.cursor)
	}
}

func TestCmdCompleterSelectEnter(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	cc, cmd := cc.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should produce a command")
	}
	msg := cmd()
	sel, ok := msg.(cmdCompleterSelectedMsg)
	if !ok {
		t.Fatalf("expected cmdCompleterSelectedMsg, got %T", msg)
	}
	if sel.Name == "" {
		t.Fatal("selected command name should not be empty")
	}
	if cc.visible {
		t.Fatal("completer should be hidden after selection")
	}
}

func TestCmdCompleterSelectTab(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	cc, cmd := cc.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("tab should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(cmdCompleterSelectedMsg); !ok {
		t.Fatalf("expected cmdCompleterSelectedMsg, got %T", msg)
	}
}

func TestCmdCompleterSelectSpace(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	cc, cmd := cc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if cmd == nil {
		t.Fatal("space should select current command")
	}
	msg := cmd()
	if _, ok := msg.(cmdCompleterSelectedMsg); !ok {
		t.Fatalf("expected cmdCompleterSelectedMsg, got %T", msg)
	}
}

func TestCmdCompleterEscCancels(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	cc, cmd := cc.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(cmdCompleterCancelMsg); !ok {
		t.Fatalf("expected cmdCompleterCancelMsg, got %T", msg)
	}
	if cc.visible {
		t.Fatal("completer should be hidden after cancel")
	}
}

func TestCmdCompleterBackspacePastSlashCancels(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	// pattern is empty, backspace should cancel
	_, cmd := cc.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd == nil {
		t.Fatal("backspace on empty pattern should cancel")
	}
	msg := cmd()
	if _, ok := msg.(cmdCompleterCancelMsg); !ok {
		t.Fatalf("expected cmdCompleterCancelMsg, got %T", msg)
	}
}

func TestCmdCompleterBackspaceNarrowing(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	// Type "he"
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	narrowed := len(cc.filtered)
	// Backspace to "h"
	cc, _ = cc.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if cc.pattern != "h" {
		t.Errorf("pattern after backspace = %q, want 'h'", cc.pattern)
	}
	if len(cc.filtered) < narrowed {
		t.Error("backspace should widen the filter, not narrow it")
	}
}

func TestCmdCompleterView(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	view := cc.View(80)
	if view == "" {
		t.Fatal("view should not be empty")
	}
	// Commands are sorted; first 8 shown. Check for ones near the top.
	if !strings.Contains(view, "/advance") {
		t.Fatal("view should contain /advance")
	}
	if !strings.Contains(view, "/help") {
		t.Fatal("view should contain /help")
	}
}

func TestCmdCompleterViewHidden(t *testing.T) {
	cc := newCmdCompleter(nil, nil)
	cc.visible = false
	if cc.View(80) != "" {
		t.Fatal("hidden completer should return empty view")
	}
}

func TestCmdCompleterPrefixMatchesFirst(t *testing.T) {
	entries := filterCommands([]cmdEntry{
		{"version", "Show version"},
		{"verbose", "Verbose mode"},
		{"advance", "Advance phase"},
	}, "v")
	if len(entries) < 2 {
		t.Fatal("should match at least version and verbose")
	}
	// Prefix matches (version, verbose) should come before substring matches
	if entries[0].name != "verbose" && entries[0].name != "version" {
		t.Errorf("first match should be a prefix match, got %q", entries[0].name)
	}
	if entries[1].name != "verbose" && entries[1].name != "version" {
		t.Errorf("second match should be a prefix match, got %q", entries[1].name)
	}
}

// Integration: typing / in prompt triggers completer
func TestPromptSlashTriggersCompleter(t *testing.T) {
	p := newPromptModel()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !p.completing {
		t.Fatal("typing / at start should activate completer")
	}
	if p.input.Value() != "/" {
		t.Errorf("input should contain '/', got %q", p.input.Value())
	}
}

func TestPromptSlashMidTextDoesNotTrigger(t *testing.T) {
	p := newPromptModel()
	p.input.SetValue("hello")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if p.completing {
		t.Fatal("/ mid-text should not activate completer")
	}
}

func TestPromptSlashWithLinesDoesNotTrigger(t *testing.T) {
	p := newPromptModel()
	p.lines = []string{"first line"}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if p.completing {
		t.Fatal("/ with accumulated lines should not activate completer")
	}
}

func TestPromptCompleterSelection(t *testing.T) {
	p := newPromptModel()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	// Simulate selection
	p, _ = p.Update(cmdCompleterSelectedMsg{Name: "help"})
	if p.completing {
		t.Fatal("completer should close after selection")
	}
	if p.input.Value() != "/help " {
		t.Errorf("input after selection = %q, want '/help '", p.input.Value())
	}
}

func TestPromptCompleterCancel(t *testing.T) {
	p := newPromptModel()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	p, _ = p.Update(cmdCompleterCancelMsg{})
	if p.completing {
		t.Fatal("completer should close after cancel")
	}
	if p.input.Value() != "" {
		t.Errorf("input after cancel = %q, want empty", p.input.Value())
	}
}
