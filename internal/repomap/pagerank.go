package repomap

// Graph holds a sparse weighted directed graph for personalized PageRank.
type Graph struct {
	edges map[uint32]map[uint32]float64
	nodes map[uint32]struct{}
}

// NewGraph returns an empty directed graph.
func NewGraph() *Graph {
	return &Graph{
		edges: make(map[uint32]map[uint32]float64),
		nodes: make(map[uint32]struct{}),
	}
}

// Link adds a weighted edge from src to dst. Weights accumulate if
// the same edge is added multiple times.
func (g *Graph) Link(src, dst uint32, weight float64) {
	g.nodes[src] = struct{}{}
	g.nodes[dst] = struct{}{}
	if g.edges[src] == nil {
		g.edges[src] = make(map[uint32]float64)
	}
	g.edges[src][dst] += weight
}

// NodeCount returns the number of distinct nodes in the graph.
func (g *Graph) NodeCount() int { return len(g.nodes) }

// Rank computes personalized PageRank via power iteration.
//
// alpha is the damping factor (typically 0.85). tol is the L1 convergence
// threshold. personalize maps node IDs to teleport weights (normalized
// internally); if nil, uniform teleportation is used. Dangling node mass
// is redistributed via the personalization vector.
//
// Iteration is capped at 100 rounds. callback is invoked once per node
// with the final rank.
func (g *Graph) Rank(alpha, tol float64, personalize map[uint32]float64,
	callback func(node uint32, rank float64)) {

	n := len(g.nodes)
	if n == 0 {
		return
	}

	// Build a stable node list and index.
	nodeList := make([]uint32, 0, n)
	for id := range g.nodes {
		nodeList = append(nodeList, id)
	}
	idx := make(map[uint32]int, n)
	for i, id := range nodeList {
		idx[id] = i
	}

	// Normalize personalization vector.
	teleport := make([]float64, n)
	if personalize != nil {
		var sum float64
		for _, id := range nodeList {
			teleport[idx[id]] = personalize[id]
			sum += personalize[id]
		}
		if sum > 0 {
			for i := range teleport {
				teleport[i] /= sum
			}
		} else {
			for i := range teleport {
				teleport[i] = 1.0 / float64(n)
			}
		}
	} else {
		for i := range teleport {
			teleport[i] = 1.0 / float64(n)
		}
	}

	// Precompute total outbound weight per node.
	outWeight := make([]float64, n)
	for src, dsts := range g.edges {
		for _, w := range dsts {
			outWeight[idx[src]] += w
		}
	}

	// Initialize uniform ranks.
	rank := make([]float64, n)
	for i := range rank {
		rank[i] = 1.0 / float64(n)
	}
	newRank := make([]float64, n)

	for iter := 0; iter < 100; iter++ {
		// Sum rank mass on dangling nodes (no outbound edges).
		var danglingSum float64
		for i := range nodeList {
			if outWeight[i] == 0 {
				danglingSum += rank[i]
			}
		}

		// Teleport + dangling redistribution via personalization vector.
		for i := range newRank {
			newRank[i] = (1-alpha)*teleport[i] + alpha*danglingSum*teleport[i]
		}

		// Propagate rank along edges.
		for src, dsts := range g.edges {
			si := idx[src]
			if outWeight[si] == 0 {
				continue
			}
			for dst, w := range dsts {
				di := idx[dst]
				newRank[di] += alpha * rank[si] * w / outWeight[si]
			}
		}

		// L1 convergence check.
		var diff float64
		for i := range rank {
			d := newRank[i] - rank[i]
			if d < 0 {
				d = -d
			}
			diff += d
		}

		rank, newRank = newRank, rank
		if diff < tol {
			break
		}
	}

	for i, id := range nodeList {
		callback(id, rank[i])
	}
}
