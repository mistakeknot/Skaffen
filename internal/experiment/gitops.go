package experiment

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// secretPatterns are filenames that must never be committed.
var secretPatterns = []string{
	".env", "*.env", "*.pem", "*.key", "*_rsa", "*.p12",
}

// GitOps provides git worktree operations for autoresearch experiment isolation.
type GitOps struct {
	repoDir      string // original repo working directory
	worktreeBase string // base dir for worktrees (~/.skaffen/worktrees/)
}

// NewGitOps creates a GitOps for the given repo. Worktrees are created under
// ~/.skaffen/worktrees/ (or the provided base, for testing).
func NewGitOps(repoDir, worktreeBase string) *GitOps {
	return &GitOps{
		repoDir:      repoDir,
		worktreeBase: worktreeBase,
	}
}

// DefaultWorktreeBase returns ~/.skaffen/worktrees/.
func DefaultWorktreeBase() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("default worktree base: %w", err)
	}
	return filepath.Join(home, ".skaffen", "worktrees"), nil
}

// WorktreeDir returns the worktree path for a campaign.
func (g *GitOps) WorktreeDir(campaignName string) string {
	return filepath.Join(g.worktreeBase, campaignName)
}

// CreateWorktree creates or reuses a git worktree for the campaign.
// On reuse (worktree already exists), cleans to known state (crash-recovery contract).
func (g *GitOps) CreateWorktree(campaignName string) error {
	if err := os.MkdirAll(g.worktreeBase, 0700); err != nil {
		return fmt.Errorf("create worktree base: %w", err)
	}

	wtDir := g.WorktreeDir(campaignName)
	branch := "autoresearch/" + campaignName

	if g.hasWorktreeEntry(campaignName) {
		// Reuse existing worktree — clean to known state (crash recovery)
		if _, err := g.runIn(wtDir, "reset", "HEAD"); err != nil {
			// Reset may fail if HEAD is detached; not fatal
			fmt.Fprintf(os.Stderr, "autoresearch: worktree reset: %v\n", err)
		}
		if _, err := g.runIn(wtDir, "clean", "-fd"); err != nil {
			return fmt.Errorf("clean worktree: %w", err)
		}
		if _, err := g.runIn(wtDir, "checkout", "--", "."); err != nil {
			return fmt.Errorf("checkout worktree: %w", err)
		}
		return nil
	}

	// Create new worktree
	_, err := g.runIn(g.repoDir, "worktree", "add", wtDir, "-b", branch)
	if err != nil {
		// Branch may already exist from a previous incomplete run
		if strings.Contains(err.Error(), "already exists") {
			_, err = g.runIn(g.repoDir, "worktree", "add", wtDir, branch)
			if err != nil {
				return fmt.Errorf("create worktree (existing branch): %w", err)
			}
			return nil
		}
		return fmt.Errorf("create worktree: %w", err)
	}
	return nil
}

// KeepChanges stages all changes, checks for secret files, and commits.
// Returns the commit SHA.
func (g *GitOps) KeepChanges(campaignName, hypothesis string, delta float64) (string, error) {
	wtDir := g.WorktreeDir(campaignName)

	// Stage all changes
	if _, err := g.runIn(wtDir, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}

	// Secret file check: reject if any staged file matches secret patterns
	staged, err := g.runIn(wtDir, "diff", "--cached", "--name-only")
	if err != nil {
		return "", fmt.Errorf("check staged files: %w", err)
	}

	for _, file := range strings.Split(strings.TrimSpace(staged), "\n") {
		if file == "" {
			continue
		}
		base := filepath.Base(file)
		for _, pattern := range secretPatterns {
			if matched, _ := filepath.Match(pattern, base); matched {
				// Unstage the secret file
				g.runIn(wtDir, "reset", "HEAD", "--", file)
				return "", fmt.Errorf("secret file detected: %q matches pattern %q — refusing to commit", file, pattern)
			}
		}
	}

	// Commit
	msg := fmt.Sprintf("experiment(%s): %s [%+.4f]", campaignName, hypothesis, delta)
	if _, err := g.runIn(wtDir, "commit", "-m", msg); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}

	// Get commit SHA
	sha, err := g.runIn(wtDir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get commit SHA: %w", err)
	}
	return strings.TrimSpace(sha), nil
}

// DiscardChanges reverts all tracked changes AND removes untracked files.
func (g *GitOps) DiscardChanges(campaignName string) error {
	wtDir := g.WorktreeDir(campaignName)

	// Remove untracked files and directories
	if _, err := g.runIn(wtDir, "clean", "-fd"); err != nil {
		return fmt.Errorf("git clean: %w", err)
	}

	// Revert tracked file changes
	if _, err := g.runIn(wtDir, "checkout", "--", "."); err != nil {
		return fmt.Errorf("git checkout: %w", err)
	}

	return nil
}

// CurrentSHA returns the HEAD commit SHA in the worktree.
func (g *GitOps) CurrentSHA(campaignName string) (string, error) {
	wtDir := g.WorktreeDir(campaignName)
	out, err := g.runIn(wtDir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("current SHA: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// RemoveWorktree removes the worktree and optionally the branch.
func (g *GitOps) RemoveWorktree(campaignName string) error {
	wtDir := g.WorktreeDir(campaignName)
	_, err := g.runIn(g.repoDir, "worktree", "remove", "--force", wtDir)
	if err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

// HasWorktree checks if a worktree exists for the campaign via git worktree list.
func (g *GitOps) HasWorktree(campaignName string) bool {
	return g.hasWorktreeEntry(campaignName)
}

// hasWorktreeEntry checks git worktree list output for the campaign path.
func (g *GitOps) hasWorktreeEntry(campaignName string) bool {
	wtDir := g.WorktreeDir(campaignName)
	out, err := g.runIn(g.repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	return strings.Contains(out, "worktree "+wtDir)
}

func (g *GitOps) runIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
