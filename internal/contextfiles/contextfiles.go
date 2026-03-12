// Package contextfiles loads project context files (CLAUDE.md, AGENTS.md)
// from the working directory upward to the user's home directory, producing
// a combined system prompt prefix.
package contextfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultFileNames lists the context files to look for at each directory level.
var DefaultFileNames = []string{"CLAUDE.md", "AGENTS.md"}

// Load walks from startDir up to the user's home directory (or filesystem root
// if HOME is unset), collecting context files at each level. Files are returned
// outermost-first (home → project) so that project-level instructions appear
// last and take precedence.
//
// Each file's content is wrapped with a header showing its path:
//
//	# Context: /path/to/CLAUDE.md
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
		for _, name := range DefaultFileNames {
			path := filepath.Join(dirs[i], name)
			content, err := os.ReadFile(path)
			if err != nil {
				continue // file doesn't exist or unreadable
			}
			text := strings.TrimSpace(string(content))
			if text == "" {
				continue
			}
			sections = append(sections, fmt.Sprintf("# Context: %s\n\n%s", path, text))
		}
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n---\n\n")
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
