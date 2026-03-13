package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
	"github.com/mistakeknot/Skaffen/internal/subagent"
)

// subagentBlock renders a single subagent result as a collapsible inline block.
type subagentBlock struct {
	id          string
	description string
	status      subagent.SubagentStatus
	turn        int
	maxTurns    int
	tokensUsed  int
	response    string
	errMsg      string
}

func newSubagentBlock(id, description string) *subagentBlock {
	return &subagentBlock{
		id:          id,
		description: description,
		status:      subagent.StatusPending,
	}
}

func (b *subagentBlock) update(u subagent.StatusUpdate) {
	b.status = u.Status
	b.turn = u.Turn
	b.maxTurns = u.MaxTurns
	b.tokensUsed = u.TokensUsed
	if u.Error != nil {
		b.errMsg = u.Error.Error()
	}
}

func (b *subagentBlock) View(width int, expanded bool) string {
	c := theme.Current().Semantic()

	var icon string
	var statusText string
	var iconColor lipgloss.Color

	switch b.status {
	case subagent.StatusPending:
		icon = "○"
		statusText = "pending"
		iconColor = c.FgDim.Color()
	case subagent.StatusRunning:
		icon = "◐"
		statusText = fmt.Sprintf("turn %d/%d", b.turn, b.maxTurns)
		iconColor = c.Primary.Color()
	case subagent.StatusDone:
		icon = "✓"
		statusText = fmt.Sprintf("done, %s tokens", formatTokens(b.tokensUsed))
		iconColor = c.Success.Color()
	case subagent.StatusFailed:
		icon = "✗"
		statusText = "failed"
		if b.errMsg != "" {
			statusText = fmt.Sprintf("failed: %s", truncate(b.errMsg, 40))
		}
		iconColor = c.Error.Color()
	}

	toggle := "▸"
	if expanded {
		toggle = "▾"
	}

	iconStyle := lipgloss.NewStyle().Foreground(iconColor)
	dimStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())

	header := fmt.Sprintf("%s %s %s %s",
		iconStyle.Render(icon),
		toggle,
		b.description,
		dimStyle.Render("("+statusText+")"),
	)

	if !expanded || b.response == "" {
		return header
	}

	// Expanded: show response below header
	bodyStyle := lipgloss.NewStyle().
		Foreground(c.Fg.Color()).
		PaddingLeft(4).
		Width(width - 4)
	body := bodyStyle.Render(b.response)

	return header + "\n" + body
}

// subagentTracker manages multiple subagent blocks for the TUI.
type subagentTracker struct {
	blocks   map[string]*subagentBlock
	order    []string // insertion order for deterministic rendering
	expanded map[string]bool
}

func newSubagentTracker() *subagentTracker {
	return &subagentTracker{
		blocks:   make(map[string]*subagentBlock),
		expanded: make(map[string]bool),
	}
}

func (t *subagentTracker) update(u subagent.StatusUpdate) {
	b, ok := t.blocks[u.ID]
	if !ok {
		b = newSubagentBlock(u.ID, u.Description)
		t.blocks[u.ID] = b
		t.order = append(t.order, u.ID)
	}
	b.update(u)
}

func (t *subagentTracker) setResponse(id, response string) {
	if b, ok := t.blocks[id]; ok {
		b.response = response
	}
}

func (t *subagentTracker) toggle(id string) {
	t.expanded[id] = !t.expanded[id]
}

func (t *subagentTracker) View(width int) string {
	if len(t.order) == 0 {
		return ""
	}
	var lines []string
	for _, id := range t.order {
		b := t.blocks[id]
		lines = append(lines, b.View(width, t.expanded[id]))
	}
	return strings.Join(lines, "\n")
}

// formatTokens returns a human-readable token count (e.g., "1.2k", "45k").
func formatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 10000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%dk", n/1000)
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
