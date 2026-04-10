package lens

import "sort"

// Graph provides read access to the lens relationship graph.
type Graph interface {
	Communities() []Community
	BridgeLenses() []LensRef
	Neighbors(lensID string, edgeType EdgeType) []LensRef
	CommunityOf(lensID string) (int, bool)
	BridgeScore(lensID string) float64
}

// adjacencyEntry holds the neighbors of a single node, grouped by edge type.
type adjacencyEntry struct {
	byType map[EdgeType][]string
}

// graph is the concrete implementation of Graph backed by adjacency lists.
type graph struct {
	adj          map[string]*adjacencyEntry
	lensMap      map[string]Lens
	edgeWeights  map[[2]string]float64 // [source,target] → total confidence
	communityOf  map[string]int
	communities  []Community
	bridgeLenses []LensRef
	bridgeScores map[string]float64
}

// NewGraph builds an adjacency-list graph from the provided lenses and edges.
// For symmetric edges, both directions are recorded.
func NewGraph(lenses []Lens, edges []Edge) *graph {
	g := &graph{
		adj:          make(map[string]*adjacencyEntry, len(lenses)),
		lensMap:      make(map[string]Lens, len(lenses)),
		edgeWeights:  make(map[[2]string]float64),
		communityOf:  make(map[string]int),
		bridgeScores: make(map[string]float64),
	}

	for i := range lenses {
		g.lensMap[lenses[i].ID] = lenses[i]
		g.adj[lenses[i].ID] = &adjacencyEntry{
			byType: make(map[EdgeType][]string),
		}
	}

	for i := range edges {
		e := &edges[i]
		g.addDirected(e.SourceID, e.TargetID, e.Type, e.Confidence)
		if e.Symmetric {
			g.addDirected(e.TargetID, e.SourceID, e.Type, e.Confidence)
		}
	}

	// Deduplicate and sort neighbor lists within each adjacency entry.
	for _, entry := range g.adj {
		for et, ids := range entry.byType {
			entry.byType[et] = sortedUnique(ids)
		}
	}

	return g
}

// addDirected adds a single directed adjacency and accumulates edge weight.
func (g *graph) addDirected(from, to string, et EdgeType, confidence float64) {
	entry := g.adj[from]
	if entry == nil {
		entry = &adjacencyEntry{byType: make(map[EdgeType][]string)}
		g.adj[from] = entry
	}
	entry.byType[et] = append(entry.byType[et], to)

	// Accumulate edge weight (keyed with sorted pair so a→b and b→a share weight).
	key := edgeKey(from, to)
	g.edgeWeights[key] += confidence
}

// edgeKey returns a canonical key for an undirected edge between a and b.
func edgeKey(a, b string) [2]string {
	if a <= b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

// sortedUnique deduplicates and sorts a string slice in place.
func sortedUnique(ids []string) []string {
	if len(ids) == 0 {
		return ids
	}
	sort.Strings(ids)
	j := 0
	for i := 1; i < len(ids); i++ {
		if ids[i] != ids[j] {
			j++
			ids[j] = ids[i]
		}
	}
	return ids[:j+1]
}

// ---------------------------------------------------------------------------
// Graph interface
// ---------------------------------------------------------------------------

// Communities returns the detected community list. Empty until SetCommunities
// is called (by the Louvain algorithm).
func (g *graph) Communities() []Community {
	return g.communities
}

// BridgeLenses returns lenses identified as inter-community bridges. Empty
// until SetBridges is called (by the betweenness algorithm).
func (g *graph) BridgeLenses() []LensRef {
	return g.bridgeLenses
}

// Neighbors returns all neighbors of lensID connected by the given edge type,
// sorted by LensRef.ID for determinism.
func (g *graph) Neighbors(lensID string, edgeType EdgeType) []LensRef {
	entry := g.adj[lensID]
	if entry == nil {
		return nil
	}
	ids := entry.byType[edgeType]
	refs := make([]LensRef, 0, len(ids))
	for _, id := range ids {
		if l, ok := g.lensMap[id]; ok {
			refs = append(refs, l.Ref())
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].ID < refs[j].ID })
	return refs
}

// CommunityOf returns the community ID for the given lens and whether the
// mapping exists. Returns (0, false) if unknown.
func (g *graph) CommunityOf(lensID string) (int, bool) {
	id, ok := g.communityOf[lensID]
	return id, ok
}

// BridgeScore returns the bridge score for the given lens. Returns 0 if the
// lens has no computed score.
func (g *graph) BridgeScore(lensID string) float64 {
	return g.bridgeScores[lensID]
}

// ---------------------------------------------------------------------------
// Helper methods for graph algorithms
// ---------------------------------------------------------------------------

// AllNeighborIDs returns every neighbor of id regardless of edge type, sorted
// and deduplicated.
func (g *graph) AllNeighborIDs(id string) []string {
	entry := g.adj[id]
	if entry == nil {
		return nil
	}
	var all []string
	for _, ids := range entry.byType {
		all = append(all, ids...)
	}
	return sortedUnique(all)
}

// SortedNodeIDs returns all node IDs in sorted order. Useful for deterministic
// iteration in Louvain and betweenness algorithms.
func (g *graph) SortedNodeIDs() []string {
	ids := make([]string, 0, len(g.adj))
	for id := range g.adj {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// SetCommunities stores Louvain-computed community assignments.
func (g *graph) SetCommunities(communities []Community, communityOf map[string]int) {
	g.communities = communities
	g.communityOf = communityOf
}

// SetBridges stores betweenness-computed bridge lenses and scores.
func (g *graph) SetBridges(bridges []LensRef, scores map[string]float64) {
	g.bridgeLenses = bridges
	g.bridgeScores = scores
}

// EdgeWeight returns the total edge weight (sum of confidence values across
// all edge types) between two nodes. Returns 0 if no edge exists.
func (g *graph) EdgeWeight(a, b string) float64 {
	return g.edgeWeights[edgeKey(a, b)]
}

// NodeCount returns the number of nodes in the graph.
func (g *graph) NodeCount() int {
	return len(g.adj)
}
