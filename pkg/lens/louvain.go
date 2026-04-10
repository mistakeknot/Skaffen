package lens

import (
	"math/rand"
	"sort"
)

// RunLouvain runs Louvain community detection on the graph and stores the
// result via g.SetCommunities(). The algorithm uses sorted iteration order
// at every step and a seeded PRNG for tie-breaking to ensure determinism.
func RunLouvain(g *graph) error {
	nodes := g.SortedNodeIDs()
	if len(nodes) == 0 {
		g.SetCommunities(nil, nil)
		return nil
	}

	rng := rand.New(rand.NewSource(42))

	n := len(nodes)
	nodeIndex := make(map[string]int, n)
	for i, id := range nodes {
		nodeIndex[id] = i
	}

	type neighbor struct {
		idx    int
		weight float64
	}

	// Build weighted adjacency list (undirected, no self-loops at level 0).
	adjList := make([][]neighbor, n)
	for i, id := range nodes {
		nbrIDs := g.AllNeighborIDs(id)
		nbrs := make([]neighbor, 0, len(nbrIDs))
		for _, nid := range nbrIDs {
			j, ok := nodeIndex[nid]
			if !ok {
				continue
			}
			w := g.EdgeWeight(id, nid)
			if w > 0 {
				nbrs = append(nbrs, neighbor{idx: j, weight: w})
			}
		}
		adjList[i] = nbrs
	}

	// computeStats returns (degree, m) from adjacency.
	// degree[i] = sum of all adjacency weights (self-loops counted once in list).
	// m = Σ_i degree[i] / 2.
	computeStats := func(adj [][]neighbor, nNodes int) ([]float64, float64) {
		deg := make([]float64, nNodes)
		for i := 0; i < nNodes; i++ {
			for _, nb := range adj[i] {
				deg[i] += nb.weight
			}
		}
		var sumDeg float64
		for i := 0; i < nNodes; i++ {
			sumDeg += deg[i]
		}
		return deg, sumDeg / 2.0
	}

	degree, m := computeStats(adjList, n)
	if m == 0 {
		comms := make([]Community, n)
		commOf := make(map[string]int, n)
		for i, id := range nodes {
			comms[i] = Community{ID: i, Members: []string{id}}
			commOf[id] = i
		}
		g.SetCommunities(comms, commOf)
		return nil
	}

	// Each node starts in its own community.
	community := make([]int, n)
	for i := range community {
		community[i] = i
	}

	// Phase 1: Local moves.
	//
	// Standard Louvain modularity gain for moving node i to community C:
	//   ΔQ = k_{i,C} / m  -  σ_tot(C) * k_i / (2m²)
	// where
	//   k_{i,C} = sum of weights from i to nodes in C (excluding i itself)
	//   σ_tot(C) = sum of degrees of nodes in C (excluding i, after i is removed)
	//   k_i = degree of node i
	//   m = total edge weight (each edge counted once, self-loops counted once)
	//
	// We compare gain(bestC) vs gain(stayInOldC) and move if better.
	localMoves := func(
		nNodes int,
		comm []int,
		deg []float64,
		adj [][]neighbor,
		totalM float64,
	) bool {
		// σ_tot[c] = sum of degrees of nodes in community c.
		sigmaTot := make(map[int]float64)
		for i := 0; i < nNodes; i++ {
			sigmaTot[comm[i]] += deg[i]
		}

		// σ_in[c] = sum of internal edge weights in community c
		// (self-loops + edges between members of c, each counted once).
		// We don't actually need this for the gain formula — only k_{i,C}.

		improved := false
		changed := true
		for changed {
			changed = false
			for i := 0; i < nNodes; i++ {
				ci := comm[i]
				ki := deg[i]

				// Weight from node i to each neighboring community.
				neighComm := make(map[int]float64)
				for _, nb := range adj[i] {
					if nb.idx == i {
						continue // skip self-loops for k_{i,C}
					}
					neighComm[comm[nb.idx]] += nb.weight
				}

				// Remove i from its community.
				sigmaTot[ci] -= ki

				// Evaluate candidates.
				bestGain := 0.0
				bestComm := ci
				bestTie := 0

				// The gain of staying = gain of moving to ci (which we evaluate too).
				// We also evaluate ci in the loop, so no need for separate stayGain.

				candidates := make([]int, 0, len(neighComm)+1)
				seen := make(map[int]bool, len(neighComm)+1)
				candidates = append(candidates, ci)
				seen[ci] = true
				for c := range neighComm {
					if !seen[c] {
						candidates = append(candidates, c)
						seen[c] = true
					}
				}
				sort.Ints(candidates)

				for _, c := range candidates {
					kiC := neighComm[c]
					gain := kiC/totalM - (sigmaTot[c]*ki)/(2.0*totalM*totalM)

					if gain > bestGain {
						bestGain = gain
						bestComm = c
						bestTie = 1
					} else if gain == bestGain && gain > 0 {
						bestTie++
						if rng.Intn(bestTie) == 0 {
							bestComm = c
						}
					}
				}

				// bestComm is the community with highest gain (possibly ci).
				// If bestComm == ci, no move. Otherwise, move.
				sigmaTot[bestComm] += ki
				if bestComm != ci {
					comm[i] = bestComm
					changed = true
					improved = true
				}
			}
		}
		return improved
	}

	// Multi-level Louvain.
	currentAdj := adjList
	currentDeg := degree
	currentComm := community
	currentN := n
	currentM := m

	// originalToSuper[i] = current super-node for original node i.
	originalToSuper := make([]int, n)
	for i := range originalToSuper {
		originalToSuper[i] = i
	}

	for {
		moved := localMoves(currentN, currentComm, currentDeg, currentAdj, currentM)
		if !moved {
			break
		}

		// Phase 2: Aggregate — collapse communities into super-nodes.
		commSet := make(map[int]int)
		for i := 0; i < currentN; i++ {
			c := currentComm[i]
			if _, ok := commSet[c]; !ok {
				commSet[c] = len(commSet)
			}
		}

		newN := len(commSet)
		if newN == currentN {
			break
		}

		// Remap community IDs to contiguous range.
		for i := 0; i < currentN; i++ {
			currentComm[i] = commSet[currentComm[i]]
		}

		// Update originalToSuper.
		for i := 0; i < n; i++ {
			originalToSuper[i] = currentComm[originalToSuper[i]]
		}

		// Build super-node adjacency with self-loops for internal edges.
		type edgePair struct{ a, b int }
		interEdges := make(map[edgePair]float64)
		selfLoopWeight := make([]float64, newN)

		for i := 0; i < currentN; i++ {
			ci := currentComm[i]
			for _, nb := range currentAdj[i] {
				cj := currentComm[nb.idx]
				if ci == cj {
					// Internal edge → self-loop on super-node.
					// In the adjacency, each edge i→j and j→i are both listed.
					// We accumulate all and then halve for self-loops later.
					// But self-loops in the original (nb.idx == i) are already
					// counted once. Non-self internal edges are counted twice.
					if nb.idx == i {
						selfLoopWeight[ci] += nb.weight
					} else if nb.idx > i {
						selfLoopWeight[ci] += nb.weight
					}
				} else if ci < cj {
					interEdges[edgePair{ci, cj}] += nb.weight
				}
				// ci > cj: will be caught from the other direction
			}
		}

		newAdj := make([][]neighbor, newN)
		for i := 0; i < newN; i++ {
			if selfLoopWeight[i] > 0 {
				newAdj[i] = append(newAdj[i], neighbor{idx: i, weight: selfLoopWeight[i]})
			}
		}
		for ep, w := range interEdges {
			newAdj[ep.a] = append(newAdj[ep.a], neighbor{idx: ep.b, weight: w})
			newAdj[ep.b] = append(newAdj[ep.b], neighbor{idx: ep.a, weight: w})
		}
		for i := 0; i < newN; i++ {
			sort.Slice(newAdj[i], func(a, b int) bool {
				return newAdj[i][a].idx < newAdj[i][b].idx
			})
		}

		newComm := make([]int, newN)
		for i := range newComm {
			newComm[i] = i
		}

		// Recompute degree and m from super-node adjacency.
		newDeg, newM := computeStats(newAdj, newN)

		currentAdj = newAdj
		currentDeg = newDeg
		currentComm = newComm
		currentN = newN
		currentM = newM
	}

	// Map final communities back to original nodes.
	finalComm := make([]int, n)
	for i := 0; i < n; i++ {
		finalComm[i] = currentComm[originalToSuper[i]]
	}

	// Renumber contiguously (in order of first appearance over sorted nodes).
	commRenumber := make(map[int]int)
	for i := 0; i < n; i++ {
		c := finalComm[i]
		if _, ok := commRenumber[c]; !ok {
			commRenumber[c] = len(commRenumber)
		}
	}
	for i := 0; i < n; i++ {
		finalComm[i] = commRenumber[finalComm[i]]
	}

	// Build output.
	commMembers := make(map[int][]string)
	communityOf := make(map[string]int, n)
	for i, id := range nodes {
		c := finalComm[i]
		commMembers[c] = append(commMembers[c], id)
		communityOf[id] = c
	}

	commIDs := make([]int, 0, len(commMembers))
	for c := range commMembers {
		commIDs = append(commIDs, c)
	}
	sort.Ints(commIDs)

	communities := make([]Community, len(commIDs))
	for i, c := range commIDs {
		members := commMembers[c]
		sort.Strings(members)
		communities[i] = Community{ID: c, Members: members}
	}

	g.SetCommunities(communities, communityOf)
	return nil
}

// modularity computes the modularity Q of the current partition.
// Used for testing and debugging.
func modularity(g *graph, communityOf map[string]int) float64 {
	nodes := g.SortedNodeIDs()
	var m2 float64
	for _, id := range nodes {
		for _, nid := range g.AllNeighborIDs(id) {
			m2 += g.EdgeWeight(id, nid)
		}
	}
	if m2 == 0 {
		return 0
	}

	var q float64
	for _, id := range nodes {
		ki := 0.0
		for _, nid := range g.AllNeighborIDs(id) {
			ki += g.EdgeWeight(id, nid)
		}
		for _, nid := range g.AllNeighborIDs(id) {
			if communityOf[id] == communityOf[nid] {
				w := g.EdgeWeight(id, nid)
				kj := 0.0
				for _, njid := range g.AllNeighborIDs(nid) {
					kj += g.EdgeWeight(nid, njid)
				}
				q += w - ki*kj/m2
			}
		}
	}
	return q / m2
}
