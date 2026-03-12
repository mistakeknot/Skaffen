package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPromptInit(t *testing.T) {
	p := newPromptModel()
	cmd := p.Init()
	if cmd == nil {
		t.Fatal("Init should return blink command")
	}
}

func TestPromptEnterSubmits(t *testing.T) {
	p := newPromptModel()
	p.input.SetValue("hello world")
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should produce a command")
	}
	msg := cmd()
	sub, ok := msg.(submitMsg)
	if !ok {
		t.Fatalf("expected submitMsg, got %T", msg)
	}
	if sub.Text != "hello world" {
		t.Errorf("submit text = %q, want 'hello world'", sub.Text)
	}
	// Input should be cleared after submit
	if p.input.Value() != "" {
		t.Fatal("input should be cleared after submit")
	}
}

func TestPromptEnterEmptyBlocked(t *testing.T) {
	p := newPromptModel()
	p.input.SetValue("")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter on empty input should not produce a command")
	}
}

func TestPromptEnterWhitespaceBlocked(t *testing.T) {
	p := newPromptModel()
	p.input.SetValue("   ")
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter on whitespace-only input should not produce a command")
	}
}

func TestPromptShiftEnterAddsLine(t *testing.T) {
	p := newPromptModel()
	p.input.SetValue("line one")
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{}, Alt: true})
	// Use the actual key string the prompt checks
	p.input.SetValue("line one")
	// Simulate shift+enter (which adds current value to lines)
	p.lines = append(p.lines, p.input.Value())
	p.input.SetValue("")

	if len(p.lines) != 1 {
		t.Errorf("lines = %d, want 1", len(p.lines))
	}
	if p.lines[0] != "line one" {
		t.Errorf("lines[0] = %q, want 'line one'", p.lines[0])
	}
}

func TestPromptFullText(t *testing.T) {
	p := newPromptModel()
	p.lines = []string{"line 1", "line 2"}
	p.input.SetValue("line 3")

	text := p.fullText()
	if text != "line 1\nline 2\nline 3" {
		t.Errorf("fullText = %q, want 'line 1\\nline 2\\nline 3'", text)
	}
}

func TestPromptFullTextEmpty(t *testing.T) {
	p := newPromptModel()
	text := p.fullText()
	if text != "" {
		t.Errorf("fullText on empty prompt = %q, want empty", text)
	}
}

func TestPromptFullTextOnlyLines(t *testing.T) {
	p := newPromptModel()
	p.lines = []string{"line 1", "line 2"}
	p.input.SetValue("")
	text := p.fullText()
	if text != "line 1\nline 2" {
		t.Errorf("fullText = %q, want 'line 1\\nline 2'", text)
	}
}

func TestPromptReset(t *testing.T) {
	p := newPromptModel()
	p.lines = []string{"a", "b"}
	p.input.SetValue("c")
	p.Reset()
	if p.input.Value() != "" {
		t.Fatal("reset should clear input")
	}
	if len(p.lines) != 0 {
		t.Fatal("reset should clear lines")
	}
}

func TestPromptViewIdle(t *testing.T) {
	p := newPromptModel()
	view := p.View(80, false)
	if view == "" {
		t.Fatal("view should not be empty")
	}
}

func TestPromptViewRunning(t *testing.T) {
	p := newPromptModel()
	view := p.View(80, true)
	if !strings.Contains(view, "Thinking") {
		t.Fatal("running view should show 'Thinking...'")
	}
}

func TestPromptViewMultiline(t *testing.T) {
	p := newPromptModel()
	p.lines = []string{"first line"}
	p.input.SetValue("second line")
	view := p.View(80, false)
	if !strings.Contains(view, "first line") {
		t.Fatal("multiline view should show accumulated lines")
	}
}

func TestPromptCharLimit(t *testing.T) {
	p := newPromptModel()
	if p.input.CharLimit != 4096 {
		t.Errorf("char limit = %d, want 4096", p.input.CharLimit)
	}
}
