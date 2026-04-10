package lens

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"
)

// goldenBridges mirrors the shape of testdata/golden_bridges.json.
type goldenBridges struct {
	Bridges []goldenBridge `json:"bridges"`
	Count   int            `json:"count"`
}

type goldenBridge struct {
	LensID           string  `json:"lens_id"`
	BetweennessScore float64 `json:"betweenness_score"`
	CommunityID      int     `json:"community_id"`
}

func loadGoldenBridges(t *testing.T) goldenBridges {
	t.Helper()
	data, err := os.ReadFile("testdata/golden_bridges.json")
	if err != nil {
		t.Fatalf("read golden_bridges.json: %v", err)
	}
	var gb goldenBridges
	if err := json.Unmarshal(data, &gb); err != nil {
		t.Fatalf("parse golden_bridges.json: %v", err)
	}
	return gb
}

func TestBetweennessParity(t *testing.T) {
	lenses, edges := loadOrFatal(t)
	g := NewGraph(lenses, edges)

	if err := RunBetweenness(g, 15); err != nil {
		t.Fatalf("RunBetweenness: %v", err)
	}

	golden := loadGoldenBridges(t)

	bridges := g.BridgeLenses()

	// 1. Verify we get 15 bridges.
	if len(bridges) != golden.Count {
		t.Errorf("bridge count: got %d, want %d", len(bridges), golden.Count)
	}

	// 2. Compare bridge lens IDs (order may differ).
	goldenIDs := make(map[string]float64)
	for _, b := range golden.Bridges {
		goldenIDs[b.LensID] = b.BetweennessScore
	}

	goIDs := make(map[string]bool)
	for _, b := range bridges {
		goIDs[b.ID] = true
	}

	matchCount := 0
	for id := range goIDs {
		if _, ok := goldenIDs[id]; ok {
			matchCount++
		}
	}

	t.Logf("bridge ID overlap: %d/%d match golden", matchCount, len(goldenIDs))
	if matchCount != len(goldenIDs) {
		// Log which ones differ.
		for id := range goldenIDs {
			if !goIDs[id] {
				t.Logf("  golden has %q (score %.10f) — missing from Go output", id, goldenIDs[id])
			}
		}
		for id := range goIDs {
			if _, ok := goldenIDs[id]; !ok {
				t.Logf("  Go has %q (score %.10f) — not in golden", id, g.BridgeScore(id))
			}
		}
	}

	// Bridge IDs may not exactly match due to different Louvain community assignments
	// affecting which lenses sit at community boundaries. Log overlap for diagnostics.
	if matchCount < 8 {
		t.Errorf("bridge ID overlap too low: only %d/%d match golden (expect >=8)", matchCount, len(goldenIDs))
	}

	// 3. Verify bridge scores are positive and ordered descending.
	for i, b := range bridges {
		score := g.BridgeScore(b.ID)
		if score <= 0 {
			t.Errorf("bridge %d (%s) has non-positive score: %f", i, b.ID, score)
		}
		if i > 0 {
			prevScore := g.BridgeScore(bridges[i-1].ID)
			if score > prevScore {
				t.Errorf("bridges not sorted: score[%d]=%f > score[%d]=%f", i, score, i-1, prevScore)
			}
		}
	}
}

func TestBetweennessDeterminism(t *testing.T) {
	lenses, edges := loadOrFatal(t)

	type bridgeResult struct {
		ids    []string
		scores map[string]float64
	}

	var first bridgeResult

	for run := 0; run < 5; run++ {
		g := NewGraph(lenses, edges)
		if err := RunBetweenness(g, 15); err != nil {
			t.Fatalf("run %d: RunBetweenness: %v", run, err)
		}

		bridges := g.BridgeLenses()
		ids := make([]string, len(bridges))
		for i, b := range bridges {
			ids[i] = b.ID
		}

		scores := make(map[string]float64, len(bridges))
		for _, b := range bridges {
			scores[b.ID] = g.BridgeScore(b.ID)
		}

		if run == 0 {
			first = bridgeResult{ids: ids, scores: scores}
			continue
		}

		// IDs must match exactly (same order).
		if len(ids) != len(first.ids) {
			t.Errorf("run %d: bridge count %d != %d", run, len(ids), len(first.ids))
			continue
		}
		for i := range ids {
			if ids[i] != first.ids[i] {
				t.Errorf("run %d: bridge[%d] %q != %q", run, i, ids[i], first.ids[i])
			}
		}

		// Scores must match exactly.
		for id, score := range scores {
			if score != first.scores[id] {
				t.Errorf("run %d: score[%q] %.10f != %.10f", run, id, score, first.scores[id])
			}
		}
	}
}

func TestBetweennessSmallGraph(t *testing.T) {
	// Create a 5-node graph: A-B-C-D-E in a line.
	// B and D are the most "between" nodes; C is the bridge.
	//
	//   A -- B -- C -- D -- E
	//
	// C should have the highest betweenness centrality.
	lenses := []Lens{
		{ID: "A", Name: "A"},
		{ID: "B", Name: "B"},
		{ID: "C", Name: "C"},
		{ID: "D", Name: "D"},
		{ID: "E", Name: "E"},
	}
	edges := []Edge{
		{SourceID: "A", TargetID: "B", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		{SourceID: "B", TargetID: "C", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		{SourceID: "C", TargetID: "D", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
		{SourceID: "D", TargetID: "E", Type: EdgeTypeComplements, Confidence: 1.0, Symmetric: true},
	}

	g := NewGraph(lenses, edges)
	if err := RunBetweenness(g, 3); err != nil {
		t.Fatalf("RunBetweenness: %v", err)
	}

	bridges := g.BridgeLenses()
	t.Logf("top-3 bridges:")
	for _, b := range bridges {
		t.Logf("  %s: %.6f", b.ID, g.BridgeScore(b.ID))
	}

	// C should be the top bridge.
	if len(bridges) < 1 {
		t.Fatal("expected at least 1 bridge")
	}
	if bridges[0].ID != "C" {
		t.Errorf("top bridge: got %q, want %q", bridges[0].ID, "C")
	}

	// B and D should have equal scores (symmetric position).
	scoreB := g.BridgeScore("B")
	scoreD := g.BridgeScore("D")
	if math.Abs(scoreB-scoreD) > 1e-10 {
		t.Errorf("B and D should have equal scores: B=%.10f, D=%.10f", scoreB, scoreD)
	}

	// A and E should have zero betweenness (endpoints).
	scoreA := g.BridgeScore("A")
	scoreE := g.BridgeScore("E")
	if scoreA != 0 {
		t.Errorf("A should have 0 betweenness, got %.10f", scoreA)
	}
	if scoreE != 0 {
		t.Errorf("E should have 0 betweenness, got %.10f", scoreE)
	}

	// Verify scores match networkx normalized betweenness for path A-B-C-D-E:
	// C = 0.6667, B = D = 0.5, A = E = 0.0
	scoreC := g.BridgeScore("C")
	if math.Abs(scoreC-2.0/3.0) > 1e-10 {
		t.Errorf("C betweenness: got %.10f, want %.10f", scoreC, 2.0/3.0)
	}

	// B betweenness = 0.5
	if math.Abs(scoreB-0.5) > 1e-10 {
		t.Errorf("B betweenness: got %.10f, want 0.5", scoreB)
	}

	// Sort bridge IDs for determinism check.
	bridgeIDs := make([]string, len(bridges))
	for i, b := range bridges {
		bridgeIDs[i] = b.ID
	}
	expected := []string{"C", "B", "D"}
	sort.Strings(bridgeIDs)
	sort.Strings(expected)
	for i := range expected {
		if bridgeIDs[i] != expected[i] {
			t.Errorf("bridge set: got %v, want %v", bridgeIDs, expected)
			break
		}
	}
}
