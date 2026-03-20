package tmuxagent

import (
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeAdapter steers Claude Code via tmux.
type ClaudeAdapter struct {
	binaryPath string
}

func init() {
	RegisterAdapter(&ClaudeAdapter{})
}

func (a *ClaudeAdapter) Name() string { return "claude-code" }

func (a *ClaudeAdapter) SpawnCmd(workDir string, cfg AgentConfig) (string, []string) {
	bin := a.binary()
	args := []string{
		"--verbose",
	}
	if cfg.PermissionMode != "" {
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	args = append(args, cfg.ExtraArgs...)
	return bin, args
}

func (a *ClaudeAdapter) ResumeCmd(sessionID string, workDir string, cfg AgentConfig) (string, []string) {
	bin := a.binary()
	args := []string{
		"--resume", sessionID,
		"--verbose",
	}
	if cfg.PermissionMode != "" {
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	args = append(args, cfg.ExtraArgs...)
	return bin, args
}

func (a *ClaudeAdapter) FormatPrompt(prompt string) string {
	// Claude Code's interactive mode accepts plain text via send-keys.
	return prompt
}

func (a *ClaudeAdapter) SessionDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

func (a *ClaudeAdapter) CassConnector() string { return "claude_code" }
func (a *ClaudeAdapter) SupportsResume() bool  { return true }

func (a *ClaudeAdapter) binary() string {
	if a.binaryPath != "" {
		return a.binaryPath
	}
	return "claude"
}

// FindLatestSession scans Claude Code's session directory for the most
// recent JSONL file matching a tmux session name.
func FindLatestSession(sessionName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}

	// Claude Code stores sessions under ~/.claude/projects/<hash>/
	// We scan for the most recently modified .jsonl file.
	base := filepath.Join(home, ".claude", "projects")
	var newest string
	var newestTime int64

	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".jsonl" && info.ModTime().Unix() > newestTime {
			newest = path
			newestTime = info.ModTime().Unix()
		}
		return nil
	})

	if newest == "" {
		return "", fmt.Errorf("no session files found under %s", base)
	}
	return newest, nil
}
