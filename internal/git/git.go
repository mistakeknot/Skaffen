package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Git provides git operations for the Skaffen agent.
type Git struct {
	workDir string
}

// New creates a Git helper for the given working directory.
func New(workDir string) *Git {
	return &Git{workDir: workDir}
}

// Status returns the git status output.
func (g *Git) Status() (string, error) {
	return g.run("status", "--porcelain")
}

// HasChanges returns true if the working directory has uncommitted changes.
func (g *Git) HasChanges() (bool, error) {
	out, err := g.Status()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// AutoCommit stages all changes and commits with the given message.
func (g *Git) AutoCommit(message string) error {
	if _, err := g.run("add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if _, err := g.run("commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// Undo reverts the last commit but keeps changes staged.
func (g *Git) Undo() error {
	_, err := g.run("reset", "--soft", "HEAD~1")
	return err
}

// Diff returns the current diff (staged + unstaged).
func (g *Git) Diff() (string, error) {
	return g.run("diff", "HEAD")
}

// DiffStaged returns only staged changes.
func (g *Git) DiffStaged() (string, error) {
	return g.run("diff", "--staged")
}

// CurrentBranch returns the current branch name.
func (g *Git) CurrentBranch() (string, error) {
	out, err := g.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// LastCommitMessage returns the last commit's message.
func (g *Git) LastCommitMessage() (string, error) {
	out, err := g.run("log", "-1", "--format=%s")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Push pushes the current branch to origin.
func (g *Git) Push() error {
	_, err := g.run("push")
	return err
}

// IsRepo returns true if the working directory is inside a git repository.
func (g *Git) IsRepo() bool {
	_, err := g.run("rev-parse", "--git-dir")
	return err == nil
}

func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
