package command

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// mdFrontmatter is the raw YAML structure parsed from command .md files.
type mdFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// LoadMarkdownDir scans dir for .md files with YAML frontmatter and returns
// command definitions. The YAML frontmatter provides name and description;
// the markdown body becomes the template text.
//
// This is used for Interverse plugin commands which use Markdown format
// (as opposed to Skaffen's native TOML format).
func LoadMarkdownDir(dir, source string) []Def {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var defs []Def
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		def, err := parseMarkdownFile(path, e.Name(), source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: markdown command %q: %v (skipping)\n", e.Name(), err)
			continue
		}
		defs = append(defs, def)
	}
	return defs
}

func parseMarkdownFile(path, filename, source string) (Def, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Def{}, fmt.Errorf("read: %w", err)
	}

	fm, body, hasFrontmatter := splitMarkdownFrontmatter(data)

	name := strings.TrimSuffix(filename, ".md")
	desc := fmt.Sprintf("Custom template command")

	if hasFrontmatter {
		var parsed mdFrontmatter
		if err := yaml.Unmarshal(fm, &parsed); err != nil {
			return Def{}, fmt.Errorf("parse frontmatter: %w", err)
		}
		if parsed.Name != "" {
			name = parsed.Name
		}
		if parsed.Description != "" {
			desc = parsed.Description
		}
	}

	template := strings.TrimSpace(body)
	if template == "" && !hasFrontmatter {
		// Entire file is the template when there's no frontmatter
		template = strings.TrimSpace(string(data))
	}

	if template == "" {
		return Def{}, fmt.Errorf("empty template")
	}

	return Def{
		Name:        name,
		Description: desc,
		Type:        TypeTemplate,
		Template:    template,
		Source:      source,
	}, nil
}

// splitMarkdownFrontmatter splits a markdown file into YAML frontmatter bytes
// and body string. Returns hasFrontmatter=false if no frontmatter delimiters found.
func splitMarkdownFrontmatter(data []byte) (fm []byte, body string, hasFrontmatter bool) {
	trimmed := bytes.TrimLeft(data, " \t\n\r")
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return nil, string(data), false
	}

	rest := trimmed[3:] // skip opening "---"
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return nil, string(data), false
	}

	yamlBytes := rest[:idx]

	// Body starts after "\n---" + optional newline
	bodyBytes := rest[idx+4:]
	if len(bodyBytes) > 0 && bodyBytes[0] == '\n' {
		bodyBytes = bodyBytes[1:]
	}

	return yamlBytes, string(bodyBytes), true
}
