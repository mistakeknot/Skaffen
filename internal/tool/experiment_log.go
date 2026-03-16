package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/mistakeknot/Skaffen/internal/experiment"
)

// LogExperimentTool logs experiment results, manages git state, and checks stop conditions.
type LogExperimentTool struct {
	store    ExperimentStore
	finder   CampaignFinder
	wt       Worktree
	icPath   string // path to ic binary (empty = unavailable)
	statusCB ExperimentStatusCallback
}

// NewLogExperimentTool creates a LogExperimentTool.
// icPath is detected at construction via exec.LookPath("ic").
func NewLogExperimentTool(store ExperimentStore, finder CampaignFinder, wt Worktree, icPath string, cb ExperimentStatusCallback) *LogExperimentTool {
	return &LogExperimentTool{
		store:    store,
		finder:   finder,
		wt:       wt,
		icPath:   icPath,
		statusCB: cb,
	}
}

func (t *LogExperimentTool) Name() string { return "log_experiment" }

func (t *LogExperimentTool) Description() string {
	return "Log experiment results and decide: keep (commit changes), discard (revert changes), or investigate (keep changes uncommitted for manual review). May override decision to discard if secondary metrics regress."
}

func (t *LogExperimentTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"campaign": {"type": "string"},
			"decision": {"type": "string", "enum": ["keep", "discard", "investigate"]},
			"metric_value": {"type": "number", "description": "Primary metric value from run_experiment"},
			"secondary_values": {"type": "object", "description": "Map of secondary metric name to value"},
			"notes": {"type": "string", "description": "Optional notes about what was learned"},
			"mutation_id": {"type": "string", "description": "Mutation ID if this experiment was driven by a structured mutation"},
			"mutation_type": {"type": "string", "description": "Mutation type (parameter_sweep, swap, toggle, etc.)"}
		},
		"required": ["campaign", "decision", "metric_value"]
	}`)
}

type logParams struct {
	Campaign        string             `json:"campaign"`
	Decision        string             `json:"decision"`
	MetricValue     float64            `json:"metric_value"`
	SecondaryValues map[string]float64 `json:"secondary_values"`
	Notes           string             `json:"notes"`
	MutationID      string             `json:"mutation_id,omitempty"`
	MutationType    string             `json:"mutation_type,omitempty"`
}

type logResult struct {
	ExperimentID      string                       `json:"experiment_id"`
	AgentDecision     string                       `json:"agent_decision"`
	EffectiveDecision string                       `json:"effective_decision"`
	OverrideReason    string                       `json:"override_reason,omitempty"`
	Delta             float64                      `json:"delta"`
	CumulativeDelta   float64                      `json:"cumulative_delta"`
	GitSHA            string                       `json:"git_sha,omitempty"`
	CampaignComplete  bool                         `json:"campaign_complete"`
	StopReason        string                       `json:"stop_reason,omitempty"`
	NextMutation      *experiment.ExpandedMutation `json:"next_mutation,omitempty"`
	PendingMutations  int                          `json:"pending_mutations"`
}

func (t *LogExperimentTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p logParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}

	campaign, err := t.finder.FindCampaign(p.Campaign)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("campaign not found: %v", err), IsError: true}
	}

	seg, err := t.store.LoadSegment(p.Campaign)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("no active segment: %v", err), IsError: true}
	}

	// Compute delta from current best
	currentBest := seg.CurrentBest()
	delta := p.MetricValue - currentBest
	cumulativeDelta := p.MetricValue - seg.OriginalBaseline()

	// Check secondary metric regressions BEFORE git operations
	agentDecision := p.Decision
	effectiveDecision := p.Decision
	overrideReason := ""

	if effectiveDecision == "keep" {
		for _, sm := range campaign.SecondaryMetrics {
			secVal, ok := p.SecondaryValues[sm.Name]
			if !ok {
				continue
			}
			regression := checkRegression(sm, secVal)
			if regression != "" {
				effectiveDecision = "discard"
				overrideReason = regression
				break
			}
		}
	}

	// Execute git operations based on effective decision
	var gitSHA string
	switch effectiveDecision {
	case "keep":
		sha, err := t.wt.KeepChanges(p.Campaign, "experiment", delta)
		if err != nil {
			return ToolResult{Content: fmt.Sprintf("keep failed: %v", err), IsError: true}
		}
		gitSHA = sha

		// Interlab bridge (best-effort) — uses exec.Command with args slice, never shell string
		t.bridgeToInterlab(campaign.Name, delta)

	case "discard":
		if err := t.wt.DiscardChanges(p.Campaign); err != nil {
			return ToolResult{Content: fmt.Sprintf("discard failed: %v", err), IsError: true}
		}

	case "investigate":
		// No git action — keep changes uncommitted for manual review
	}

	// Determine experiment status
	status := "completed"
	if overrideReason != "" {
		status = "rejected_secondary"
	}

	// Write experiment record to JSONL
	rec := experiment.ExperimentRecord{
		Hypothesis:     p.Notes,
		Status:         status,
		MetricBefore:   currentBest,
		MetricAfter:    p.MetricValue,
		Delta:          delta,
		Secondary:      p.SecondaryValues,
		AgentDecision:  agentDecision,
		Decision:       effectiveDecision,
		OverrideReason: overrideReason,
		GitSHA:         gitSHA,
		Notes:          p.Notes,
		MutationID:     p.MutationID,
		MutationType:   p.MutationType,
	}

	if err := seg.LogExperiment(rec); err != nil {
		return ToolResult{Content: fmt.Sprintf("log failed: %v", err), IsError: true}
	}

	// Check stop conditions
	stop, stopReason := seg.ShouldStop(
		campaign.Budget.MaxExperimentsOrDefault(),
		campaign.Budget.MaxConsecutiveFailuresOrDefault(),
	)

	// Send TUI status update
	if t.statusCB != nil {
		t.statusCB(seg.Snapshot(
			campaign.Budget.MaxExperimentsOrDefault(),
			campaign.Metric.Unit,
		))
	}

	result := logResult{
		ExperimentID:      fmt.Sprintf("exp-%03d", seg.ExperimentCount()),
		AgentDecision:     agentDecision,
		EffectiveDecision: effectiveDecision,
		OverrideReason:    overrideReason,
		Delta:             delta,
		CumulativeDelta:   cumulativeDelta,
		GitSHA:            gitSHA,
		CampaignComplete:  stop,
		StopReason:        stopReason,
		NextMutation:      seg.NextMutation(),
		PendingMutations:  seg.PendingMutationCount(),
	}

	data, _ := json.Marshal(result)

	summary := fmt.Sprintf("Experiment logged: decision=%s", effectiveDecision)
	if overrideReason != "" {
		summary += fmt.Sprintf(" (overridden from %s: %s)", agentDecision, overrideReason)
	}
	summary += fmt.Sprintf("\nDelta: %+.4f | Cumulative: %+.4f", delta, cumulativeDelta)
	if stop {
		summary += fmt.Sprintf("\nCampaign complete: %s", stopReason)
	}

	return ToolResult{Content: summary + "\n\n```json\n" + string(data) + "\n```"}
}

// checkRegression returns a reason string if the secondary metric regressed beyond threshold.
func checkRegression(sm experiment.SecondaryMetric, value float64) string {
	switch sm.Direction {
	case experiment.Maximize:
		// Regression = value decreased below baseline - threshold
		if sm.Baseline-value > sm.RegressionThreshold {
			return fmt.Sprintf("secondary metric %q regressed by %.4f (threshold %.4f)",
				sm.Name, sm.Baseline-value, sm.RegressionThreshold)
		}
	case experiment.Minimize:
		// Regression = value increased above baseline + threshold
		if value-sm.Baseline > sm.RegressionThreshold {
			return fmt.Sprintf("secondary metric %q regressed by %.4f (threshold %.4f)",
				sm.Name, value-sm.Baseline, sm.RegressionThreshold)
		}
	}
	return ""
}

// bridgeToInterlab sends a mutation event to intercore (best-effort).
// Uses exec.Command with args slice — never shell string interpolation.
func (t *LogExperimentTool) bridgeToInterlab(campaignName string, delta float64) {
	if t.icPath == "" {
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"campaign": campaignName,
		"delta":    delta,
		"source":   "autoresearch",
	})

	cmd := exec.Command(t.icPath,
		"events", "record",
		"--source=autoresearch",
		"--type=mutation_kept",
		"--payload="+string(payload),
	)
	cmd.Run() // ignore errors — interlab bridge is best-effort
}
