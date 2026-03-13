package subagent

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// TypeRegistry holds subagent type definitions.
type TypeRegistry struct {
	types map[string]SubagentType
}

// NewTypeRegistry creates a registry with built-in types and loads custom
// types from configDir (e.g., ".skaffen/agents/"). If configDir is empty
// or doesn't exist, only built-in types are available.
func NewTypeRegistry(configDir string) *TypeRegistry {
	r := &TypeRegistry{types: make(map[string]SubagentType)}

	// Built-in: explore (read-only)
	r.types["explore"] = SubagentType{
		Name:         "explore",
		Description:  "Fast read-only agent for codebase exploration. Has access to Read, Grep, Glob, and Ls tools only.",
		Tools:        []string{"read", "grep", "glob", "ls"},
		ReadOnly:     true,
		MaxTurns:     10,
		SystemPrompt: "You are a focused codebase exploration agent. Answer the question using only the available read-only tools. Be concise and direct.\n\n{{.TaskPrompt}}",
	}

	// Built-in: general (full access)
	r.types["general"] = SubagentType{
		Name:         "general",
		Description:  "General-purpose agent with full tool access for multi-step tasks.",
		Tools:        nil, // nil = all tools
		ReadOnly:     false,
		MaxTurns:     25,
		SystemPrompt: "You are a focused agent working on a specific task. Complete the task using the available tools. Be concise.\n\n{{.TaskPrompt}}",
	}

	// Load custom types from config dir
	if configDir != "" {
		r.loadFromDir(configDir)
	}

	return r
}

// Get returns a subagent type by name.
func (r *TypeRegistry) Get(name string) (SubagentType, error) {
	st, ok := r.types[name]
	if !ok {
		return SubagentType{}, fmt.Errorf("unknown subagent type %q", name)
	}
	return st, nil
}

// List returns all registered types sorted by name.
func (r *TypeRegistry) List() []SubagentType {
	names := make([]string, 0, len(r.types))
	for name := range r.types {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]SubagentType, len(names))
	for i, name := range names {
		result[i] = r.types[name]
	}
	return result
}

// Names returns sorted type names (for Agent tool schema enum).
func (r *TypeRegistry) Names() []string {
	names := make([]string, 0, len(r.types))
	for name := range r.types {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *TypeRegistry) loadFromDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // dir doesn't exist — silently skip
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var st SubagentType
		if _, err := toml.DecodeFile(path, &st); err != nil {
			slog.Warn("skipping invalid subagent type", "path", path, "error", err)
			continue
		}
		if err := st.Validate(); err != nil {
			slog.Warn("skipping invalid subagent type", "path", path, "error", err)
			continue
		}
		r.types[st.Name] = st
	}
}
