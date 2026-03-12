package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/masaq/theme"
)

type promptModel struct {
	input   textinput.Model
	lines   []string
	picker  filePickerModel
	picking bool
	workDir string
}

func newPromptModel() promptModel {
	ti := textinput.New()
	ti.Placeholder = "Ask anything... (Enter to send, Shift+Enter for newline, @ to mention files)"
	ti.Focus()
	ti.CharLimit = 4096
	return promptModel{input: ti}
}

func (p promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (p promptModel) Update(msg tea.Msg) (promptModel, tea.Cmd) {
	// Handle file picker messages
	switch msg.(type) {
	case filePickerSelectedMsg:
		sel := msg.(filePickerSelectedMsg)
		p.picking = false
		// Insert @path at current cursor position
		v := p.input.Value()
		// Remove trailing @ that triggered the picker
		if strings.HasSuffix(v, "@") {
			v = v[:len(v)-1]
		}
		p.input.SetValue(v + "@" + sel.Path + " ")
		p.input.CursorEnd()
		return p, nil
	case filePickerCancelMsg:
		p.picking = false
		return p, nil
	}

	// Delegate to picker when active
	if p.picking {
		var cmd tea.Cmd
		p.picker, cmd = p.picker.Update(msg)
		return p, cmd
	}

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
		default:
			// Check for @ trigger: if the rune typed is '@', activate picker
			if len(msg.Runes) == 1 && msg.Runes[0] == '@' {
				// Let textinput handle the '@' character first
				var cmd tea.Cmd
				p.input, cmd = p.input.Update(msg)
				// Then open the picker
				root := p.workDir
				if root == "" {
					root = "."
				}
				p.picker = newFilePicker(root)
				p.picking = true
				return p, cmd
			}
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

	result := border.Render(display)

	// Show file picker overlay above prompt if active
	if p.picking {
		pickerView := p.picker.View(width)
		if pickerView != "" {
			result = pickerView + "\n" + result
		}
	}

	return result
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
	p.picking = false
}
