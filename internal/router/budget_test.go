package router

import (
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

func TestBudgetGracefulUnderThreshold(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8,
	})
	model, reason := bt.MaybeDegrade(ModelOpus, "phase-default")
	if model != ModelOpus {
		t.Errorf("at 0%%: model = %q, want %q", model, ModelOpus)
	}
	if reason != "phase-default" {
		t.Errorf("at 0%%: reason = %q, want phase-default", reason)
	}
}

func TestBudgetGracefulAtThreshold(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8,
	})
	bt.Record(provider.Usage{InputTokens: 500, OutputTokens: 300})
	model, reason := bt.MaybeDegrade(ModelOpus, "phase-default")
	if model != ModelHaiku {
		t.Errorf("at 80%%: model = %q, want %q", model, ModelHaiku)
	}
	if reason != "budget-degrade" {
		t.Errorf("at 80%%: reason = %q, want budget-degrade", reason)
	}
}

func TestBudgetGracefulOverBudget(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8,
	})
	bt.Record(provider.Usage{InputTokens: 700, OutputTokens: 400})
	model, reason := bt.MaybeDegrade(ModelOpus, "phase-default")
	if model != ModelHaiku {
		t.Errorf("at 110%%: model = %q, want %q", model, ModelHaiku)
	}
	if reason != "budget-exceeded" {
		t.Errorf("at 110%%: reason = %q, want budget-exceeded", reason)
	}
}

func TestBudgetHardStop(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "hard-stop", DegradeAt: 0.8,
	})
	bt.Record(provider.Usage{InputTokens: 600, OutputTokens: 500})
	model, reason := bt.MaybeDegrade(ModelOpus, "phase-default")
	if model != "" {
		t.Errorf("hard-stop over budget: model = %q, want empty", model)
	}
	if reason != "budget-exhausted" {
		t.Errorf("hard-stop: reason = %q, want budget-exhausted", reason)
	}
}

func TestBudgetAdvisory(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "advisory", DegradeAt: 0.8,
	})
	bt.Record(provider.Usage{InputTokens: 600, OutputTokens: 500})
	model, reason := bt.MaybeDegrade(ModelOpus, "phase-default")
	if model != ModelOpus {
		t.Errorf("advisory: model = %q, want %q", model, ModelOpus)
	}
	if reason != "phase-default" {
		t.Errorf("advisory: reason = %q, want phase-default", reason)
	}
}

func TestBudgetState(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8,
	})
	bt.Record(provider.Usage{InputTokens: 300, OutputTokens: 200})
	state := bt.State()
	if state.Spent != 500 {
		t.Errorf("spent = %d, want 500", state.Spent)
	}
	if state.Max != 1000 {
		t.Errorf("max = %d, want 1000", state.Max)
	}
	if state.Percentage < 0.49 || state.Percentage > 0.51 {
		t.Errorf("percentage = %f, want ~0.5", state.Percentage)
	}
}

func TestBudgetCumulativeRecording(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8,
	})
	bt.Record(provider.Usage{InputTokens: 200, OutputTokens: 100})
	bt.Record(provider.Usage{InputTokens: 200, OutputTokens: 100})
	bt.Record(provider.Usage{InputTokens: 200, OutputTokens: 100})
	state := bt.State()
	if state.Spent != 900 {
		t.Errorf("cumulative spent = %d, want 900", state.Spent)
	}
}

func TestBudgetContextTracking(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8, Tracking: "context",
	})
	// In context mode, cache tokens count toward budget
	bt.Record(provider.Usage{
		InputTokens:              100,
		OutputTokens:             100,
		CacheCreationInputTokens: 200,
		CacheReadInputTokens:     300,
	})
	state := bt.State()
	// 100 + 100 + 200 + 300 = 700
	if state.Spent != 700 {
		t.Errorf("context tracking: spent = %d, want 700", state.Spent)
	}
	if state.Tracking != "context" {
		t.Errorf("tracking = %q, want context", state.Tracking)
	}
}

func TestBudgetBillingTracking_IgnoresCache(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8, Tracking: "billing",
	})
	bt.Record(provider.Usage{
		InputTokens:              100,
		OutputTokens:             100,
		CacheCreationInputTokens: 200,
		CacheReadInputTokens:     300,
	})
	state := bt.State()
	// billing mode: only 100 + 100 = 200 (cache tokens ignored)
	if state.Spent != 200 {
		t.Errorf("billing tracking: spent = %d, want 200", state.Spent)
	}
	if state.Tracking != "billing" {
		t.Errorf("tracking = %q, want billing", state.Tracking)
	}
}

func TestBudgetDefaultTracking_IsBilling(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8,
	})
	// No Tracking field set — should default to billing
	bt.Record(provider.Usage{
		InputTokens:              100,
		OutputTokens:             100,
		CacheCreationInputTokens: 500,
		CacheReadInputTokens:     500,
	})
	state := bt.State()
	// Default billing: 100 + 100 = 200, NOT 1200
	if state.Spent != 200 {
		t.Errorf("default tracking: spent = %d, want 200", state.Spent)
	}
	if state.Tracking != "billing" {
		t.Errorf("default tracking mode = %q, want billing", state.Tracking)
	}
}

func TestBudgetContextTracking_DegradeAtThreshold(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8, Tracking: "context",
	})
	// Push to 80% via cache tokens: 100+100+200+400 = 800
	bt.Record(provider.Usage{
		InputTokens:              100,
		OutputTokens:             100,
		CacheCreationInputTokens: 200,
		CacheReadInputTokens:     400,
	})
	model, reason := bt.MaybeDegrade(ModelOpus, "phase-default")
	if model != ModelHaiku {
		t.Errorf("context at 80%%: model = %q, want %q", model, ModelHaiku)
	}
	if reason != "budget-degrade" {
		t.Errorf("context at 80%%: reason = %q, want budget-degrade", reason)
	}
}

func TestBudgetHardStopUnderBudget(t *testing.T) {
	bt := newBudgetTracker(&BudgetConfig{
		MaxTokens: 1000, Mode: "hard-stop", DegradeAt: 0.8,
	})
	bt.Record(provider.Usage{InputTokens: 300, OutputTokens: 200})
	model, reason := bt.MaybeDegrade(ModelOpus, "phase-default")
	if model != ModelOpus {
		t.Errorf("hard-stop under budget: model = %q, want opus", model)
	}
	if reason != "phase-default" {
		t.Errorf("hard-stop under budget: reason = %q, want phase-default", reason)
	}
}
