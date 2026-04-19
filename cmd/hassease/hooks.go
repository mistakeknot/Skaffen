package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/hooks"
)

// loadHassHooks loads hooks from the same config paths as skaffen:
//   - ~/.skaffen/hooks.json (user-global)
//   - .skaffen/hooks.json (per-project, relative to workDir)
//
// Shared config ensures plugins that gate tool calls (hookify, etc.)
// work identically regardless of which binary runs the loop.
func loadHassHooks(sessionID, workDir string) *hooks.Executor {
	homeDir, _ := os.UserHomeDir()
	userDir := filepath.Join(homeDir, ".skaffen")

	var hookPaths []string
	userPath := filepath.Join(userDir, "hooks.json")
	if fileExists(userPath) {
		hookPaths = append(hookPaths, userPath)
	}
	projPath := filepath.Join(workDir, ".skaffen", "hooks.json")
	if fileExists(projPath) {
		hookPaths = append(hookPaths, projPath)
	}

	if len(hookPaths) == 0 {
		return nil
	}

	base, err := hooks.LoadConfig(hookPaths[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "hassease: warning: hook config: %v\n", err)
		return nil
	}
	merged := base
	if len(hookPaths) > 1 {
		project, err := hooks.LoadConfig(hookPaths[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "hassease: warning: project hook config: %v\n", err)
		} else {
			merged = hooks.MergeConfig(base, project)
		}
	}

	return hooks.NewExecutor(merged, sessionID, workDir, "act")
}

// hookAdapter bridges hooks.Executor -> agentloop.HookRunner.
// Mirrors the adapter in internal/agent/agent.go:377.
type hookAdapter struct {
	exec *hooks.Executor
}

func (a *hookAdapter) PreToolUse(ctx context.Context, toolName string, input json.RawMessage) (string, error) {
	decision, err := a.exec.PreToolUse(ctx, toolName, input)
	return string(decision), err
}

func (a *hookAdapter) PostToolUse(ctx context.Context, toolName string, input json.RawMessage, result string, isError bool) {
	a.exec.PostToolUse(ctx, toolName, input, result, isError)
}

// wrapWithHooks returns an agentloop.Option that wires hooks, or nil if no hooks configured.
func wrapWithHooks(sessionID, workDir string) (agentloop.Option, *hooks.Executor) {
	exec := loadHassHooks(sessionID, workDir)
	if exec == nil {
		return nil, nil
	}
	return agentloop.WithHooks(&hookAdapter{exec: exec}), exec
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
