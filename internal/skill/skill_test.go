package skill

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkill creates a skill directory with a SKILL.md file.
func writeSkill(t *testing.T, dir, name, content string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDir_BasicSkill(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "greet", `---
name: greet
description: Say hello
---
You are a greeting bot.
`)

	defs := LoadDir(dir, "project")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	d := defs[0]
	if d.Name != "greet" {
		t.Errorf("Name = %q, want %q", d.Name, "greet")
	}
	if d.Description != "Say hello" {
		t.Errorf("Description = %q, want %q", d.Description, "Say hello")
	}
	if !d.UserInvocable {
		t.Error("UserInvocable should default to true")
	}
	if d.Source != "project" {
		t.Errorf("Source = %q, want %q", d.Source, "project")
	}
	if d.Body != "" {
		t.Error("Body should be empty before LoadBody")
	}
}

func TestLoadDir_RichFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "deploy", `---
name: deploy
description: Deploy to production
user_invocable: false
triggers:
  - deploy this
  - ship it
args: "<env> [--dry-run]"
model: opus
---
Deployment instructions here.
`)

	defs := LoadDir(dir, "project")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	d := defs[0]
	if d.UserInvocable {
		t.Error("UserInvocable should be false")
	}
	if len(d.Triggers) != 2 {
		t.Fatalf("got %d triggers, want 2", len(d.Triggers))
	}
	if d.Triggers[0] != "deploy this" || d.Triggers[1] != "ship it" {
		t.Errorf("Triggers = %v", d.Triggers)
	}
	if d.Args != "<env> [--dry-run]" {
		t.Errorf("Args = %q", d.Args)
	}
	if d.Model != "opus" {
		t.Errorf("Model = %q", d.Model)
	}
}

func TestLoadDir_MissingDir(t *testing.T) {
	defs := LoadDir("/nonexistent/path/that/does/not/exist", "user")
	if len(defs) != 0 {
		t.Fatalf("got %d defs, want 0 for missing dir", len(defs))
	}
}

func TestLoadDir_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "noname", `---
description: I have no name
---
Body text.
`)

	defs := LoadDir(dir, "project")
	if len(defs) != 0 {
		t.Fatalf("got %d defs, want 0 (missing name should be skipped)", len(defs))
	}
}

func TestLoadDir_MissingDescription(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "nodesc", `---
name: nodesc
---
Body text.
`)

	defs := LoadDir(dir, "project")
	if len(defs) != 0 {
		t.Fatalf("got %d defs, want 0 (missing description should be skipped)", len(defs))
	}
}

func TestLoadDir_BadFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "badyaml", `---
name: [this is not valid
description: broken
---
Body.
`)

	defs := LoadDir(dir, "project")
	if len(defs) != 0 {
		t.Fatalf("got %d defs, want 0 (bad YAML should be skipped)", len(defs))
	}
}

func TestLoadDir_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "nofm", `Just plain text, no frontmatter delimiters.`)

	defs := LoadDir(dir, "project")
	if len(defs) != 0 {
		t.Fatalf("got %d defs, want 0 (no frontmatter should be skipped)", len(defs))
	}
}

func TestLoadDir_MultipleSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha", `---
name: alpha
description: First skill
---
Alpha body.
`)
	writeSkill(t, dir, "beta", `---
name: beta
description: Second skill
---
Beta body.
`)

	defs := LoadDir(dir, "project")
	if len(defs) != 2 {
		t.Fatalf("got %d defs, want 2", len(defs))
	}

	names := map[string]bool{}
	for _, d := range defs {
		names[d.Name] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("expected alpha and beta, got %v", names)
	}
}

func TestLoadAll_Shadowing(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()

	writeSkill(t, userDir, "greet", `---
name: greet
description: User greeting
---
User version.
`)
	writeSkill(t, projDir, "greet", `---
name: greet
description: Project greeting
---
Project version.
`)

	// LoadAll sources: "user", "user-plugin", "project", "project-plugin".
	// Place projDir at index 2 ("project") so it shadows userDir at index 0 ("user").
	emptyA := t.TempDir()
	result := LoadAll(userDir, emptyA, projDir)
	if len(result) != 1 {
		t.Fatalf("got %d skills, want 1", len(result))
	}
	d, ok := result["greet"]
	if !ok {
		t.Fatal("missing 'greet' in result")
	}
	if d.Description != "Project greeting" {
		t.Errorf("Description = %q, want %q (project should shadow user)", d.Description, "Project greeting")
	}
	if d.Source != "project" {
		t.Errorf("Source = %q, want %q", d.Source, "project")
	}
}

func TestLoadBody_LazyLoad(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "lazy", `---
name: lazy
description: Lazy skill
---
This is the body content.
It spans multiple lines.
`)

	defs := LoadDir(dir, "project")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	d := defs[0]

	// Body should be empty before LoadBody.
	if d.Body != "" {
		t.Errorf("Body should be empty before LoadBody, got %q", d.Body)
	}

	// First call loads body.
	body, err := LoadBody(&d)
	if err != nil {
		t.Fatalf("LoadBody: %v", err)
	}
	expected := "This is the body content.\nIt spans multiple lines.\n"
	if body != expected {
		t.Errorf("body = %q, want %q", body, expected)
	}
	if d.Body != expected {
		t.Errorf("d.Body should be cached, got %q", d.Body)
	}

	// Second call returns cached value (even if file were deleted).
	os.RemoveAll(filepath.Join(dir, "lazy"))
	body2, err := LoadBody(&d)
	if err != nil {
		t.Fatalf("LoadBody (cached): %v", err)
	}
	if body2 != expected {
		t.Errorf("cached body = %q, want %q", body2, expected)
	}
}
