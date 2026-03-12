package hooks

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadConfig reads hooks config from a JSON file.
// Returns empty config (not error) if file doesn't exist.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{Hooks: make(map[Event][]HookGroup)}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read hooks config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse hooks config %s: %w", path, err)
	}

	if cfg.Hooks == nil {
		cfg.Hooks = make(map[Event][]HookGroup)
	}
	return cfg, nil
}

// MergeConfig combines global and project hook configs.
// Per-project hook groups append AFTER global groups within each event.
// Returns a new Config — neither global nor project is modified.
func MergeConfig(global, project *Config) *Config {
	merged := &Config{
		Hooks: make(map[Event][]HookGroup, len(global.Hooks)+len(project.Hooks)),
	}
	// Deep-copy global hooks (including inner []HookDef slices)
	for event, groups := range global.Hooks {
		cp := make([]HookGroup, len(groups))
		for i, g := range groups {
			cp[i] = HookGroup{Matcher: g.Matcher}
			cp[i].Hooks = make([]HookDef, len(g.Hooks))
			copy(cp[i].Hooks, g.Hooks)
		}
		merged.Hooks[event] = cp
	}
	// Deep-copy and append project hooks after global
	for event, groups := range project.Hooks {
		for _, g := range groups {
			cpg := HookGroup{Matcher: g.Matcher}
			cpg.Hooks = make([]HookDef, len(g.Hooks))
			copy(cpg.Hooks, g.Hooks)
			merged.Hooks[event] = append(merged.Hooks[event], cpg)
		}
	}
	return merged
}
