package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mistakeknot/Skaffen/internal/experiment"
)

// InitExperimentTool initializes an experiment campaign session.
type InitExperimentTool struct {
	store     ExperimentStore
	finder    CampaignFinder
	wt        Worktree
	sessionID string
}

// NewInitExperimentTool creates an InitExperimentTool.
func NewInitExperimentTool(store ExperimentStore, finder CampaignFinder, wt Worktree, sessionID string) *InitExperimentTool {
	return &InitExperimentTool{
		store:     store,
		finder:    finder,
		wt:        wt,
		sessionID: sessionID,
	}
}

func (t *InitExperimentTool) Name() string { return "init_experiment" }

func (t *InitExperimentTool) Description() string {
	return "Initialize an experiment with a hypothesis. Creates or resumes a campaign session, sets up git worktree if needed."
}

func (t *InitExperimentTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"campaign": {
				"type": "string",
				"description": "Campaign name (matches YAML file)"
			},
			"hypothesis": {
				"type": "string",
				"description": "What you expect this change to achieve"
			}
		},
		"required": ["campaign", "hypothesis"]
	}`)
}

type initParams struct {
	Campaign   string `json:"campaign"`
	Hypothesis string `json:"hypothesis"`
}

type initResult struct {
	CampaignName     string  `json:"campaign_name"`
	Resumed          bool    `json:"resumed"`
	OriginalBaseline float64 `json:"original_baseline"`
	CurrentBest      float64 `json:"current_best"`
	ExperimentCount  int     `json:"experiment_count"`
	WorktreeDir      string  `json:"worktree_dir"`
	Hypothesis       string  `json:"hypothesis"`
}

func (t *InitExperimentTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p initParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}

	if p.Campaign == "" {
		return ToolResult{Content: "campaign name is required", IsError: true}
	}

	// Load campaign YAML
	campaign, err := t.finder.FindCampaign(p.Campaign)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("campaign not found: %v", err), IsError: true}
	}

	// Create or reuse git worktree
	if campaign.Git.UseWorktree() {
		if err := t.wt.CreateWorktree(p.Campaign); err != nil {
			return ToolResult{Content: fmt.Sprintf("worktree setup failed: %v", err), IsError: true}
		}
	}

	// Open or resume segment
	seg, resumed, err := t.store.OpenSegment(campaign, t.sessionID)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("segment open failed: %v", err), IsError: true}
	}

	result := initResult{
		CampaignName:     campaign.Name,
		Resumed:          resumed,
		OriginalBaseline: seg.OriginalBaseline(),
		CurrentBest:      seg.CurrentBest(),
		ExperimentCount:  seg.ExperimentCount(),
		WorktreeDir:      t.wt.WorktreeDir(p.Campaign),
		Hypothesis:       p.Hypothesis,
	}

	data, _ := json.Marshal(result)

	var status string
	if resumed {
		status = fmt.Sprintf("Resumed campaign %q (experiment %d, baseline: %.4f, current best: %.4f). Worktree: %s\nHypothesis: %s",
			campaign.Name, seg.ExperimentCount(), seg.OriginalBaseline(), seg.CurrentBest(), t.wt.WorktreeDir(p.Campaign), p.Hypothesis)
	} else {
		status = fmt.Sprintf("Initialized campaign %q (baseline: %.4f). Worktree: %s\nHypothesis: %s",
			campaign.Name, campaign.Metric.Baseline, t.wt.WorktreeDir(p.Campaign), p.Hypothesis)
	}

	// Include structured JSON for machine consumption
	return ToolResult{Content: status + "\n\n```json\n" + string(data) + "\n```"}
}

// FindCampaignFunc wraps the package-level FindCampaign as a CampaignFinder.
type FindCampaignFunc struct{}

func (f FindCampaignFunc) FindCampaign(name string) (*experiment.Campaign, error) {
	return experiment.FindCampaign(name)
}
