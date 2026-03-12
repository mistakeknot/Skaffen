package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// PluginConfig holds the resolved configuration for one plugin.
// Phases are stored as plain strings; conversion to tool.Phase happens at registration.
type PluginConfig struct {
	Name    string
	Phases  []string
	Servers map[string]ServerConfig
}

// ServerConfig describes one MCP server from plugin.json mcpServers.
type ServerConfig struct {
	Type    string            // "stdio" only
	Command string            // resolved command path
	Args    []string          // command arguments
	Env     map[string]string // resolved environment variables
}

// tomlConfig is the raw TOML structure.
type tomlConfig struct {
	Plugins map[string]tomlPlugin `toml:"plugins"`
}

type tomlPlugin struct {
	Path   string            `toml:"path"`
	Phases []string          `toml:"phases"`
	Env    map[string]string `toml:"env"` // extra env overrides
}

// pluginJSON is the structure of plugin.json's mcpServers field.
type pluginJSON struct {
	Name       string                     `json:"name"`
	MCPServers map[string]pluginJSONServer `json:"mcpServers"`
}

type pluginJSONServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// LoadConfig reads plugins.toml and resolves each plugin's MCP servers.
// Returns an empty map (not error) if the config file doesn't exist.
// Skips plugins whose plugin.json is missing or malformed (logs to stderr).
func LoadConfig(tomlPath string) (map[string]PluginConfig, error) {
	result := make(map[string]PluginConfig)

	data, err := os.ReadFile(tomlPath)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read plugins config: %w", err)
	}

	var raw tomlConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse plugins config: %w", err)
	}

	configDir := filepath.Dir(tomlPath)

	for name, entry := range raw.Plugins {
		pc, err := resolvePlugin(name, entry, configDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: plugin %q: %v (skipping)\n", name, err)
			continue
		}
		result[name] = pc
	}

	return result, nil
}

func resolvePlugin(name string, entry tomlPlugin, configDir string) (PluginConfig, error) {
	// Resolve plugin.json path relative to config dir
	pluginPath := entry.Path
	if !filepath.IsAbs(pluginPath) {
		pluginPath = filepath.Join(configDir, pluginPath)
	}
	pluginPath = expandEnv(pluginPath)

	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return PluginConfig{}, fmt.Errorf("read plugin.json: %w", err)
	}

	var pj pluginJSON
	if err := json.Unmarshal(data, &pj); err != nil {
		return PluginConfig{}, fmt.Errorf("parse plugin.json: %w", err)
	}

	// Resolve phases (default: build only)
	phases := entry.Phases
	if len(phases) == 0 {
		phases = []string{"build"}
	}

	// CLAUDE_PLUGIN_ROOT = directory containing plugin.json
	pluginRoot := filepath.Dir(pluginPath)

	// Resolve MCP servers
	servers := make(map[string]ServerConfig, len(pj.MCPServers))
	for srvName, srv := range pj.MCPServers {
		if srv.Type != "" && srv.Type != "stdio" {
			fmt.Fprintf(os.Stderr, "skaffen: warning: plugin %q server %q: unsupported type %q (skipping)\n", name, srvName, srv.Type)
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

		// Merge extra env overrides from plugins.toml
		for k, v := range entry.Env {
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
		return PluginConfig{}, fmt.Errorf("no stdio MCP servers found in plugin.json")
	}

	return PluginConfig{
		Name:    name,
		Phases:  phases,
		Servers: servers,
	}, nil
}

// expandEnv replaces ${VAR} patterns with values from os.Environ.
// Does not expand $VAR (bare dollar) or ${VAR:-default} syntax.
func expandEnv(s string) string {
	return os.Expand(s, os.Getenv)
}
