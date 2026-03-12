package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecutorPreToolUseAllow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	script := writeScript(t, `#!/bin/sh
read input
echo '{"decision":"allow"}'`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{
				Matcher: "bash",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 5}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	result, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{"command":"ls"}`))
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result != DecisionAllow {
		t.Errorf("decision = %q, want %q", result, DecisionAllow)
	}
}

func TestExecutorPreToolUseDeny(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	script := writeScript(t, `#!/bin/sh
read input
echo '{"decision":"deny"}'`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{
				Matcher: "*",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 5}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	result, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{"command":"rm -rf /"}`))
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result != DecisionDeny {
		t.Errorf("decision = %q, want %q", result, DecisionDeny)
	}
}

func TestExecutorPreToolUseMatcherFilters(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	script := writeScript(t, `#!/bin/sh
echo '{"decision":"deny"}'`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{
				Matcher: "bash",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 5}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	// "read" does not match "bash" matcher — should get allow (no hooks ran)
	result, err := exec.PreToolUse(context.Background(), "read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result != DecisionAllow {
		t.Errorf("non-matching tool: decision = %q, want %q", result, DecisionAllow)
	}
}

func TestExecutorTimeoutFailOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	script := writeScript(t, `#!/bin/sh
sleep 30`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{
				Matcher: "*",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 1}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	start := time.Now()
	result, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{}`))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("timeout should not return error (fail-open): %v", err)
	}
	if result != DecisionAllow {
		t.Errorf("timeout: decision = %q, want %q (fail-open)", result, DecisionAllow)
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %v — timeout not working", elapsed)
	}
}

func TestExecutorCrashFailOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	script := writeScript(t, `#!/bin/sh
exit 1`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{
				Matcher: "*",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 5}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	result, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("crash should not return error (fail-open): %v", err)
	}
	if result != DecisionAllow {
		t.Errorf("crash: decision = %q, want %q (fail-open)", result, DecisionAllow)
	}
}

func TestExecutorCrashFailClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	script := writeScript(t, `#!/bin/sh
exit 1`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{
				Matcher: "*",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 5, OnError: "deny"}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	result, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("crash with on_error=deny should not return error: %v", err)
	}
	if result != DecisionDeny {
		t.Errorf("crash with on_error=deny: decision = %q, want %q (fail-closed)", result, DecisionDeny)
	}
}

func TestExecutorFirstDenyShortCircuits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	denyScript := writeScript(t, `#!/bin/sh
echo '{"decision":"deny"}'`)
	// This second hook should never run due to short-circuit
	allowScript := writeScript(t, `#!/bin/sh
echo '{"decision":"allow"}'`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {
				{Matcher: "*", Hooks: []HookDef{{Type: "command", Command: denyScript, Timeout: 5}}},
				{Matcher: "*", Hooks: []HookDef{{Type: "command", Command: allowScript, Timeout: 5}}},
			},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	result, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result != DecisionDeny {
		t.Errorf("decision = %q, want %q", result, DecisionDeny)
	}
}

func TestExecutorAskCollectsMostRestrictive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	allowScript := writeScript(t, `#!/bin/sh
echo '{"decision":"allow"}'`)
	askScript := writeScript(t, `#!/bin/sh
echo '{"decision":"ask"}'`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {
				{Matcher: "*", Hooks: []HookDef{{Type: "command", Command: allowScript, Timeout: 5}}},
				{Matcher: "*", Hooks: []HookDef{{Type: "command", Command: askScript, Timeout: 5}}},
			},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	result, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("PreToolUse: %v", err)
	}
	if result != DecisionAsk {
		t.Errorf("decision = %q, want %q (most restrictive)", result, DecisionAsk)
	}
}

func TestExecutorPostToolUse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	// PostToolUse hooks are advisory — just verify they don't error
	script := writeScript(t, `#!/bin/sh
read input
echo "ok"`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPostToolUse: {{
				Matcher: "*",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 5}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")
	exec.PostToolUse(context.Background(), "bash", json.RawMessage(`{}`), "output", false)
	// No error = pass (advisory hook)
}

func TestExecutorSessionStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	script := writeScript(t, `#!/bin/sh
read input
echo "ok"`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventSessionStart: {{
				Matcher: "*",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 5}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")
	exec.SessionStart(context.Background(), "tui")
	// No error = pass (advisory hook)
}

func TestExecutorInvalidJSONFailOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	// Hook returns non-JSON — should fail-open (allow)
	script := writeScript(t, `#!/bin/sh
echo 'not json at all'`)

	cfg := &Config{
		Hooks: map[Event][]HookGroup{
			EventPreToolUse: {{
				Matcher: "*",
				Hooks:   []HookDef{{Type: "command", Command: script, Timeout: 5}},
			}},
		},
	}
	exec := NewExecutor(cfg, "test-session", "/tmp", "build")

	result, err := exec.PreToolUse(context.Background(), "bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("invalid JSON should not return error (fail-open): %v", err)
	}
	if result != DecisionAllow {
		t.Errorf("invalid JSON: decision = %q, want %q (fail-open)", result, DecisionAllow)
	}
}

func TestSafeEnvStripsCredentials(t *testing.T) {
	// Set test env vars that should be stripped
	t.Setenv("ANTHROPIC_API_KEY", "secret1")
	t.Setenv("MY_CUSTOM_SECRET", "secret2")    // suffix _SECRET
	t.Setenv("GITLAB_TOKEN", "secret3")         // prefix match
	t.Setenv("APP_API_KEY", "secret4")          // suffix _API_KEY
	t.Setenv("DB_PASSWORD", "secret5")          // suffix _PASSWORD
	t.Setenv("SAFE_VARIABLE", "visible")        // should NOT be stripped
	t.Setenv("SKAFFEN_TEST_MARKER", "visible2") // should NOT be stripped

	env := safeEnv()
	envMap := make(map[string]bool)
	for _, kv := range env {
		eqIdx := strings.IndexByte(kv, '=')
		if eqIdx > 0 {
			envMap[kv[:eqIdx]] = true
		}
	}

	// Should be stripped
	for _, key := range []string{"ANTHROPIC_API_KEY", "MY_CUSTOM_SECRET", "GITLAB_TOKEN", "APP_API_KEY", "DB_PASSWORD"} {
		if envMap[key] {
			t.Errorf("expected %s to be stripped from env", key)
		}
	}

	// Should be present
	for _, key := range []string{"SAFE_VARIABLE", "SKAFFEN_TEST_MARKER"} {
		if !envMap[key] {
			t.Errorf("expected %s to be present in env", key)
		}
	}
}

// writeScript creates a temp executable script and returns its path.
func writeScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
