// Package command discovers and loads disk-based slash commands from
// TOML files in ~/.skaffen/commands/ and .skaffen/commands/.
//
// Two command types are supported:
//   - template: injects the template text as a user prompt
//   - script: executes a shell script and displays output
package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Type distinguishes template commands from executable scripts.
type Type string

const (
	TypeTemplate Type = "template"
	TypeScript   Type = "script"
)

// Def is a parsed command definition from a TOML file.
type Def struct {
	Name        string // derived from filename (without .toml)
	Description string // shown in /help and completer
	Type        Type   // "template" or "script"
	Template    string // prompt text (template commands)
	Script      string // shell command (script commands)
	Source      string // "user" or "project"
}

// tomlDef is the raw TOML structure.
type tomlDef struct {
	Description string `toml:"description"`
	Type        string `toml:"type"`
	Template    string `toml:"template"`
	Script      string `toml:"script"`
}

// LoadDir reads all .toml files from a directory and returns command definitions.
// source is "user" or "project" — used for display/debugging.
// Returns empty slice (not error) if the directory doesn't exist.
// Malformed files are skipped with a warning on stderr.
func LoadDir(dir, source string) []Def {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var defs []Def
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".toml")
		path := filepath.Join(dir, e.Name())

		def, err := parseFile(path, name, source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: command %q: %v (skipping)\n", name, err)
			continue
		}
		defs = append(defs, def)
	}
	return defs
}

func parseFile(path, name, source string) (Def, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Def{}, fmt.Errorf("read: %w", err)
	}

	var raw tomlDef
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Def{}, fmt.Errorf("parse: %w", err)
	}

	typ := Type(raw.Type)
	if typ == "" {
		typ = TypeTemplate // default
	}
	if typ != TypeTemplate && typ != TypeScript {
		return Def{}, fmt.Errorf("unknown type %q (want template or script)", raw.Type)
	}

	if typ == TypeTemplate && raw.Template == "" {
		return Def{}, fmt.Errorf("template command requires 'template' field")
	}
	if typ == TypeScript && raw.Script == "" {
		return Def{}, fmt.Errorf("script command requires 'script' field")
	}

	desc := raw.Description
	if desc == "" {
		desc = fmt.Sprintf("Custom %s command", typ)
	}

	return Def{
		Name:        name,
		Description: desc,
		Type:        typ,
		Template:    raw.Template,
		Script:      raw.Script,
		Source:      source,
	}, nil
}

// LoadAll loads commands from multiple directories and merges them.
// Later directories override earlier ones on name collision (project > user).
func LoadAll(dirs ...string) map[string]Def {
	sources := []string{"user", "project"}
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
