package experiment

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateResultsMarkdown_Basic(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "results-test",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.5, Delta: 0.5,
		AgentDecision: "keep", Decision: "keep",
	})
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.5, MetricAfter: 1.3, Delta: -0.2,
		AgentDecision: "discard", Decision: "discard",
	})
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.5, MetricAfter: 2.0, Delta: 0.5,
		AgentDecision: "keep", Decision: "keep",
	})

	path := filepath.Join(dir, "results-test.jsonl")
	analysis, err := AnalyzeCampaign(path)
	if err != nil {
		t.Fatal(err)
	}

	md := GenerateResultsMarkdown(analysis)

	// Check key sections exist
	checks := []string{
		"# Autoresearch Results: results-test",
		"## Summary",
		"Total experiments | 3",
		"Kept | 2",
		"Discarded | 1",
		"Keep rate | 67%",
		"## Convergence",
	}
	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("missing %q in output:\n%s", check, md)
		}
	}
}

func TestGenerateResultsMarkdown_WithMutations(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "mut-results",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore: 1.0, MetricAfter: 1.2, Delta: 0.2,
		AgentDecision: "keep", Decision: "keep",
		MutationID: "mutation:toggle:debug", MutationType: "toggle",
	})

	path := filepath.Join(dir, "mut-results.jsonl")
	analysis, _ := AnalyzeCampaign(path)
	md := GenerateResultsMarkdown(analysis)

	if !strings.Contains(md, "## Mutation Type Effectiveness") {
		t.Error("missing mutation section")
	}
	if !strings.Contains(md, "toggle") {
		t.Error("missing toggle mutation type in output")
	}
}

func TestGenerateResultsMarkdown_WithOverrides(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	campaign := &Campaign{
		Name:   "override-results",
		Metric: MetricConfig{Name: "speed", Baseline: 1.0},
	}

	seg, _, _ := store.OpenSegment(campaign, "session-1")
	seg.LogExperiment(ExperimentRecord{
		MetricBefore:   1.0,
		MetricAfter:    1.5,
		Delta:          0.5,
		AgentDecision:  "keep",
		Decision:       "discard",
		OverrideReason: "secondary metric regressed",
	})

	path := filepath.Join(dir, "override-results.jsonl")
	analysis, _ := AnalyzeCampaign(path)
	md := GenerateResultsMarkdown(analysis)

	if !strings.Contains(md, "## Decision Overrides") {
		t.Error("missing overrides section")
	}
	if !strings.Contains(md, "secondary metric regressed") {
		t.Error("missing override reason")
	}
}

func TestGenerateResultsMarkdown_Empty(t *testing.T) {
	analysis := &CampaignAnalysis{
		Campaign:         "empty",
		OriginalBaseline: 1.0,
	}

	md := GenerateResultsMarkdown(analysis)
	if !strings.Contains(md, "# Autoresearch Results: empty") {
		t.Error("missing title for empty campaign")
	}
	if !strings.Contains(md, "Total experiments | 0") {
		t.Error("should show 0 experiments")
	}
}
