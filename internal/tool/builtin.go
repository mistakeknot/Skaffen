package tool

import (
	"os/exec"

	"github.com/mistakeknot/Skaffen/internal/sandbox"
)

// RegisterBuiltins adds all built-in tools to the registry.
func RegisterBuiltins(r *Registry) {
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&EditTool{})
	r.Register(&BashTool{})
	r.Register(&GrepTool{})
	r.Register(&GlobTool{})
	r.Register(&LsTool{})

	// Web tools — gated to brainstorm, plan, and build phases
	webPhases := []Phase{PhaseOrient, PhaseDecide, PhaseAct}
	r.RegisterForPhases(NewWebSearchTool(), webPhases)
	r.RegisterForPhases(NewWebFetchTool(), webPhases)
}

// RegisterQualityHistory adds the quality_history tool gated to Orient phase.
// Called separately because it requires a SignalReader from the mutations store.
func RegisterQualityHistory(r *Registry, store SignalReader) {
	r.RegisterForPhases(NewQualityHistoryTool(store), []Phase{PhaseOrient})
}

// RegisterExperimentTools adds the three experiment tools gated to Act phase.
// run_experiment gets RequirePrompt for sandbox safety (benchmark command confirmation).
// log_experiment is also available in Reflect phase.
func RegisterExperimentTools(r *Registry, store ExperimentStore, finder CampaignFinder, wt Worktree, sb *sandbox.Sandbox, sessionID string, statusCB ExperimentStatusCallback) {
	icPath, _ := exec.LookPath("ic")

	expPhases := []Phase{PhaseAct}
	r.RegisterForPhases(NewInitExperimentTool(store, finder, wt, sessionID), expPhases)

	// run_experiment requires user confirmation before first benchmark execution
	r.RegisterForPhasesWithConstraint(
		NewRunExperimentTool(store, finder, wt, sb),
		expPhases,
		&GateConstraint{RequirePrompt: true},
	)

	// log_experiment available in both Act and Reflect
	r.RegisterForPhases(
		NewLogExperimentTool(store, finder, wt, icPath, statusCB),
		[]Phase{PhaseAct, PhaseReflect},
	)
}
