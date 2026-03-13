package skill

import (
	"os"
	"path/filepath"
	"strings"
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

// --- Injection tests ---

func TestFormatInjection_Basic(t *testing.T) {
	d := &Def{
		Name:        "test-skill",
		Description: "A test skill",
		Path:        "/fake/path/SKILL.md",
		Body:        "Do the thing.\nSecond line.\n",
	}

	msg := FormatInjection(d, "")
	if !strings.Contains(msg, "test-skill") {
		t.Error("injection should contain skill name")
	}
	if !strings.Contains(msg, "Do the thing.") {
		t.Error("injection should contain skill body")
	}
}

func TestFormatInjection_WithArgs(t *testing.T) {
	d := &Def{
		Name: "review",
		Body: "Review the code.\n",
	}

	msg := FormatInjection(d, "src/main.go")
	if !strings.Contains(msg, "src/main.go") {
		t.Error("injection should contain user arguments")
	}
	if !strings.Contains(msg, "Review the code.") {
		t.Error("injection should contain body")
	}
}

func TestFormatInjection_SizeLimit(t *testing.T) {
	d := &Def{
		Name: "huge",
		Body: strings.Repeat("x", MaxBodyChars+1),
	}

	_, err := FormatInjectionSafe(d, "")
	if err == nil {
		t.Error("expected error for oversized body")
	}
}

func TestFormatInjection_EmptyBody(t *testing.T) {
	d := &Def{
		Name:        "empty",
		Description: "Empty skill",
	}

	msg := FormatInjection(d, "")
	// Should still produce a message (metadata tags, just no body)
	if msg == "" {
		t.Error("injection should produce output even with empty body")
	}
}

// --- Trigger matching tests ---

func TestMatchTriggers_SingleMatch(t *testing.T) {
	skills := map[string]Def{
		"review": {
			Name:          "review",
			UserInvocable: true,
			Triggers:      []string{"review my code", "check changes"},
		},
		"deploy": {
			Name:          "deploy",
			UserInvocable: true,
			Triggers:      []string{"deploy to prod"},
		},
	}

	matched := MatchTriggers(skills, "can you review my code please?")
	if len(matched) != 1 {
		t.Fatalf("got %d matches, want 1", len(matched))
	}
	if matched[0].Name != "review" {
		t.Errorf("matched %q, want review", matched[0].Name)
	}
}

func TestMatchTriggers_MultiMatch(t *testing.T) {
	skills := map[string]Def{
		"review": {
			Name:          "review",
			UserInvocable: true,
			Triggers:      []string{"review"},
		},
		"test": {
			Name:          "test",
			UserInvocable: true,
			Triggers:      []string{"review"},
		},
	}

	matched := MatchTriggers(skills, "please review this")
	if len(matched) != 2 {
		t.Fatalf("got %d matches, want 2", len(matched))
	}
}

func TestMatchTriggers_CaseInsensitive(t *testing.T) {
	skills := map[string]Def{
		"review": {
			Name:          "review",
			UserInvocable: true,
			Triggers:      []string{"Review My Code"},
		},
	}

	matched := MatchTriggers(skills, "review my code")
	if len(matched) != 1 {
		t.Fatalf("got %d matches, want 1 (case insensitive)", len(matched))
	}
}

func TestMatchTriggers_NoMatch(t *testing.T) {
	skills := map[string]Def{
		"review": {
			Name:          "review",
			UserInvocable: true,
			Triggers:      []string{"review my code"},
		},
	}

	matched := MatchTriggers(skills, "deploy to production")
	if len(matched) != 0 {
		t.Errorf("got %d matches, want 0", len(matched))
	}
}

func TestMatchTriggers_SkipsNonInvocable(t *testing.T) {
	skills := map[string]Def{
		"internal": {
			Name:          "internal",
			UserInvocable: false,
			Triggers:      []string{"do something"},
		},
	}

	matched := MatchTriggers(skills, "do something")
	if len(matched) != 0 {
		t.Errorf("got %d matches, want 0 (user_invocable=false should be skipped)", len(matched))
	}
}

func TestMatchTriggers_NoTriggers(t *testing.T) {
	skills := map[string]Def{
		"manual": {
			Name:          "manual",
			UserInvocable: true,
			Triggers:      nil,
		},
	}

	matched := MatchTriggers(skills, "anything at all")
	if len(matched) != 0 {
		t.Errorf("got %d matches, want 0 (no triggers defined)", len(matched))
	}
}

// --- Pinner tests ---

func TestPinner_PinUnpin(t *testing.T) {
	skills := map[string]Def{
		"review": {Name: "review"},
		"deploy": {Name: "deploy"},
	}
	p := NewPinner(skills)

	// Pin
	if err := p.Pin("review"); err != nil {
		t.Fatalf("Pin error: %v", err)
	}
	pinned := p.Pinned()
	if len(pinned) != 1 || pinned[0] != "review" {
		t.Errorf("Pinned = %v, want [review]", pinned)
	}

	// Unpin
	p.Unpin("review")
	if len(p.Pinned()) != 0 {
		t.Error("Pinned should be empty after unpin")
	}
}

func TestPinner_DuplicatePin(t *testing.T) {
	skills := map[string]Def{"review": {Name: "review"}}
	p := NewPinner(skills)

	p.Pin("review")
	p.Pin("review") // duplicate — should be a no-op
	if len(p.Pinned()) != 1 {
		t.Error("duplicate pin should not add twice")
	}
}

func TestPinner_PinNonExistent(t *testing.T) {
	skills := map[string]Def{"review": {Name: "review"}}
	p := NewPinner(skills)

	err := p.Pin("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent skill")
	}
}

func TestPinner_UnpinNonExistent(t *testing.T) {
	skills := map[string]Def{"review": {Name: "review"}}
	p := NewPinner(skills)

	// Should not panic
	p.Unpin("nonexistent")
}

func TestPinner_MultiplePins(t *testing.T) {
	skills := map[string]Def{
		"review": {Name: "review"},
		"deploy": {Name: "deploy"},
		"test":   {Name: "test"},
	}
	p := NewPinner(skills)

	p.Pin("review")
	p.Pin("deploy")

	pinned := p.Pinned()
	if len(pinned) != 2 {
		t.Fatalf("got %d pinned, want 2", len(pinned))
	}
}
