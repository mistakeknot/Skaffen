package tool

import (
	"github.com/mistakeknot/Skaffen/internal/experiment"
)

// Worktree provides git worktree operations for experiment tools.
// Matches the methods on experiment.GitOps but allows test substitution.
type Worktree interface {
	CreateWorktree(campaignName string) error
	KeepChanges(campaignName, hypothesis string, delta float64) (string, error)
	DiscardChanges(campaignName string) error
	HasWorktree(campaignName string) bool
	WorktreeDir(campaignName string) string
	CurrentSHA(campaignName string) (string, error)
	RemoveWorktree(campaignName string) error
}

// ExperimentStore provides campaign loading and segment management.
// Matches the methods on experiment.Store but allows test substitution.
type ExperimentStore interface {
	OpenSegment(campaign *experiment.Campaign, sessionID string) (*experiment.Segment, bool, error)
	LoadSegment(campaignName string) (*experiment.Segment, error)
}

// CampaignFinder locates campaign YAML files by name.
type CampaignFinder interface {
	FindCampaign(name string) (*experiment.Campaign, error)
}

// ExperimentStatusCallback sends experiment state updates to the TUI.
type ExperimentStatusCallback func(experiment.ExperimentStatus)
