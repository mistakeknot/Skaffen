package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/masaq/theme"
)

type promptModel struct {
	input textinput.Model
	lines []string
}

func newPromptModel() promptModel {
	ti := textinput.New()
	ti.Placeholder = "Ask anything... (Enter to send, Shift+Enter for newline)"
	ti.Focus()
	ti.CharLimit = 4096
	return promptModel{input: ti}
}

func (p promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (p promptModel) Update(msg tea.Msg) (promptModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			text := p.fullText()
			if strings.TrimSpace(text) == "" {
				return p, nil
			}
			p.input.SetValue("")
			p.lines = nil
			return p, func() tea.Msg { return submitMsg{Text: text} }
		case "shift+enter", "alt+enter":
			// Add newline
			p.lines = append(p.lines, p.input.Value())
			p.input.SetValue("")
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p promptModel) View(width int, running bool) string {
	c := theme.Current().Semantic()
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.Border.Color()).
		Width(width - 2).
		Padding(0, 1)

	if running {
		spinStyle := lipgloss.NewStyle().Foreground(c.Primary.Color())
		return border.Render(spinStyle.Render("Thinking..."))
	}

	// Show accumulated lines + current input
	var display string
	if len(p.lines) > 0 {
		display = strings.Join(p.lines, "\n") + "\n"
	}
	display += p.input.View()

	return border.Render(display)
}

func (p promptModel) fullText() string {
	parts := make([]string, 0, len(p.lines)+1)
	parts = append(parts, p.lines...)
	if v := p.input.Value(); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, "\n")
}

// Reset clears the input.
func (p *promptModel) Reset() {
	p.input.SetValue("")
	p.lines = nil
}
