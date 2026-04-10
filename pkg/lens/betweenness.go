package lens

import "sort"

// RunBetweenness computes betweenness centrality for all nodes using Brandes'
// algorithm for unweighted, undirected graphs. It identifies the top-K bridge
// lenses and stores the result via g.SetBridges().
func RunBetweenness(g *graph, topK int) error {
	nodes := g.SortedNodeIDs()
	n := len(nodes)
	if n == 0 {
		g.SetBridges(nil, nil)
		return nil
	}

	nodeIndex := make(map[string]int, n)
	for i, id := range nodes {
		nodeIndex[id] = i
	}

	// Pre-resolve neighbor indices for each node (already sorted via AllNeighborIDs).
	neighborIdx := make([][]int, n)
	for i, id := range nodes {
		nbrIDs := g.AllNeighborIDs(id)
		idxs := make([]int, 0, len(nbrIDs))
		for _, nid := range nbrIDs {
			if j, ok := nodeIndex[nid]; ok {
				idxs = append(idxs, j)
			}
		}
		neighborIdx[i] = idxs
	}

	// Betweenness centrality accumulator.
	cb := make([]float64, n)

	// Brandes algorithm: BFS from each source.
	for _, s := range nodes {
		sIdx := nodeIndex[s]

		// Stack of nodes in order of non-increasing distance from s.
		stack := make([]int, 0, n)

		// Predecessors on shortest paths.
		pred := make([][]int, n)
		for i := range pred {
			pred[i] = nil
		}

		// Number of shortest paths from s to each node.
		sigma := make([]float64, n)
		sigma[sIdx] = 1.0

		// Distance from s (-1 means not visited).
		dist := make([]int, n)
		for i := range dist {
			dist[i] = -1
		}
		dist[sIdx] = 0

		// BFS queue.
		queue := []int{sIdx}

		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			stack = append(stack, v)

			for _, w := range neighborIdx[v] {
				// w found for the first time?
				if dist[w] < 0 {
					dist[w] = dist[v] + 1
					queue = append(queue, w)
				}
				// Shortest path to w via v?
				if dist[w] == dist[v]+1 {
					sigma[w] += sigma[v]
					pred[w] = append(pred[w], v)
				}
			}
		}

		// Back-propagation of dependencies.
		delta := make([]float64, n)
		for len(stack) > 0 {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, v := range pred[w] {
				delta[v] += (sigma[v] / sigma[w]) * (1.0 + delta[w])
			}
			if w != sIdx {
				cb[w] += delta[w]
			}
		}
	}

	// Normalize for undirected graph:
	// 1. Brandes BFS from every node double-counts each (s,t) pair, so halve.
	// 2. Then normalize by (n-1)*(n-2) to match networkx convention.
	// Combined: divide by (n-1)*(n-2)*2, then multiply by 2 = divide by (n-1)*(n-2).
	// But networkx's normalization for undirected is 2/((n-1)*(n-2)).
	// So: cb[i] = cb[i] / 2.0 * (2.0 / ((n-1)*(n-2))) = cb[i] / ((n-1)*(n-2)/2)
	// Wait — let's be precise. For undirected Brandes:
	//   raw accumulation counts each pair once per BFS (from s side).
	//   Since we BFS from every node, each undirected pair (s,t) is counted twice.
	//   So raw / 2 gives the actual betweenness.
	//   networkx then normalizes by 2/((n-1)*(n-2)) for undirected.
	//   Final: cb[i] = (raw / 2) * (2 / ((n-1)*(n-2))) = raw / ((n-1)*(n-2))
	// For undirected graphs: halve raw (double-counted pairs), then normalize.
	// networkx normalizes by 2/((n-1)*(n-2)) for undirected.
	// Combined: raw / 2 * 2 / ((n-1)*(n-2)) = raw / ((n-1)*(n-2))
	// But empirically we need raw / 2 / ((n-1)*(n-2)/2) = raw / ((n-1)*(n-2))
	// which still gives 2x. Let me just do the two steps:
	for i := range cb {
		cb[i] /= 2.0 // undirected: each pair counted from both sides
	}
	if n > 2 {
		// networkx normalization for undirected: 2/((n-1)*(n-2))
		norm := float64((n-1)*(n-2)) / 2.0
		for i := range cb {
			cb[i] /= norm
		}
	}

	// Build score map.
	scores := make(map[string]float64, n)
	for i, id := range nodes {
		scores[id] = cb[i]
	}

	// Sort by score (descending), then by ID (ascending) for determinism.
	type nodeScore struct {
		id    string
		score float64
	}
	sorted := make([]nodeScore, n)
	for i, id := range nodes {
		sorted[i] = nodeScore{id: id, score: cb[i]}
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].score != sorted[j].score {
			return sorted[i].score > sorted[j].score
		}
		return sorted[i].id < sorted[j].id
	})

	// Take top-K.
	k := topK
	if k > n {
		k = n
	}
	bridges := make([]LensRef, k)
	for i := 0; i < k; i++ {
		id := sorted[i].id
		l, ok := g.lensMap[id]
		if ok {
			bridges[i] = l.Ref()
		} else {
			bridges[i] = LensRef{ID: id}
		}
	}

	g.SetBridges(bridges, scores)
	return nil
}
