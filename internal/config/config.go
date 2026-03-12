// Package config discovers and resolves Skaffen configuration from
// user-global (~/.skaffen/) and per-project (.skaffen/) directories.
//
// Config hierarchy (lowest → highest precedence):
//  1. User-global: ~/.skaffen/
//  2. Per-project: .skaffen/ at git root or nearest ancestor
//  3. CLI flags (applied by caller after Load)
package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Config holds resolved paths for user-global and per-project configuration.
type Config struct {
	userDir    string // ~/.skaffen
	projectDir string // project root containing .skaffen/, empty if none
	workDir    string // current working directory
}

// Load discovers user-global and per-project config directories.
func Load(workDir string) (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return &Config{
		userDir:    filepath.Join(home, ".skaffen"),
		projectDir: findProjectRoot(workDir, home),
		workDir:    workDir,
	}, nil
}

// RoutingPaths returns routing config paths to load, ordered user-global first.
// Load both and merge with router.MergeConfig (first as base, second as overlay).
// Returns only paths that exist on disk.
func (c *Config) RoutingPaths() []string {
	var paths []string
	userPath := filepath.Join(c.userDir, "routing.json")
	if fileExists(userPath) {
		paths = append(paths, userPath)
	}
	if c.projectDir != "" {
		projPath := filepath.Join(c.projectDir, ".skaffen", "routing.json")
		if fileExists(projPath) {
			paths = append(paths, projPath)
		}
	}
	return paths
}

// PluginPaths returns plugin config paths to load (user-global + per-project).
// Both are loaded; per-project plugins merge with user-global.
// Returns only paths that exist on disk.
func (c *Config) PluginPaths() []string {
	var paths []string
	userPath := filepath.Join(c.userDir, "plugins.toml")
	if fileExists(userPath) {
		paths = append(paths, userPath)
	}
	if c.projectDir != "" {
		projPath := filepath.Join(c.projectDir, ".skaffen", "plugins.toml")
		if fileExists(projPath) {
			paths = append(paths, projPath)
		}
	}
	return paths
}

// CommandDirs returns command directories to scan for custom slash commands.
// Returns user-global first, then per-project (for merge precedence).
// Returns only directories that exist on disk.
func (c *Config) CommandDirs() []string {
	var dirs []string
	userDir := filepath.Join(c.userDir, "commands")
	if dirExists(userDir) {
		dirs = append(dirs, userDir)
	}
	if c.projectDir != "" {
		projDir := filepath.Join(c.projectDir, ".skaffen", "commands")
		if dirExists(projDir) {
			dirs = append(dirs, projDir)
		}
	}
	return dirs
}

// SessionDir returns the user-global sessions directory (always ~/.skaffen/sessions).
func (c *Config) SessionDir() string { return filepath.Join(c.userDir, "sessions") }

// EvidenceDir returns the user-global evidence directory (always ~/.skaffen/evidence).
func (c *Config) EvidenceDir() string { return filepath.Join(c.userDir, "evidence") }

// ProjectDir returns the project root (parent of .skaffen/), empty if none found.
func (c *Config) ProjectDir() string { return c.projectDir }

// WorkDir returns the working directory used for config resolution.
func (c *Config) WorkDir() string { return c.workDir }

// UserDir returns the user-global config directory (~/.skaffen).
func (c *Config) UserDir() string { return c.userDir }

// findProjectRoot tries git root first (with 2s timeout), then walks up.
// Only accepts git root if .skaffen/ exists there.
func findProjectRoot(startDir, homeDir string) string {
	if root := gitRoot(startDir); root != "" {
		if dirExists(filepath.Join(root, ".skaffen")) {
			return root
		}
	}
	return walkUpForDir(startDir, homeDir, ".skaffen")
}

func gitRoot(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// walkUpForDir walks from startDir toward stopDir looking for a directory named target.
// Returns the parent directory containing target, or empty string if not found.
func walkUpForDir(startDir, stopDir, target string) string {
	dir := filepath.Clean(startDir)
	stopDir = filepath.Clean(stopDir)
	for {
		if dirExists(filepath.Join(dir, target)) {
			return dir
		}
		if dir == stopDir {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // filesystem root
		}
		dir = parent
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
