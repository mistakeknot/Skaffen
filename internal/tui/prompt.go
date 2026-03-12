package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
)

type promptModel struct {
	input      textinput.Model
	lines      []string
	picker     filePickerModel
	picking    bool
	completer  cmdCompleterModel
	completing bool
	workDir    string
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
	case cmdCompleterSelectedMsg:
		sel := msg.(cmdCompleterSelectedMsg)
		p.completing = false
		// Replace input with the full slash command
		p.input.SetValue("/" + sel.Name + " ")
		p.input.CursorEnd()
		return p, nil
	case cmdCompleterCancelMsg:
		p.completing = false
		// Clear the "/" that triggered the completer
		p.input.SetValue("")
		return p, nil
	}

	// Delegate to picker when active
	if p.picking {
		var cmd tea.Cmd
		p.picker, cmd = p.picker.Update(msg)
		return p, cmd
	}

	// Delegate to completer when active
	if p.completing {
		var cmd tea.Cmd
		p.completer, cmd = p.completer.Update(msg)
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
			// Check for / trigger at start of empty input (no accumulated lines)
			if len(msg.Runes) == 1 && msg.Runes[0] == '/' &&
				len(p.lines) == 0 && p.input.Value() == "" {
				// Let textinput handle the '/' character first
				var cmd tea.Cmd
				p.input, cmd = p.input.Update(msg)
				p.completer = newCmdCompleter()
				p.completing = true
				return p, cmd
			}
			// Check for @ trigger
			if len(msg.Runes) == 1 && msg.Runes[0] == '@' {
				var cmd tea.Cmd
				p.input, cmd = p.input.Update(msg)
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

	// Block mouse events — textinput renders them as raw escape text.
	if _, ok := msg.(tea.MouseMsg); ok {
		return p, nil
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

	// Show file picker below prompt input
	if p.picking {
		pickerView := p.picker.View(width)
		if pickerView != "" {
			result = result + "\n" + pickerView
		}
	}

	// Show command completer below prompt input
	if p.completing {
		completerView := p.completer.View(width)
		if completerView != "" {
			result = result + "\n" + completerView
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
	p.completing = false
}
