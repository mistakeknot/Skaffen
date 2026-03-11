package router

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/provider"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

func TestFullRoutingPipeline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	data := `{
		"phases": {"review": "haiku"},
		"budget": {"max_tokens": 10000, "mode": "graceful", "degrade_at": 0.8},
		"complexity": {"mode": "shadow"}
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	r := New(cfg)

	// Turn 1: brainstorm -> opus (phase default, not overridden)
	model, reason := r.SelectModel(tool.PhaseBrainstorm)
	if model != ModelOpus {
		t.Errorf("turn1 brainstorm: model = %q, want opus", model)
	}
	if reason != "phase-default" {
		t.Errorf("turn1: reason = %q", reason)
	}

	// Turn 2: review -> haiku (config override)
	model, reason = r.SelectModel(tool.PhaseReview)
	if model != ModelHaiku {
		t.Errorf("turn2 review: model = %q, want haiku", model)
	}
	if reason != "config-file" {
		t.Errorf("turn2: reason = %q, want config-file", reason)
	}

	// Record usage: 8000 tokens -> 80% of budget -> should degrade
	r.RecordUsage(provider.Usage{InputTokens: 5000, OutputTokens: 3000})
	model, reason = r.SelectModel(tool.PhaseBrainstorm)
	if model != ModelHaiku {
		t.Errorf("after 80%%: model = %q, want haiku (degraded)", model)
	}
	if reason != "budget-degrade" {
		t.Errorf("after 80%%: reason = %q, want budget-degrade", reason)
	}

	// Budget state check
	spent, max, pct := r.BudgetState()
	if spent != 8000 {
		t.Errorf("spent = %d, want 8000", spent)
	}
	if max != 10000 {
		t.Errorf("max = %d, want 10000", max)
	}
	if pct < 0.79 || pct > 0.81 {
		t.Errorf("pct = %f, want ~0.8", pct)
	}
}

func TestEnvVarOverridesJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	data := `{"phases": {"build": "haiku"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ := LoadConfig(path)
	t.Setenv("SKAFFEN_MODEL_BUILD", "opus")
	r := New(cfg)

	model, reason := r.SelectModel(tool.PhaseBuild)
	if model != ModelOpus {
		t.Errorf("model = %q, want opus (env overrides JSON)", model)
	}
	if reason != "env-override" {
		t.Errorf("reason = %q, want env-override", reason)
	}
}

func TestComplexityWithBudgetInteraction(t *testing.T) {
	cfg := &Config{
		Phases:     map[tool.Phase]string{},
		Budget:     &BudgetConfig{MaxTokens: 1000, Mode: "graceful", DegradeAt: 0.8},
		Complexity: &ComplexityConfig{Mode: "enforce"},
	}
	r := New(cfg)

	// C5 promotes to opus, but budget at 90% should degrade to haiku
	r.SetInputTokens(5000)
	r.RecordUsage(provider.Usage{InputTokens: 600, OutputTokens: 300})
	model, reason := r.SelectModel(tool.PhaseBuild)
	// Complexity promotes to opus, then budget degrades to haiku
	if model != ModelHaiku {
		t.Errorf("complexity+budget: model = %q, want haiku (budget wins)", model)
	}
	if reason != "budget-degrade" {
		t.Errorf("complexity+budget: reason = %q, want budget-degrade", reason)
	}
}
