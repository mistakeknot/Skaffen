package router

import "github.com/mistakeknot/Skaffen/internal/tool"

// ShadowProfile defines an alternative model routing configuration
// that is evaluated but not applied. Results are logged for comparison.
type ShadowProfile struct {
	Name   string                // e.g., "economy", "balanced", "premium"
	Phases map[tool.Phase]string // phase → model mapping
}

// ShadowResult records what a shadow profile would have selected.
type ShadowResult struct {
	Profile string `json:"shadow_profile"`
	Model   string `json:"shadow_model"`
	Phase   string `json:"phase"`
	Actual  string `json:"actual_model"`
}

// DefaultShadowProfiles returns the built-in experiment profiles.
func DefaultShadowProfiles() []ShadowProfile {
	return []ShadowProfile{
		{
			Name: "economy",
			Phases: map[tool.Phase]string{
				tool.PhaseObserve:  ModelHaiku,
				tool.PhaseOrient:   ModelSonnet,
				tool.PhaseDecide:   ModelSonnet,
				tool.PhaseAct:      ModelSonnet,
				tool.PhaseReflect:  ModelHaiku,
				tool.PhaseCompound: ModelHaiku,
			},
		},
		{
			Name: "balanced",
			Phases: map[tool.Phase]string{
				tool.PhaseObserve:  ModelSonnet,
				tool.PhaseOrient:   ModelOpus,
				tool.PhaseDecide:   ModelOpus,
				tool.PhaseAct:      ModelSonnet,
				tool.PhaseReflect:  ModelSonnet,
				tool.PhaseCompound: ModelSonnet,
			},
		},
	}
}

// shadowProfiles holds the active shadow experiment profiles.
// nil means shadow experiments are disabled.
func (r *DefaultRouter) shadowProfiles() []ShadowProfile {
	return r.shadows
}

// SetShadowProfiles configures shadow experiment profiles.
// Pass nil to disable shadow experiments.
func (r *DefaultRouter) SetShadowProfiles(profiles []ShadowProfile) {
	r.shadows = profiles
}

// EvaluateShadows computes what each shadow profile would select for the
// given phase and returns the results. Called internally by SelectModel
// when shadow profiles are configured.
func (r *DefaultRouter) EvaluateShadows(phase tool.Phase, actualModel string) []ShadowResult {
	if len(r.shadows) == 0 {
		return nil
	}
	results := make([]ShadowResult, 0, len(r.shadows))
	for _, sp := range r.shadows {
		shadowModel := sp.Phases[phase]
		if shadowModel == "" {
			shadowModel = ModelSonnet // default fallback for unconfigured phases
		}
		results = append(results, ShadowResult{
			Profile: sp.Name,
			Model:   shadowModel,
			Phase:   string(phase),
			Actual:  actualModel,
		})
	}
	return results
}
