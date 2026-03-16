package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPluginsEmpty(t *testing.T) {
	dir := t.TempDir()

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}

func TestDiscoverPluginsFindsPlugin(t *testing.T) {
	dir := t.TempDir()

	// Create a fake interverse plugin with one MCP server
	pluginDir := filepath.Join(dir, "interflux", ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pluginJSON := `{
		"name": "interflux",
		"mcpServers": {
			"interflux": {
				"type": "stdio",
				"command": "${CLAUDE_PLUGIN_ROOT}/bin/server",
				"args": ["--port", "${CLAUDE_PLUGIN_ROOT}/config.json"],
				"env": {
					"DATA_DIR": "${CLAUDE_PLUGIN_ROOT}/data"
				}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(plugins))
	}

	pc, ok := plugins["interflux"]
	if !ok {
		t.Fatal("missing plugin 'interflux'")
	}

	if pc.Name != "interflux" {
		t.Errorf("Name = %q, want 'interflux'", pc.Name)
	}

	// Default discovery phases should be ["act", "build"]
	if len(pc.Phases) != 2 || pc.Phases[0] != "act" || pc.Phases[1] != "build" {
		t.Errorf("Phases = %v, want [act build]", pc.Phases)
	}

	if len(pc.Servers) != 1 {
		t.Fatalf("Servers: got %d, want 1", len(pc.Servers))
	}

	srv := pc.Servers["interflux"]

	// ${CLAUDE_PLUGIN_ROOT} should be expanded to the .claude-plugin directory
	expectedCmd := filepath.Join(pluginDir, "bin", "server")
	if srv.Command != expectedCmd {
		t.Errorf("Command = %q, want %q", srv.Command, expectedCmd)
	}

	expectedArg1 := filepath.Join(pluginDir, "config.json")
	if len(srv.Args) != 2 || srv.Args[1] != expectedArg1 {
		t.Errorf("Args = %v, want [--port %s]", srv.Args, expectedArg1)
	}

	expectedEnv := filepath.Join(pluginDir, "data")
	if srv.Env["DATA_DIR"] != expectedEnv {
		t.Errorf("Env[DATA_DIR] = %q, want %q", srv.Env["DATA_DIR"], expectedEnv)
	}
}

func TestDiscoverPluginsSkipsMalformed(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin with malformed JSON
	pluginDir := filepath.Join(dir, "broken-plugin", ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte("{invalid json!!!"), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins should not error on malformed JSON: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0 (malformed should be skipped)", len(plugins))
	}
}

func TestDiscoverPluginsSkipsNoMCP(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin with no MCP servers (skills-only plugin)
	pluginDir := filepath.Join(dir, "skills-only", ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pluginJSON := `{
		"name": "skills-only",
		"description": "A skills-only plugin",
		"mcpServers": {}
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0 (no-MCP should be skipped)", len(plugins))
	}
}

func TestDiscoverPluginsMergeWithExplicit(t *testing.T) {
	dir := t.TempDir()

	// Create two discovered plugins
	for _, name := range []string{"alpha", "beta"} {
		pluginDir := filepath.Join(dir, name, ".claude-plugin")
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			t.Fatal(err)
		}
		pj := `{"name":"` + name + `","mcpServers":{"` + name + `":{"type":"stdio","command":"echo"}}}`
		if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pj), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	discovered, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(discovered) != 2 {
		t.Fatalf("discovered %d plugins, want 2", len(discovered))
	}

	// Explicit plugins should override discovered ones
	explicit := map[string]PluginConfig{
		"alpha": {
			Name:   "alpha",
			Phases: []string{"build", "review"},
			Servers: map[string]ServerConfig{
				"alpha": {Type: "stdio", Command: "/usr/local/bin/alpha-custom"},
			},
		},
	}

	// MergePluginConfigs: explicit (second arg) wins on collision
	merged := MergePluginConfigs(discovered, explicit)

	if len(merged) != 2 {
		t.Fatalf("merged has %d plugins, want 2", len(merged))
	}

	// alpha should use the explicit config, not discovered
	alphaPC := merged["alpha"]
	if alphaPC.Servers["alpha"].Command != "/usr/local/bin/alpha-custom" {
		t.Errorf("alpha command = %q, want explicit override", alphaPC.Servers["alpha"].Command)
	}
	if len(alphaPC.Phases) != 2 || alphaPC.Phases[0] != "build" {
		t.Errorf("alpha phases = %v, want [build review] from explicit", alphaPC.Phases)
	}

	// beta should keep the discovered config
	betaPC := merged["beta"]
	if betaPC.Servers["beta"].Command != "echo" {
		t.Errorf("beta command = %q, want 'echo' from discovery", betaPC.Servers["beta"].Command)
	}
	if len(betaPC.Phases) != 2 || betaPC.Phases[0] != "act" {
		t.Errorf("beta phases = %v, want [act build] from discovery", betaPC.Phases)
	}
}

func TestDiscoverPluginsSkipsFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a regular file (not a directory) — should be skipped
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0 (files should be skipped)", len(plugins))
	}
}

func TestDiscoverPluginsNoPluginJSON(t *testing.T) {
	dir := t.TempDir()

	// Create a directory without .claude-plugin/plugin.json
	if err := os.MkdirAll(filepath.Join(dir, "empty-dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}

func TestDiscoverPluginsFallbackName(t *testing.T) {
	dir := t.TempDir()

	// Create a plugin with empty name — should fall back to directory name
	pluginDir := filepath.Join(dir, "my-tool", ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pluginJSON := `{
		"mcpServers": {
			"default": {
				"type": "stdio",
				"command": "echo"
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(plugins))
	}

	pc, ok := plugins["my-tool"]
	if !ok {
		t.Fatal("missing plugin 'my-tool' (should use directory name as fallback)")
	}
	if pc.Name != "my-tool" {
		t.Errorf("Name = %q, want 'my-tool'", pc.Name)
	}
}
