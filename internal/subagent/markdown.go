package subagent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// agentFrontmatter is the raw YAML structure parsed from agent .md files.
type agentFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Model       string `yaml:"model"`
}

// LoadMarkdownAgents reads agent .md files and returns SubagentType definitions.
// The YAML frontmatter provides name, description, and model; the markdown body
// becomes the SystemPrompt. Agent names are qualified as "pluginName:agentName".
//
// agentPaths are relative to pluginDir. Missing files are skipped with a warning.
func LoadMarkdownAgents(pluginName, pluginDir string, agentPaths []string) []SubagentType {
	var types []SubagentType
	for _, relPath := range agentPaths {
		absPath := filepath.Join(pluginDir, relPath)
		st, err := parseAgentMarkdown(pluginName, absPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: agent %q: %v (skipping)\n", relPath, err)
			continue
		}
		types = append(types, st)
	}
	return types
}

func parseAgentMarkdown(pluginName, path string) (SubagentType, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SubagentType{}, fmt.Errorf("read: %w", err)
	}

	fm, body, hasFrontmatter := splitAgentFrontmatter(data)

	var parsed agentFrontmatter
	if hasFrontmatter {
		if err := yaml.Unmarshal(fm, &parsed); err != nil {
			return SubagentType{}, fmt.Errorf("parse frontmatter: %w", err)
		}
	}

	// Derive name from filename if not in frontmatter
	name := parsed.Name
	if name == "" {
		base := filepath.Base(path)
		name = strings.TrimSuffix(base, ".md")
	}

	// Qualify with plugin name
	qualifiedName := pluginName + ":" + name

	systemPrompt := strings.TrimSpace(body)
	if systemPrompt == "" && !hasFrontmatter {
		systemPrompt = strings.TrimSpace(string(data))
	}

	description := parsed.Description
	if description == "" {
		description = fmt.Sprintf("Agent from %s plugin", pluginName)
	}

	return SubagentType{
		Name:         qualifiedName,
		Description:  description,
		SystemPrompt: systemPrompt,
		Model:        parsed.Model,
		MaxTurns:     25,
		ReadOnly:     true,
		Timeout:      Duration{120 * time.Second},
	}, nil
}

// splitAgentFrontmatter splits a markdown file into YAML frontmatter bytes
// and body string. Returns hasFrontmatter=false if no frontmatter delimiters found.
func splitAgentFrontmatter(data []byte) (fm []byte, body string, hasFrontmatter bool) {
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
