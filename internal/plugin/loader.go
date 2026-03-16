// Package plugin provides unified discovery and injection of Interverse
// plugins into Skaffen's runtime. It scans the interverse/ directory for
// plugin.json manifests and resolves all five capability types: MCP servers,
// skills, commands, agents, and hooks.
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/hooks"
	"github.com/mistakeknot/Skaffen/internal/mcp"
	"github.com/mistakeknot/Skaffen/internal/skill"
	"github.com/mistakeknot/Skaffen/internal/subagent"
)

// Plugin holds all resolved capabilities from a single Interverse plugin.
type Plugin struct {
	Name     string
	Dir      string // absolute path to the plugin directory (e.g., interverse/interflux)
	MCP      map[string]mcp.PluginConfig
	Skills   []skill.Def
	Commands []command.Def
	Agents   []subagent.SubagentType
	Hooks    *hooks.Config
}

// manifest is the plugin.json structure for non-MCP capabilities.
// MCP servers are handled by mcp.DiscoverPlugins separately.
type manifest struct {
	Name     string   `json:"name"`
	Skills   []string `json:"skills"`
	Commands []string `json:"commands"`
	Agents   []string `json:"agents"`
}

// Discover scans interverseDir for plugins and resolves all capabilities.
// MCP servers are discovered via mcp.DiscoverPlugins. Skills, commands,
// agents, and hooks are loaded from each plugin's manifest and directory.
//
// Plugins that fail to parse are skipped with a warning on stderr.
func Discover(interverseDir string) ([]Plugin, error) {
	entries, err := os.ReadDir(interverseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read interverse dir: %w", err)
	}

	// Get MCP configs from mcp.DiscoverPlugins (handles server resolution)
	mcpConfigs, _ := mcp.DiscoverPlugins(interverseDir)

	var plugins []Plugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(interverseDir, entry.Name())
		manifestPath := filepath.Join(pluginDir, ".claude-plugin", "plugin.json")

		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue // no plugin.json — not an Interverse plugin
		}

		var m manifest
		if err := json.Unmarshal(data, &m); err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: plugin %s: parse plugin.json: %v\n", entry.Name(), err)
			continue
		}

		name := m.Name
		if name == "" {
			name = entry.Name()
		}

		p := Plugin{
			Name: name,
			Dir:  pluginDir,
		}

		// MCP servers (already resolved by mcp.DiscoverPlugins)
		if mcpCfg, ok := mcpConfigs[name]; ok {
			p.MCP = map[string]mcp.PluginConfig{name: mcpCfg}
		}

		// Skills: each declared path points to a skill subdirectory
		// (e.g., "./skills/flux-drive"). LoadDir expects the parent directory
		// and scans its subdirs for SKILL.md. So we pass the parent of each
		// declared skill path. Deduplicate parents to avoid double-loading.
		skillParents := make(map[string]bool)
		for _, skillPath := range m.Skills {
			resolved := filepath.Join(pluginDir, skillPath)
			parent := filepath.Dir(resolved)
			skillParents[parent] = true
		}
		for parent := range skillParents {
			if dirExists(parent) {
				p.Skills = append(p.Skills, skill.LoadDir(parent, "interverse-plugin")...)
			}
		}

		// Commands: collect unique command directories, then load each once
		cmdDirs := make(map[string]bool)
		for _, cmdPath := range m.Commands {
			resolved := filepath.Join(pluginDir, cmdPath)
			dir := filepath.Dir(resolved)
			cmdDirs[dir] = true
		}
		for dir := range cmdDirs {
			if dirExists(dir) {
				p.Commands = append(p.Commands, command.LoadMarkdownDir(dir, "interverse-plugin")...)
			}
		}
		p.Commands = deduplicateCommands(p.Commands)

		// Agents: load markdown agent definitions
		if len(m.Agents) > 0 {
			p.Agents = subagent.LoadMarkdownAgents(name, pluginDir, m.Agents)
		}

		// Hooks: load plugin hooks with CLAUDE_PLUGIN_ROOT expansion
		hookCfg, err := hooks.LoadPluginHooks(pluginDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: plugin %s hooks: %v\n", name, err)
		} else if len(hookCfg.Hooks) > 0 {
			p.Hooks = hookCfg
		}

		plugins = append(plugins, p)
	}

	return plugins, nil
}

// Inject registers a plugin's capabilities into the runtime registries.
func Inject(p Plugin, mcpCfg map[string]mcp.PluginConfig, skills map[string]skill.Def,
	cmds map[string]command.Def, subReg *subagent.TypeRegistry, hookCfg *hooks.Config) {

	// MCP servers — merge into the MCP config map (explicit wins)
	for name, cfg := range p.MCP {
		if _, exists := mcpCfg[name]; !exists {
			mcpCfg[name] = cfg
		}
	}

	// Skills — merge (existing wins on name collision)
	for _, s := range p.Skills {
		if _, exists := skills[s.Name]; !exists {
			skills[s.Name] = s
		}
	}

	// Commands — merge (existing wins on name collision)
	for _, c := range p.Commands {
		if _, exists := cmds[c.Name]; !exists {
			cmds[c.Name] = c
		}
	}

	// Agents — register in type registry
	if subReg != nil {
		for _, a := range p.Agents {
			subReg.Register(a)
		}
	}

	// Hooks — merge (append after existing hooks)
	if p.Hooks != nil && hookCfg != nil {
		merged := hooks.MergeConfig(hookCfg, p.Hooks)
		*hookCfg = *merged
	}
}

func deduplicateCommands(cmds []command.Def) []command.Def {
	seen := make(map[string]bool)
	var result []command.Def
	for _, c := range cmds {
		if !seen[c.Name] {
			seen[c.Name] = true
			result = append(result, c)
		}
	}
	return result
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
