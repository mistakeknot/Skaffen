package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/masaq/theme"
)

// OODARC phases
var phaseOrder = []string{"brainstorm", "plan", "build", "review", "ship"}

// phaseColor returns the lipgloss color for a given phase.
func phaseColor(phase string) lipgloss.Color {
	c := theme.Current().Semantic()
	switch phase {
	case "brainstorm":
		return c.Secondary.Color()
	case "plan":
		return c.Primary.Color()
	case "build":
		return c.Success.Color()
	case "review":
		return c.Warning.Color()
	case "ship":
		return c.Error.Color()
	default:
		return c.FgDim.Color()
	}
}

// PhaseTransition renders a phase change message for the chat stream.
func PhaseTransition(from, to string) string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Border.Color())
	fromStyle := lipgloss.NewStyle().Foreground(phaseColor(from))
	toStyle := lipgloss.NewStyle().Foreground(phaseColor(to)).Bold(true)

	return borderStyle.Render("───") + " " +
		fromStyle.Render(from) + " → " + toStyle.Render(to) +
		" " + borderStyle.Render("───")
}

// NextPhase returns the phase after the current one.
// Returns empty string if at the end.
func NextPhase(current string) string {
	for i, p := range phaseOrder {
		if p == current && i < len(phaseOrder)-1 {
			return phaseOrder[i+1]
		}
	}
	return ""
}

// PhaseLabel returns a styled phase label for the status bar.
func PhaseLabel(phase string) string {
	style := lipgloss.NewStyle().
		Foreground(phaseColor(phase)).
		Bold(true)
	return style.Render(fmt.Sprintf("⬡ %s", phase))
}
