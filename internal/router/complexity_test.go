package router

import (
	"testing"
)

func TestClassifyComplexity(t *testing.T) {
	cc := newComplexityClassifier(&ComplexityConfig{Mode: "shadow"})
	tests := []struct {
		tokens int
		want   int
	}{
		{100, 1},
		{299, 1},
		{300, 2},
		{799, 2},
		{800, 3},
		{1999, 3},
		{2000, 4},
		{3999, 4},
		{4000, 5},
		{10000, 5},
	}
	for _, tt := range tests {
		got := cc.Classify(tt.tokens)
		if got != tt.want {
			t.Errorf("Classify(%d) = C%d, want C%d", tt.tokens, got, tt.want)
		}
	}
}

func TestComplexityShadowMode(t *testing.T) {
	cc := newComplexityClassifier(&ComplexityConfig{Mode: "shadow"})
	model, reason, override := cc.MaybeOverride(ModelSonnet, "phase-default", 100)
	if model != ModelSonnet {
		t.Errorf("shadow mode changed model to %q", model)
	}
	if reason != "phase-default" {
		t.Errorf("shadow mode changed reason to %q", reason)
	}
	if override == nil {
		t.Fatal("shadow mode should still return override info")
	}
	if override.Tier != 1 {
		t.Errorf("override tier = %d, want 1", override.Tier)
	}
	if override.Applied {
		t.Error("shadow mode should not apply override")
	}
}

func TestComplexityEnforcePromote(t *testing.T) {
	cc := newComplexityClassifier(&ComplexityConfig{Mode: "enforce"})
	model, reason, override := cc.MaybeOverride(ModelSonnet, "phase-default", 2500)
	if model != ModelOpus {
		t.Errorf("enforce C4: model = %q, want opus", model)
	}
	if reason != "complexity-promote" {
		t.Errorf("enforce C4: reason = %q, want complexity-promote", reason)
	}
	if !override.Applied {
		t.Error("enforce mode should apply override")
	}
}

func TestComplexityEnforceDemote(t *testing.T) {
	cc := newComplexityClassifier(&ComplexityConfig{Mode: "enforce"})
	model, reason, override := cc.MaybeOverride(ModelSonnet, "phase-default", 100)
	if model != ModelHaiku {
		t.Errorf("enforce C1: model = %q, want haiku", model)
	}
	if reason != "complexity-demote" {
		t.Errorf("enforce C1: reason = %q, want complexity-demote", reason)
	}
	if !override.Applied {
		t.Error("enforce C1 should apply override")
	}
}

func TestComplexityEnforceNoChange(t *testing.T) {
	cc := newComplexityClassifier(&ComplexityConfig{Mode: "enforce"})
	model, reason, _ := cc.MaybeOverride(ModelSonnet, "phase-default", 1000)
	if model != ModelSonnet {
		t.Errorf("enforce C3: model = %q, want sonnet (no change)", model)
	}
	if reason != "phase-default" {
		t.Errorf("enforce C3: reason = %q, want phase-default", reason)
	}
}

func TestComplexityDefaultMode(t *testing.T) {
	cc := newComplexityClassifier(&ComplexityConfig{})
	// Default mode should be shadow
	model, _, override := cc.MaybeOverride(ModelSonnet, "phase-default", 100)
	if model != ModelSonnet {
		t.Errorf("default mode changed model")
	}
	if override.Applied {
		t.Error("default mode should be shadow (not applied)")
	}
}

func TestComplexityC5Promote(t *testing.T) {
	cc := newComplexityClassifier(&ComplexityConfig{Mode: "enforce"})
	model, reason, _ := cc.MaybeOverride(ModelSonnet, "phase-default", 5000)
	if model != ModelOpus {
		t.Errorf("enforce C5: model = %q, want opus", model)
	}
	if reason != "complexity-promote" {
		t.Errorf("enforce C5: reason = %q", reason)
	}
}

func TestComplexityAlreadyOpusNoPromote(t *testing.T) {
	cc := newComplexityClassifier(&ComplexityConfig{Mode: "enforce"})
	// Already opus — C5 should not re-promote (no change)
	model, reason, override := cc.MaybeOverride(ModelOpus, "phase-default", 5000)
	if model != ModelOpus {
		t.Errorf("already opus: model changed to %q", model)
	}
	if reason != "phase-default" {
		t.Errorf("already opus: reason = %q", reason)
	}
	if override.Applied {
		t.Error("already opus: should not apply (already at target)")
	}
}
