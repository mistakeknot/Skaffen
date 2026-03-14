package tui

import (
	"fmt"
	"math"
	"math/rand"
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

const (
	numParticles = 48
	revealFrames = 30
)

// Brand attractor colors — particles converge toward these through mixing.
// Millennial pink, complementary blue, complementary purple.
var brandAttractors = [3][3]float64{
	{244, 163, 176}, // millennial pink  #f4a3b0
	{122, 162, 247}, // blue             #7aa2f7
	{187, 154, 247}, // purple           #bb9af7
}

// brandAttractorHex for post-collapse branding.
var brandAttractorHex = [3]string{"#f4a3b0", "#7aa2f7", "#bb9af7"}

type logoTickMsg time.Time

// particle is a point that bounces within the logo bounding box, carrying color.
type particle struct {
	x, y   float64 // position in grid coordinates (col, row)
	vx, vy float64 // velocity
	r, g, b float64 // current color (0-255 range)
}

// logoModel manages the spiral reveal + particle swarm coloring.
type logoModel struct {
	frame      int
	active     bool
	collapsed  bool // true after first user interaction — hides logo entirely
	width      int
	versions   string
	grid       [][]rune
	revealMap  [][]int
	totalCells int
	rows, cols int
	particles  []particle
	solid      [][]bool // true = non-space cell (for rendering, not collision)
}

// collapse hides the logo permanently to reclaim viewport space.
func (l *logoModel) collapse() {
	l.active = false
	l.collapsed = true
}

func newLogoModel(skaffenVer, masaqVer string) logoModel {
	ver := fmt.Sprintf("skaffen %s · masaq %s", skaffenVer, masaqVer)

	allLines := append(append([]string{}, skaffenLines...), masaqLines...)
	maxWidth := 0
	for _, line := range allLines {
		w := len([]rune(line))
		if w > maxWidth {
			maxWidth = w
		}
	}
	nRows := len(allLines)
	grid := make([][]rune, nRows)
	solid := make([][]bool, nRows)
	for r, line := range allLines {
		runes := []rune(line)
		for len(runes) < maxWidth {
			runes = append(runes, ' ')
		}
		grid[r] = runes
		solid[r] = make([]bool, maxWidth)
		for c, ch := range runes {
			solid[r][c] = ch != ' '
		}
	}

	revealMap, totalCells := buildSpiralRevealMap(grid, nRows, maxWidth)

	// Spawn particles across the bounding box with full 256-color spectrum
	rng := rand.New(rand.NewSource(42))
	particles := make([]particle, numParticles)
	for i := range particles {
		px := rng.Float64() * float64(maxWidth)
		py := rng.Float64() * float64(nRows)
		// Random velocity, biased horizontal since chars are wider
		vx := (rng.Float64() - 0.5) * 2.5
		vy := (rng.Float64() - 0.5) * 1.2
		// Start color: evenly distributed around the HSV wheel
		hue := float64(i) / float64(numParticles)
		cr, cg, cb := hsvToRGB(hue, 0.85, 1.0)
		particles[i] = particle{
			x: px, y: py,
			vx: vx, vy: vy,
			r: cr, g: cg, b: cb,
		}
	}

	return logoModel{
		active:     true,
		versions:   ver,
		grid:       grid,
		revealMap:  revealMap,
		totalCells: totalCells,
		rows:       nRows,
		cols:       maxWidth,
		particles:  particles,
		solid:      solid,
	}
}

func buildSpiralRevealMap(grid [][]rune, rows, cols int) ([][]int, int) {
	revealMap := make([][]int, rows)
	for r := range revealMap {
		revealMap[r] = make([]int, cols)
		for c := range revealMap[r] {
			revealMap[r][c] = -1
		}
	}

	type cell struct{ row, col int }
	var nonSpace []cell
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if grid[r][c] != ' ' {
				nonSpace = append(nonSpace, cell{r, c})
			}
		}
	}

	type corner struct {
		row, col, phase int
	}
	corners := []corner{
		{0, 0, 0},
		{rows - 1, cols - 1, 1},
		{rows - 1, 0, 2},
		{0, cols - 1, 3},
	}

	for _, c := range nonSpace {
		bestTime := math.MaxInt32
		for _, cn := range corners {
			dr := c.row - cn.row
			if dr < 0 {
				dr = -dr
			}
			dc := c.col - cn.col
			if dc < 0 {
				dc = -dc
			}
			dist := dr*3 + dc
			t := dist*4 + cn.phase
			if t < bestTime {
				bestTime = t
			}
		}
		revealMap[c.row][c.col] = bestTime
	}

	maxTime := 0
	for _, c := range nonSpace {
		if t := revealMap[c.row][c.col]; t > maxTime {
			maxTime = t
		}
	}
	if maxTime > 0 {
		for _, c := range nonSpace {
			t := revealMap[c.row][c.col]
			revealMap[c.row][c.col] = t * revealFrames / maxTime
		}
	}

	return revealMap, len(nonSpace)
}

func (l logoModel) tick() tea.Cmd {
	if !l.active {
		return nil
	}
	return tea.Tick(60*time.Millisecond, func(t time.Time) tea.Msg {
		return logoTickMsg(t)
	})
}

func (l *logoModel) stop() {
	l.active = false
}

// spatialKey returns a grid cell key for spatial hashing.
func spatialKey(x, y float64) uint64 {
	cx := uint32(x / 2.5)
	cy := uint32(y / 2.5)
	return uint64(cx)<<32 | uint64(cy)
}

// step advances all particles, bouncing off the bounding box edges and
// mixing colors on collision. Uses spatial hashing for O(n) proximity checks.
func (l *logoModel) step() {
	// Build spatial hash: bucket particles by grid cell
	type cellKey = uint64
	grid := make(map[cellKey][]int, len(l.particles))
	for i := range l.particles {
		k := spatialKey(l.particles[i].x, l.particles[i].y)
		grid[k] = append(grid[k], i)
	}

	for i := range l.particles {
		p := &l.particles[i]

		// Color mixing on proximity — check only neighboring cells
		cx := int(p.x / 2.5)
		cy := int(p.y / 2.5)
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nk := uint64(uint32(cx+dx))<<32 | uint64(uint32(cy+dy))
				for _, j := range grid[nk] {
					if j <= i {
						continue // avoid double-processing pairs
					}
					q := &l.particles[j]
					ddx := p.x - q.x
					ddy := (p.y - q.y) * 2.0 // scale Y for char aspect ratio
					dist2 := ddx*ddx + ddy*ddy
					if dist2 < 6.0 && dist2 > 0.01 {
						dist := math.Sqrt(dist2)
						mix := 0.08 * (1.0 - dist/math.Sqrt(6.0))
						p.r += (q.r - p.r) * mix
						p.g += (q.g - p.g) * mix
						p.b += (q.b - p.b) * mix
						q.r += (p.r - q.r) * mix
						q.g += (p.g - q.g) * mix
						q.b += (p.b - q.b) * mix

						force := 0.1 / (dist + 0.1)
						nx := ddx / dist * force
						ny := (p.y - q.y) / dist * force
						p.vx += nx
						p.vy += ny
						q.vx -= nx
						q.vy -= ny
					}
				}
			}
		}

		// Drift toward nearest brand attractor color
		bestDist := math.MaxFloat64
		bestIdx := 0
		for ai, a := range brandAttractors {
			dr := p.r - a[0]
			dg := p.g - a[1]
			db := p.b - a[2]
			d := dr*dr + dg*dg + db*db
			if d < bestDist {
				bestDist = d
				bestIdx = ai
			}
		}
		attr := brandAttractors[bestIdx]
		// Gentle pull — gets stronger as we get closer (positive feedback)
		pull := 0.008
		if bestDist < 3000 { // within ~55 units of a brand color
			pull = 0.015
		}
		p.r += (attr[0] - p.r) * pull
		p.g += (attr[1] - p.g) * pull
		p.b += (attr[2] - p.b) * pull

		// Dampen velocity
		p.vx *= 0.96
		p.vy *= 0.96

		// Minimum speed so particles keep moving
		speed := math.Sqrt(p.vx*p.vx + p.vy*p.vy)
		if speed < 0.3 {
			p.vx *= 1.8
			p.vy *= 1.8
		}

		// Move
		nx := p.x + p.vx
		ny := p.y + p.vy

		// Bounce off bounding box edges (the outer rectangle of the words)
		if nx < 0 {
			nx = -nx
			p.vx = -p.vx
		}
		if nx >= float64(l.cols) {
			nx = float64(l.cols)*2 - nx - 1
			p.vx = -p.vx
		}
		if ny < 0 {
			ny = -ny
			p.vy = -p.vy
		}
		if ny >= float64(l.rows) {
			ny = float64(l.rows)*2 - ny - 1
			p.vy = -p.vy
		}

		p.x = nx
		p.y = ny
	}
}

// View renders the logo with spiral reveal + particle-driven coloring.
// After collapse, shows a one-line version string in brand colors.
func (l logoModel) View() string {
	if l.collapsed {
		c := theme.Current().Semantic()
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(brandAttractorHex[0])).Bold(true)
		verStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
		sepStyle := lipgloss.NewStyle().Foreground(c.Border.Color())
		w := l.width
		if w < 1 {
			w = 60
		}
		return nameStyle.Render("skaffen") + verStyle.Render(" · "+l.versions) + "\n" +
			sepStyle.Render(strings.Repeat("─", w)) + "\n"
	}
	c := theme.Current().Semantic()

	// Build per-cell color influence from particles
	type colorAccum struct {
		r, g, b float64
		weight  float64
	}
	cellColors := make([][]colorAccum, l.rows)
	for r := range cellColors {
		cellColors[r] = make([]colorAccum, l.cols)
	}

	for _, p := range l.particles {
		radius := 10.0

		minR := int(math.Max(0, p.y-radius))
		maxR := int(math.Min(float64(l.rows-1), p.y+radius))
		minC := int(math.Max(0, p.x-radius))
		maxC := int(math.Min(float64(l.cols-1), p.x+radius))

		for r := minR; r <= maxR; r++ {
			for ci := minC; ci <= maxC; ci++ {
				dx := float64(ci) - p.x
				dy := (float64(r) - p.y) * 2.0
				dist2 := dx*dx + dy*dy
				if dist2 < radius*radius {
					w := 1.0 / (1.0 + dist2*0.2)
					cellColors[r][ci].r += p.r * w
					cellColors[r][ci].g += p.g * w
					cellColors[r][ci].b += p.b * w
					cellColors[r][ci].weight += w
				}
			}
		}
	}

	// Base color (dim) for cells with no particle influence
	rM, gM, bM := hexToRGB(c.Muted.Dark)

	var sb strings.Builder

	for r, row := range l.grid {
		for ci, ch := range row {
			if ch == ' ' {
				sb.WriteRune(' ')
				continue
			}
			revealAt := l.revealMap[r][ci]
			if revealAt < 0 || l.frame < revealAt {
				sb.WriteRune(' ')
				continue
			}

			acc := cellColors[r][ci]
			var fr, fg, fb float64
			if acc.weight > 0.01 {
				pr := acc.r / acc.weight
				pg := acc.g / acc.weight
				pb := acc.b / acc.weight
				blend := math.Min(1.0, acc.weight*0.4)
				fr = float64(rM)*(1-blend) + pr*blend
				fg = float64(gM)*(1-blend) + pg*blend
				fb = float64(bM)*(1-blend) + pb*blend
			} else {
				fr = float64(rM)
				fg = float64(gM)
				fb = float64(bM)
			}

			hex := fmt.Sprintf("#%02x%02x%02x",
				uint8(math.Min(255, math.Max(0, fr))),
				uint8(math.Min(255, math.Max(0, fg))),
				uint8(math.Min(255, math.Max(0, fb))))
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
			sb.WriteString(style.Render(string(ch)))
		}
		sb.WriteRune('\n')
	}

	// Version line
	verStyle := lipgloss.NewStyle().Foreground(c.FgDim.Color())
	if l.frame >= 25 {
		sb.WriteString(verStyle.Render("  "+l.versions) + "\n")
	} else {
		sb.WriteRune('\n')
	}

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(c.Border.Color())
	w := l.width
	if w < 1 {
		w = 60
	}
	sb.WriteString(sepStyle.Render(strings.Repeat("─", w)) + "\n")

	return sb.String()
}

// hsvToRGB converts HSV (h in [0,1], s in [0,1], v in [0,1]) to RGB (0-255).
func hsvToRGB(h, s, v float64) (float64, float64, float64) {
	h = h - math.Floor(h) // wrap to [0,1)
	c := v * s
	x := c * (1.0 - math.Abs(math.Mod(h*6.0, 2.0)-1.0))
	m := v - c

	var r, g, b float64
	switch {
	case h < 1.0/6.0:
		r, g, b = c, x, 0
	case h < 2.0/6.0:
		r, g, b = x, c, 0
	case h < 3.0/6.0:
		r, g, b = 0, c, x
	case h < 4.0/6.0:
		r, g, b = 0, x, c
	case h < 5.0/6.0:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return (r + m) * 255, (g + m) * 255, (b + m) * 255
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
