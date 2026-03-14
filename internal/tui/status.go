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
// Uses brand colors (Primary→Secondary gradient) instead of traffic-light colors.
func newTokenSparkline(width int) sparkline.Model {
	sparkWidth := tokenSparklineWidth(width)
	s := sparkline.New(sparkWidth)
	s.ColorOverride = brandSparklineColor
	return s
}

// brandSparklineColor maps sparkline values to the theme's brand palette:
// low values use Muted, mid values use Primary, high values use Secondary.
func brandSparklineColor(t float64, sem theme.SemanticColors) lipgloss.Color {
	switch {
	case t >= 0.8:
		return sem.Secondary.Color() // purple — stands out for peaks
	case t >= 0.4:
		return sem.Primary.Color() // blue — the dominant brand color
	default:
		return sem.Info.Color() // cyan — subtle for low values
	}
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
// Phase is shown in the breadcrumb, not here — the status bar shows runtime state.
// planMode adds a "PLAN" badge; sandboxLabel adds a trailing badge.
func updateStatusSlots(sb *statusbar.Model, phase, model string, cost, contextPct float64, turns int, planMode bool, sandboxLabel string) {
	c := theme.Current().Semantic()

	var slots []statusbar.Slot

	// PLAN mode badge (modal state, not phase)
	if planMode {
		slots = append(slots, statusbar.Slot{Label: "", Value: "PLAN", Color: c.Info.Color()})
	}

	slots = append(slots,
		statusbar.Slot{Label: "", Value: model, Color: c.FgDim.Color()},
		statusbar.Slot{Label: "", Value: fmt.Sprintf("$%.2f", cost), Color: costColor(cost)},
		statusbar.Slot{Label: "", Value: fmt.Sprintf("%d turns", turns), Color: c.Fg.Color()},
	)

	if sandboxLabel != "" {
		col := c.Success.Color()
		if sandboxLabel == "YOLO" {
			col = c.Warning.Color()
		}
		slots = append(slots, statusbar.Slot{Label: "", Value: sandboxLabel, Color: col})
	}
	sb.SetSlots(slots)
}

// renderMeterRow renders the meter + sparkline as a single status row.
func renderMeterRow(m meter.Model, s sparkline.Model, width int) string {
	meterView := m.View()
	sparkView := s.View()
	if meterView == "" && sparkView == "" {
		return ""
	}

	sep := lipgloss.NewStyle().Foreground(theme.Current().Semantic().Border.Color()).Render("  │  ")
	var parts []string
	if meterView != "" {
		parts = append(parts, meterView)
	}
	if sparkView != "" {
		parts = append(parts, sparkView)
	}
	row := " " + strings.Join(parts, sep)

	// Truncate to terminal width if needed
	_ = width
	return row
}

// costColor returns a brand color based on accumulated cost.
// Uses a muted→primary→secondary gradient instead of traffic-light colors.
func costColor(cost float64) lipgloss.Color {
	c := theme.Current().Semantic()
	switch {
	case cost >= 2.0:
		return c.Secondary.Color() // purple — high spend, stands out
	case cost >= 0.5:
		return c.Primary.Color() // blue — moderate spend
	default:
		return c.FgDim.Color() // muted — low spend, unobtrusive
	}
}

