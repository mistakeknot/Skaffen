package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadPluginHooks reads hooks.json from an Interverse plugin directory and
// returns a Config with ${CLAUDE_PLUGIN_ROOT} expanded in all command paths.
//
// Checks two locations:
//  1. pluginDir/hooks/hooks.json (legacy layout)
//  2. pluginDir/.claude-plugin/hooks/hooks.json
//
// Returns empty config if neither file exists.
func LoadPluginHooks(pluginDir string) (*Config, error) {
	candidates := []string{
		filepath.Join(pluginDir, "hooks", "hooks.json"),
		filepath.Join(pluginDir, ".claude-plugin", "hooks", "hooks.json"),
	}

	var hookPath string
	for _, c := range candidates {
		if fileExists(c) {
			hookPath = c
			break
		}
	}
	if hookPath == "" {
		return &Config{Hooks: make(map[Event][]HookGroup)}, nil
	}

	cfg, err := LoadConfig(hookPath)
	if err != nil {
		return nil, fmt.Errorf("load plugin hooks from %s: %w", hookPath, err)
	}

	// Expand ${CLAUDE_PLUGIN_ROOT} in all command fields.
	// The plugin root is the .claude-plugin/ directory (matching Claude Code convention).
	pluginRoot := filepath.Join(pluginDir, ".claude-plugin")
	if !dirExists(pluginRoot) {
		pluginRoot = pluginDir
	}

	for event, groups := range cfg.Hooks {
		for gi, g := range groups {
			for hi, h := range g.Hooks {
				if strings.Contains(h.Command, "${CLAUDE_PLUGIN_ROOT}") {
					cfg.Hooks[event][gi].Hooks[hi].Command = strings.ReplaceAll(h.Command, "${CLAUDE_PLUGIN_ROOT}", pluginRoot)
				}
			}
		}
	}

	return cfg, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
