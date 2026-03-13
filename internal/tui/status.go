package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/meter"
	"github.com/mistakeknot/Masaq/sparkline"
	"github.com/mistakeknot/Masaq/statusbar"
	"github.com/mistakeknot/Masaq/theme"
)

// contextMaxTokens is the assumed context window size for the meter gauge.
const contextMaxTokens = 200_000

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

// newContextMeter creates a context window meter gauge.
func newContextMeter(width int) meter.Model {
	meterWidth := contextMeterWidth(width)
	m := meter.New(meterWidth)
	m.SetValue(0, contextMaxTokens)
	m.SetForecast(float64(contextMaxTokens) * 0.8) // auto-compact threshold
	m.SetLabel("ctx")
	return m
}

// contextMeterWidth returns the meter width scaled to the terminal width.
// Uses roughly 40% of terminal width, clamped between 20 and 60.
func contextMeterWidth(termWidth int) int {
	w := termWidth * 2 / 5
	if w < 20 {
		w = 20
	}
	if w > 60 {
		w = 60
	}
	return w
}

// newTokenSparkline creates a sparkline for tracking input tokens per turn.
func newTokenSparkline(width int) sparkline.Model {
	sparkWidth := tokenSparklineWidth(width)
	s := sparkline.New(sparkWidth)
	s.WarnThreshold = 0.75
	s.CritThreshold = 0.90
	return s
}

// tokenSparklineWidth returns the sparkline width. Uses 15 columns or
// whatever fits after the meter.
func tokenSparklineWidth(termWidth int) int {
	w := 15
	if w > termWidth/4 {
		w = termWidth / 4
	}
	if w < 5 {
		w = 5
	}
	return w
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
		{Label: "", Value: fmt.Sprintf("%d turns", turns), Color: c.Fg.Color()},
	})
}

// renderMeterRow renders the meter + sparkline as a single status row.
func renderMeterRow(m meter.Model, s sparkline.Model, width int) string {
	meterView := m.View()
	sparkView := s.View()
	if meterView == "" && sparkView == "" {
		return ""
	}

	sep := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Border.Color()).Render(" │ ")
	var parts []string
	if meterView != "" {
		parts = append(parts, meterView)
	}
	if sparkView != "" {
		parts = append(parts, sparkView)
	}
	row := strings.Join(parts, sep)

	// Truncate to terminal width if needed
	_ = width
	return row
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
