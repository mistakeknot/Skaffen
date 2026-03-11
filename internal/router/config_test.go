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
