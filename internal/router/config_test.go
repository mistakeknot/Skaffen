package router

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/routing.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Phases) != 0 {
		t.Errorf("expected empty phases, got %v", cfg.Phases)
	}
}

func TestLoadConfigFromJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	data := `{"phases": {"brainstorm": "sonnet", "build": "haiku"}, "budget": {"max_tokens": 500000, "mode": "graceful", "degrade_at": 0.8}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Phases[tool.PhaseBrainstorm] != "sonnet" {
		t.Errorf("brainstorm = %q, want sonnet", cfg.Phases[tool.PhaseBrainstorm])
	}
	if cfg.Phases[tool.PhaseBuild] != "haiku" {
		t.Errorf("build = %q, want haiku", cfg.Phases[tool.PhaseBuild])
	}
	if cfg.Budget == nil || cfg.Budget.MaxTokens != 500000 {
		t.Errorf("budget max_tokens = %v, want 500000", cfg.Budget)
	}
	if cfg.Budget.Mode != "graceful" {
		t.Errorf("budget mode = %q, want graceful", cfg.Budget.Mode)
	}
}

func TestEnvVarOverride(t *testing.T) {
	cfg := &Config{}
	t.Setenv("SKAFFEN_MODEL_BUILD", "haiku")
	got := cfg.envOverride(tool.PhaseBuild)
	if got != "haiku" {
		t.Errorf("envOverride(build) = %q, want haiku", got)
	}
}

func TestEnvVarOverrideMissing(t *testing.T) {
	cfg := &Config{}
	got := cfg.envOverride(tool.PhaseBuild)
	if got != "" {
		t.Errorf("envOverride(build) = %q, want empty", got)
	}
}

func TestResolutionOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	data := `{"phases": {"brainstorm": "haiku"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := LoadConfig(path)
	t.Setenv("SKAFFEN_MODEL_BRAINSTORM", "sonnet")

	r := New(cfg)
	model, reason := r.SelectModel(tool.PhaseBrainstorm)
	if model != ModelSonnet {
		t.Errorf("model = %q, want %q (env should override JSON)", model, ModelSonnet)
	}
	if reason != "env-override" {
		t.Errorf("reason = %q, want env-override", reason)
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadConfigComplexity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	data := `{"complexity": {"mode": "enforce"}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Complexity == nil || cfg.Complexity.Mode != "enforce" {
		t.Errorf("complexity mode = %v, want enforce", cfg.Complexity)
	}
}

func TestLoadConfigBudgetDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routing.json")
	data := `{"budget": {"max_tokens": 1000}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Budget.Mode != "graceful" {
		t.Errorf("default mode = %q, want graceful", cfg.Budget.Mode)
	}
	if cfg.Budget.DegradeAt != 0.8 {
		t.Errorf("default degrade_at = %f, want 0.8", cfg.Budget.DegradeAt)
	}
}

func TestMergeConfigPhases(t *testing.T) {
	base := &Config{
		Phases: map[tool.Phase]string{
			tool.PhaseBuild:     "sonnet",
			tool.PhaseBrainstorm: "haiku",
		},
	}
	project := &Config{
		Phases: map[tool.Phase]string{
			tool.PhaseBuild: "opus", // override
		},
	}
	merged := MergeConfig(base, project)

	if merged.Phases[tool.PhaseBuild] != "opus" {
		t.Errorf("build = %q, want opus (project override)", merged.Phases[tool.PhaseBuild])
	}
	if merged.Phases[tool.PhaseBrainstorm] != "haiku" {
		t.Errorf("brainstorm = %q, want haiku (base preserved)", merged.Phases[tool.PhaseBrainstorm])
	}
	// Verify base is NOT mutated
	if base.Phases[tool.PhaseBuild] != "sonnet" {
		t.Errorf("base.build mutated to %q, want sonnet", base.Phases[tool.PhaseBuild])
	}
}

func TestMergeConfigBudget(t *testing.T) {
	base := &Config{
		Phases: map[tool.Phase]string{},
		Budget: &BudgetConfig{MaxTokens: 100000, Mode: "graceful"},
	}
	project := &Config{
		Phases: map[tool.Phase]string{},
		Budget: &BudgetConfig{MaxTokens: 50000, Mode: "hard-stop"},
	}
	merged := MergeConfig(base, project)
	if merged.Budget.MaxTokens != 50000 {
		t.Errorf("budget = %d, want 50000 (project override)", merged.Budget.MaxTokens)
	}
	if merged.Budget.Mode != "hard-stop" {
		t.Errorf("mode = %q, want hard-stop", merged.Budget.Mode)
	}
}

func TestMergeConfigEmpty(t *testing.T) {
	base := &Config{
		Phases: map[tool.Phase]string{tool.PhaseBuild: "sonnet"},
		Budget: &BudgetConfig{MaxTokens: 100000},
	}
	project := &Config{
		Phases: map[tool.Phase]string{},
	}
	merged := MergeConfig(base, project)
	if merged.Phases[tool.PhaseBuild] != "sonnet" {
		t.Errorf("build = %q, want sonnet (base preserved)", merged.Phases[tool.PhaseBuild])
	}
	if merged.Budget == nil || merged.Budget.MaxTokens != 100000 {
		t.Error("budget should be preserved from base")
	}
}

func TestMergeConfigNilMaps(t *testing.T) {
	base := &Config{} // Phases and ContextWindows both nil
	project := &Config{
		Phases:         map[tool.Phase]string{tool.PhaseBuild: "opus"},
		ContextWindows: map[string]int{"opus": 200000},
	}
	merged := MergeConfig(base, project)
	if merged.Phases[tool.PhaseBuild] != "opus" {
		t.Errorf("build = %q, want opus", merged.Phases[tool.PhaseBuild])
	}
	if merged.ContextWindows["opus"] != 200000 {
		t.Errorf("context_windows[opus] = %d, want 200000", merged.ContextWindows["opus"])
	}
}

func TestMergeConfigNoAlias(t *testing.T) {
	base := &Config{
		Phases: map[tool.Phase]string{tool.PhaseBuild: "sonnet"},
		ContextWindows: map[string]int{"sonnet": 200000},
	}
	project := &Config{
		Phases: map[tool.Phase]string{},
	}
	merged := MergeConfig(base, project)

	// Mutate merged — should NOT affect base
	merged.Phases[tool.PhaseReview] = "haiku"
	merged.ContextWindows["haiku"] = 100000

	if _, ok := base.Phases[tool.PhaseReview]; ok {
		t.Error("mutating merged.Phases affected base.Phases (aliased)")
	}
	if _, ok := base.ContextWindows["haiku"]; ok {
		t.Error("mutating merged.ContextWindows affected base.ContextWindows (aliased)")
	}
}

func TestMergeConfigContextWindows(t *testing.T) {
	base := &Config{
		Phases:         map[tool.Phase]string{},
		ContextWindows: map[string]int{"sonnet": 200000, "haiku": 100000},
	}
	project := &Config{
		Phases:         map[tool.Phase]string{},
		ContextWindows: map[string]int{"sonnet": 300000}, // override
	}
	merged := MergeConfig(base, project)
	if merged.ContextWindows["sonnet"] != 300000 {
		t.Errorf("sonnet = %d, want 300000 (project override)", merged.ContextWindows["sonnet"])
	}
	if merged.ContextWindows["haiku"] != 100000 {
		t.Errorf("haiku = %d, want 100000 (base preserved)", merged.ContextWindows["haiku"])
	}
}
