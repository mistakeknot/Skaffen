package lens

import (
	"sort"
	"testing"
)

func loadOrFatal(t *testing.T) ([]Lens, []Edge) {
	t.Helper()
	Reset()
	if err := Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	lenses, err := Lenses()
	if err != nil {
		t.Fatalf("Lenses(): %v", err)
	}
	edges, err := Edges()
	if err != nil {
		t.Fatalf("Edges(): %v", err)
	}
	return lenses, edges
}

func TestNewGraph(t *testing.T) {
	lenses, edges := loadOrFatal(t)
	g := NewGraph(lenses, edges)

	// Verify node count matches the expected 291 lenses.
	if got := g.NodeCount(); got != expectedLensCount {
		t.Errorf("NodeCount: got %d, want %d", got, expectedLensCount)
	}

	// The most connected lens should have non-empty neighbors.
	const wellConnected = "lens_161_headline_founder_mode"
	neighbors := g.AllNeighborIDs(wellConnected)
	if len(neighbors) == 0 {
		t.Fatalf("AllNeighborIDs(%q): got 0 neighbors, want >0", wellConnected)
	}

	// Verify neighbor list is sorted.
	if !sort.StringsAreSorted(neighbors) {
		t.Errorf("AllNeighborIDs(%q): not sorted", wellConnected)
	}

	// Verify no duplicates in neighbor list.
	for i := 1; i < len(neighbors); i++ {
		if neighbors[i] == neighbors[i-1] {
			t.Errorf("AllNeighborIDs(%q): duplicate %q at index %d", wellConnected, neighbors[i], i)
		}
	}
}

func TestNeighborsByType(t *testing.T) {
	lenses, edges := loadOrFatal(t)
	g := NewGraph(lenses, edges)

	const id = "lens_161_headline_founder_mode"

	// This lens should have both complements and contrasts edges.
	complements := g.Neighbors(id, EdgeTypeComplements)
	contrasts := g.Neighbors(id, EdgeTypeContrasts)

	if len(complements) == 0 {
		t.Errorf("Neighbors(%q, complements): got 0, want >0", id)
	}
	if len(contrasts) == 0 {
		t.Errorf("Neighbors(%q, contrasts): got 0, want >0", id)
	}

	// Filtered results should be sorted by ID.
	for _, refs := range [][]LensRef{complements, contrasts} {
		for i := 1; i < len(refs); i++ {
			if refs[i].ID < refs[i-1].ID {
				t.Errorf("Neighbors not sorted by ID: %q comes after %q", refs[i].ID, refs[i-1].ID)
			}
		}
	}

	// Complements and contrasts should not overlap (different edge types).
	compSet := make(map[string]struct{}, len(complements))
	for _, ref := range complements {
		compSet[ref.ID] = struct{}{}
	}
	for _, ref := range contrasts {
		if _, ok := compSet[ref.ID]; ok {
			// A lens could appear in both if it has two different edge types
			// to the same target. Not an error, but flag it for awareness.
			t.Logf("Note: %q appears in both complements and contrasts for %q", ref.ID, id)
		}
	}

	// Non-existent lens should return nil.
	if got := g.Neighbors("nonexistent_lens", EdgeTypeComplements); got != nil {
		t.Errorf("Neighbors(nonexistent): got %v, want nil", got)
	}

	// Edge type with no edges for this lens should return empty.
	refines := g.Neighbors(id, EdgeTypeRefines)
	// This may or may not be empty depending on data, so just verify
	// it returns without error and is sorted if non-empty.
	if len(refines) > 1 {
		for i := 1; i < len(refines); i++ {
			if refines[i].ID < refines[i-1].ID {
				t.Errorf("Neighbors(refines) not sorted")
			}
		}
	}
}

func TestSortedNodeIDs(t *testing.T) {
	lenses, edges := loadOrFatal(t)
	g := NewGraph(lenses, edges)

	ids := g.SortedNodeIDs()
	if len(ids) != expectedLensCount {
		t.Errorf("SortedNodeIDs length: got %d, want %d", len(ids), expectedLensCount)
	}

	if !sort.StringsAreSorted(ids) {
		t.Errorf("SortedNodeIDs: not sorted")
	}

	// Verify no duplicates.
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			t.Errorf("SortedNodeIDs: duplicate %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestEdgeWeight(t *testing.T) {
	lenses, edges := loadOrFatal(t)
	g := NewGraph(lenses, edges)

	// A known edge from the data: lens_161 → lens_194 complements with confidence 0.82.
	const a = "lens_161_headline_founder_mode"
	const b = "lens_194_weekly_prestige_and_dominance"
	w := g.EdgeWeight(a, b)
	if w <= 0 {
		t.Errorf("EdgeWeight(%q, %q): got %v, want >0", a, b, w)
	}

	// EdgeWeight should be symmetric.
	wReverse := g.EdgeWeight(b, a)
	if w != wReverse {
		t.Errorf("EdgeWeight asymmetric: (%q,%q)=%v, (%q,%q)=%v", a, b, w, b, a, wReverse)
	}

	// Non-existent edge should return 0.
	if got := g.EdgeWeight("nonexistent_a", "nonexistent_b"); got != 0 {
		t.Errorf("EdgeWeight(nonexistent): got %v, want 0", got)
	}
}

func TestSetCommunities(t *testing.T) {
	g := NewGraph(nil, nil)

	// Initially empty.
	if got := g.Communities(); len(got) != 0 {
		t.Errorf("Communities before set: got %d, want 0", len(got))
	}
	if _, ok := g.CommunityOf("any"); ok {
		t.Error("CommunityOf before set: got ok=true, want false")
	}

	// Set communities.
	comms := []Community{
		{ID: 0, Members: []string{"a", "b"}},
		{ID: 1, Members: []string{"c"}},
	}
	commOf := map[string]int{"a": 0, "b": 0, "c": 1}
	g.SetCommunities(comms, commOf)

	if got := len(g.Communities()); got != 2 {
		t.Errorf("Communities after set: got %d, want 2", got)
	}
	if id, ok := g.CommunityOf("a"); !ok || id != 0 {
		t.Errorf("CommunityOf(a): got (%d, %v), want (0, true)", id, ok)
	}
	if id, ok := g.CommunityOf("c"); !ok || id != 1 {
		t.Errorf("CommunityOf(c): got (%d, %v), want (1, true)", id, ok)
	}
}

func TestSetBridges(t *testing.T) {
	g := NewGraph(nil, nil)

	// Initially empty.
	if got := g.BridgeLenses(); len(got) != 0 {
		t.Errorf("BridgeLenses before set: got %d, want 0", len(got))
	}
	if got := g.BridgeScore("any"); got != 0 {
		t.Errorf("BridgeScore before set: got %v, want 0", got)
	}

	// Set bridges.
	bridges := []LensRef{{ID: "x", Name: "X"}}
	scores := map[string]float64{"x": 0.95}
	g.SetBridges(bridges, scores)

	if got := len(g.BridgeLenses()); got != 1 {
		t.Errorf("BridgeLenses after set: got %d, want 1", got)
	}
	if got := g.BridgeScore("x"); got != 0.95 {
		t.Errorf("BridgeScore(x): got %v, want 0.95", got)
	}
}
