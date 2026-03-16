package experiment

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a git repo in a temp dir with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %s", args, err, out)
		}
	}

	// Create initial file and commit
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmds = [][]string{
		{"git", "add", "README.md"},
		{"git", "commit", "-m", "initial commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %s", args, err, out)
		}
	}

	return dir
}

func TestGitOpsCreateAndRemoveWorktree(t *testing.T) {
	repo := initTestRepo(t)
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	ops := NewGitOps(repo, wtBase)

	// Create
	err := ops.CreateWorktree("test-campaign")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	wtDir := ops.WorktreeDir("test-campaign")
	if _, err := os.Stat(wtDir); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	// HasWorktree should return true
	if !ops.HasWorktree("test-campaign") {
		t.Error("HasWorktree = false, want true")
	}

	// README.md should exist in worktree
	if _, err := os.Stat(filepath.Join(wtDir, "README.md")); err != nil {
		t.Error("README.md not found in worktree")
	}

	// Remove
	err = ops.RemoveWorktree("test-campaign")
	if err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	if ops.HasWorktree("test-campaign") {
		t.Error("HasWorktree = true after remove, want false")
	}
}

func TestGitOpsKeepChanges(t *testing.T) {
	repo := initTestRepo(t)
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	ops := NewGitOps(repo, wtBase)
	ops.CreateWorktree("keep-test")

	// Make a change in the worktree
	wtDir := ops.WorktreeDir("keep-test")
	os.WriteFile(filepath.Join(wtDir, "new-file.go"), []byte("package main\n"), 0644)

	sha, err := ops.KeepChanges("keep-test", "add new file", 0.1)
	if err != nil {
		t.Fatalf("KeepChanges: %v", err)
	}
	if sha == "" {
		t.Error("expected non-empty SHA")
	}

	// Verify commit message
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = wtDir
	out, _ := cmd.Output()
	msg := strings.TrimSpace(string(out))
	if !strings.Contains(msg, "experiment(keep-test)") {
		t.Errorf("commit message = %q, want to contain 'experiment(keep-test)'", msg)
	}

	ops.RemoveWorktree("keep-test")
}

func TestGitOpsKeepChanges_RejectsSecretFile(t *testing.T) {
	repo := initTestRepo(t)
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	ops := NewGitOps(repo, wtBase)
	ops.CreateWorktree("secret-test")

	// Create a .env file in the worktree
	wtDir := ops.WorktreeDir("secret-test")
	os.WriteFile(filepath.Join(wtDir, ".env"), []byte("SECRET=value\n"), 0644)

	_, err := ops.KeepChanges("secret-test", "add config", 0.1)
	if err == nil {
		t.Fatal("expected error for .env file")
	}
	if !strings.Contains(err.Error(), "secret file detected") {
		t.Errorf("error = %v, want to contain 'secret file detected'", err)
	}

	ops.RemoveWorktree("secret-test")
}

func TestGitOpsDiscardChanges(t *testing.T) {
	repo := initTestRepo(t)
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	ops := NewGitOps(repo, wtBase)
	ops.CreateWorktree("discard-test")

	wtDir := ops.WorktreeDir("discard-test")

	// Modify tracked file
	os.WriteFile(filepath.Join(wtDir, "README.md"), []byte("# Modified\n"), 0644)

	// Create untracked file
	os.WriteFile(filepath.Join(wtDir, "untracked.txt"), []byte("should be removed\n"), 0644)

	err := ops.DiscardChanges("discard-test")
	if err != nil {
		t.Fatalf("DiscardChanges: %v", err)
	}

	// Tracked file should be reverted
	data, _ := os.ReadFile(filepath.Join(wtDir, "README.md"))
	if string(data) != "# Test\n" {
		t.Errorf("README.md = %q, want original content", string(data))
	}

	// Untracked file should be removed
	if _, err := os.Stat(filepath.Join(wtDir, "untracked.txt")); !os.IsNotExist(err) {
		t.Error("untracked.txt should be removed by DiscardChanges")
	}

	ops.RemoveWorktree("discard-test")
}

func TestGitOpsReuseWorktree_CleansState(t *testing.T) {
	repo := initTestRepo(t)
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	ops := NewGitOps(repo, wtBase)

	// Create and dirty the worktree
	ops.CreateWorktree("reuse-test")
	wtDir := ops.WorktreeDir("reuse-test")
	os.WriteFile(filepath.Join(wtDir, "dirty.txt"), []byte("dirty\n"), 0644)
	os.WriteFile(filepath.Join(wtDir, "README.md"), []byte("# Dirty\n"), 0644)

	// Simulate crash-recovery: create worktree again (reuse path)
	err := ops.CreateWorktree("reuse-test")
	if err != nil {
		t.Fatalf("CreateWorktree (reuse): %v", err)
	}

	// After reuse, worktree should be clean
	if _, err := os.Stat(filepath.Join(wtDir, "dirty.txt")); !os.IsNotExist(err) {
		t.Error("dirty.txt should be cleaned on reuse")
	}

	data, _ := os.ReadFile(filepath.Join(wtDir, "README.md"))
	if string(data) != "# Test\n" {
		t.Errorf("README.md = %q, want original content after reuse cleanup", string(data))
	}

	ops.RemoveWorktree("reuse-test")
}

func TestGitOpsCurrentSHA(t *testing.T) {
	repo := initTestRepo(t)
	wtBase := filepath.Join(t.TempDir(), "worktrees")
	ops := NewGitOps(repo, wtBase)
	ops.CreateWorktree("sha-test")

	sha, err := ops.CurrentSHA("sha-test")
	if err != nil {
		t.Fatalf("CurrentSHA: %v", err)
	}
	if len(sha) < 7 {
		t.Errorf("SHA = %q, expected at least 7 chars", sha)
	}

	ops.RemoveWorktree("sha-test")
}
