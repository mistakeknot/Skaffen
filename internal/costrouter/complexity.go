package costrouter

import (
	"github.com/mistakeknot/Skaffen/internal/agentloop"
)

// Escalation thresholds — tunable per config, with sensible defaults.
const (
	defaultMaxCheapTurns     = 8  // escalate if cheap model uses >N turns
	defaultMaxUniqueFiles    = 4  // escalate if >N unique files touched
	defaultMaxConsecFailures = 2  // escalate after N consecutive failures
)

// ComplexityConfig holds thresholds for proactive escalation.
type ComplexityConfig struct {
	MaxCheapTurns     int `yaml:"max_cheap_turns"`     // 0 = use default
	MaxUniqueFiles    int `yaml:"max_unique_files"`    // 0 = use default
	MaxConsecFailures int `yaml:"max_consec_failures"` // 0 = use default
}

// complexityTracker monitors per-turn behavioral signals and recommends
// proactive model escalation before the cheap model fails.
type complexityTracker struct {
	cfg ComplexityConfig

	// Accumulated state from Evidence emissions.
	cheapTurns     int            // turns spent on non-Claude models
	uniqueFiles    map[string]bool // unique file paths touched (writes/edits only)
	consecFailures int            // consecutive turns with failures
	totalTurns     int            // all turns regardless of model

	// Per-model cost tracking.
	modelTokens map[string]modelCost
}

// modelCost tracks token spend for a single model.
type modelCost struct {
	InputTokens  int
	OutputTokens int
	Turns        int
}

// ModelCostReport is the exported view of per-model cost tracking.
type ModelCostReport struct {
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	Turns        int    `json:"turns"`
}

func newComplexityTracker(cfg ComplexityConfig) *complexityTracker {
	return &complexityTracker{
		cfg:         cfg,
		uniqueFiles: make(map[string]bool),
		modelTokens: make(map[string]modelCost),
	}
}

// observe records signals from a turn's evidence.
func (ct *complexityTracker) observe(ev agentloop.Evidence) {
	ct.totalTurns++

	// Track per-model costs.
	if ev.Model != "" {
		mc := ct.modelTokens[ev.Model]
		mc.InputTokens += ev.TokensIn
		mc.OutputTokens += ev.TokensOut
		mc.Turns++
		ct.modelTokens[ev.Model] = mc
	}

	// Track cheap model turns (non-Claude).
	isClaude := len(ev.Model) >= 7 && ev.Model[:7] == "claude-"
	if !isClaude {
		ct.cheapTurns++
	} else {
		// Reset cheap turn counter when Claude takes over — the problem
		// that caused escalation may be resolved.
		ct.cheapTurns = 0
	}

	// Track unique files from write/edit operations.
	for _, fa := range ev.FileActivity {
		if fa.Operation == "write" || fa.Operation == "edit" {
			ct.uniqueFiles[fa.Path] = true
		}
	}

	// Track consecutive failures.
	if ev.Failure != "" && ev.Failure != agentloop.FailNone {
		ct.consecFailures++
	} else {
		ct.consecFailures = 0
	}
}

// shouldEscalate returns true if behavioral signals indicate the cheap model
// is struggling and a stronger model should be used. Returns the reason.
func (ct *complexityTracker) shouldEscalate() (bool, string) {
	maxTurns := ct.cfg.MaxCheapTurns
	if maxTurns == 0 {
		maxTurns = defaultMaxCheapTurns
	}
	maxFiles := ct.cfg.MaxUniqueFiles
	if maxFiles == 0 {
		maxFiles = defaultMaxUniqueFiles
	}
	maxFail := ct.cfg.MaxConsecFailures
	if maxFail == 0 {
		maxFail = defaultMaxConsecFailures
	}

	if ct.cheapTurns > maxTurns {
		return true, "cheap-turn-limit"
	}

	if len(ct.uniqueFiles) > maxFiles {
		return true, "file-scope-escalation"
	}

	if ct.consecFailures >= maxFail {
		return true, "consecutive-failures"
	}

	return false, ""
}

// reset clears accumulated state. Called when the escalated model completes
// successfully, indicating the complex section is resolved.
func (ct *complexityTracker) reset() {
	ct.cheapTurns = 0
	ct.uniqueFiles = make(map[string]bool)
	ct.consecFailures = 0
}

// costReport returns per-model cost breakdown.
func (ct *complexityTracker) costReport() []ModelCostReport {
	var reports []ModelCostReport
	for model, mc := range ct.modelTokens {
		reports = append(reports, ModelCostReport{
			Model:        model,
			InputTokens:  mc.InputTokens,
			OutputTokens: mc.OutputTokens,
			TotalTokens:  mc.InputTokens + mc.OutputTokens,
			Turns:        mc.Turns,
		})
	}
	return reports
}
