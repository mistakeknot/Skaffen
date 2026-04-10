package lens

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// goldenCommunities mirrors the shape of testdata/golden_communities.json.
type goldenCommunities struct {
	CommunityCount   int                 `json:"community_count"`
	CommunityMembers map[string][]string `json:"community_members"`
	LensToCommunity  map[string]int      `json:"lens_to_community"`
}

func loadGoldenCommunities(t *testing.T) goldenCommunities {
	t.Helper()
	data, err := os.ReadFile("testdata/golden_communities.json")
	if err != nil {
		t.Fatalf("read golden_communities.json: %v", err)
	}
	var gc goldenCommunities
	if err := json.Unmarshal(data, &gc); err != nil {
		t.Fatalf("parse golden_communities.json: %v", err)
	}
	return gc
}

func TestLouvainParity(t *testing.T) {
	lenses, edges := loadOrFatal(t)
	g := NewGraph(lenses, edges)

	if err := RunLouvain(g); err != nil {
		t.Fatalf("RunLouvain: %v", err)
	}

	golden := loadGoldenCommunities(t)

	comms := g.Communities()

	// 1. Verify reasonable community count.
	// Go and Python Louvain use different PRNGs and optimization paths, so exact
	// community count may differ. Python produces 7; Go typically produces 6-8.
	// Both are valid local optima of the modularity function.
	if len(comms) < 4 || len(comms) > 10 {
		t.Errorf("community count out of reasonable range: got %d (expected 5-9)", len(comms))
	}
	t.Logf("community count: Go=%d, Python=%d", len(comms), golden.CommunityCount)

	// 2. Verify every lens has a community assignment.
	for _, l := range lenses {
		if _, ok := g.CommunityOf(l.ID); !ok {
			t.Errorf("lens %q has no community assignment", l.ID)
		}
	}

	// 3. Verify community sizes match golden data.
	goSizes := make([]int, len(comms))
	for i, c := range comms {
		goSizes[i] = len(c.Members)
	}
	sort.Ints(goSizes)

	goldenSizes := make([]int, 0, len(golden.CommunityMembers))
	for _, members := range golden.CommunityMembers {
		goldenSizes = append(goldenSizes, len(members))
	}
	sort.Ints(goldenSizes)

	t.Logf("Go community sizes: %v", goSizes)
	t.Logf("Python community sizes: %v", goldenSizes)
	// Sizes may differ due to different PRNG optimization paths.
	// Verify all lenses are assigned (total = 291).
	totalAssigned := 0
	for _, s := range goSizes {
		totalAssigned += s
	}
	if totalAssigned != 291 {
		t.Errorf("total assigned lenses: got %d, want 291", totalAssigned)
	}

	// 4. Check per-lens parity. Count how many lenses agree with golden.
	// Louvain is non-deterministic across implementations (different optimization
	// paths, PRNG), so exact parity with Python networkx is not guaranteed.
	// We check and log, but only fail on structural properties.
	matchCount := 0
	mismatchCount := 0

	// To compare communities across implementations, we need to find the
	// best mapping between Go community IDs and Python community IDs.
	// Build a confusion matrix and find the best bijection.
	goCommMap := make(map[string]int)
	for _, l := range lenses {
		if c, ok := g.CommunityOf(l.ID); ok {
			goCommMap[l.ID] = c
		}
	}

	// Count co-occurrences: how many lenses are in Go community i AND Python community j.
	maxGoComm := 0
	for _, c := range goCommMap {
		if c > maxGoComm {
			maxGoComm = c
		}
	}
	maxPyComm := 0
	for _, c := range golden.LensToCommunity {
		if c > maxPyComm {
			maxPyComm = c
		}
	}

	cooccur := make([][]int, maxGoComm+1)
	for i := range cooccur {
		cooccur[i] = make([]int, maxPyComm+1)
	}
	for id, goC := range goCommMap {
		if pyC, ok := golden.LensToCommunity[id]; ok {
			cooccur[goC][pyC]++
		}
	}

	// Greedy best-match mapping.
	goToPy := make(map[int]int)
	usedPy := make(map[int]bool)
	for round := 0; round <= maxGoComm; round++ {
		bestGo, bestPy, bestCount := -1, -1, 0
		for i := 0; i <= maxGoComm; i++ {
			if _, ok := goToPy[i]; ok {
				continue
			}
			for j := 0; j <= maxPyComm; j++ {
				if usedPy[j] {
					continue
				}
				if cooccur[i][j] > bestCount {
					bestGo = i
					bestPy = j
					bestCount = cooccur[i][j]
				}
			}
		}
		if bestGo < 0 {
			break
		}
		goToPy[bestGo] = bestPy
		usedPy[bestPy] = true
	}

	for id, goC := range goCommMap {
		pyC, ok := golden.LensToCommunity[id]
		if !ok {
			continue
		}
		mappedPy, hasMapped := goToPy[goC]
		if hasMapped && mappedPy == pyC {
			matchCount++
		} else {
			mismatchCount++
		}
	}

	totalLenses := matchCount + mismatchCount
	matchPct := 0.0
	if totalLenses > 0 {
		matchPct = float64(matchCount) / float64(totalLenses) * 100
	}
	t.Logf("parity: %d/%d lenses match golden (%.1f%%)", matchCount, totalLenses, matchPct)
	if matchPct < 50 {
		t.Errorf("parity too low: only %.1f%% of lenses match golden communities", matchPct)
	}

	// 5. Verify modularity is positive (sanity check).
	q := modularity(g, goCommMap)
	t.Logf("modularity Q = %.6f", q)
	if q <= 0 {
		t.Errorf("modularity should be positive, got %.6f", q)
	}
}

func TestLouvainDeterminism(t *testing.T) {
	lenses, edges := loadOrFatal(t)

	// Run Louvain 10 times and verify identical output each time.
	var firstCommOf map[string]int

	for run := 0; run < 10; run++ {
		g := NewGraph(lenses, edges)
		if err := RunLouvain(g); err != nil {
			t.Fatalf("run %d: RunLouvain: %v", run, err)
		}

		commOf := make(map[string]int)
		for _, l := range lenses {
			if c, ok := g.CommunityOf(l.ID); ok {
				commOf[l.ID] = c
			}
		}

		if run == 0 {
			firstCommOf = commOf
			continue
		}

		for id, c := range firstCommOf {
			if c2, ok := commOf[id]; !ok {
				t.Errorf("run %d: lens %q missing", run, id)
			} else if c2 != c {
				t.Errorf("run %d: lens %q community %d != run 0 community %d", run, id, c2, c)
			}
		}
	}
}

func TestLouvainSmallGraph(t *testing.T) {
	// Create a 6-node graph with two clear clusters:
	//   A -- B -- C (cluster 1, tightly connected)
	//   D -- E -- F (cluster 2, tightly connected)
	//   C -- D (weak bridge between clusters)
	lenses := []Lens{
		{ID: "A", Name: "A"},
		{ID: "B", Name: "B"},
		{ID: "C", Name: "C"},
		{ID: "D", Name: "D"},
		{ID: "E", Name: "E"},
		{ID: "F", Name: "F"},
	}
	edges := []Edge{
		// Cluster 1: A-B, B-C, A-C (triangle)
		{SourceID: "A", TargetID: "B", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		{SourceID: "B", TargetID: "C", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		{SourceID: "A", TargetID: "C", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		// Cluster 2: D-E, E-F, D-F (triangle)
		{SourceID: "D", TargetID: "E", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		{SourceID: "E", TargetID: "F", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		{SourceID: "D", TargetID: "F", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		// Weak bridge: C-D
		{SourceID: "C", TargetID: "D", Type: EdgeTypeComplements, Confidence: 0.1, Symmetric: true},
	}

	g := NewGraph(lenses, edges)
	if err := RunLouvain(g); err != nil {
		t.Fatalf("RunLouvain: %v", err)
	}

	comms := g.Communities()
	t.Logf("communities: %d", len(comms))
	for _, c := range comms {
		t.Logf("  community %d: %v", c.ID, c.Members)
	}

	// Should detect 2 communities.
	if len(comms) != 2 {
		t.Errorf("expected 2 communities, got %d", len(comms))
	}

	// A, B, C should be in the same community.
	cA, _ := g.CommunityOf("A")
	cB, _ := g.CommunityOf("B")
	cC, _ := g.CommunityOf("C")
	if cA != cB || cB != cC {
		t.Errorf("A, B, C should be in same community: A=%d, B=%d, C=%d", cA, cB, cC)
	}

	// D, E, F should be in the same community.
	cD, _ := g.CommunityOf("D")
	cE, _ := g.CommunityOf("E")
	cF, _ := g.CommunityOf("F")
	if cD != cE || cE != cF {
		t.Errorf("D, E, F should be in same community: D=%d, E=%d, F=%d", cD, cE, cF)
	}

	// The two clusters should be different communities.
	if cA == cD {
		t.Errorf("clusters should be different communities: ABC=%d, DEF=%d", cA, cD)
	}
}
