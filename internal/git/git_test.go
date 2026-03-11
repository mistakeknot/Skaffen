package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/Skaffen/internal/git"
)

func setupTestRepo(t *testing.T) (string, *git.Git) {
	t.Helper()
	dir := t.TempDir()
	g := git.New(dir)

	// Initialize a git repo
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create initial commit
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	return dir, g
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, out)
	}
}

func TestIsRepo(t *testing.T) {
	_, g := setupTestRepo(t)
	if !g.IsRepo() {
		t.Fatal("should detect git repo")
	}

	notRepo := git.New(t.TempDir())
	if notRepo.IsRepo() {
		t.Fatal("should not detect non-repo")
	}
}

func TestHasChanges(t *testing.T) {
	dir, g := setupTestRepo(t)

	// No changes initially
	has, err := g.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("should have no changes after clean commit")
	}

	// Create a file
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644)
	has, err = g.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("should detect new file")
	}
}

func TestAutoCommit(t *testing.T) {
	dir, g := setupTestRepo(t)

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	if err := g.AutoCommit("test commit"); err != nil {
		t.Fatal(err)
	}

	msg, err := g.LastCommitMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg != "test commit" {
		t.Fatalf("got %q, want %q", msg, "test commit")
	}
}

func TestUndo(t *testing.T) {
	dir, g := setupTestRepo(t)

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	g.AutoCommit("will undo")

	if err := g.Undo(); err != nil {
		t.Fatal(err)
	}

	// Changes should be back as staged
	has, _ := g.HasChanges()
	if !has {
		t.Fatal("should have staged changes after undo")
	}
}

func TestCurrentBranch(t *testing.T) {
	_, g := setupTestRepo(t)
	branch, err := g.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	// Could be "main" or "master" depending on git config
	if branch != "main" && branch != "master" {
		t.Fatalf("unexpected branch %q", branch)
	}
}
