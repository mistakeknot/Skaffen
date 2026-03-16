package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPluginHooksNoFile(t *testing.T) {
	cfg, err := LoadPluginHooks(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Hooks) != 0 {
		t.Fatalf("expected empty hooks, got %d events", len(cfg.Hooks))
	}
}

func TestLoadPluginHooksExpandsRoot(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".claude-plugin")
	hooksDir := filepath.Join(dir, "hooks")
	os.MkdirAll(pluginDir, 0755)
	os.MkdirAll(hooksDir, 0755)

	hookJSON := `{
		"hooks": {
			"SessionStart": [{
				"hooks": [{
					"type": "command",
					"command": "${CLAUDE_PLUGIN_ROOT}/bin/start.sh",
					"timeout": 5
				}]
			}]
		}
	}`
	os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte(hookJSON), 0644)

	cfg, err := LoadPluginHooks(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	groups := cfg.Hooks[EventSessionStart]
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(groups[0].Hooks))
	}

	expected := filepath.Join(pluginDir, "bin", "start.sh")
	got := groups[0].Hooks[0].Command
	if got != expected {
		t.Errorf("expected command %q, got %q", expected, got)
	}
}

func TestLoadPluginHooksMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	os.MkdirAll(hooksDir, 0755)
	os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte("not json"), 0644)

	_, err := LoadPluginHooks(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
