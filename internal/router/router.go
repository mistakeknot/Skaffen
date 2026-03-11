package router

import (
	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// Canonical model IDs.
const (
	ModelOpus   = "claude-opus-4-6"
	ModelSonnet = "claude-sonnet-4-6"
	ModelHaiku  = "claude-haiku-4-5-20251001"
)

// Hardcoded fallback chain: opus -> sonnet -> haiku.
var fallbackChain = []string{ModelOpus, ModelSonnet, ModelHaiku}

// Phase defaults from Clavain's economy routing table.
// brainstorm=opus (creative exploration needs heavy reasoning),
// all other phases=sonnet (proven sufficient for plan/build/review/ship).
var phaseDefaults = map[tool.Phase]string{
	tool.PhaseBrainstorm: ModelOpus,
	tool.PhasePlan:       ModelSonnet,
	tool.PhaseBuild:      ModelSonnet,
	tool.PhaseReview:     ModelSonnet,
	tool.PhaseShip:       ModelSonnet,
}

// DefaultRouter selects models based on phase, config overrides, and budget.
type DefaultRouter struct {
	cfg         *Config
	budget      *BudgetTracker
	complexity  *ComplexityClassifier
	inputTokens int                // set before SelectModel for complexity
	lastOverride *ComplexityOverride // last complexity result for evidence
}

// New creates a DefaultRouter. Pass nil config to use hardcoded defaults.
func New(cfg *Config) *DefaultRouter {
	if cfg == nil {
		cfg = &Config{}
	}
	r := &DefaultRouter{cfg: cfg}
	if cfg.Budget != nil && cfg.Budget.MaxTokens > 0 {
		r.budget = newBudgetTracker(cfg.Budget)
	}
	if cfg.Complexity != nil {
		r.complexity = newComplexityClassifier(cfg.Complexity)
	}
	return r
}

// SelectModel returns the model and reason for the given phase.
// Resolution order: budget degradation > complexity > env var > config file > phase default.
func (r *DefaultRouter) SelectModel(phase tool.Phase) (string, string) {
	// Start with phase default
	model := phaseDefaults[phase]
	reason := "phase-default"
	if model == "" {
		model = ModelSonnet
		reason = "fallback-default"
	}

	// Config file override
	if m, ok := r.cfg.Phases[phase]; ok && m != "" {
		model = resolveModelAlias(m)
		reason = "config-file"
	}

	// Env var override (highest priority for explicit user control)
	if m := r.cfg.envOverride(phase); m != "" {
		model = resolveModelAlias(m)
		reason = "env-override"
	}

	// Complexity override (shadow logs but doesn't change; enforce applies)
	r.lastOverride = nil
	if r.complexity != nil {
		model, reason, r.lastOverride = r.complexity.MaybeOverride(model, reason, r.inputTokens)
	}

	// Budget degradation (overrides everything when budget is exhausted)
	if r.budget != nil {
		model, reason = r.budget.MaybeDegrade(model, reason)
	}

	return model, reason
}

// RecordUsage feeds the budget tracker with token consumption.
func (r *DefaultRouter) RecordUsage(usage provider.Usage) {
	if r.budget != nil {
		r.budget.Record(usage)
	}
}

// BudgetState returns current budget consumption as (spent, max, percentage).
func (r *DefaultRouter) BudgetState() (int, int, float64) {
	if r.budget == nil {
		return 0, 0, 0
	}
	s := r.budget.State()
	return s.Spent, s.Max, s.Percentage
}

// FallbackChain returns the hardcoded model fallback chain.
func (r *DefaultRouter) FallbackChain() []string {
	return fallbackChain
}

// SetInputTokens sets the current turn's input token count for complexity classification.
func (r *DefaultRouter) SetInputTokens(n int) {
	r.inputTokens = n
}

// LastComplexityOverride returns the complexity override from the last SelectModel call.
func (r *DefaultRouter) LastComplexityOverride() *ComplexityOverride {
	return r.lastOverride
}

// resolveModelAlias converts short aliases to canonical model IDs.
func resolveModelAlias(alias string) string {
	switch alias {
	case "opus":
		return ModelOpus
	case "sonnet":
		return ModelSonnet
	case "haiku":
		return ModelHaiku
	default:
		return alias // assume it's already a full model ID
	}
}
