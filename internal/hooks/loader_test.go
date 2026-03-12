package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	os.WriteFile(path, []byte(`{
		"hooks": {
			"PreToolUse": [
				{"matcher": "bash", "hooks": [{"type": "command", "command": "echo ok", "timeout": 5}]}
			]
		}
	}`), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	groups := cfg.Hooks[EventPreToolUse]
	if len(groups) != 1 {
		t.Fatalf("expected 1 hook group, got %d", len(groups))
	}
	if groups[0].Matcher != "bash" {
		t.Errorf("matcher = %q, want %q", groups[0].Matcher, "bash")
	}
	if len(groups[0].Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(groups[0].Hooks))
	}
	if groups[0].Hooks[0].Timeout != 5 {
		t.Errorf("timeout = %d, want 5", groups[0].Hooks[0].Timeout)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/hooks.json")
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(cfg.Hooks) != 0 {
		t.Errorf("expected empty hooks, got %d events", len(cfg.Hooks))
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	os.WriteFile(path, []byte(`{not json}`), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMergeConfigAppends(t *testing.T) {
	global := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{Matcher: "bash", Hooks: []HookDef{{Type: "command", Command: "global.sh"}}}},
		},
	}
	project := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{Matcher: "bash", Hooks: []HookDef{{Type: "command", Command: "project.sh"}}}},
		},
	}

	merged := MergeConfig(global, project)
	groups := merged.Hooks[EventPreToolUse]
	if len(groups) != 2 {
		t.Fatalf("expected 2 hook groups after merge, got %d", len(groups))
	}
	if groups[0].Hooks[0].Command != "global.sh" {
		t.Errorf("first group should be global, got %q", groups[0].Hooks[0].Command)
	}
	if groups[1].Hooks[0].Command != "project.sh" {
		t.Errorf("second group should be project, got %q", groups[1].Hooks[0].Command)
	}
}

func TestMergeConfigNoAlias(t *testing.T) {
	global := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{Matcher: "bash", Hooks: []HookDef{{Type: "command", Command: "global.sh"}}}},
		},
	}
	project := &Config{Hooks: map[Event][]HookGroup{}}

	merged := MergeConfig(global, project)
	// Mutate merged — should not affect global
	merged.Hooks[EventPreToolUse] = append(merged.Hooks[EventPreToolUse],
		HookGroup{Matcher: "*", Hooks: []HookDef{{Type: "command", Command: "extra.sh"}}})

	if len(global.Hooks[EventPreToolUse]) != 1 {
		t.Fatal("merge aliased the original — mutation leaked to global config")
	}
}

func TestMergeConfigInnerNoAlias(t *testing.T) {
	global := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{Matcher: "bash", Hooks: []HookDef{{Type: "command", Command: "global.sh"}}}},
		},
	}
	project := &Config{Hooks: map[Event][]HookGroup{}}

	merged := MergeConfig(global, project)
	// Mutate inner HookDef slice — should not affect global
	merged.Hooks[EventPreToolUse][0].Hooks[0].Command = "mutated.sh"

	if global.Hooks[EventPreToolUse][0].Hooks[0].Command != "global.sh" {
		t.Fatal("merge shallow-copied inner []HookDef — mutation leaked to global config")
	}
}
