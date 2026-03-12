package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
)

type statusModel struct {
	width int
}

func newStatusModel() statusModel {
	return statusModel{}
}

// View renders the status bar: phase | model | cost | context% | turns
func (s statusModel) View(phase, model string, cost, contextPct float64, turns int) string {
	c := theme.Current().Semantic()

	bg := lipgloss.NewStyle().
		Background(c.BgLight.Color()).
		Width(s.width)

	phaseStyle := lipgloss.NewStyle().
		Foreground(phaseColor(phase)).
		Bold(true).
		Background(c.BgLight.Color())

	mutedStyle := lipgloss.NewStyle().
		Foreground(c.Muted.Color()).
		Background(c.BgLight.Color())

	sep := mutedStyle.Render(" | ")

	costStyle := lipgloss.NewStyle().Background(c.BgLight.Color())
	switch {
	case cost >= 2.0:
		costStyle = costStyle.Foreground(c.Error.Color())
	case cost >= 0.5:
		costStyle = costStyle.Foreground(c.Warning.Color())
	default:
		costStyle = costStyle.Foreground(c.Success.Color())
	}

	ctxStyle := lipgloss.NewStyle().Background(c.BgLight.Color())
	switch {
	case contextPct >= 80:
		ctxStyle = ctxStyle.Foreground(c.Error.Color())
	case contextPct >= 50:
		ctxStyle = ctxStyle.Foreground(c.Warning.Color())
	default:
		ctxStyle = ctxStyle.Foreground(c.Success.Color())
	}

	turnStyle := lipgloss.NewStyle().
		Foreground(c.Fg.Color()).
		Background(c.BgLight.Color())

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
