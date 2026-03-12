package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDir_TemplateCommand(t *testing.T) {
	dir := t.TempDir()
	toml := `description = "Review current changes"
type = "template"
template = "Please review the git diff and suggest improvements."
`
	os.WriteFile(filepath.Join(dir, "review.toml"), []byte(toml), 0o644)

	defs := LoadDir(dir, "user")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	d := defs[0]
	if d.Name != "review" {
		t.Errorf("Name = %q, want review", d.Name)
	}
	if d.Type != TypeTemplate {
		t.Errorf("Type = %q, want template", d.Type)
	}
	if d.Template == "" {
		t.Error("Template is empty")
	}
	if d.Description != "Review current changes" {
		t.Errorf("Description = %q", d.Description)
	}
	if d.Source != "user" {
		t.Errorf("Source = %q, want user", d.Source)
	}
}

func TestLoadDir_ScriptCommand(t *testing.T) {
	dir := t.TempDir()
	toml := `description = "Run tests"
type = "script"
script = "go test ./..."
`
	os.WriteFile(filepath.Join(dir, "test.toml"), []byte(toml), 0o644)

	defs := LoadDir(dir, "project")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	if defs[0].Type != TypeScript {
		t.Errorf("Type = %q, want script", defs[0].Type)
	}
	if defs[0].Script != "go test ./..." {
		t.Errorf("Script = %q", defs[0].Script)
	}
}

func TestLoadDir_DefaultType(t *testing.T) {
	dir := t.TempDir()
	toml := `template = "Hello world"
`
	os.WriteFile(filepath.Join(dir, "greet.toml"), []byte(toml), 0o644)

	defs := LoadDir(dir, "user")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	if defs[0].Type != TypeTemplate {
		t.Errorf("Type = %q, want template (default)", defs[0].Type)
	}
	if defs[0].Description != "Custom template command" {
		t.Errorf("Description = %q, want default", defs[0].Description)
	}
}

func TestLoadDir_MissingDir(t *testing.T) {
	defs := LoadDir("/nonexistent/commands", "user")
	if len(defs) != 0 {
		t.Errorf("got %d defs, want 0 for missing dir", len(defs))
	}
}

func TestLoadDir_SkipsBadFiles(t *testing.T) {
	dir := t.TempDir()
	// Valid command
	os.WriteFile(filepath.Join(dir, "good.toml"), []byte(`template = "works"`), 0o644)
	// Invalid TOML
	os.WriteFile(filepath.Join(dir, "bad.toml"), []byte(`{invalid`), 0o644)
	// Missing template
	os.WriteFile(filepath.Join(dir, "empty.toml"), []byte(`type = "template"`), 0o644)
	// Non-TOML file
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0o644)

	defs := LoadDir(dir, "user")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1 (only good.toml)", len(defs))
	}
	if defs[0].Name != "good" {
		t.Errorf("Name = %q, want good", defs[0].Name)
	}
}

func TestLoadDir_UnknownType(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.toml"), []byte(`type = "python"
template = "x"
`), 0o644)

	defs := LoadDir(dir, "user")
	if len(defs) != 0 {
		t.Errorf("got %d defs, want 0 (unknown type should be skipped)", len(defs))
	}
}

func TestLoadDir_ScriptMissingScript(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.toml"), []byte(`type = "script"
`), 0o644)

	defs := LoadDir(dir, "user")
	if len(defs) != 0 {
		t.Errorf("got %d defs, want 0 (script without script field)", len(defs))
	}
}

func TestLoadAll_MergesProjectOverUser(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()

	// User has "review" and "deploy"
	os.WriteFile(filepath.Join(userDir, "review.toml"), []byte(`
description = "User review"
template = "user review template"
`), 0o644)
	os.WriteFile(filepath.Join(userDir, "deploy.toml"), []byte(`
description = "Deploy to prod"
type = "script"
script = "deploy.sh"
`), 0o644)

	// Project overrides "review"
	os.WriteFile(filepath.Join(projDir, "review.toml"), []byte(`
description = "Project review"
template = "project review template"
`), 0o644)

	merged := LoadAll(userDir, projDir)
	if len(merged) != 2 {
		t.Fatalf("got %d defs, want 2", len(merged))
	}

	// "review" should be project version
	review := merged["review"]
	if review.Source != "project" {
		t.Errorf("review.Source = %q, want project", review.Source)
	}
	if review.Description != "Project review" {
		t.Errorf("review.Description = %q, want Project review", review.Description)
	}

	// "deploy" should be user version
	deploy := merged["deploy"]
	if deploy.Source != "user" {
		t.Errorf("deploy.Source = %q, want user", deploy.Source)
	}
}

func TestLoadAll_EmptyDirs(t *testing.T) {
	merged := LoadAll("/nonexistent/a", "/nonexistent/b")
	if len(merged) != 0 {
		t.Errorf("got %d defs, want 0", len(merged))
	}
}

func TestLoadDir_MultipleCommands(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "alpha.toml"), []byte(`template = "a"`), 0o644)
	os.WriteFile(filepath.Join(dir, "beta.toml"), []byte(`template = "b"`), 0o644)
	os.WriteFile(filepath.Join(dir, "gamma.toml"), []byte(`
type = "script"
script = "echo gamma"
`), 0o644)

	defs := LoadDir(dir, "user")
	if len(defs) != 3 {
		t.Fatalf("got %d defs, want 3", len(defs))
	}
}
