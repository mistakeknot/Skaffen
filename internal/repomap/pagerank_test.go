package repomap

import (
	"math"
	"testing"
)

func TestPageRank_SimpleChain(t *testing.T) {
	// A -> B -> C: C should rank highest (sink node accumulates rank).
	g := NewGraph()
	a, b, c := uint32(0), uint32(1), uint32(2)
	g.Link(a, b, 1.0)
	g.Link(b, c, 1.0)

	ranks := make(map[uint32]float64)
	g.Rank(0.85, 1e-6, nil, func(node uint32, rank float64) {
		ranks[node] = rank
	})

	if ranks[c] <= ranks[a] {
		t.Errorf("C should rank higher than A: C=%f A=%f", ranks[c], ranks[a])
	}
}

func TestPageRank_Personalization(t *testing.T) {
	// Star graph: A->B, A->C, A->D.
	// With personalization on B, B should rank highest.
	g := NewGraph()
	a, b, c, d := uint32(0), uint32(1), uint32(2), uint32(3)
	g.Link(a, b, 1.0)
	g.Link(a, c, 1.0)
	g.Link(a, d, 1.0)

	ranks := make(map[uint32]float64)
	pers := map[uint32]float64{b: 10.0, a: 1.0, c: 1.0, d: 1.0}
	g.Rank(0.85, 1e-6, pers, func(node uint32, rank float64) {
		ranks[node] = rank
	})

	if ranks[b] <= ranks[c] || ranks[b] <= ranks[d] {
		t.Errorf("B should rank highest with personalization: B=%f C=%f D=%f",
			ranks[b], ranks[c], ranks[d])
	}
}

func TestPageRank_SumsToOne(t *testing.T) {
	// Ring of 10 nodes.
	g := NewGraph()
	for i := uint32(0); i < 10; i++ {
		g.Link(i, (i+1)%10, 1.0)
	}

	var total float64
	g.Rank(0.85, 1e-6, nil, func(_ uint32, rank float64) {
		total += rank
	})

	if math.Abs(total-1.0) > 0.01 {
		t.Errorf("ranks should sum to ~1.0, got %f", total)
	}
}

func TestPageRank_EmptyGraph(t *testing.T) {
	g := NewGraph()
	called := false
	g.Rank(0.85, 1e-6, nil, func(_ uint32, _ float64) {
		called = true
	})
	if called {
		t.Error("callback should not be called on empty graph")
	}
}
