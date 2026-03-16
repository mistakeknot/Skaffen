package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMarkdownDirBasic(t *testing.T) {
	dir := t.TempDir()
	md := `---
name: review
description: Review current changes
---
Please review the git diff and suggest improvements.
`
	os.WriteFile(filepath.Join(dir, "review.md"), []byte(md), 0o644)

	defs := LoadMarkdownDir(dir, "plugin")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	d := defs[0]
	if d.Name != "review" {
		t.Errorf("Name = %q, want review", d.Name)
	}
	if d.Description != "Review current changes" {
		t.Errorf("Description = %q, want 'Review current changes'", d.Description)
	}
	if d.Type != TypeTemplate {
		t.Errorf("Type = %q, want template", d.Type)
	}
	if d.Template != "Please review the git diff and suggest improvements." {
		t.Errorf("Template = %q", d.Template)
	}
	if d.Source != "plugin" {
		t.Errorf("Source = %q, want plugin", d.Source)
	}
}

func TestLoadMarkdownDirNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	md := `Just a plain markdown template with no frontmatter.
`
	os.WriteFile(filepath.Join(dir, "simple-cmd.md"), []byte(md), 0o644)

	defs := LoadMarkdownDir(dir, "plugin")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	d := defs[0]
	if d.Name != "simple-cmd" {
		t.Errorf("Name = %q, want simple-cmd (derived from filename)", d.Name)
	}
	if d.Description != "Custom template command" {
		t.Errorf("Description = %q, want default description", d.Description)
	}
	if d.Template != "Just a plain markdown template with no frontmatter." {
		t.Errorf("Template = %q", d.Template)
	}
}

func TestLoadMarkdownDirMultiple(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "alpha.md"), []byte(`---
name: alpha
description: First command
---
Alpha template
`), 0o644)

	os.WriteFile(filepath.Join(dir, "beta.md"), []byte(`---
name: beta
description: Second command
---
Beta template
`), 0o644)

	os.WriteFile(filepath.Join(dir, "gamma.md"), []byte(`Gamma template without frontmatter`), 0o644)

	defs := LoadMarkdownDir(dir, "plugin")
	if len(defs) != 3 {
		t.Fatalf("got %d defs, want 3", len(defs))
	}

	// Verify we got all three names (order depends on readdir)
	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !names[want] {
			t.Errorf("missing command %q", want)
		}
	}
}

func TestLoadMarkdownDirEmpty(t *testing.T) {
	dir := t.TempDir()
	defs := LoadMarkdownDir(dir, "plugin")
	if defs != nil {
		t.Errorf("got %v, want nil for empty dir", defs)
	}
}

func TestLoadMarkdownDirMissingDir(t *testing.T) {
	defs := LoadMarkdownDir("/nonexistent/commands", "plugin")
	if defs != nil {
		t.Errorf("got %v, want nil for missing dir", defs)
	}
}

func TestLoadMarkdownDirSkipsNonMd(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.md"), []byte(`Template content`), 0o644)
	os.WriteFile(filepath.Join(dir, "ignore.toml"), []byte(`template = "x"`), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a command"), 0o644)

	defs := LoadMarkdownDir(dir, "plugin")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1 (only .md files)", len(defs))
	}
	if defs[0].Name != "good" {
		t.Errorf("Name = %q, want good", defs[0].Name)
	}
}

func TestLoadMarkdownDirNameFromFrontmatterOverridesFilename(t *testing.T) {
	dir := t.TempDir()
	md := `---
name: custom-name
description: Has a custom name
---
Template body here.
`
	os.WriteFile(filepath.Join(dir, "filename.md"), []byte(md), 0o644)

	defs := LoadMarkdownDir(dir, "plugin")
	if len(defs) != 1 {
		t.Fatalf("got %d defs, want 1", len(defs))
	}
	if defs[0].Name != "custom-name" {
		t.Errorf("Name = %q, want custom-name (from frontmatter, not filename)", defs[0].Name)
	}
}
