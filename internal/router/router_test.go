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
		{tool.PhaseOrient, ModelOpus},
		{tool.PhaseDecide, ModelOpus},
		{tool.PhaseAct, ModelOpus},
		{tool.PhaseReflect, ModelOpus},
		{tool.PhaseCompound, ModelOpus},
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
	_, reason := r.SelectModel(tool.PhaseOrient)
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
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("shadow complexity changed model to %q", model)
	}
	if reason != "phase-default" {
		t.Errorf("shadow complexity changed reason to %q", reason)
	}
}

func TestRouterWithComplexityEnforcePromoteNoop(t *testing.T) {
	// C5 promote is a no-op when default is already Opus
	cfg := &Config{
		Phases:     map[tool.Phase]string{},
		Complexity: &ComplexityConfig{Mode: "enforce"},
	}
	r := New(cfg)
	r.SetInputTokens(5000) // C5 — but already opus, nothing to promote
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("enforce C5: model = %q, want opus", model)
	}
	if reason != "phase-default" {
		t.Errorf("enforce C5: reason = %q, want phase-default (already opus)", reason)
	}
}

func TestRouterWithComplexityEnforceDemote(t *testing.T) {
	// C1 demotes opus → haiku in enforce mode
	cfg := &Config{
		Phases:     map[tool.Phase]string{},
		Complexity: &ComplexityConfig{Mode: "enforce"},
	}
	r := New(cfg)
	r.SetInputTokens(100) // C1 -> demote to haiku
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelHaiku {
		t.Errorf("enforce C1: model = %q, want haiku", model)
	}
	if reason != "complexity-demote" {
		t.Errorf("enforce C1: reason = %q, want complexity-demote", reason)
	}
}

func TestRouterWithComplexityEnforcePromoteFromSonnet(t *testing.T) {
	// C5 promotes when runtime override has set model to sonnet
	cfg := &Config{
		Phases:     map[tool.Phase]string{},
		Complexity: &ComplexityConfig{Mode: "enforce"},
	}
	r := New(cfg)
	r.SetModelOverride("sonnet")
	r.SetInputTokens(5000) // C5 -> promote sonnet to opus
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("enforce C5 from sonnet: model = %q, want opus", model)
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
	r.SelectModel(tool.PhaseAct)
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
	r.overrides = map[string]string{"act": ModelOpus}
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("override: model = %q, want opus", model)
	}
	if reason != "intercore-override" {
		t.Errorf("override: reason = %q, want intercore-override", reason)
	}
}

func TestOverrideNotAppliedForOtherPhase(t *testing.T) {
	r := New(&Config{})
	r.overrides = map[string]string{"act": ModelOpus}
	_, reason := r.SelectModel(tool.PhaseOrient)
	if reason == "intercore-override" {
		t.Error("override should not apply to orient")
	}
}

func TestEnvOverrideBeatsIntercoreOverride(t *testing.T) {
	cfg := &Config{}
	r := New(cfg)
	r.overrides = map[string]string{"act": ModelHaiku}
	t.Setenv("SKAFFEN_MODEL_ACT", "opus")
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("env should beat intercore: model = %q, want opus", model)
	}
	if reason != "env-override" {
		t.Errorf("reason = %q, want env-override", reason)
	}
}

func TestNewWithIC_NilIC(t *testing.T) {
	r := NewWithIC(&Config{}, nil, "test-session")
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("nil IC: model = %q, want opus", model)
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

func TestBuildDecisionRecord(t *testing.T) {
	r := New(&Config{})
	r.sessionID = "test-session"
	rec := r.buildDecisionRecord(tool.PhaseAct, "claude-sonnet-4-6", "phase-default")
	if rec.Agent != "skaffen" {
		t.Errorf("agent = %q, want skaffen", rec.Agent)
	}
	if rec.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q", rec.Model)
	}
	if rec.Rule != "phase-default" {
		t.Errorf("rule = %q", rec.Rule)
	}
	if rec.Phase != "act" {
		t.Errorf("phase = %q", rec.Phase)
	}
	if rec.SessionID != "test-session" {
		t.Errorf("session = %q", rec.SessionID)
	}
}

func TestSetModelOverride(t *testing.T) {
	r := New(nil)

	// Default: all phases return opus
	model, _ := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("default build = %q, want opus", model)
	}

	// Set runtime override to sonnet
	r.SetModelOverride("sonnet")
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelSonnet {
		t.Errorf("after override: model = %q, want sonnet", model)
	}
	if reason != "runtime-override" {
		t.Errorf("after override: reason = %q, want runtime-override", reason)
	}

	// Override applies to all phases
	model, _ = r.SelectModel(tool.PhaseOrient)
	if model != ModelSonnet {
		t.Errorf("orient after override = %q, want sonnet", model)
	}

	// Clear override
	r.SetModelOverride("")
	model, reason = r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("after clear: model = %q, want opus", model)
	}
	if reason != "phase-default" {
		t.Errorf("after clear: reason = %q, want phase-default", reason)
	}
}

func TestModelOverride(t *testing.T) {
	r := New(nil)
	if o := r.ModelOverride(); o != "" {
		t.Errorf("initial override = %q, want empty", o)
	}
	r.SetModelOverride("haiku")
	if o := r.ModelOverride(); o != ModelHaiku {
		t.Errorf("after set: override = %q, want %q", o, ModelHaiku)
	}
	r.SetModelOverride("")
	if o := r.ModelOverride(); o != "" {
		t.Errorf("after clear: override = %q, want empty", o)
	}
}

func TestRuntimeOverrideBelowConfigFile(t *testing.T) {
	cfg := &Config{
		Phases: map[tool.Phase]string{
			tool.PhaseAct: "haiku",
		},
	}
	r := New(cfg)
	r.SetModelOverride("sonnet")

	// Config file beats runtime override
	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelHaiku {
		t.Errorf("config should beat runtime: model = %q, want haiku", model)
	}
	if reason != "config-file" {
		t.Errorf("reason = %q, want config-file", reason)
	}

	// But runtime override works for phases without config override
	model, reason = r.SelectModel(tool.PhaseDecide)
	if model != ModelSonnet {
		t.Errorf("decide should use runtime override: model = %q, want sonnet", model)
	}
	if reason != "runtime-override" {
		t.Errorf("reason = %q, want runtime-override", reason)
	}
}

func TestBuildDecisionRecordWithComplexity(t *testing.T) {
	r := New(&Config{
		Complexity: &ComplexityConfig{Mode: "shadow"},
	})
	r.sessionID = "test-session"
	r.SetInputTokens(5000)       // Will classify as C5
	r.SelectModel(tool.PhaseAct) // Populates lastOverride
	rec := r.buildDecisionRecord(tool.PhaseAct, ModelSonnet, "phase-default")
	if rec.Complexity != 5 {
		t.Errorf("complexity = %d, want 5", rec.Complexity)
	}
}

func TestHardwareConstrainedDowngradesOpus(t *testing.T) {
	r := New(nil)
	r.SetHardwareProfile(HardwareProfile{CPUCores: 1, MemoryMB: 2048, Tier: TierConstrained})

	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelSonnet {
		t.Errorf("constrained hardware: model = %q, want sonnet", model)
	}
	if reason != "hardware-constrained" {
		t.Errorf("constrained hardware: reason = %q, want hardware-constrained", reason)
	}
}

func TestHardwareStandardKeepsOpus(t *testing.T) {
	r := New(nil)
	r.SetHardwareProfile(HardwareProfile{CPUCores: 4, MemoryMB: 8192, Tier: TierStandard})

	model, reason := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Errorf("standard hardware: model = %q, want opus", model)
	}
	if reason != "phase-default" {
		t.Errorf("standard hardware: reason = %q, want phase-default", reason)
	}
}

func TestHardwareConstrainedNoEffectOnSonnet(t *testing.T) {
	r := New(nil)
	r.SetHardwareProfile(HardwareProfile{CPUCores: 1, MemoryMB: 2048, Tier: TierConstrained})
	r.SetModelOverride("sonnet") // already sonnet — hardware should not further downgrade

	model, _ := r.SelectModel(tool.PhaseAct)
	if model != ModelSonnet {
		t.Errorf("constrained + sonnet override: model = %q, want sonnet", model)
	}
}

func TestHardwareInfoNil(t *testing.T) {
	r := New(nil)
	if r.HardwareInfo() != nil {
		t.Error("expected nil hardware info before SetHardwareProfile")
	}
}

func TestHardwareInfoSet(t *testing.T) {
	r := New(nil)
	r.SetHardwareProfile(HardwareProfile{CPUCores: 8, MemoryMB: 32768, Tier: TierCapable})
	p := r.HardwareInfo()
	if p == nil {
		t.Fatal("expected non-nil hardware info")
	}
	if p.CPUCores != 8 || p.MemoryMB != 32768 || p.Tier != TierCapable {
		t.Errorf("hardware = %+v, want 8 cores / 32768 MB / capable", *p)
	}
}
