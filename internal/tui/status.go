package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/statusbar"
	"github.com/mistakeknot/Masaq/theme"
)

// newStatusBar creates a status bar pre-configured with the standard Skaffen
// slots: phase, model, cost, context%, turns.
func newStatusBar(width int) statusbar.Model {
	sb := statusbar.New(width)
	sb.SetSlots([]statusbar.Slot{
		{Label: "", Value: "build"},  // phase (no label, colored)
		{Label: "", Value: "opus"},   // model
		{Label: "", Value: "$0.00"},  // cost
		{Label: "", Value: "0%"},     // context
		{Label: "", Value: "0 turns"},// turns
	})
	return sb
}

// updateStatusSlots refreshes status bar slots with current agent state.
func updateStatusSlots(sb *statusbar.Model, phase, model string, cost, contextPct float64, turns int) {
	c := theme.Current().Semantic()

	// Phase slot: colored by OODARC phase.
	sb.SetSlots([]statusbar.Slot{
		{Label: "", Value: phase, Color: phaseColor(phase)},
		{Label: "", Value: model, Color: c.FgDim.Color()},
		{Label: "", Value: fmt.Sprintf("$%.2f", cost), Color: costColor(cost)},
		{Label: "", Value: fmt.Sprintf("%.0f%%", contextPct), Color: contextColor(contextPct)},
		{Label: "", Value: fmt.Sprintf("%d turns", turns), Color: c.Fg.Color()},
	})
}

// costColor returns a semantic color based on accumulated cost.
func costColor(cost float64) lipgloss.Color {
	c := theme.Current().Semantic()
	switch {
	case cost >= 2.0:
		return c.Error.Color()
	case cost >= 0.5:
		return c.Warning.Color()
	default:
		return c.Success.Color()
	}
}

// contextColor returns a semantic color based on context window usage.
func contextColor(pct float64) lipgloss.Color {
	c := theme.Current().Semantic()
	switch {
	case pct >= 80:
		return c.Error.Color()
	case pct >= 50:
		return c.Warning.Color()
	default:
		return c.Success.Color()
	}
}
