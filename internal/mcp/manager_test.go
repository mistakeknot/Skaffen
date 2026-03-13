package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

func TestManager_LoadAll_RegistersTools(t *testing.T) {
	binary := buildTestServer(t)

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "echo-plugin", ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	pluginJSON := `{"name":"echo-plugin","mcpServers":{"echo-plugin":{"type":"stdio","command":"` + binary + `"}}}`
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644)

	tomlPath := filepath.Join(dir, "plugins.toml")
	tomlContent := `[plugins.echo-plugin]
path = "` + filepath.Join(pluginDir, "plugin.json") + `"
phases = ["brainstorm", "build"]
`
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	cfg, err := LoadConfig(tomlPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mgr := NewManager(cfg, reg, nil)
	defer mgr.Shutdown()

	if err := mgr.LoadAll(ctx); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// Check that echo tool is registered in brainstorm phase
	names := make(map[string]bool)
	for _, td := range reg.Tools(tool.PhaseBrainstorm) {
		names[td.Name] = true
	}
	if !names["echo-plugin_echo-plugin_echo"] {
		t.Errorf("echo tool not in brainstorm. Available: %v", names)
	}

	// Check that echo tool is registered in build phase
	names = make(map[string]bool)
	for _, td := range reg.Tools(tool.PhaseBuild) {
		names[td.Name] = true
	}
	if !names["echo-plugin_echo-plugin_echo"] {
		t.Errorf("echo tool not in build. Available: %v", names)
	}

	// Check that echo tool is NOT in review phase
	names = make(map[string]bool)
	for _, td := range reg.Tools(tool.PhaseReview) {
		names[td.Name] = true
	}
	if names["echo-plugin_echo-plugin_echo"] {
		t.Error("echo tool should not be in review phase")
	}
}

func TestManager_ExecuteThroughRegistry(t *testing.T) {
	binary := buildTestServer(t)

	cfg := map[string]PluginConfig{
		"echo": {
			Name:   "echo",
			Phases: []string{"build"},
			Servers: map[string]ServerConfig{
				"echo": {
					Type:    "stdio",
					Command: binary,
				},
			},
		},
	}

	reg := tool.NewRegistry()
	mgr := NewManager(cfg, reg, nil)
	defer mgr.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := mgr.LoadAll(ctx); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	result := reg.Execute(ctx, tool.PhaseBuild, "echo_echo_echo", []byte(`{"text":"round-trip"}`))
	if result.IsError {
		t.Fatalf("Execute error: %s", result.Content)
	}
	if result.Content != "echo: round-trip" {
		t.Errorf("Content = %q", result.Content)
	}
}

func TestManager_Shutdown(t *testing.T) {
	binary := buildTestServer(t)

	cfg := map[string]PluginConfig{
		"echo": {
			Name:   "echo",
			Phases: []string{"build"},
			Servers: map[string]ServerConfig{
				"echo": {Type: "stdio", Command: binary},
			},
		},
	}

	reg := tool.NewRegistry()
	mgr := NewManager(cfg, reg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mgr.LoadAll(ctx)
	mgr.Shutdown()

	// After shutdown, tool calls should return errors
	result := reg.Execute(ctx, tool.PhaseBuild, "echo_echo_echo", []byte(`{"text":"dead"}`))
	if !result.IsError {
		t.Error("expected error after shutdown")
	}
}

func TestManager_MissingServer_GracefulDegradation(t *testing.T) {
	cfg := map[string]PluginConfig{
		"broken": {
			Name:   "broken",
			Phases: []string{"build"},
			Servers: map[string]ServerConfig{
				"broken": {Type: "stdio", Command: "/nonexistent/binary"},
			},
		},
	}

	reg := tool.NewRegistry()
	mgr := NewManager(cfg, reg, nil)
	defer mgr.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// LoadAll should NOT error — it should skip broken plugins
	err := mgr.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll should not error on broken plugin: %v", err)
	}

	// No tools should be registered for the broken plugin
	names := make(map[string]bool)
	for _, td := range reg.Tools(tool.PhaseBuild) {
		names[td.Name] = true
	}
	if names["broken_broken_echo"] {
		t.Error("broken plugin tools should not be registered")
	}
}

func TestManager_EndToEnd_ConfigToExecution(t *testing.T) {
	// Full integration: config file → parse → connect → register → execute
	binary := buildTestServer(t)

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "e2e", ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	pluginJSON := `{"name":"e2e","mcpServers":{"e2e":{"type":"stdio","command":"` + binary + `"}}}`
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(pluginJSON), 0o644)

	tomlPath := filepath.Join(dir, "plugins.toml")
	tomlContent := `[plugins.e2e]
path = "` + filepath.Join(pluginDir, "plugin.json") + `"
phases = ["build"]
`
	os.WriteFile(tomlPath, []byte(tomlContent), 0o644)

	// Step 1: Load config
	cfg, err := LoadConfig(tomlPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Step 2: Create registry and manager
	reg := tool.NewRegistry()
	tool.RegisterBuiltins(reg)
	mgr := NewManager(cfg, reg, nil)
	defer mgr.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Step 3: Load all plugins
	if err := mgr.LoadAll(ctx); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// Step 4: Execute MCP tool through the registry (same path as agent loop)
	result := reg.Execute(ctx, tool.PhaseBuild, "e2e_e2e_echo", []byte(`{"text":"integration"}`))
	if result.IsError {
		t.Fatalf("Execute error: %s", result.Content)
	}
	if result.Content != "echo: integration" {
		t.Errorf("Content = %q, want %q", result.Content, "echo: integration")
	}

	// Step 5: Verify phase gating (not in brainstorm)
	result = reg.Execute(ctx, tool.PhaseBrainstorm, "e2e_e2e_echo", []byte(`{"text":"blocked"}`))
	if !result.IsError {
		t.Error("expected error for brainstorm phase (tool only in build)")
	}
}
