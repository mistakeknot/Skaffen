package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

type statusModel struct {
	width int
}

func newStatusModel() statusModel {
	return statusModel{}
}

// View renders the status bar: phase | model | cost | context% | turns
func (s statusModel) View(phase, model string, cost, contextPct float64, turns int) string {
	bg := lipgloss.NewStyle().
		Background(lipgloss.Color("#24283b")).
		Width(s.width)

	phaseStyle := lipgloss.NewStyle().
		Foreground(phaseColor(phase)).
		Bold(true).
		Background(lipgloss.Color("#24283b"))

	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		Background(lipgloss.Color("#24283b"))

	sep := mutedStyle.Render(" | ")

	costStyle := lipgloss.NewStyle().Background(lipgloss.Color("#24283b"))
	switch {
	case cost >= 2.0:
		costStyle = costStyle.Foreground(lipgloss.Color("#f7768e"))
	case cost >= 0.5:
		costStyle = costStyle.Foreground(lipgloss.Color("#e0af68"))
	default:
		costStyle = costStyle.Foreground(lipgloss.Color("#9ece6a"))
	}

	ctxStyle := lipgloss.NewStyle().Background(lipgloss.Color("#24283b"))
	switch {
	case contextPct >= 80:
		ctxStyle = ctxStyle.Foreground(lipgloss.Color("#f7768e"))
	case contextPct >= 50:
		ctxStyle = ctxStyle.Foreground(lipgloss.Color("#e0af68"))
	default:
		ctxStyle = ctxStyle.Foreground(lipgloss.Color("#9ece6a"))
	}

	turnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Background(lipgloss.Color("#24283b"))

	bar := phaseStyle.Render(phase) +
		sep +
		mutedStyle.Render(model) +
		sep +
		costStyle.Render(fmt.Sprintf("$%.2f", cost)) +
		sep +
		ctxStyle.Render(fmt.Sprintf("%.0f%%", contextPct)) +
		sep +
		turnStyle.Render(fmt.Sprintf("%d turns", turns))

	return bg.Render(bar)
}
