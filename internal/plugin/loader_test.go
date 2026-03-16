package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/command"
	"github.com/mistakeknot/Skaffen/internal/hooks"
	"github.com/mistakeknot/Skaffen/internal/mcp"
	"github.com/mistakeknot/Skaffen/internal/skill"
	"github.com/mistakeknot/Skaffen/internal/subagent"
)

func setupTestPlugin(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	pluginDir := filepath.Join(base, "testplugin")
	claudeDir := filepath.Join(pluginDir, ".claude-plugin")
	os.MkdirAll(claudeDir, 0755)

	// plugin.json
	manifest := `{
		"name": "testplugin",
		"mcpServers": {
			"test": {
				"type": "stdio",
				"command": "${CLAUDE_PLUGIN_ROOT}/bin/server.sh",
				"args": [],
				"env": {}
			}
		},
		"skills": ["./skills/myskill"],
		"commands": ["./commands/mycmd.md"],
		"agents": ["./agents/myagent.md"]
	}`
	os.WriteFile(filepath.Join(claudeDir, "plugin.json"), []byte(manifest), 0644)

	// Skill
	skillDir := filepath.Join(pluginDir, "skills", "myskill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: myskill\ndescription: A test skill\n---\nSkill body"), 0644)

	// Command
	cmdDir := filepath.Join(pluginDir, "commands")
	os.MkdirAll(cmdDir, 0755)
	os.WriteFile(filepath.Join(cmdDir, "mycmd.md"), []byte("---\nname: mycmd\ndescription: A test command\n---\nCommand body"), 0644)

	// Agent
	agentDir := filepath.Join(pluginDir, "agents")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "myagent.md"), []byte("---\nname: myagent\ndescription: A test agent\nmodel: sonnet\n---\nAgent system prompt"), 0644)

	return base
}

func TestDiscoverFindsPlugin(t *testing.T) {
	base := setupTestPlugin(t)

	plugins, err := Discover(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	p := plugins[0]
	if p.Name != "testplugin" {
		t.Errorf("expected name testplugin, got %s", p.Name)
	}
}

func TestDiscoverFindsSkills(t *testing.T) {
	base := setupTestPlugin(t)
	plugins, _ := Discover(base)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	if len(plugins[0].Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(plugins[0].Skills))
	}
	if plugins[0].Skills[0].Name != "myskill" {
		t.Errorf("expected skill name myskill, got %s", plugins[0].Skills[0].Name)
	}
}

func TestDiscoverFindsCommands(t *testing.T) {
	base := setupTestPlugin(t)
	plugins, _ := Discover(base)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	if len(plugins[0].Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(plugins[0].Commands))
	}
	if plugins[0].Commands[0].Name != "mycmd" {
		t.Errorf("expected command name mycmd, got %s", plugins[0].Commands[0].Name)
	}
}

func TestDiscoverFindsAgents(t *testing.T) {
	base := setupTestPlugin(t)
	plugins, _ := Discover(base)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	if len(plugins[0].Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(plugins[0].Agents))
	}
	if plugins[0].Agents[0].Name != "testplugin:myagent" {
		t.Errorf("expected qualified name testplugin:myagent, got %s", plugins[0].Agents[0].Name)
	}
}

func TestDiscoverEmptyDir(t *testing.T) {
	plugins, err := Discover(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestDiscoverNonexistentDir(t *testing.T) {
	plugins, err := Discover("/nonexistent/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plugins != nil {
		t.Fatalf("expected nil, got %v", plugins)
	}
}

func TestInjectMergesCapabilities(t *testing.T) {
	p := Plugin{
		Name: "test",
		Skills: []skill.Def{
			{Name: "s1", Description: "skill 1"},
		},
		Commands: []command.Def{
			{Name: "c1", Description: "cmd 1"},
		},
		Agents: []subagent.SubagentType{
			{Name: "test:a1", Description: "agent 1", SystemPrompt: "prompt"},
		},
	}

	mcpCfg := make(map[string]mcp.PluginConfig)
	skills := make(map[string]skill.Def)
	cmds := make(map[string]command.Def)
	subReg := subagent.NewTypeRegistry("")
	hookCfg := &hooks.Config{Hooks: make(map[hooks.Event][]hooks.HookGroup)}

	Inject(p, mcpCfg, skills, cmds, subReg, hookCfg)

	if _, ok := skills["s1"]; !ok {
		t.Error("skill s1 not injected")
	}
	if _, ok := cmds["c1"]; !ok {
		t.Error("command c1 not injected")
	}
	if _, err := subReg.Get("test:a1"); err != nil {
		t.Errorf("agent test:a1 not registered: %v", err)
	}
}

func TestInjectExistingWins(t *testing.T) {
	p := Plugin{
		Name: "test",
		Skills: []skill.Def{
			{Name: "existing", Description: "from plugin"},
		},
	}

	skills := map[string]skill.Def{
		"existing": {Name: "existing", Description: "from user"},
	}

	mcpCfg := make(map[string]mcp.PluginConfig)
	cmds := make(map[string]command.Def)

	Inject(p, mcpCfg, skills, cmds, nil, nil)

	if skills["existing"].Description != "from user" {
		t.Error("existing skill should not be overridden by plugin")
	}
}
