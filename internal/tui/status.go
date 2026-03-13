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
// When planMode is true, prepends "PLAN " to the phase with an accent color.
func updateStatusSlots(sb *statusbar.Model, phase, model string, cost, contextPct float64, turns int, planMode bool) {
	c := theme.Current().Semantic()

	phaseVal := phase
	phaseCol := phaseColor(phase)
	if planMode {
		phaseVal = "PLAN " + phase
		phaseCol = c.Info.Color()
	}

	sb.SetSlots([]statusbar.Slot{
		{Label: "", Value: phaseVal, Color: phaseCol},
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
