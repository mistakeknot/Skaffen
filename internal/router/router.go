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
	cfg          *Config
	budget       *BudgetTracker
	complexity   *ComplexityClassifier
	inputTokens  int                // set before SelectModel for complexity
	lastOverride *ComplexityOverride // last complexity result for evidence
	overrides    map[string]string  // phase -> model, from ic route model
	ic           *ICClient
	sessionID    string
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

// NewWithIC creates a DefaultRouter with Intercore integration.
// Overrides are queried once at construction and cached for the session.
func NewWithIC(cfg *Config, ic *ICClient, sessionID string) *DefaultRouter {
	r := New(cfg)
	r.ic = ic
	r.sessionID = sessionID
	if ic != nil {
		r.overrides = make(map[string]string)
		for _, phase := range []string{"brainstorm", "plan", "build", "review", "ship"} {
			if model := ic.QueryOverride(phase); model != "" {
				r.overrides[phase] = model
			}
		}
	}
	return r
}

// SelectModel returns the model and reason for the given phase.
// Resolution order: budget degradation > complexity > env var > intercore override > config file > phase default.
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

	// Intercore override (above config, below env)
	if r.overrides != nil {
		if m, ok := r.overrides[string(phase)]; ok && m != "" {
			model = resolveModelAlias(m)
			reason = "intercore-override"
		}
	}

	// Env var override (highest explicit priority)
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

// Default context window sizes per model (tokens).
var defaultContextWindows = map[string]int{
	ModelOpus:   200000,
	ModelSonnet: 200000,
	ModelHaiku:  200000,
}

// ContextWindow returns the context window size for the given model.
// Accepts both aliases ("opus") and canonical IDs ("claude-opus-4-6").
func (r *DefaultRouter) ContextWindow(model string) int {
	canonical := resolveModelAlias(model)
	if r.cfg.ContextWindows != nil {
		if w, ok := r.cfg.ContextWindows[canonical]; ok {
			return w
		}
	}
	if w, ok := defaultContextWindows[canonical]; ok {
		return w
	}
	return 200000 // safe default
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
