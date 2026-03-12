package tui

import (
	"strings"
	"testing"
)

func TestNewLogoModel(t *testing.T) {
	l := newLogoModel("v0.1", "v0.1")
	if !l.active {
		t.Error("logo should start active")
	}
	if l.frame != 0 {
		t.Error("logo should start at frame 0")
	}
	if !strings.Contains(l.versions, "v0.1") {
		t.Errorf("versions = %q, want to contain v0.1", l.versions)
	}
}

func TestLogoViewContainsBothLogos(t *testing.T) {
	l := newLogoModel("v0.1", "v0.1")
	l.width = 80
	v := l.View()
	if !strings.Contains(v, "███") {
		t.Error("logo view should contain block characters")
	}
	if !strings.Contains(v, "v0.1") {
		t.Error("logo view should contain version")
	}
	// Both logos should have the same number of lines
	if len(skaffenLines) != len(masaqLines) {
		t.Errorf("skaffenLines has %d lines, masaqLines has %d — should match", len(skaffenLines), len(masaqLines))
	}
}

func TestLogoStop(t *testing.T) {
	l := newLogoModel("dev", "dev")
	l.stop()
	if l.active {
		t.Error("logo should be inactive after stop")
	}
	if l.tick() != nil {
		t.Error("tick should return nil when inactive")
	}
}

func TestLogoFrameAdvances(t *testing.T) {
	l := newLogoModel("dev", "dev")
	l.width = 80
	v1 := l.View()
	l.frame = 5
	v2 := l.View()
	if v1 == "" || v2 == "" {
		t.Error("logo views should not be empty")
	}
}

func TestWaveColor(t *testing.T) {
	c := waveColor(0, 0, "#7aa2f7", "#bb9af7")
	if c == "" || c[0] != '#' || len(c) != 7 {
		t.Errorf("waveColor returned %q, want #RRGGBB format", c)
	}
}

func TestHexToRGB(t *testing.T) {
	r, g, b := hexToRGB("#ff0000")
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("hexToRGB(#ff0000) = %d,%d,%d", r, g, b)
	}
}

func TestLogoSeparatorUsesWidth(t *testing.T) {
	l := newLogoModel("dev", "dev")
	l.width = 40
	v := l.View()
	if !strings.Contains(v, "──") {
		t.Error("logo should have a separator line")
	}
}
