package skill

import (
	"fmt"
	"sort"
)

// Pinner manages session-scoped skill pinning.
// Pinned skills are re-injected as user-role messages on every turn.
type Pinner struct {
	skills map[string]Def
	pinned map[string]bool
}

// NewPinner creates a Pinner backed by the given skills map.
func NewPinner(skills map[string]Def) *Pinner {
	return &Pinner{
		skills: skills,
		pinned: make(map[string]bool),
	}
}

// Pin adds a skill to the pinned set. Returns error if skill doesn't exist.
func (p *Pinner) Pin(name string) error {
	if _, ok := p.skills[name]; !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	p.pinned[name] = true
	return nil
}

// Unpin removes a skill from the pinned set. No-op if not pinned.
func (p *Pinner) Unpin(name string) {
	delete(p.pinned, name)
}

// Pinned returns the list of currently pinned skill names, sorted.
func (p *Pinner) Pinned() []string {
	names := make([]string, 0, len(p.pinned))
	for name := range p.pinned {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsPinned returns whether a skill is currently pinned.
func (p *Pinner) IsPinned(name string) bool {
	return p.pinned[name]
}
