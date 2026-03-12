package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPhaseColorAllPhases(t *testing.T) {
	phases := []string{"brainstorm", "plan", "build", "review", "ship"}
	for _, p := range phases {
		c := phaseColor(p)
		if c == lipgloss.Color("") {
			t.Errorf("phaseColor(%q) returned empty color", p)
		}
	}
}

func TestPhaseColorUnknown(t *testing.T) {
	c := phaseColor("unknown")
	if c == lipgloss.Color("") {
		t.Fatal("unknown phase should still return a color (FgDim)")
	}
}

func TestPhaseTransition(t *testing.T) {
	result := PhaseTransition("build", "review")
	if !strings.Contains(result, "build") {
		t.Fatal("transition should contain 'from' phase")
	}
	if !strings.Contains(result, "review") {
		t.Fatal("transition should contain 'to' phase")
	}
	if !strings.Contains(result, "→") {
		t.Fatal("transition should contain arrow")
	}
}

func TestNextPhase(t *testing.T) {
	tests := []struct {
		current string
		want    string
	}{
		{"brainstorm", "plan"},
		{"plan", "build"},
		{"build", "review"},
		{"review", "ship"},
		{"ship", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := NextPhase(tt.current)
		if got != tt.want {
			t.Errorf("NextPhase(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestPhaseLabel(t *testing.T) {
	label := PhaseLabel("build")
	if !strings.Contains(label, "build") {
		t.Fatal("PhaseLabel should contain phase name")
	}
}

func TestPhaseOrderLength(t *testing.T) {
	if len(phaseOrder) != 5 {
		t.Errorf("phaseOrder has %d entries, want 5", len(phaseOrder))
	}
}
