package subagent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMarkdownAgentsBasic(t *testing.T) {
	dir := t.TempDir()
	md := `---
name: reviewer
description: Reviews code for quality
model: claude-sonnet-4-20250514
---
You are a code review agent. Analyze the provided code for:
- Correctness
- Performance
- Readability
`
	os.WriteFile(filepath.Join(dir, "reviewer.md"), []byte(md), 0o644)

	agents := LoadMarkdownAgents("interflux", dir, []string{"reviewer.md"})
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	a := agents[0]
	if a.Name != "interflux:reviewer" {
		t.Errorf("Name = %q, want interflux:reviewer", a.Name)
	}
	if a.Description != "Reviews code for quality" {
		t.Errorf("Description = %q", a.Description)
	}
	if a.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q", a.Model)
	}
	if a.SystemPrompt == "" {
		t.Error("SystemPrompt is empty")
	}
	if a.SystemPrompt != "You are a code review agent. Analyze the provided code for:\n- Correctness\n- Performance\n- Readability" {
		t.Errorf("SystemPrompt = %q", a.SystemPrompt)
	}
}

func TestLoadMarkdownAgentsQualifiedName(t *testing.T) {
	dir := t.TempDir()
	md := `---
name: scout
description: Explores code
---
You explore codebases.
`
	os.WriteFile(filepath.Join(dir, "scout.md"), []byte(md), 0o644)

	agents := LoadMarkdownAgents("myplugin", dir, []string{"scout.md"})
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	if agents[0].Name != "myplugin:scout" {
		t.Errorf("Name = %q, want myplugin:scout", agents[0].Name)
	}
}

func TestLoadMarkdownAgentsDefaults(t *testing.T) {
	dir := t.TempDir()
	md := `---
name: helper
description: A helper agent
---
Help with tasks.
`
	os.WriteFile(filepath.Join(dir, "helper.md"), []byte(md), 0o644)

	agents := LoadMarkdownAgents("testplugin", dir, []string{"helper.md"})
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	a := agents[0]
	if a.MaxTurns != 25 {
		t.Errorf("MaxTurns = %d, want 25", a.MaxTurns)
	}
	if !a.ReadOnly {
		t.Error("ReadOnly = false, want true")
	}
	if a.Model != "" {
		t.Errorf("Model = %q, want empty (inherit default)", a.Model)
	}
	if a.Timeout.Duration != 120*time.Second {
		t.Errorf("Timeout = %v, want 120s", a.Timeout.Duration)
	}
}

func TestLoadMarkdownAgentsMissingFile(t *testing.T) {
	dir := t.TempDir()
	// Create one valid agent
	os.WriteFile(filepath.Join(dir, "exists.md"), []byte(`---
name: exists
description: I exist
---
I am here.
`), 0o644)

	// Reference both a valid and a missing file
	agents := LoadMarkdownAgents("plugin", dir, []string{"exists.md", "missing.md"})
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1 (missing file should be skipped)", len(agents))
	}
	if agents[0].Name != "plugin:exists" {
		t.Errorf("Name = %q, want plugin:exists", agents[0].Name)
	}
}

func TestLoadMarkdownAgentsNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	md := `You are a simple agent with no frontmatter.
Just do the thing.
`
	os.WriteFile(filepath.Join(dir, "simple.md"), []byte(md), 0o644)

	agents := LoadMarkdownAgents("plugin", dir, []string{"simple.md"})
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	a := agents[0]
	if a.Name != "plugin:simple" {
		t.Errorf("Name = %q, want plugin:simple (derived from filename)", a.Name)
	}
	if a.Description != "Agent from plugin plugin" {
		t.Errorf("Description = %q, want default description", a.Description)
	}
}

func TestLoadMarkdownAgentsSubdir(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "agents")
	os.MkdirAll(subdir, 0o755)
	md := `---
name: nested
description: In a subdirectory
---
Nested agent prompt.
`
	os.WriteFile(filepath.Join(subdir, "nested.md"), []byte(md), 0o644)

	// Path is relative to pluginDir
	agents := LoadMarkdownAgents("plugin", dir, []string{"agents/nested.md"})
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	if agents[0].Name != "plugin:nested" {
		t.Errorf("Name = %q, want plugin:nested", agents[0].Name)
	}
}
