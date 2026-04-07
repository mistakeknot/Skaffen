package main

import (
	"encoding/json"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
)

// headlessApprover creates a ToolApprover that denies mutating tools by default.
// In headless mode there is no human in the loop, so:
//   - Tools in autoApprove → always allowed (reads, greps, globs, ls)
//   - Tools in requireApprove → denied unless unlocked by CLI flags
//   - Tools not in allowed list → denied
//
// This bypasses trust.Evaluator entirely to avoid its safeTools list
// (which auto-allows edit/write) and its auto-promote mechanism (which
// would pollute TUI-mode trust scope from headless sessions).
func headlessApprover(allowed, autoApprove map[string]bool, approveEdits, approveBash bool) agentloop.ToolApprover {
	return func(toolName string, _ json.RawMessage) bool {
		if !allowed[toolName] {
			return false
		}
		if autoApprove[toolName] {
			return true
		}

		// Mutating tools: require explicit CLI flag unlock.
		switch toolName {
		case "bash":
			return approveBash
		case "edit", "write":
			return approveEdits
		default:
			return false
		}
	}
}

// makeStringSet converts a string slice to a set.
func makeStringSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}
