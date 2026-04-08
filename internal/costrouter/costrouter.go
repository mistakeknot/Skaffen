// Package costrouter implements cost-optimized model routing for Hassease.
// It implements agentloop.Router and owns the model→provider dispatch map.
package costrouter

import (
	"context"
	"fmt"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/provider"
)

// Config holds cost router settings loaded from YAML.
type Config struct {
	DefaultModel    string            `yaml:"default_model"`
	EscalationModel string            `yaml:"escalation_model"`
	PlanningModel   string            `yaml:"planning_model"`
	ReadModel       string            `yaml:"read_model"`
	MaxTokens       int               `yaml:"max_tokens"`       // budget cap (0 = unlimited)
	ContextWindows  map[string]int    `yaml:"context_windows"`  // model → window size overrides
	Complexity      ComplexityConfig  `yaml:"complexity"`       // proactive escalation thresholds
}

// Backend maps a model prefix to a provider instance.
type Backend struct {
	Prefix   string            // e.g. "glm-", "qwen-", "claude-"
	Provider provider.Provider
}

// CostRouter selects the cheapest adequate model per turn and dispatches
// Stream() calls to the correct provider backend.
//
// Implements: agentloop.Router, agentloop.Emitter (for failure feedback).
type CostRouter struct {
	cfg      Config
	backends []Backend // ordered: first matching prefix wins

	// Budget tracking.
	spent int

	// Failure state from previous turn (set by Emit, read by SelectModel).
	lastFailure agentloop.FailureType

	// Proactive complexity tracking.
	tracker *complexityTracker
}

// New creates a CostRouter with the given config and provider backends.
func New(cfg Config, backends []Backend) *CostRouter {
	return &CostRouter{
		cfg:      cfg,
		backends: backends,
		tracker:  newComplexityTracker(cfg.Complexity),
	}
}

// --- agentloop.Router implementation ---

// SelectModel picks the cheapest model that clears the bar for this task.
func (r *CostRouter) SelectModel(hints agentloop.SelectionHints) (string, string) {
	// Escalation: if previous turn failed, upgrade to a stronger model.
	if r.lastFailure == agentloop.FailToolError || r.lastFailure == agentloop.FailHallucination || r.lastFailure == agentloop.FailSyntaxError {
		r.lastFailure = "" // consume — only escalate once
		model := r.cfg.EscalationModel
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		return model, "escalation-after-failure"
	}

	// Proactive escalation: behavioral signals suggest the task is too complex
	// for the cheap model, even before an explicit failure.
	if escalate, reason := r.tracker.shouldEscalate(); escalate {
		r.tracker.reset() // consume — avoid re-triggering next turn
		model := r.cfg.EscalationModel
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		return model, reason
	}

	// Task-type routing using documented SelectionHints.TaskType values.
	switch hints.TaskType {
	case "analysis":
		// Reviews, planning, architecture — needs strong reasoning.
		model := r.cfg.PlanningModel
		if model == "" {
			model = "claude-opus-4-6"
		}
		return model, "analysis-task"

	case "code":
		// Code edits — route by urgency.
		if hints.Urgency == "batch" || hints.Urgency == "background" {
			model := r.cfg.ReadModel
			if model == "" {
				model = "glm-4-plus"
			}
			return model, "batch-code-cheap"
		}
		// Interactive code → default model.
	}

	// Default: mid-tier model.
	model := r.cfg.DefaultModel
	if model == "" {
		model = "qwen-plus-latest"
	}
	return model, "default"
}

// RecordUsage tracks token spend for budget enforcement.
func (r *CostRouter) RecordUsage(usage provider.Usage) {
	r.spent += usage.InputTokens + usage.OutputTokens
}

// BudgetState returns current budget consumption.
func (r *CostRouter) BudgetState() agentloop.BudgetState {
	max := r.cfg.MaxTokens
	if max == 0 {
		return agentloop.BudgetState{}
	}
	pct := float64(r.spent) / float64(max)
	return agentloop.BudgetState{
		Spent:      r.spent,
		Max:        max,
		Percentage: pct,
	}
}

// ContextWindow returns the context window for the given model.
func (r *CostRouter) ContextWindow(model string) int {
	if w, ok := r.cfg.ContextWindows[model]; ok {
		return w
	}
	// Sensible defaults by model family.
	switch {
	case strings.HasPrefix(model, "claude-"):
		return 200000
	case strings.HasPrefix(model, "glm-"):
		return 128000
	case strings.HasPrefix(model, "qwen-"):
		return 131072
	default:
		return 128000
	}
}

// --- agentloop.Emitter implementation (for failure feedback) ---

// Emit records per-turn evidence. Feeds the complexity tracker with behavioral
// signals and the failure type for reactive escalation.
func (r *CostRouter) Emit(ev agentloop.Evidence) error {
	r.lastFailure = ev.Failure
	r.tracker.observe(ev)
	return nil
}

// CostReport returns per-model token spend breakdown.
func (r *CostRouter) CostReport() []ModelCostReport {
	return r.tracker.costReport()
}

// --- Provider dispatch ---

// Dispatch returns the provider backend for the given model name.
// Matches by prefix (e.g. "glm-4-plus" matches prefix "glm-").
func (r *CostRouter) Dispatch(model string) (provider.Provider, error) {
	for _, b := range r.backends {
		if strings.HasPrefix(model, b.Prefix) {
			return b.Provider, nil
		}
	}
	return nil, fmt.Errorf("costrouter: no backend for model %q", model)
}

// DispatchProvider wraps a CostRouter to satisfy provider.Provider.
// The agentloop takes a single Provider; this adapter routes Stream() calls
// to the correct backend based on config.Model.
type DispatchProvider struct {
	Router *CostRouter
}

// Stream delegates to the backend matching config.Model.
func (d *DispatchProvider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	if config.Model == "" {
		return nil, fmt.Errorf("costrouter: Stream called with empty model")
	}
	backend, err := d.Router.Dispatch(config.Model)
	if err != nil {
		return nil, err
	}
	return backend.Stream(ctx, messages, tools, config)
}

// Name returns "hassease" (the composite provider identity).
func (d *DispatchProvider) Name() string { return "hassease" }
