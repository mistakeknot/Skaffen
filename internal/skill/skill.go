// Package skill discovers, parses, and caches SKILL.md files that define
// agent skills (slash commands and implicit triggers).
//
// Skills live in directories: each skill is a subdirectory containing a
// SKILL.md file with YAML frontmatter and a markdown body.
package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Def is a parsed skill definition from a SKILL.md file.
type Def struct {
	Name          string   // skill identifier, used as slash command name
	Description   string   // one-line description for help and metadata
	UserInvocable bool     // true = user can invoke via /name
	Triggers      []string // implicit activation trigger phrases
	Args          string   // argument hint for help display
	Model         string   // preferred model hint (optional)
	Source        string   // source tier: "project", "project-plugin", "user", "user-plugin"
	Path          string   // filesystem path to the SKILL.md file
	Body          string   // skill body (empty until LoadBody is called)
}

// frontmatter is the raw YAML structure parsed from SKILL.md files.
type frontmatter struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	UserInvocable *bool    `yaml:"user_invocable"` // pointer to detect absence (default true)
	Triggers      []string `yaml:"triggers"`
	Args          string   `yaml:"args"`
	Model         string   `yaml:"model"`
}

// LoadDir reads all SKILL.md files from subdirectories of dir.
// Each skill lives in dir/<skill-name>/SKILL.md.
// Returns empty slice for missing dir. Skips malformed skills with stderr warning.
// Only parses YAML frontmatter (body NOT loaded — lazy loading via LoadBody).
func LoadDir(dir, source string) []Def {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var defs []Def
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillPath := filepath.Join(dir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue // no SKILL.md in this subdir, skip silently
		}

		fm, err := parseFrontmatter(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: skill %q: %v (skipping)\n", e.Name(), err)
			continue
		}

		if fm.Name == "" {
			fmt.Fprintf(os.Stderr, "skaffen: warning: skill %q: missing name (skipping)\n", e.Name())
			continue
		}
		if fm.Description == "" {
			fmt.Fprintf(os.Stderr, "skaffen: warning: skill %q: missing description (skipping)\n", e.Name())
			continue
		}

		invocable := true
		if fm.UserInvocable != nil {
			invocable = *fm.UserInvocable
		}

		defs = append(defs, Def{
			Name:          fm.Name,
			Description:   fm.Description,
			UserInvocable: invocable,
			Triggers:      fm.Triggers,
			Args:          fm.Args,
			Model:         fm.Model,
			Source:        source,
			Path:          skillPath,
		})
	}
	return defs
}

// LoadAll loads skills from multiple directories and merges them.
// Later directories override earlier ones on name collision.
// Source labels are assigned in order: "user", "user-plugin", "project", "project-plugin".
func LoadAll(dirs ...string) map[string]Def {
	sources := []string{"user", "user-plugin", "project", "project-plugin"}
	result := make(map[string]Def)
	for i, dir := range dirs {
		source := "user"
		if i < len(sources) {
			source = sources[i]
		}
		for _, def := range LoadDir(dir, source) {
			result[def.Name] = def
		}
	}
	return result
}

// LoadBody lazily loads the body of a skill (everything after the closing ---).
// The body is cached in d.Body after the first call.
func LoadBody(d *Def) (string, error) {
	if d.Body != "" {
		return d.Body, nil
	}

	data, err := os.ReadFile(d.Path)
	if err != nil {
		return "", fmt.Errorf("read skill body %q: %w", d.Path, err)
	}

	d.Body = extractBody(data)
	return d.Body, nil
}

// parseFrontmatter extracts YAML between --- delimiters.
func parseFrontmatter(data []byte) (frontmatter, error) {
	// Find opening ---
	if !bytes.HasPrefix(bytes.TrimLeft(data, " \t\n\r"), []byte("---")) {
		return frontmatter{}, fmt.Errorf("no frontmatter (missing opening ---)")
	}

	// Trim any leading whitespace, then skip the opening ---
	trimmed := bytes.TrimLeft(data, " \t\n\r")
	rest := trimmed[3:] // skip "---"

	// Find closing ---
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return frontmatter{}, fmt.Errorf("no frontmatter (missing closing ---)")
	}

	yamlBytes := rest[:idx]

	var fm frontmatter
	if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
		return frontmatter{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, nil
}

// extractBody returns everything after the closing --- delimiter.
func extractBody(data []byte) string {
	trimmed := bytes.TrimLeft(data, " \t\n\r")
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return string(data)
	}

	rest := trimmed[3:] // skip opening "---"
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return ""
	}

	// Skip past "\n---" and optional newline
	body := rest[idx+4:]
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}
	return string(body)
}
