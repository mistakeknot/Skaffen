package repomap

import "sort"

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

	// Build a sorted stable node list and index.
	nodeList := make([]uint32, 0, n)
	for id := range g.nodes {
		nodeList = append(nodeList, id)
	}
	sort.Slice(nodeList, func(i, j int) bool { return nodeList[i] < nodeList[j] })
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

	// Flatten edges into CSR (Compressed Sparse Row) format for cache-friendly
	// iteration without map lookups in the hot loop. Weights are pre-divided
	// by the source's total outbound weight.
	//
	// Count total edges.
	totalEdges := 0
	for _, dsts := range g.edges {
		totalEdges += len(dsts)
	}
	csrRowPtr := make([]int, n+1)
	csrCol := make([]int, totalEdges)
	csrWeight := make([]float64, totalEdges)

	// First pass: count edges per source node.
	for src, dsts := range g.edges {
		csrRowPtr[idx[src]+1] = len(dsts)
	}
	// Prefix sum to get row pointers.
	for i := 1; i <= n; i++ {
		csrRowPtr[i] += csrRowPtr[i-1]
	}
	// Second pass: fill column indices and pre-divided weights.
	offset := make([]int, n) // tracks fill position per row
	for src, dsts := range g.edges {
		si := idx[src]
		ow := outWeight[si]
		for dst, w := range dsts {
			pos := csrRowPtr[si] + offset[si]
			csrCol[pos] = idx[dst]
			csrWeight[pos] = w / ow
			offset[si]++
		}
	}

	// Pre-compute list of dangling node indices (no outbound edges).
	var danglingNodes []int
	for i := range nodeList {
		if outWeight[i] == 0 {
			danglingNodes = append(danglingNodes, i)
		}
	}

	// Initialize uniform ranks.
	rank := make([]float64, n)
	for i := range rank {
		rank[i] = 1.0 / float64(n)
	}
	newRank := make([]float64, n)

	for iter := 0; iter < 100; iter++ {
		// Sum rank mass on dangling nodes.
		var danglingSum float64
		for _, i := range danglingNodes {
			danglingSum += rank[i]
		}

		// Teleport + dangling redistribution via personalization vector.
		alphaDangling := alpha * danglingSum
		oneMinusAlpha := 1 - alpha
		for i, t := range teleport {
			newRank[i] = oneMinusAlpha*t + alphaDangling*t
		}

		// Propagate rank along edges using CSR format.
		for si := 0; si < n; si++ {
			start := csrRowPtr[si]
			end := csrRowPtr[si+1]
			if start == end {
				continue
			}
			contribution := alpha * rank[si]
			for e := start; e < end; e++ {
				newRank[csrCol[e]] += contribution * csrWeight[e]
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
