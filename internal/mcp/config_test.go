package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_BasicParsing(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal plugin.json
	pluginDir := filepath.Join(dir, "my-plugin", ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	pluginJSON := `{
		"name": "my-plugin",
		"mcpServers": {
			"my-plugin": {
				"type": "stdio",
				"command": "${CLAUDE_PLUGIN_ROOT}/bin/launch-mcp.sh",
				"args": ["--verbose"],
				"env": {
					"API_KEY": "${MY_API_KEY}"
				}
			}
		}
	}`
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644)

	// Create plugins.toml referencing it
	tomlPath := filepath.Join(dir, "plugins.toml")
	tomlContent := `[plugins.my-plugin]
path = "` + filepath.Join(pluginDir, "plugin.json") + `"
phases = ["brainstorm", "build"]
`
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	// Set env var for expansion
	t.Setenv("MY_API_KEY", "test-key-123")

	cfg, err := LoadConfig(tomlPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(cfg) != 1 {
		t.Fatalf("got %d plugins, want 1", len(cfg))
	}

	pc := cfg["my-plugin"]
	if pc.Name != "my-plugin" {
		t.Errorf("Name = %q", pc.Name)
	}
	if len(pc.Phases) != 2 {
		t.Errorf("Phases = %v, want [brainstorm build]", pc.Phases)
	}
	if len(pc.Servers) != 1 {
		t.Fatalf("Servers: got %d, want 1", len(pc.Servers))
	}

	srv := pc.Servers["my-plugin"]
	if srv.Command == "" {
		t.Error("Command is empty")
	}
	// ${CLAUDE_PLUGIN_ROOT} expanded to plugin.json parent dir
	if srv.Command != filepath.Join(pluginDir, "bin", "launch-mcp.sh") {
		t.Errorf("Command = %q, want launcher path", srv.Command)
	}
	if len(srv.Args) != 1 || srv.Args[0] != "--verbose" {
		t.Errorf("Args = %v, want [--verbose]", srv.Args)
	}
	if srv.Env["API_KEY"] != "test-key-123" {
		t.Errorf("Env[API_KEY] = %q", srv.Env["API_KEY"])
	}
}

func TestLoadConfig_ArgsEnvExpansion(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "arg-test", ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	pluginJSON := `{
		"name": "arg-test",
		"mcpServers": {
			"arg-test": {
				"type": "stdio",
				"command": "echo",
				"args": ["--config", "${CLAUDE_PLUGIN_ROOT}/config.json"]
			}
		}
	}`
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644)

	tomlPath := filepath.Join(dir, "plugins.toml")
	tomlContent := `[plugins.arg-test]
path = "` + filepath.Join(pluginDir, "plugin.json") + `"
phases = ["build"]
`
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	cfg, err := LoadConfig(tomlPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	srv := cfg["arg-test"].Servers["arg-test"]
	expectedArg := filepath.Join(pluginDir, "config.json")
	if len(srv.Args) != 2 || srv.Args[1] != expectedArg {
		t.Errorf("Args = %v, want [--config %s]", srv.Args, expectedArg)
	}
}

func TestLoadConfig_MissingPluginJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "plugins.toml")
	tomlContent := `[plugins.missing]
path = "/nonexistent/plugin.json"
phases = ["build"]
`
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	cfg, err := LoadConfig(tomlPath)
	if err != nil {
		t.Fatalf("LoadConfig should not error on missing plugin.json: %v", err)
	}
	// Missing plugin should be skipped
	if len(cfg) != 0 {
		t.Errorf("got %d plugins, want 0 (missing should be skipped)", len(cfg))
	}
}

func TestLoadConfig_DefaultPhases(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "simple", ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	pluginJSON := `{"name":"simple","mcpServers":{"simple":{"type":"stdio","command":"echo"}}}`
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644)

	tomlPath := filepath.Join(dir, "plugins.toml")
	tomlContent := `[plugins.simple]
path = "` + filepath.Join(pluginDir, "plugin.json") + `"
`
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	cfg, err := LoadConfig(tomlPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	pc := cfg["simple"]
	// Default phases should be ["build"]
	if len(pc.Phases) != 1 || pc.Phases[0] != "build" {
		t.Errorf("default Phases = %v, want [build]", pc.Phases)
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/plugins.toml")
	if err != nil {
		t.Fatalf("missing config should return empty, not error: %v", err)
	}
	if len(cfg) != 0 {
		t.Errorf("got %d plugins for missing config", len(cfg))
	}
}

func TestMergePluginConfigsBasic(t *testing.T) {
	base := map[string]PluginConfig{
		"global-search": {Name: "global-search", Phases: []string{"build"}},
	}
	project := map[string]PluginConfig{
		"project-db": {Name: "project-db", Phases: []string{"build", "review"}},
	}
	merged := MergePluginConfigs(base, project)
	if len(merged) != 2 {
		t.Fatalf("merged has %d plugins, want 2", len(merged))
	}
	if _, ok := merged["global-search"]; !ok {
		t.Error("missing global-search from base")
	}
	if _, ok := merged["project-db"]; !ok {
		t.Error("missing project-db from project")
	}
}

func TestMergePluginConfigsNameCollision(t *testing.T) {
	base := map[string]PluginConfig{
		"shared": {Name: "shared", Phases: []string{"build"}},
	}
	project := map[string]PluginConfig{
		"shared": {Name: "shared", Phases: []string{"build", "review"}},
	}
	merged := MergePluginConfigs(base, project)
	if len(merged) != 1 {
		t.Fatalf("merged has %d plugins, want 1", len(merged))
	}
	if len(merged["shared"].Phases) != 2 {
		t.Errorf("phases = %v, want [build review] (project override)", merged["shared"].Phases)
	}
}

func TestMergePluginConfigsNilInputs(t *testing.T) {
	var base map[string]PluginConfig
	project := map[string]PluginConfig{
		"only": {Name: "only"},
	}
	merged := MergePluginConfigs(base, project)
	if len(merged) != 1 {
		t.Fatalf("merged has %d plugins, want 1", len(merged))
	}
}
