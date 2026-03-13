// Package contextfiles loads project context files (SKAFFEN.md, CLAUDE.md,
// AGENTS.md) from the working directory upward to the user's home directory,
// producing a combined system prompt prefix.
//
// At each directory level, the package checks:
//  1. Top-level files: SKAFFEN.md, CLAUDE.md, AGENTS.md
//  2. .skaffen/ subdirectory: .skaffen/SKAFFEN.md
//
// SKAFFEN.md is Skaffen's native context file, analogous to CLAUDE.md for
// Claude Code or AGENTS.md for Codex CLI.
package contextfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultFileNames lists the context files to look for at each directory level.
// SKAFFEN.md is listed first as the native context file.
var DefaultFileNames = []string{"SKAFFEN.md", "CLAUDE.md", "AGENTS.md"}

// Load walks from startDir up to the user's home directory (or filesystem root
// if HOME is unset), collecting context files at each level. Files are returned
// outermost-first (home → project) so that project-level instructions appear
// last and take precedence.
//
// At each directory level, the package looks for top-level context files and
// also checks .skaffen/ subdirectories for SKAFFEN.md.
//
// Each file's content is wrapped with a header showing its path:
//
//	# Context: /path/to/SKAFFEN.md
//	<file content>
//
// Returns empty string if no context files are found.
func Load(startDir string) string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/"
	}

	// Collect directories from startDir up to home (inclusive).
	dirs := walkUp(startDir, home)

	// Read files at each level — outermost first.
	var sections []string
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		// Check top-level context files.
		for _, name := range DefaultFileNames {
			if s := readContextFile(filepath.Join(dir, name)); s != "" {
				sections = append(sections, s)
			}
		}
		// Check .skaffen/ subdirectory for SKAFFEN.md.
		if s := readContextFile(filepath.Join(dir, ".skaffen", "SKAFFEN.md")); s != "" {
			sections = append(sections, s)
		}
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n---\n\n")
}

// readContextFile reads a single file and wraps it with a context header.
// Returns empty string if the file doesn't exist, is unreadable, or is empty.
func readContextFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(content))
	if text == "" {
		return ""
	}
	return fmt.Sprintf("# Context: %s\n\n%s", path, text)
}

// walkUp returns the path from startDir up to stopDir (inclusive).
// If startDir is not under stopDir, returns startDir alone.
func walkUp(startDir, stopDir string) []string {
	startDir = filepath.Clean(startDir)
	stopDir = filepath.Clean(stopDir)

	var dirs []string
	dir := startDir
	for {
		dirs = append(dirs, dir)
		if dir == stopDir {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	return dirs
}
