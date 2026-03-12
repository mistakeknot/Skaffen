package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIntegrationFullPipeline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}

	// Create a temp dir with both global and project hooks
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(globalDir, 0755)
	os.MkdirAll(projectDir, 0755)

	// Global hook: log to a file on PreToolUse
	logFile := filepath.Join(tmpDir, "hook.log")
	globalScript := writeScript(t, `#!/bin/sh
read input
echo "$input" >> `+logFile+`
echo '{"decision":"allow"}'`)

	// Project hook: deny "rm" tool specifically
	projectScript := writeScript(t, `#!/bin/sh
read input
tool=$(echo "$input" | grep -o '"tool_name":"[^"]*"' | cut -d'"' -f4)
if [ "$tool" = "rm" ]; then
  echo '{"decision":"deny"}'
else
  echo '{"decision":"allow"}'
fi`)

	globalPath := filepath.Join(globalDir, "hooks.json")
	os.WriteFile(globalPath, []byte(`{
		"hooks": {
			"PreToolUse": [
				{"matcher": "*", "hooks": [{"type": "command", "command": "`+globalScript+`"}]}
			]
		}
	}`), 0644)

	projectPath := filepath.Join(projectDir, "hooks.json")
	os.WriteFile(projectPath, []byte(`{
		"hooks": {
			"PreToolUse": [
				{"matcher": "*", "hooks": [{"type": "command", "command": "`+projectScript+`"}]}
			]
		}
	}`), 0644)

	// Load and merge
	globalCfg, err := LoadConfig(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	projectCfg, err := LoadConfig(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeConfig(globalCfg, projectCfg)

	exec := NewExecutor(merged, "integration-test", tmpDir, "build")

	// Test 1: "bash" tool should be allowed (both hooks say allow)
	decision, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{"command":"ls"}`))
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if decision != DecisionAllow {
		t.Errorf("bash: decision = %q, want allow", decision)
	}

	// Test 2: "rm" tool should be denied (project hook denies)
	decision, err = exec.PreToolUse(context.Background(), "rm", json.RawMessage(`{"path":"/"}`))
	if err != nil {
		t.Fatalf("rm: %v", err)
	}
	if decision != DecisionDeny {
		t.Errorf("rm: decision = %q, want deny", decision)
	}

	// Verify global hook logged both calls
	logData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(logData) == 0 {
		t.Error("global hook log file is empty — hook didn't run")
	}
}

func TestIntegrationNoHooksConfigured(t *testing.T) {
	cfg := &Config{Hooks: make(map[Event][]HookGroup)}
	exec := NewExecutor(cfg, "test", "/tmp", "build")

	decision, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != DecisionAllow {
		t.Errorf("no hooks: decision = %q, want allow", decision)
	}
}
