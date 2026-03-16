package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverPlugins scans baseDir for Interverse plugin directories.
// Each subdirectory is checked for .claude-plugin/plugin.json.
// MCP servers are resolved with ${CLAUDE_PLUGIN_ROOT} expanded to the
// plugin's .claude-plugin/ directory. Non-MCP capabilities (skills,
// commands, agents) are ignored — those are handled by the plugin package.
//
// Returns only plugins that have at least one MCP server.
// Plugins that fail to parse are skipped with a warning on stderr.
func DiscoverPlugins(baseDir string) (map[string]PluginConfig, error) {
	result := make(map[string]PluginConfig)

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("read interverse directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginJSONPath := filepath.Join(baseDir, entry.Name(), ".claude-plugin", "plugin.json")
		data, err := os.ReadFile(pluginJSONPath)
		if err != nil {
			// No plugin.json — silently skip (not all subdirs are plugins)
			continue
		}

		var pj pluginJSON
		if err := json.Unmarshal(data, &pj); err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: interverse plugin %q: parse plugin.json: %v (skipping)\n", entry.Name(), err)
			continue
		}

		pluginRoot := filepath.Join(baseDir, entry.Name(), ".claude-plugin")

		// Resolve MCP servers
		servers := make(map[string]ServerConfig, len(pj.MCPServers))
		for srvName, srv := range pj.MCPServers {
			if srv.Type != "" && srv.Type != "stdio" {
				fmt.Fprintf(os.Stderr, "skaffen: warning: interverse plugin %q server %q: unsupported type %q (skipping)\n", entry.Name(), srvName, srv.Type)
				continue
			}

			// Expand ${CLAUDE_PLUGIN_ROOT} and env vars in command
			cmd := srv.Command
			cmd = strings.ReplaceAll(cmd, "${CLAUDE_PLUGIN_ROOT}", pluginRoot)
			cmd = expandEnv(cmd)

			// Expand ${CLAUDE_PLUGIN_ROOT} and env vars in args
			args := make([]string, len(srv.Args))
			for i, a := range srv.Args {
				a = strings.ReplaceAll(a, "${CLAUDE_PLUGIN_ROOT}", pluginRoot)
				args[i] = expandEnv(a)
			}

			// Expand env vars in server env
			env := make(map[string]string, len(srv.Env))
			for k, v := range srv.Env {
				v = strings.ReplaceAll(v, "${CLAUDE_PLUGIN_ROOT}", pluginRoot)
				env[k] = expandEnv(v)
			}

			servers[srvName] = ServerConfig{
				Type:    "stdio",
				Command: cmd,
				Args:    args,
				Env:     env,
			}
		}

		if len(servers) == 0 {
			// Plugin has no MCP servers (skills/commands only) — skip
			continue
		}

		// Use the plugin name from plugin.json, falling back to directory name
		name := pj.Name
		if name == "" {
			name = entry.Name()
		}

		result[name] = PluginConfig{
			Name:    name,
			Phases:  []string{"act", "build"},
			Servers: servers,
		}
	}

	return result, nil
}
