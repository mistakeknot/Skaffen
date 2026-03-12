package contextfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFindsClaudeMD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project\nBe helpful."), 0644); err != nil {
		t.Fatal(err)
	}

	result := Load(dir)
	if !strings.Contains(result, "Be helpful.") {
		t.Fatalf("expected CLAUDE.md content, got: %s", result)
	}
	if !strings.Contains(result, "# Context:") {
		t.Fatal("expected context header")
	}
}

func TestLoadFindsAgentsMD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Agents\nUse tools."), 0644); err != nil {
		t.Fatal(err)
	}

	result := Load(dir)
	if !strings.Contains(result, "Use tools.") {
		t.Fatalf("expected AGENTS.md content, got: %s", result)
	}
}

func TestLoadHierarchical(t *testing.T) {
	// Create parent/child directories with context files
	parent := t.TempDir()
	child := filepath.Join(parent, "subproject")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(parent, "CLAUDE.md"), []byte("parent instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "CLAUDE.md"), []byte("child instructions"), 0644); err != nil {
		t.Fatal(err)
	}

	// Override HOME so walkUp stops at parent
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", parent)
	defer os.Setenv("HOME", oldHome)

	result := Load(child)

	// Parent should appear before child (outermost first)
	parentIdx := strings.Index(result, "parent instructions")
	childIdx := strings.Index(result, "child instructions")

	if parentIdx < 0 || childIdx < 0 {
		t.Fatalf("expected both parent and child content, got: %s", result)
	}
	if parentIdx >= childIdx {
		t.Fatal("parent instructions should appear before child (outermost first)")
	}
}

func TestLoadEmptyDir(t *testing.T) {
	dir := t.TempDir()

	// Override HOME to the same dir so we don't walk real filesystem
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	result := Load(dir)
	if result != "" {
		t.Fatalf("expected empty result for dir with no context files, got: %s", result)
	}
}

func TestLoadSkipsEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("   \n  "), 0644); err != nil {
		t.Fatal(err)
	}

	// Override HOME
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	result := Load(dir)
	if result != "" {
		t.Fatalf("expected empty result for whitespace-only files, got: %s", result)
	}
}

func TestLoadSeparator(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}

	result := Load(dir)
	if !strings.Contains(result, "---") {
		t.Fatal("expected separator between sections")
	}
}

func TestWalkUpStopsAtHome(t *testing.T) {
	home := "/home/user"
	dirs := walkUp("/home/user/projects/foo", home)

	expected := []string{
		"/home/user/projects/foo",
		"/home/user/projects",
		"/home/user",
	}
	if len(dirs) != len(expected) {
		t.Fatalf("expected %d dirs, got %d: %v", len(expected), len(dirs), dirs)
	}
	for i, d := range dirs {
		if d != expected[i] {
			t.Errorf("dirs[%d] = %q, want %q", i, d, expected[i])
		}
	}
}

func TestWalkUpStartAtHome(t *testing.T) {
	dirs := walkUp("/home/user", "/home/user")
	if len(dirs) != 1 || dirs[0] != "/home/user" {
		t.Fatalf("expected [/home/user], got %v", dirs)
	}
}

func TestWalkUpOutsideHome(t *testing.T) {
	// startDir is not under home — should just return startDir up to root
	dirs := walkUp("/tmp/work", "/home/user")
	// Should walk all the way up since /tmp/work is not under /home/user
	found := false
	for _, d := range dirs {
		if d == "/tmp/work" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected startDir in result")
	}
}
