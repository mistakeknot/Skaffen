package tui

import (
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/skill"
	"github.com/mistakeknot/Masaq/theme"
)

type promptModel struct {
	input        textinput.Model
	lines        []string
	picker       filePickerModel
	picking      bool
	completer    cmdCompleterModel
	completing   bool
	history      historyModel
	searching    bool
	historyStore *historyStore
	keyHelp      keyHelpModel
	helping      bool
	workDir      string
	customCmds   map[string]command.Def
	skills       map[string]skill.Def
}

func newPromptModel() promptModel {
	ti := textinput.New()
	ti.Placeholder = "Ask anything... (Enter to send, Shift+Enter for newline, ? for help)"
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
	case historySelectedMsg:
		sel := msg.(historySelectedMsg)
		p.searching = false
		p.input.SetValue(sel.Text)
		p.input.CursorEnd()
		return p, nil
	case historyCancelMsg:
		p.searching = false
		return p, nil
	case keyHelpDismissMsg:
		p.helping = false
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

	// Delegate to history search when active
	if p.searching {
		var cmd tea.Cmd
		p.history, cmd = p.history.Update(msg)
		return p, cmd
	}

	// Delegate to key help when active
	if p.helping {
		var cmd tea.Cmd
		p.keyHelp, cmd = p.keyHelp.Update(msg)
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
		case "ctrl+w":
			// Delete previous word (standard Unix terminal behavior)
			v := p.input.Value()
			pos := p.input.Position()
			if pos > 0 {
				before := v[:pos]
				after := v[pos:]
				// Skip trailing spaces, then skip non-spaces
				i := len(before) - 1
				for i >= 0 && before[i] == ' ' {
					i--
				}
				for i >= 0 && before[i] != ' ' {
					i--
				}
				p.input.SetValue(before[:i+1] + after)
				p.input.SetCursor(i + 1)
			}
			return p, nil
		case "ctrl+g":
			return p, openEditor(p.fullText())
		case "ctrl+r":
			if p.historyStore != nil {
				p.history = newHistoryModel(p.historyStore, p.input.Value())
				p.searching = true
				return p, nil
			}
		default:
			// Check for ? trigger on empty prompt (no accumulated lines)
			if len(msg.Runes) == 1 && msg.Runes[0] == '?' &&
				len(p.lines) == 0 && p.input.Value() == "" {
				p.keyHelp = newKeyHelpModel()
				p.helping = true
				return p, nil
			}
			// Check for / trigger at start of empty input (no accumulated lines)
			if len(msg.Runes) == 1 && msg.Runes[0] == '/' &&
				len(p.lines) == 0 && p.input.Value() == "" {
				// Let textinput handle the '/' character first
				var cmd tea.Cmd
				p.input, cmd = p.input.Update(msg)
				p.completer = newCmdCompleter(p.customCmds, p.skills)
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

func (p promptModel) View(width int, running bool, spinnerView string) string {
	c := theme.Current().Semantic()
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.Border.Color()).
		Width(width - 2).
		Padding(0, 1)

	if running {
		return border.Render(spinnerView)
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

	// Show history search below prompt input
	if p.searching {
		historyView := p.history.View(width)
		if historyView != "" {
			result = result + "\n" + historyView
		}
	}

	// Show key help overlay below prompt
	if p.helping {
		helpView := p.keyHelp.View(width)
		if helpView != "" {
			result = result + "\n" + helpView
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
	p.searching = false
	p.helping = false
}

// editorResultMsg carries the result of an external editor session.
type editorResultMsg struct {
	Text string
	Err  error
}

// openEditor launches $VISUAL/$EDITOR on a temp file containing the current
// prompt text. Bubble Tea suspends the alt screen while the editor runs.
func openEditor(currentText string) tea.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	f, err := os.CreateTemp("", "skaffen-*.md")
	if err != nil {
		return func() tea.Msg { return editorResultMsg{Err: err} }
	}
	if currentText != "" {
		f.WriteString(currentText)
	}
	f.Close()
	path := f.Name()

	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		defer os.Remove(path)
		if err != nil {
			return editorResultMsg{Err: err}
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return editorResultMsg{Err: readErr}
		}
		return editorResultMsg{Text: strings.TrimRight(string(content), "\n")}
	})
}
