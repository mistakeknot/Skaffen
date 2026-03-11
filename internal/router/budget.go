package router

import (
	"sync"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// BudgetState reports current budget consumption.
type BudgetState struct {
	Spent      int     `json:"spent"`
	Max        int     `json:"max"`
	Percentage float64 `json:"percentage"`
	Mode       string  `json:"mode"`
}

// BudgetTracker tracks cumulative token usage against a budget.
type BudgetTracker struct {
	maxTokens int
	degradeAt float64
	mode      string // "graceful", "hard-stop", "advisory"
	spent     int
	mu        sync.Mutex
}

func newBudgetTracker(cfg *BudgetConfig) *BudgetTracker {
	mode := cfg.Mode
	if mode == "" {
		mode = "graceful"
	}
	degradeAt := cfg.DegradeAt
	if degradeAt == 0 {
		degradeAt = 0.8
	}
	return &BudgetTracker{
		maxTokens: cfg.MaxTokens,
		degradeAt: degradeAt,
		mode:      mode,
	}
}

// Record adds token consumption from a single turn.
func (bt *BudgetTracker) Record(usage provider.Usage) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.spent += usage.InputTokens + usage.OutputTokens
}

// MaybeDegrade returns a (possibly degraded) model and reason.
// If no degradation is needed, returns the input model and reason unchanged.
func (bt *BudgetTracker) MaybeDegrade(model, reason string) (string, string) {
	bt.mu.Lock()
	pct := float64(bt.spent) / float64(bt.maxTokens)
	mode := bt.mode
	bt.mu.Unlock()

	switch mode {
	case "advisory":
		// Never change model, just track
		return model, reason

	case "hard-stop":
		if pct >= 1.0 {
			return "", "budget-exhausted"
		}
		return model, reason

	default: // "graceful"
		if pct >= 1.0 {
			return ModelHaiku, "budget-exceeded"
		}
		if pct >= bt.degradeAt {
			return ModelHaiku, "budget-degrade"
		}
		return model, reason
	}
}

// State returns the current budget consumption state.
func (bt *BudgetTracker) State() BudgetState {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	pct := 0.0
	if bt.maxTokens > 0 {
		pct = float64(bt.spent) / float64(bt.maxTokens)
	}
	return BudgetState{
		Spent:      bt.spent,
		Max:        bt.maxTokens,
		Percentage: pct,
		Mode:       bt.mode,
	}
}
