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

// Phase defaults — Opus is the default for all phases.
// Override per-phase via config file, env vars, or runtime SetModelOverride.
var phaseDefaults = map[tool.Phase]string{
	tool.PhaseBrainstorm: ModelOpus,
	tool.PhasePlan:       ModelOpus,
	tool.PhaseBuild:      ModelOpus,
	tool.PhaseReview:     ModelOpus,
	tool.PhaseShip:       ModelOpus,
}

// DefaultRouter selects models based on phase, config overrides, and budget.
type DefaultRouter struct {
	cfg            *Config
	budget         *BudgetTracker
	complexity     *ComplexityClassifier
	inputTokens    int                // set before SelectModel for complexity
	lastOverride   *ComplexityOverride // last complexity result for evidence
	overrides      map[string]string  // phase -> model, from ic route model
	ic             *ICClient
	sessionID      string
	runtimeModel   string // runtime override from /model command (empty = use defaults)
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
		for phase := range phaseDefaults {
			if model := ic.QueryOverride(string(phase)); model != "" {
				r.overrides[string(phase)] = model
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

	// Runtime override (from /model command — above phase default, below config file)
	if r.runtimeModel != "" {
		model = r.runtimeModel
		reason = "runtime-override"
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

	// Record routing decision to Intercore (fire-and-forget)
	if r.ic != nil {
		r.ic.RecordDecision(r.buildDecisionRecord(phase, model, reason))
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

// SetModelOverride sets a runtime model override for all phases.
// Pass an alias ("opus", "sonnet", "haiku") or a full model ID.
// Pass empty string to clear the override and revert to defaults.
func (r *DefaultRouter) SetModelOverride(model string) {
	if model == "" {
		r.runtimeModel = ""
		return
	}
	r.runtimeModel = resolveModelAlias(model)
}

// ModelOverride returns the current runtime model override, or empty string if none.
func (r *DefaultRouter) ModelOverride() string {
	return r.runtimeModel
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

// buildDecisionRecord creates a DecisionRecord from the current routing state.
func (r *DefaultRouter) buildDecisionRecord(phase tool.Phase, model, reason string) DecisionRecord {
	rec := DecisionRecord{
		Agent:     "skaffen",
		Model:     model,
		Rule:      reason,
		Phase:     string(phase),
		SessionID: r.sessionID,
	}
	if r.lastOverride != nil {
		rec.Complexity = r.lastOverride.Tier
	}
	return rec
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
