package agent

import (
	"fmt"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

// phaseOrder is the canonical OODARC phase sequence.
var phaseOrder = []tool.Phase{
	tool.PhaseBrainstorm,
	tool.PhasePlan,
	tool.PhaseBuild,
	tool.PhaseReview,
	tool.PhaseShip,
}

// phaseFSM manages phase transitions.
type phaseFSM struct {
	current tool.Phase
	index   int
}

func newPhaseFSM(start tool.Phase) *phaseFSM {
	idx := 0
	for i, p := range phaseOrder {
		if p == start {
			idx = i
			break
		}
	}
	return &phaseFSM{current: start, index: idx}
}

// Current returns the current phase.
func (f *phaseFSM) Current() tool.Phase { return f.current }

// Advance moves to the next phase. Returns error if already at the end.
func (f *phaseFSM) Advance() error {
	if f.index >= len(phaseOrder)-1 {
		return fmt.Errorf("cannot advance past %s", f.current)
	}
	f.index++
	f.current = phaseOrder[f.index]
	return nil
}

// IsTerminal returns true if we're at the last phase (ship).
func (f *phaseFSM) IsTerminal() bool {
	return f.index >= len(phaseOrder)-1
}
