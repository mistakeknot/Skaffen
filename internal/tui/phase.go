package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// OODARC phases
var phaseOrder = []string{"brainstorm", "plan", "build", "review", "ship"}

// phaseColor returns the lipgloss color for a given phase.
func phaseColor(phase string) lipgloss.Color {
	switch phase {
	case "brainstorm":
		return lipgloss.Color("#bb9af7") // purple
	case "plan":
		return lipgloss.Color("#7aa2f7") // blue
	case "build":
		return lipgloss.Color("#9ece6a") // green
	case "review":
		return lipgloss.Color("#e0af68") // yellow
	case "ship":
		return lipgloss.Color("#f7768e") // red/pink
	default:
		return lipgloss.Color("#a9b1d6") // muted
	}
}

// PhaseTransition renders a phase change message for the chat stream.
func PhaseTransition(from, to string) string {
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
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
