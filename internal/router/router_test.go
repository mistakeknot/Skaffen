package router

import (
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

func TestPhaseDefaults(t *testing.T) {
	r := New(nil) // nil config = use hardcoded defaults
	tests := []struct {
		phase tool.Phase
		want  string
	}{
		{tool.PhaseBrainstorm, ModelOpus},
		{tool.PhasePlan, ModelSonnet},
		{tool.PhaseBuild, ModelSonnet},
		{tool.PhaseReview, ModelSonnet},
		{tool.PhaseShip, ModelSonnet},
	}
	for _, tt := range tests {
		model, reason := r.SelectModel(tt.phase)
		if model != tt.want {
			t.Errorf("SelectModel(%s) = %q, want %q", tt.phase, model, tt.want)
		}
		if reason == "" {
			t.Errorf("SelectModel(%s) returned empty reason", tt.phase)
		}
	}
}

func TestFallbackChain(t *testing.T) {
	r := New(nil)
	chain := r.FallbackChain()
	if len(chain) != 3 {
		t.Fatalf("fallback chain length = %d, want 3", len(chain))
	}
	if chain[0] != ModelOpus {
		t.Errorf("chain[0] = %q, want %q", chain[0], ModelOpus)
	}
	if chain[1] != ModelSonnet {
		t.Errorf("chain[1] = %q, want %q", chain[1], ModelSonnet)
	}
	if chain[2] != ModelHaiku {
		t.Errorf("chain[2] = %q, want %q", chain[2], ModelHaiku)
	}
}

func TestReasonStrings(t *testing.T) {
	r := New(nil)
	_, reason := r.SelectModel(tool.PhaseBrainstorm)
	if reason != "phase-default" {
		t.Errorf("brainstorm reason = %q, want %q", reason, "phase-default")
	}
}

func TestRecordUsageNoOp(t *testing.T) {
	r := New(nil)
	// Should not panic with no budget set
	r.RecordUsage(provider.Usage{InputTokens: 100, OutputTokens: 50})
}

func TestBudgetStateNoBudget(t *testing.T) {
	r := New(nil)
	spent, max, pct := r.BudgetState()
	if spent != 0 || max != 0 || pct != 0 {
		t.Errorf("no budget: got (%d, %d, %f)", spent, max, pct)
	}
}

func TestRouterWithComplexityShadow(t *testing.T) {
	cfg := &Config{
		Phases:     map[tool.Phase]string{},
		Complexity: &ComplexityConfig{Mode: "shadow"},
	}
	r := New(cfg)
	model, reason := r.SelectModel(tool.PhaseBuild)
	if model != ModelSonnet {
		t.Errorf("shadow complexity changed model to %q", model)
	}
	if reason != "phase-default" {
		t.Errorf("shadow complexity changed reason to %q", reason)
	}
}

func TestRouterWithComplexityEnforce(t *testing.T) {
	cfg := &Config{
		Phases:     map[tool.Phase]string{},
		Complexity: &ComplexityConfig{Mode: "enforce"},
	}
	r := New(cfg)
	r.SetInputTokens(5000) // C5 -> promote to opus
	model, reason := r.SelectModel(tool.PhaseBuild)
	if model != ModelOpus {
		t.Errorf("enforce C5: model = %q, want opus", model)
	}
	if reason != "complexity-promote" {
		t.Errorf("enforce C5: reason = %q, want complexity-promote", reason)
	}
}

func TestRouterLastOverride(t *testing.T) {
	cfg := &Config{
		Phases:     map[tool.Phase]string{},
		Complexity: &ComplexityConfig{Mode: "shadow"},
	}
	r := New(cfg)
	r.SetInputTokens(100)
	r.SelectModel(tool.PhaseBuild)
	override := r.LastComplexityOverride()
	if override == nil {
		t.Fatal("expected complexity override info")
	}
	if override.Tier != 1 {
		t.Errorf("tier = %d, want 1", override.Tier)
	}
	if override.Applied {
		t.Error("shadow mode should not apply")
	}
}

func TestResolveModelAlias(t *testing.T) {
	tests := []struct {
		alias string
		want  string
	}{
		{"opus", ModelOpus},
		{"sonnet", ModelSonnet},
		{"haiku", ModelHaiku},
		{"claude-opus-4-6", "claude-opus-4-6"},
		{"custom-model-v1", "custom-model-v1"},
	}
	for _, tt := range tests {
		got := resolveModelAlias(tt.alias)
		if got != tt.want {
			t.Errorf("resolveModelAlias(%q) = %q, want %q", tt.alias, got, tt.want)
		}
	}
}
