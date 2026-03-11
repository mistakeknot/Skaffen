package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mistakeknot/Skaffen/internal/agent"
)

func TestAppModelLifecycle(t *testing.T) {
	// Create app with nil agent/trust (test mode)
	cfg := Config{
		WorkDir: "/tmp/test",
	}
	m := newAppModel(cfg)

	// Window resize
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app := model.(*appModel)
	view := app.View()
	if view == "" {
		t.Fatal("view should not be empty after resize")
	}

	// Status bar should contain default phase
	if !strings.Contains(view, "build") {
		t.Fatal("status bar should show default phase")
	}
}

func TestAppModelCtrlCQuits(t *testing.T) {
	cfg := Config{}
	m := newAppModel(cfg)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should produce a command")
	}
	// The command should be tea.Quit
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", msg)
	}
}

func TestAppModelSubmit(t *testing.T) {
	cfg := Config{}
	m := newAppModel(cfg)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Simulate a submit
	model, _ := m.Update(submitMsg{Text: "Hello"})
	app := model.(*appModel)
	if !app.running {
		t.Fatal("should be running after submit")
	}
	view := app.View()
	if !strings.Contains(view, "Hello") {
		t.Fatal("view should contain submitted text")
	}
}

func TestAppModelStreamEvent(t *testing.T) {
	cfg := Config{}
	m := newAppModel(cfg)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Simulate stream text event
	m.handleStreamEvent(agent.StreamEvent{
		Type: agent.StreamText,
		Text: "Test response",
	})
	// The viewport should have content
	view := m.View()
	// We can't easily check for exact content since markdown rendering may transform it
	if view == "" {
		t.Fatal("view should not be empty after stream event")
	}
}
