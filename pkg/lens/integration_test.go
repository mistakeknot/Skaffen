package lens

import (
	"context"
	"fmt"
	"testing"
)

func TestIntegrationFullPipeline(t *testing.T) {
	// 1. Load data.
	lenses, edges := loadOrFatal(t)
	t.Logf("loaded %d lenses, %d edges", len(lenses), len(edges))

	// 2. Build graph.
	g := NewGraph(lenses, edges)
	if g.NodeCount() != expectedLensCount {
		t.Fatalf("node count: got %d, want %d", g.NodeCount(), expectedLensCount)
	}

	// 3. Run Louvain community detection.
	if err := RunLouvain(g); err != nil {
		t.Fatalf("RunLouvain: %v", err)
	}
	comms := g.Communities()
	if len(comms) < 4 {
		t.Fatalf("too few communities: %d", len(comms))
	}
	t.Logf("communities: %d", len(comms))

	// Verify every lens has a community assignment.
	for _, l := range lenses {
		if _, ok := g.CommunityOf(l.ID); !ok {
			t.Errorf("lens %q has no community assignment after Louvain", l.ID)
		}
	}

	// 4. Run betweenness centrality.
	if err := RunBetweenness(g, 15); err != nil {
		t.Fatalf("RunBetweenness: %v", err)
	}
	bridges := g.BridgeLenses()
	if len(bridges) != 15 {
		t.Fatalf("expected 15 bridges, got %d", len(bridges))
	}
	t.Logf("top bridge: %s (score %.6f)", bridges[0].ID, g.BridgeScore(bridges[0].ID))

	// Verify bridge scores are positive and descending.
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

	// 5. Selector with mock provider.
	// Build a response that selects indices 1, 3, 5 (1-indexed).
	mock := &mockProvider{response: `[1, 3, 5]`}
	sel := NewLLMSelector(mock, lenses)

	refs, err := sel.Select(context.Background(), "I need help understanding complex systems", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected 3 selected refs, got %d", len(refs))
	}
	// Indices 1, 3, 5 should map to lenses[0], lenses[2], lenses[4].
	if refs[0].ID != lenses[0].ID {
		t.Errorf("refs[0].ID = %q, want %q", refs[0].ID, lenses[0].ID)
	}
	if refs[1].ID != lenses[2].ID {
		t.Errorf("refs[1].ID = %q, want %q", refs[1].ID, lenses[2].ID)
	}
	if refs[2].ID != lenses[4].ID {
		t.Errorf("refs[2].ID = %q, want %q", refs[2].ID, lenses[4].ID)
	}
	t.Logf("selected lenses: %s, %s, %s", refs[0].Name, refs[1].Name, refs[2].Name)

	// 6. Evolution tracker.
	store := NewMemoryStore()
	tracker := NewTracker(store)

	// Use the first bridge lens for tracking.
	trackLensID := bridges[0].ID
	if err := tracker.RecordEvent(trackLensID, "user_1", "engaged"); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	eff := tracker.Effectiveness(trackLensID)
	if eff <= 0 {
		t.Errorf("effectiveness should be positive after engaged event, got %f", eff)
	}
	t.Logf("effectiveness of %s after engaged: %.4f", trackLensID, eff)

	// Record a second event and verify score changes.
	if err := tracker.RecordEvent(trackLensID, "user_1", "pushed_back"); err != nil {
		t.Fatalf("RecordEvent (pushed_back): %v", err)
	}
	eff2 := tracker.Effectiveness(trackLensID)
	t.Logf("effectiveness of %s after pushed_back: %.4f", trackLensID, eff2)

	// Classify engagement uses the tracker method.
	classification := tracker.ClassifyEngagement("good point, let me think", nil)
	if classification != "engaged" {
		t.Errorf("ClassifyEngagement: got %q, want %q", classification, "engaged")
	}

	// 7. Stack orchestrator using top-3 bridge lens IDs.
	bridgeIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		bridgeIDs[i] = bridges[i].ID
	}
	orch := NewOrchestrator(bridgeIDs, DepthShallowGold)

	for i := 0; i < 3; i++ {
		input := fmt.Sprintf("phase %d input", i)
		result, err := orch.NextPhase(input)
		if err != nil {
			t.Fatalf("phase %d: %v", i, err)
		}
		if result.Lens != bridgeIDs[i] {
			t.Errorf("phase %d: lens = %q, want %q", i, result.Lens, bridgeIDs[i])
		}
		t.Logf("phase %d: lens=%s", i, result.Lens)

		// Phases after the first should have transition text.
		if i > 0 && result.TransitionText == "" {
			t.Errorf("phase %d: expected non-empty transition text", i)
		}
	}

	if !orch.IsComplete() {
		t.Error("stack should be complete after 3 phases")
	}

	// Verify exhaustion.
	_, err = orch.NextPhase("too many")
	if err != ErrStackExhausted {
		t.Errorf("expected ErrStackExhausted, got %v", err)
	}

	// 8. Graph queries: verify bridges have community assignments.
	for _, b := range bridges[:3] {
		cid, ok := g.CommunityOf(b.ID)
		if !ok {
			t.Errorf("bridge %s has no community", b.ID)
		}
		t.Logf("bridge %s: community %d, score %.6f", b.ID, cid, g.BridgeScore(b.ID))
	}
}
