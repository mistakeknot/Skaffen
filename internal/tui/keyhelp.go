package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
)

// keyHelpDismissMsg is sent when the help overlay is dismissed.
type keyHelpDismissMsg struct{}

type keyBinding struct {
	Key  string
	Desc string
}

type keyCategory struct {
	Name     string
	Bindings []keyBinding
}

// keyHelpModel displays an overlay of keyboard shortcuts.
type keyHelpModel struct {
	visible    bool
	categories []keyCategory
}

func newKeyHelpModel() keyHelpModel {
	return keyHelpModel{
		visible: true,
		categories: []keyCategory{
			{
				Name: "Input",
				Bindings: []keyBinding{
					{"Enter", "Send message"},
					{"Shift+Enter", "New line"},
					{"Ctrl+W", "Delete previous word"},
					{"Ctrl+G", "Open editor"},
					{"Ctrl+R", "Search history"},
					{"/", "Slash commands (with autocomplete)"},
					{"@", "File picker"},
					{"!", "Shell escape (!command)"},
					{"?", "This help"},
				},
			},
			{
				Name: "Navigation",
				Bindings: []keyBinding{
					{"PgUp/PgDn", "Scroll viewport"},
					{"Home/End", "Jump to top/bottom"},
					{"Ctrl+U/D", "Half-page scroll (vim)"},
					{"Mouse wheel", "Scroll 3 lines"},
				},
			},
			{
				Name: "Panels",
				Bindings: []keyBinding{
					{"Ctrl+B", "Toggle sidebar"},
				},
			},
			{
				Name: "Session",
				Bindings: []keyBinding{
					{"Esc", "Stop current agent run"},
					{"Shift+Tab", "Toggle plan mode"},
					{"Ctrl+C", "Quit"},
				},
			},
		},
	}
}

func (k keyHelpModel) Update(msg tea.Msg) (keyHelpModel, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		k.visible = false
		return k, func() tea.Msg { return keyHelpDismissMsg{} }
	}
	return k, nil
}

func (k keyHelpModel) View(width int) string {
	if !k.visible {
		return ""
	}

	c := theme.Current().Semantic()
	titleStyle := lipgloss.NewStyle().Foreground(c.Primary.Color()).Bold(true)
	catStyle := lipgloss.NewStyle().Foreground(c.Secondary.Color()).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(c.Info.Color())
	descStyle := lipgloss.NewStyle().Foreground(c.Fg.Color())
	dimStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())

	var b strings.Builder
	b.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n")

	for i, cat := range k.categories {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(catStyle.Render(cat.Name))
		b.WriteString("\n")
		for _, bind := range cat.Bindings {
			// Pad key to 14 chars for alignment
			key := bind.Key
			pad := 14 - len(key)
			if pad < 1 {
				pad = 1
			}
			b.WriteString(fmt.Sprintf("  %s%s%s\n",
				keyStyle.Render(key),
				strings.Repeat(" ", pad),
				descStyle.Render(bind.Desc),
			))
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press any key to dismiss"))

	return b.String()
}
