package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/signal"
)

// headlessApprover creates a ToolApprover that denies mutating tools by default.
// In headless mode there is no human in the loop, so:
//   - Tools in autoApprove -> always allowed (reads, greps, globs, ls)
//   - Tools in requireApprove -> denied unless unlocked by CLI flags
//   - Tools not in allowed list -> denied
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

// signalApprover creates a ToolApprover that sends approval requests via
// Signal and blocks until the builder replies y/n. Supports "show" replies
// to preview proposed changes before deciding.
//
// The ctx is captured so the approver can be cancelled when the main loop
// shuts down (e.g., SIGINT).
func signalApprover(ctx context.Context, client *signal.Client, allowed, autoApprove map[string]bool, testPatterns []string) agentloop.ToolApprover {
	return func(toolName string, input json.RawMessage) bool {
		if !allowed[toolName] {
			return false
		}
		if autoApprove[toolName] {
			return true
		}

		// Auto-approve edits to test files (low risk).
		if (toolName == "edit" || toolName == "write") && isTestFile(input, testPatterns) {
			return true
		}

		// Format the approval request.
		summary := formatApprovalMessage(toolName, input)
		detail := formatApprovalDetail(toolName, input)

		// Send and wait for reply, handling "show" requests.
		return requestSignalApproval(ctx, client, summary, detail)
	}
}

// requestSignalApproval sends a message and loops until y/n or timeout.
func requestSignalApproval(ctx context.Context, client *signal.Client, summary, detail string) bool {
	msg := summary + "\n\nReply: y/n/show"
	if err := client.Send(ctx, msg); err != nil {
		fmt.Fprintf(os.Stderr, "hassease: signal send failed: %v — denying\n", err)
		return false
	}

	for {
		reply, err := client.WaitForReply(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hassease: signal receive failed: %v — denying\n", err)
			return false
		}

		switch signal.ClassifyReply(reply) {
		case signal.Approved:
			return true
		case signal.Denied:
			return false
		case signal.ShowRequest:
			if detail != "" {
				_ = client.Send(ctx, detail)
			} else {
				_ = client.Send(ctx, "(no additional detail available)")
			}
			// Loop back to wait for y/n.
		default:
			_ = client.Send(ctx, "Unrecognized reply. Please reply: y / n / show")
		}
	}
}

// formatApprovalMessage builds a concise one-line description of the tool call.
func formatApprovalMessage(toolName string, input json.RawMessage) string {
	switch toolName {
	case "edit":
		var p struct {
			FilePath  string `json:"file_path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if json.Unmarshal(input, &p) == nil && p.FilePath != "" {
			return fmt.Sprintf("[edit] %s — replace %d chars with %d chars",
				p.FilePath, len(p.OldString), len(p.NewString))
		}
	case "write":
		var p struct {
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		}
		if json.Unmarshal(input, &p) == nil && p.FilePath != "" {
			return fmt.Sprintf("[write] %s (%d bytes)", p.FilePath, len(p.Content))
		}
	case "bash":
		var p struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(input, &p) == nil && p.Command != "" {
			cmd := p.Command
			if len(cmd) > 120 {
				cmd = cmd[:120] + "..."
			}
			return fmt.Sprintf("[bash] %s", cmd)
		}
	}
	return fmt.Sprintf("[%s] (no summary available)", toolName)
}

// formatApprovalDetail builds a longer view for "show" requests.
func formatApprovalDetail(toolName string, input json.RawMessage) string {
	switch toolName {
	case "edit":
		var p struct {
			FilePath   string `json:"file_path"`
			OldString  string `json:"old_string"`
			NewString  string `json:"new_string"`
			ReplaceAll bool   `json:"replace_all"`
		}
		if json.Unmarshal(input, &p) != nil {
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, "File: %s\n", p.FilePath)
		if p.ReplaceAll {
			b.WriteString("Mode: replace all\n")
		}
		b.WriteString("\n--- old ---\n")
		b.WriteString(truncateLines(p.OldString, 30))
		b.WriteString("\n+++ new +++\n")
		b.WriteString(truncateLines(p.NewString, 30))
		return b.String()

	case "write":
		var p struct {
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		}
		if json.Unmarshal(input, &p) != nil {
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, "File: %s\n\n", p.FilePath)
		b.WriteString(truncateLines(p.Content, 40))
		return b.String()

	case "bash":
		var p struct {
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		}
		if json.Unmarshal(input, &p) != nil {
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Command:\n%s\n", p.Command)
		if p.Timeout > 0 {
			fmt.Fprintf(&b, "Timeout: %ds\n", p.Timeout)
		}
		return b.String()
	}
	return ""
}

// isTestFile returns true if the tool input targets a test file.
func isTestFile(input json.RawMessage, patterns []string) bool {
	var p struct {
		FilePath string `json:"file_path"`
	}
	if json.Unmarshal(input, &p) != nil || p.FilePath == "" {
		return false
	}

	base := filepath.Base(p.FilePath)
	dir := filepath.Dir(p.FilePath)

	for _, pat := range patterns {
		if matched, _ := filepath.Match(pat, base); matched {
			return true
		}
	}

	// Also check if the file lives under a test directory.
	parts := strings.Split(dir, string(filepath.Separator))
	for _, part := range parts {
		switch part {
		case "tests", "test", "__tests__", "testdata":
			return true
		}
	}

	return false
}

// truncateLines limits text to n lines, appending a truncation notice.
func truncateLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-n)
}

// makeStringSet converts a string slice to a set.
func makeStringSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}

// defaultTestPatterns returns glob patterns that match test files.
func defaultTestPatterns() []string {
	return []string{
		"*_test.go",
		"test_*.py",
		"*_test.py",
		"*.test.ts",
		"*.test.js",
		"*.test.tsx",
		"*.test.jsx",
		"*.spec.ts",
		"*.spec.js",
	}
}
