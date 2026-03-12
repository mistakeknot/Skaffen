package tui

import (
	"math"
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
	if len(l.grid) != 12 {
		t.Errorf("grid should have 12 rows (6 skaffen + 6 masaq), got %d", len(l.grid))
	}
	if l.totalCells == 0 {
		t.Error("totalCells should be > 0")
	}
}

func TestLogoFrame0MostlyHidden(t *testing.T) {
	l := newLogoModel("v0.1", "v0.1")
	l.width = 80
	l.frame = 0
	v := l.View()
	lines := strings.Split(v, "\n")
	visibleBlocks := 0
	for i := 0; i < 12 && i < len(lines); i++ {
		for _, r := range lines[i] {
			if r == '█' || r == '╗' || r == '╔' || r == '║' || r == '╝' || r == '╚' {
				visibleBlocks++
			}
		}
	}
	if visibleBlocks > l.totalCells/2 {
		t.Errorf("frame 0 should show less than half the cells, got %d/%d", visibleBlocks, l.totalCells)
	}
}

func TestLogoFrame30FullyRevealed(t *testing.T) {
	l := newLogoModel("v0.1", "v0.1")
	l.width = 80
	l.frame = 30
	v := l.View()
	if !strings.Contains(v, "███") {
		t.Error("frame 30 should show full block characters")
	}
}

func TestLogoVersionAppearsLate(t *testing.T) {
	l := newLogoModel("v0.1", "v0.1")
	l.width = 80

	l.frame = 5
	v5 := l.View()
	if strings.Contains(v5, "v0.1") {
		t.Error("version should not appear at frame 5")
	}

	l.frame = 25
	v25 := l.View()
	if !strings.Contains(v25, "v0.1") {
		t.Error("version should appear at frame 25")
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

func TestHexToRGB(t *testing.T) {
	r, g, b := hexToRGB("#ff0000")
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("hexToRGB(#ff0000) = %d,%d,%d", r, g, b)
	}
}

func TestRevealMapCoversAllNonSpace(t *testing.T) {
	l := newLogoModel("dev", "dev")
	assigned := 0
	for r, row := range l.revealMap {
		for c, val := range row {
			if l.grid[r][c] != ' ' && val >= 0 {
				assigned++
			}
		}
	}
	if assigned != l.totalCells {
		t.Errorf("revealMap assigned %d cells, but totalCells is %d", assigned, l.totalCells)
	}
}

func TestParticlesInitialized(t *testing.T) {
	l := newLogoModel("dev", "dev")
	if len(l.particles) != numParticles {
		t.Errorf("expected %d particles, got %d", numParticles, len(l.particles))
	}
	for i, p := range l.particles {
		// Particles start with full-spectrum colors (non-zero RGB)
		if p.r == 0 && p.g == 0 && p.b == 0 {
			t.Errorf("particle %d has zero color", i)
		}
	}
}

func TestParticlesStartWithSpectrumColors(t *testing.T) {
	l := newLogoModel("dev", "dev")
	// Particles should have diverse colors at init — check that not all are the same
	type rgb struct{ r, g, b int }
	seen := make(map[rgb]bool)
	for _, p := range l.particles {
		seen[rgb{int(p.r), int(p.g), int(p.b)}] = true
	}
	if len(seen) < numParticles/2 {
		t.Errorf("expected diverse spectrum colors, only got %d unique colors from %d particles", len(seen), numParticles)
	}
}

func TestStepParticlesMove(t *testing.T) {
	l := newLogoModel("dev", "dev")
	type pos struct{ x, y float64 }
	initial := make([]pos, len(l.particles))
	for i, p := range l.particles {
		initial[i] = pos{p.x, p.y}
	}
	for i := 0; i < 10; i++ {
		l.step()
	}
	moved := 0
	for i, p := range l.particles {
		if p.x != initial[i].x || p.y != initial[i].y {
			moved++
		}
	}
	if moved == 0 {
		t.Error("no particles moved after 10 steps")
	}
}

func TestStepParticlesStayInBounds(t *testing.T) {
	l := newLogoModel("dev", "dev")
	for i := 0; i < 200; i++ {
		l.step()
	}
	for i, p := range l.particles {
		if p.x < 0 || p.x >= float64(l.cols) {
			t.Errorf("particle %d x=%f out of bounds [0,%d)", i, p.x, l.cols)
		}
		if p.y < 0 || p.y >= float64(l.rows) {
			t.Errorf("particle %d y=%f out of bounds [0,%d)", i, p.y, l.rows)
		}
	}
}

func TestColorConvergesTowardBrand(t *testing.T) {
	l := newLogoModel("dev", "dev")

	// Measure initial average distance to nearest brand attractor
	avgDist := func() float64 {
		total := 0.0
		for _, p := range l.particles {
			best := math.MaxFloat64
			for _, a := range brandAttractors {
				dr := p.r - a[0]
				dg := p.g - a[1]
				db := p.b - a[2]
				d := math.Sqrt(dr*dr + dg*dg + db*db)
				if d < best {
					best = d
				}
			}
			total += best
		}
		return total / float64(len(l.particles))
	}

	initial := avgDist()

	// Run many steps
	for i := 0; i < 500; i++ {
		l.step()
	}

	final := avgDist()

	if final >= initial {
		t.Errorf("colors should converge toward brand attractors: initial avg dist = %.1f, final = %.1f", initial, final)
	}
}

func TestSolidMapMatchesGrid(t *testing.T) {
	l := newLogoModel("dev", "dev")
	for r := 0; r < l.rows; r++ {
		for c := 0; c < l.cols; c++ {
			isSolid := l.grid[r][c] != ' '
			if l.solid[r][c] != isSolid {
				t.Errorf("solid[%d][%d] = %v but grid char = %c", r, c, l.solid[r][c], l.grid[r][c])
			}
		}
	}
}

func TestHsvToRGB(t *testing.T) {
	// Red at h=0
	r, g, b := hsvToRGB(0, 1, 1)
	if math.Abs(r-255) > 1 || g > 1 || b > 1 {
		t.Errorf("hsvToRGB(0,1,1) = (%.0f,%.0f,%.0f), want (255,0,0)", r, g, b)
	}
	// Green at h=1/3
	r, g, b = hsvToRGB(1.0/3, 1, 1)
	if r > 1 || math.Abs(g-255) > 1 || b > 1 {
		t.Errorf("hsvToRGB(1/3,1,1) = (%.0f,%.0f,%.0f), want (0,255,0)", r, g, b)
	}
	// Blue at h=2/3
	r, g, b = hsvToRGB(2.0/3, 1, 1)
	if r > 1 || g > 1 || math.Abs(b-255) > 1 {
		t.Errorf("hsvToRGB(2/3,1,1) = (%.0f,%.0f,%.0f), want (0,0,255)", r, g, b)
	}
}
