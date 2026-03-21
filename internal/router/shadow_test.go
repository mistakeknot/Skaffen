package router

import (
	"testing"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

func TestDefaultShadowProfiles(t *testing.T) {
	profiles := DefaultShadowProfiles()
	if len(profiles) != 2 {
		t.Fatalf("expected 2 default profiles, got %d", len(profiles))
	}
	if profiles[0].Name != "economy" {
		t.Errorf("profile[0] = %q, want economy", profiles[0].Name)
	}
	if profiles[1].Name != "balanced" {
		t.Errorf("profile[1] = %q, want balanced", profiles[1].Name)
	}
}

func TestShadowExperimentResults(t *testing.T) {
	r := New(nil)
	r.SetShadowProfiles(DefaultShadowProfiles())

	model, _ := r.SelectModel(tool.PhaseAct)
	if model != ModelOpus {
		t.Fatalf("actual model = %q, want opus", model)
	}

	results := r.LastShadowResults()
	if len(results) != 2 {
		t.Fatalf("expected 2 shadow results, got %d", len(results))
	}

	// Economy profile: act phase → sonnet
	if results[0].Profile != "economy" {
		t.Errorf("result[0].Profile = %q", results[0].Profile)
	}
	if results[0].Model != ModelSonnet {
		t.Errorf("economy act model = %q, want sonnet", results[0].Model)
	}
	if results[0].Actual != ModelOpus {
		t.Errorf("economy actual = %q, want opus", results[0].Actual)
	}

	// Balanced profile: act phase → sonnet
	if results[1].Profile != "balanced" {
		t.Errorf("result[1].Profile = %q", results[1].Profile)
	}
	if results[1].Model != ModelSonnet {
		t.Errorf("balanced act model = %q, want sonnet", results[1].Model)
	}
}

func TestShadowDisabledByDefault(t *testing.T) {
	r := New(nil)
	r.SelectModel(tool.PhaseAct)
	if results := r.LastShadowResults(); len(results) != 0 {
		t.Errorf("expected no shadow results when disabled, got %d", len(results))
	}
}

func TestShadowOrientPhase(t *testing.T) {
	r := New(nil)
	r.SetShadowProfiles(DefaultShadowProfiles())

	r.SelectModel(tool.PhaseOrient)
	results := r.LastShadowResults()

	// Economy orient → sonnet, balanced orient → opus
	if results[0].Model != ModelSonnet {
		t.Errorf("economy orient = %q, want sonnet", results[0].Model)
	}
	if results[1].Model != ModelOpus {
		t.Errorf("balanced orient = %q, want opus", results[1].Model)
	}
}

func TestShadowReflectPhase(t *testing.T) {
	r := New(nil)
	r.SetShadowProfiles(DefaultShadowProfiles())

	r.SelectModel(tool.PhaseReflect)
	results := r.LastShadowResults()

	// Economy reflect → haiku, balanced reflect → sonnet
	if results[0].Model != ModelHaiku {
		t.Errorf("economy reflect = %q, want haiku", results[0].Model)
	}
	if results[1].Model != ModelSonnet {
		t.Errorf("balanced reflect = %q, want sonnet", results[1].Model)
	}
}

func TestEvaluateShadowsNilProfiles(t *testing.T) {
	r := New(nil)
	results := r.EvaluateShadows(tool.PhaseAct, ModelOpus)
	if results != nil {
		t.Errorf("expected nil results with no profiles, got %d", len(results))
	}
}
