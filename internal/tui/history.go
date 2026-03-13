package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
)

// historySelectedMsg is sent when the user selects a history entry.
type historySelectedMsg struct {
	Text string
}

// historyCancelMsg is sent when the user cancels history search.
type historyCancelMsg struct{}

const historyMaxVisible = 8

// historyModel provides an incremental reverse-search overlay for prompt history.
type historyModel struct {
	store   *historyStore
	query   string
	matches []string
	cursor  int
	visible bool
}

func newHistoryModel(store *historyStore, seedQuery string) historyModel {
	hm := historyModel{
		store:   store,
		query:   seedQuery,
		visible: true,
	}
	hm.matches = store.Search(seedQuery)
	return hm
}

func (h historyModel) Update(msg tea.Msg) (historyModel, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}

	switch km.Type {
	case tea.KeyEnter:
		h.visible = false
		if len(h.matches) == 0 {
			return h, func() tea.Msg { return historyCancelMsg{} }
		}
		text := h.matches[h.cursor]
		return h, func() tea.Msg { return historySelectedMsg{Text: text} }

	case tea.KeyEsc:
		h.visible = false
		return h, func() tea.Msg { return historyCancelMsg{} }

	case tea.KeyUp, tea.KeyCtrlP:
		if h.cursor > 0 {
			h.cursor--
		}
		return h, nil

	case tea.KeyDown, tea.KeyCtrlN:
		if h.cursor < len(h.matches)-1 {
			h.cursor++
		}
		return h, nil

	case tea.KeyBackspace:
		if len(h.query) > 0 {
			h.query = h.query[:len(h.query)-1]
			h.matches = h.store.Search(h.query)
			h.cursor = 0
		}
		return h, nil

	case tea.KeyRunes:
		h.query += string(km.Runes)
		h.matches = h.store.Search(h.query)
		h.cursor = 0
		return h, nil
	}

	return h, nil
}

func (h historyModel) View(width int) string {
	if !h.visible {
		return ""
	}
	c := theme.Current().Semantic()
	headerStyle := lipgloss.NewStyle().Foreground(c.Primary.Color()).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
	selectedStyle := lipgloss.NewStyle().Foreground(c.Secondary.Color()).Bold(true)

	var b strings.Builder
	if h.query == "" {
		b.WriteString(headerStyle.Render("reverse-i-search: "))
	} else {
		b.WriteString(headerStyle.Render(fmt.Sprintf("reverse-i-search: %s", h.query)))
	}
	b.WriteString("\n")

	if len(h.matches) == 0 {
		b.WriteString(dimStyle.Render("  (no matches)"))
		return b.String()
	}

	// Show up to historyMaxVisible entries
	visible := h.matches
	if len(visible) > historyMaxVisible {
		visible = visible[:historyMaxVisible]
	}
	for i, entry := range visible {
		// Truncate long entries
		display := entry
		maxLen := width - 4
		if maxLen < 20 {
			maxLen = 20
		}
		if len(display) > maxLen {
			display = display[:maxLen-3] + "..."
		}
		if i == h.cursor {
			b.WriteString(selectedStyle.Render("▸ " + display))
		} else {
			b.WriteString(dimStyle.Render("  " + display))
		}
		if i < len(visible)-1 {
			b.WriteString("\n")
		}
	}

	if len(h.matches) > historyMaxVisible {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ... %d more", len(h.matches)-historyMaxVisible)))
	}

	return b.String()
}
