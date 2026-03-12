package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mistakeknot/Masaq/theme"
)

// SKAFFEN in large block art
var skaffenLines = []string{
	`███████╗██╗  ██╗ █████╗ ███████╗███████╗███████╗███╗   ██╗`,
	`██╔════╝██║ ██╔╝██╔══██╗██╔════╝██╔════╝██╔════╝████╗  ██║`,
	`███████╗█████╔╝ ███████║█████╗  █████╗  █████╗  ██╔██╗ ██║`,
	`╚════██║██╔═██╗ ██╔══██║██╔══╝  ██╔══╝  ██╔══╝  ██║╚██╗██║`,
	`███████║██║  ██╗██║  ██║██║     ██║     ███████╗██║ ╚████║`,
	`╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝     ╚══════╝╚═╝  ╚═══╝`,
}

// MASAQ in the same block art style
var masaqLines = []string{
	`███╗   ███╗ █████╗ ███████╗ █████╗  ██████╗ `,
	`████╗ ████║██╔══██╗██╔════╝██╔══██╗██╔═══██╗`,
	`██╔████╔██║███████║███████╗███████║██║   ██║`,
	`██║╚██╔╝██║██╔══██║╚════██║██╔══██║██║▄▄ ██║`,
	`██║ ╚═╝ ██║██║  ██║███████║██║  ██║╚██████╔╝`,
	`╚═╝     ╚═╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝ ╚══▀▀═╝`,
}

type logoTickMsg time.Time

// logoModel manages the animated gradient effect on the logo.
type logoModel struct {
	frame    int
	active   bool // animation running
	width    int
	versions string // "skaffen v0.1 · masaq v0.1"
}

func newLogoModel(skaffenVer, masaqVer string) logoModel {
	ver := fmt.Sprintf("skaffen %s · masaq %s", skaffenVer, masaqVer)
	return logoModel{
		active:   true,
		versions: ver,
	}
}

func (l logoModel) tick() tea.Cmd {
	if !l.active {
		return nil
	}
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return logoTickMsg(t)
	})
}

func (l *logoModel) stop() {
	l.active = false
}

// View renders the logo with an animated color wave.
func (l logoModel) View() string {
	c := theme.Current().Semantic()
	primary := c.Primary
	secondary := c.Secondary

	var sb strings.Builder

	// Render SKAFFEN with animated gradient wave
	for _, line := range skaffenLines {
		renderWaveLine(&sb, line, l.frame, primary.Dark, secondary.Dark)
	}

	// Render MASAQ with same style, offset wave so colors flow between the two
	offset := len([]rune(skaffenLines[0])) // continue wave from where SKAFFEN ended
	for _, line := range masaqLines {
		renderWaveLineOffset(&sb, line, l.frame, primary.Dark, secondary.Dark, offset)
	}

	// Version line
	verStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
	sb.WriteString(verStyle.Render("  "+l.versions) + "\n")

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(c.Border.Color())
	w := l.width
	if w < 1 {
		w = 60
	}
	sb.WriteString(sepStyle.Render(strings.Repeat("─", w)) + "\n")

	return sb.String()
}

func renderWaveLine(sb *strings.Builder, line string, frame int, colorA, colorB string) {
	renderWaveLineOffset(sb, line, frame, colorA, colorB, 0)
}

func renderWaveLineOffset(sb *strings.Builder, line string, frame int, colorA, colorB string, charOffset int) {
	runes := []rune(line)
	for i, r := range runes {
		if r == ' ' {
			sb.WriteRune(' ')
			continue
		}
		col := waveColor(i+charOffset, frame, colorA, colorB)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(col))
		sb.WriteString(style.Render(string(r)))
	}
	sb.WriteRune('\n')
}

// waveColor interpolates between two hex colors based on position and frame,
// creating a traveling gradient wave across the ASCII art.
func waveColor(charIdx, frame int, colorA, colorB string) string {
	wavelength := 20.0
	speed := 0.5
	phase := float64(charIdx)/wavelength - float64(frame)*speed

	// Smooth oscillation between 0 and 1
	t := (math.Sin(phase) + 1.0) / 2.0

	rA, gA, bA := hexToRGB(colorA)
	rB, gB, bB := hexToRGB(colorB)

	r := uint8(float64(rA) + t*(float64(rB)-float64(rA)))
	g := uint8(float64(gA) + t*(float64(gB)-float64(gA)))
	b := uint8(float64(bA) + t*(float64(bB)-float64(bA)))

	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func hexToRGB(hex string) (uint8, uint8, uint8) {
	if len(hex) == 7 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return 128, 128, 128
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}
