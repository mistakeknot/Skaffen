package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLoadNoProjectDir(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ProjectDir() != "" {
		t.Errorf("ProjectDir = %q, want empty", cfg.ProjectDir())
	}
	if cfg.WorkDir() != dir {
		t.Errorf("WorkDir = %q, want %q", cfg.WorkDir(), dir)
	}
}

func TestLoadWithProjectDir(t *testing.T) {
	dir := t.TempDir()
	skaffenDir := filepath.Join(dir, ".skaffen")
	if err := os.Mkdir(skaffenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Work from a subdirectory
	subDir := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(subDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ProjectDir() != dir {
		t.Errorf("ProjectDir = %q, want %q", cfg.ProjectDir(), dir)
	}
}

func TestFindProjectRootWalkUp(t *testing.T) {
	root := t.TempDir()
	skaffenDir := filepath.Join(root, ".skaffen")
	if err := os.Mkdir(skaffenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// walkUpForDir should find .skaffen at root
	result := walkUpForDir(subDir, "/", ".skaffen")
	if result != root {
		t.Errorf("walkUpForDir = %q, want %q", result, root)
	}
}

func TestFindProjectRootWalkUpNotFound(t *testing.T) {
	dir := t.TempDir()
	result := walkUpForDir(dir, "/", ".skaffen")
	if result != "" {
		t.Errorf("walkUpForDir = %q, want empty", result)
	}
}

func TestFindProjectRootGitSkip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create a git repo without .skaffen/ — should NOT be accepted as project root
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	result := findProjectRoot(dir, "/tmp")
	if result != "" {
		t.Errorf("findProjectRoot (git without .skaffen/) = %q, want empty", result)
	}
}

func TestFindProjectRootGitWithSkaffen(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".skaffen"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Work from a subdirectory
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	result := findProjectRoot(subDir, "/tmp")
	if result != dir {
		t.Errorf("findProjectRoot (git with .skaffen/) = %q, want %q", result, dir)
	}
}

func TestRoutingPaths(t *testing.T) {
	root := t.TempDir()

	// Create user dir with routing.json
	userDir := filepath.Join(root, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "routing.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create project dir with .skaffen/routing.json
	projDir := filepath.Join(root, "project")
	projSkaffen := filepath.Join(projDir, ".skaffen")
	if err := os.MkdirAll(projSkaffen, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projSkaffen, "routing.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		userDir:    userDir,
		projectDir: projDir,
		workDir:    projDir,
	}

	paths := cfg.RoutingPaths()
	if len(paths) != 2 {
		t.Fatalf("RoutingPaths = %v, want 2 paths", paths)
	}
	if paths[0] != filepath.Join(userDir, "routing.json") {
		t.Errorf("paths[0] = %q, want user routing", paths[0])
	}
	if paths[1] != filepath.Join(projSkaffen, "routing.json") {
		t.Errorf("paths[1] = %q, want project routing", paths[1])
	}
}

func TestRoutingPathsUserOnly(t *testing.T) {
	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "routing.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		userDir:    userDir,
		projectDir: "", // no project
		workDir:    root,
	}

	paths := cfg.RoutingPaths()
	if len(paths) != 1 {
		t.Fatalf("RoutingPaths = %v, want 1 path", paths)
	}
}

func TestRoutingPathsNone(t *testing.T) {
	cfg := &Config{
		userDir:    "/nonexistent/user",
		projectDir: "",
		workDir:    "/tmp",
	}

	paths := cfg.RoutingPaths()
	if len(paths) != 0 {
		t.Fatalf("RoutingPaths = %v, want 0 paths", paths)
	}
}

func TestPluginPaths(t *testing.T) {
	root := t.TempDir()

	userDir := filepath.Join(root, "user")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "plugins.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	projDir := filepath.Join(root, "project")
	projSkaffen := filepath.Join(projDir, ".skaffen")
	if err := os.MkdirAll(projSkaffen, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projSkaffen, "plugins.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		userDir:    userDir,
		projectDir: projDir,
		workDir:    projDir,
	}

	paths := cfg.PluginPaths()
	if len(paths) != 2 {
		t.Fatalf("PluginPaths = %v, want 2 paths", paths)
	}
}

func TestSessionDirAlwaysUserGlobal(t *testing.T) {
	cfg := &Config{
		userDir:    "/home/test/.skaffen",
		projectDir: "/projects/myapp",
		workDir:    "/projects/myapp",
	}

	sessionDir := cfg.SessionDir()
	if sessionDir != "/home/test/.skaffen/sessions" {
		t.Errorf("SessionDir = %q, want user-global sessions dir", sessionDir)
	}

	evidenceDir := cfg.EvidenceDir()
	if evidenceDir != "/home/test/.skaffen/evidence" {
		t.Errorf("EvidenceDir = %q, want user-global evidence dir", evidenceDir)
	}
}

func TestCommandDirs(t *testing.T) {
	root := t.TempDir()

	userDir := filepath.Join(root, "user")
	userCmds := filepath.Join(userDir, "commands")
	if err := os.MkdirAll(userCmds, 0o755); err != nil {
		t.Fatal(err)
	}

	projDir := filepath.Join(root, "project")
	projCmds := filepath.Join(projDir, ".skaffen", "commands")
	if err := os.MkdirAll(projCmds, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		userDir:    userDir,
		projectDir: projDir,
		workDir:    projDir,
	}

	dirs := cfg.CommandDirs()
	if len(dirs) != 2 {
		t.Fatalf("CommandDirs = %v, want 2 dirs", dirs)
	}
	if dirs[0] != userCmds {
		t.Errorf("dirs[0] = %q, want user commands dir", dirs[0])
	}
	if dirs[1] != projCmds {
		t.Errorf("dirs[1] = %q, want project commands dir", dirs[1])
	}
}

func TestCommandDirsNone(t *testing.T) {
	cfg := &Config{
		userDir:    "/nonexistent/user",
		projectDir: "",
		workDir:    "/tmp",
	}
	dirs := cfg.CommandDirs()
	if len(dirs) != 0 {
		t.Fatalf("CommandDirs = %v, want 0 dirs", dirs)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !fileExists(f) {
		t.Error("fileExists should return true for existing file")
	}
	if fileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("fileExists should return false for missing file")
	}
	if fileExists(dir) {
		t.Error("fileExists should return false for directory")
	}
}

func TestDirExists(t *testing.T) {
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Error("dirExists should return true for existing dir")
	}
	if dirExists(filepath.Join(dir, "nope")) {
		t.Error("dirExists should return false for missing dir")
	}
}
