package repomap

import (
	"math/rand"
	"testing"
)

func buildGraph(nodes, edges int, seed int64) *Graph {
	rng := rand.New(rand.NewSource(seed))
	g := NewGraph()
	for i := 0; i < edges; i++ {
		src := uint32(rng.Intn(nodes))
		dst := uint32(rng.Intn(nodes))
		w := rng.Float64() + 0.1
		g.Link(src, dst, w)
	}
	return g
}

func benchRank(b *testing.B, nodes, edges int) {
	g := buildGraph(nodes, edges, 42)
	// Small personalization vector: top 10 nodes.
	pers := make(map[uint32]float64)
	for i := uint32(0); i < 10 && i < uint32(nodes); i++ {
		pers[i] = 1.0
	}
	b.ResetTimer()
	for b.Loop() {
		g.Rank(0.85, 1e-6, pers, func(node uint32, rank float64) {})
	}
}

func BenchmarkRank100(b *testing.B)  { benchRank(b, 100, 500) }
func BenchmarkRank500(b *testing.B)  { benchRank(b, 500, 2500) }
func BenchmarkRank1000(b *testing.B) { benchRank(b, 1000, 5000) }
