package experiment

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeCampaign_Basic(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "analyze-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")

	// 3 keeps, 2 discards
	experiments := []struct {
		after    float64
		decision string
	}{
		{1.5, "keep"},
		{1.3, "discard"},
		{1.8, "keep"},
		{1.7, "discard"},
		{2.0, "keep"},
	}

	for _, e := range experiments {
		before := seg.CurrentBest()
		seg.LogExperiment(ExperimentRecord{
			MetricBefore:  before,
			MetricAfter:   e.after,
			Delta:         e.after - before,
			AgentDecision: e.decision,
			Decision:      e.decision,
		})
	}

	// Analyze
	path := filepath.Join(dir, "analyze-test.jsonl")
	analysis, err := AnalyzeCampaign(path)
	if err != nil {
		t.Fatalf("AnalyzeCampaign: %v", err)
	}

	if analysis.Campaign != "analyze-test" {
		t.Errorf("Campaign = %q, want analyze-test", analysis.Campaign)
	}
	if analysis.TotalExperiments != 5 {
		t.Errorf("TotalExperiments = %d, want 5", analysis.TotalExperiments)
	}
	if analysis.Kept != 3 {
		t.Errorf("Kept = %d, want 3", analysis.Kept)
	}
	if analysis.Discarded != 2 {
		t.Errorf("Discarded = %d, want 2", analysis.Discarded)
	}
	if analysis.OriginalBaseline != 1.0 {
		t.Errorf("OriginalBaseline = %v, want 1.0", analysis.OriginalBaseline)
	}
	if analysis.FinalBest != 2.0 {
		t.Errorf("FinalBest = %v, want 2.0", analysis.FinalBest)
	}

	// Convergence curve should have 5 points
	if len(analysis.Convergence) != 5 {
		t.Errorf("Convergence points = %d, want 5", len(analysis.Convergence))
	}

	// Verify JSON marshaling works
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal: %v", err)
	}
	if len(data) == 0 {
		t.Error("JSON output should not be empty")
	}
}

func TestAnalyzeCampaign_WithMutations(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "mut-analyze",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")

	// Mix of mutation types
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.2, Delta: 0.2,
		AgentDecision: "keep", Decision: "keep",
		MutationID: "mutation:parameter_sweep:lr:0.1", MutationType: "parameter_sweep",
	})
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.2, MetricAfter: 1.1, Delta: -0.1,
		AgentDecision: "discard", Decision: "discard",
		MutationID: "mutation:parameter_sweep:lr:0.2", MutationType: "parameter_sweep",
	})
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.2, MetricAfter: 1.4, Delta: 0.2,
		AgentDecision: "keep", Decision: "keep",
		MutationID: "mutation:swap:A:B", MutationType: "swap",
	})

	path := filepath.Join(dir, "mut-analyze.jsonl")
	analysis, err := AnalyzeCampaign(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(analysis.MutationStats) != 2 {
		t.Fatalf("MutationStats = %d types, want 2", len(analysis.MutationStats))
	}

	sweep := analysis.MutationStats["parameter_sweep"]
	if sweep.Total != 2 {
		t.Errorf("parameter_sweep.Total = %d, want 2", sweep.Total)
	}
	if sweep.Kept != 1 {
		t.Errorf("parameter_sweep.Kept = %d, want 1", sweep.Kept)
	}
	if sweep.KeepRate != 0.5 {
		t.Errorf("parameter_sweep.KeepRate = %v, want 0.5", sweep.KeepRate)
	}

	swapStat := analysis.MutationStats["swap"]
	if swapStat.Total != 1 || swapStat.Kept != 1 {
		t.Errorf("swap stats wrong: total=%d kept=%d", swapStat.Total, swapStat.Kept)
	}
}

func TestAnalyzeCampaign_Overrides(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "override-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")

	// Agent said keep, system overrode to discard
	seg.LogExperiment(ExperimentRecord{
		MetricBefore:   1.0,
		MetricAfter:    1.5,
		Delta:          0.5,
		AgentDecision:  "keep",
		Decision:       "discard",
		OverrideReason: "secondary metric test_pass_rate regressed by 0.05",
	})

	path := filepath.Join(dir, "override-test.jsonl")
	analysis, err := AnalyzeCampaign(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(analysis.Overrides) != 1 {
		t.Fatalf("Overrides = %d, want 1", len(analysis.Overrides))
	}
	if analysis.Overrides[0].AgentDecision != "keep" {
		t.Errorf("override agent_decision = %q, want keep", analysis.Overrides[0].AgentDecision)
	}
	if analysis.Overrides[0].EffectiveDecn != "discard" {
		t.Errorf("override effective = %q, want discard", analysis.Overrides[0].EffectiveDecn)
	}
}

func TestAnalyzeCampaign_DiminishingReturns(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "diminish-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")

	// First 10: high keep rate, large deltas
	for i := 0; i < 10; i++ {
		seg.LogExperiment(ExperimentRecord{
			MetricBefore: 1.0, MetricAfter: 1.0 + float64(i)*0.1, Delta: float64(i) * 0.1,
			AgentDecision: "keep", Decision: "keep",
		})
	}

	// Last 10: low keep rate, small deltas
	for i := 0; i < 10; i++ {
		decision := "discard"
		if i%5 == 0 {
			decision = "keep"
		}
		seg.LogExperiment(ExperimentRecord{
			MetricBefore: 1.9, MetricAfter: 1.91, Delta: 0.01,
			AgentDecision: decision, Decision: decision,
		})
	}

	path := filepath.Join(dir, "diminish-test.jsonl")
	analysis, err := AnalyzeCampaign(path)
	if err != nil {
		t.Fatal(err)
	}

	if analysis.DiminishingReturns == nil {
		t.Fatal("DiminishingReturns should not be nil")
	}
	if !analysis.DiminishingReturns.Detected {
		t.Error("expected diminishing returns to be detected")
	}
}

func TestAnalyzeCampaign_EmptyCampaign(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "empty-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	store.OpenSegment(campaign, "session-1")

	path := filepath.Join(dir, "empty-test.jsonl")
	analysis, err := AnalyzeCampaign(path)
	if err != nil {
		t.Fatal(err)
	}

	if analysis.TotalExperiments != 0 {
		t.Errorf("TotalExperiments = %d, want 0", analysis.TotalExperiments)
	}
}

func TestAnalyzeCampaign_NotFound(t *testing.T) {
	_, err := AnalyzeCampaign("/nonexistent/path.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestAnalyzeCampaignByName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "byname-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.5, Delta: 0.5,
		AgentDecision: "keep", Decision: "keep",
	})

	analysis, err := AnalyzeCampaignByName(dir, "byname-test")
	if err != nil {
		t.Fatal(err)
	}
	if analysis.TotalExperiments != 1 {
		t.Errorf("TotalExperiments = %d, want 1", analysis.TotalExperiments)
	}
}

func TestAnalyzeCampaign_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "json-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.5, Delta: 0.5,
		AgentDecision: "keep", Decision: "keep",
		MutationID: "mutation:toggle:debug", MutationType: "toggle",
	})

	path := filepath.Join(dir, "json-test.jsonl")
	analysis, err := AnalyzeCampaign(path)
	if err != nil {
		t.Fatal(err)
	}

	// Verify round-trip through JSON
	data, err := json.Marshal(analysis)
	if err != nil {
		t.Fatal(err)
	}

	var decoded CampaignAnalysis
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON round-trip failed: %v", err)
	}

	if decoded.Campaign != analysis.Campaign {
		t.Errorf("round-trip Campaign = %q, want %q", decoded.Campaign, analysis.Campaign)
	}
	if decoded.TotalExperiments != analysis.TotalExperiments {
		t.Errorf("round-trip TotalExperiments = %d, want %d", decoded.TotalExperiments, analysis.TotalExperiments)
	}

	// Write to file for inspection
	outPath := filepath.Join(dir, "analysis.json")
	os.WriteFile(outPath, data, 0600)
	t.Logf("Analysis JSON written to %s (%d bytes)", outPath, len(data))
}
