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

func TestContextWindowCanonicalID(t *testing.T) {
	r := New(nil)
	tests := []struct {
		model string
		want  int
	}{
		{ModelOpus, 200000},
		{ModelSonnet, 200000},
		{ModelHaiku, 200000},
	}
	for _, tt := range tests {
		if w := r.ContextWindow(tt.model); w != tt.want {
			t.Errorf("ContextWindow(%q) = %d, want %d", tt.model, w, tt.want)
		}
	}
}

func TestContextWindowAlias(t *testing.T) {
	r := New(nil)
	for _, alias := range []string{"opus", "sonnet", "haiku"} {
		if w := r.ContextWindow(alias); w != 200000 {
			t.Errorf("ContextWindow(%q) = %d, want 200000", alias, w)
		}
	}
}

func TestContextWindowUnknownModel(t *testing.T) {
	r := New(nil)
	if w := r.ContextWindow("unknown-model-xyz"); w != 200000 {
		t.Errorf("ContextWindow(unknown) = %d, want 200000", w)
	}
}

func TestContextWindowConfigOverride(t *testing.T) {
	cfg := &Config{
		ContextWindows: map[string]int{
			ModelOpus: 150000,
		},
	}
	r := New(cfg)
	if w := r.ContextWindow("opus"); w != 150000 {
		t.Errorf("ContextWindow(opus with override) = %d, want 150000", w)
	}
	// Sonnet should still use default
	if w := r.ContextWindow("sonnet"); w != 200000 {
		t.Errorf("ContextWindow(sonnet no override) = %d, want 200000", w)
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

func TestOverrideApplied(t *testing.T) {
	r := New(&Config{})
	r.overrides = map[string]string{"build": ModelOpus}
	model, reason := r.SelectModel(tool.PhaseBuild)
	if model != ModelOpus {
		t.Errorf("override: model = %q, want opus", model)
	}
	if reason != "intercore-override" {
		t.Errorf("override: reason = %q, want intercore-override", reason)
	}
}

func TestOverrideNotAppliedForOtherPhase(t *testing.T) {
	r := New(&Config{})
	r.overrides = map[string]string{"build": ModelOpus}
	_, reason := r.SelectModel(tool.PhaseBrainstorm)
	if reason == "intercore-override" {
		t.Error("override should not apply to brainstorm")
	}
}

func TestEnvOverrideBeatsIntercoreOverride(t *testing.T) {
	cfg := &Config{}
	r := New(cfg)
	r.overrides = map[string]string{"build": ModelHaiku}
	t.Setenv("SKAFFEN_MODEL_BUILD", "opus")
	model, reason := r.SelectModel(tool.PhaseBuild)
	if model != ModelOpus {
		t.Errorf("env should beat intercore: model = %q, want opus", model)
	}
	if reason != "env-override" {
		t.Errorf("reason = %q, want env-override", reason)
	}
}

func TestNewWithIC_NilIC(t *testing.T) {
	r := NewWithIC(&Config{}, nil, "test-session")
	model, reason := r.SelectModel(tool.PhaseBuild)
	if model != ModelSonnet {
		t.Errorf("nil IC: model = %q, want sonnet", model)
	}
	if reason != "phase-default" {
		t.Errorf("nil IC: reason = %q, want phase-default", reason)
	}
}

func TestNewWithIC_SessionID(t *testing.T) {
	r := NewWithIC(&Config{}, nil, "my-session")
	if r.sessionID != "my-session" {
		t.Errorf("sessionID = %q, want my-session", r.sessionID)
	}
}
