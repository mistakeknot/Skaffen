package router

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

// Config holds routing configuration from JSON + env vars.
type Config struct {
	Phases         map[tool.Phase]string `json:"phases,omitempty"`
	Budget         *BudgetConfig         `json:"budget,omitempty"`
	Complexity     *ComplexityConfig      `json:"complexity,omitempty"`
	ContextWindows map[string]int        `json:"context_windows,omitempty"`
}

// BudgetConfig controls per-session token budget enforcement.
type BudgetConfig struct {
	MaxTokens int     `json:"max_tokens"`
	Mode      string  `json:"mode"`       // "graceful" (default), "hard-stop", "advisory"
	DegradeAt float64 `json:"degrade_at"` // 0-1, default 0.8
}

// ComplexityConfig controls prompt complexity classification.
type ComplexityConfig struct {
	Mode string `json:"mode"` // "shadow" (default), "enforce"
}

// LoadConfig reads routing config from a JSON file.
// Returns empty config (not error) if file doesn't exist.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Phases: make(map[tool.Phase]string),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read routing config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse routing config %s: %w", path, err)
	}

	// Normalize budget defaults
	if cfg.Budget != nil {
		if cfg.Budget.Mode == "" {
			cfg.Budget.Mode = "graceful"
		}
		if cfg.Budget.DegradeAt == 0 {
			cfg.Budget.DegradeAt = 0.8
		}
	}

	if cfg.Phases == nil {
		cfg.Phases = make(map[tool.Phase]string)
	}

	return cfg, nil
}

// envOverride checks for SKAFFEN_MODEL_<PHASE> env var.
func (c *Config) envOverride(phase tool.Phase) string {
	key := "SKAFFEN_MODEL_" + strings.ToUpper(string(phase))
	return os.Getenv(key)
}
