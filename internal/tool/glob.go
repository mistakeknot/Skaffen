package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GlobTool matches files by pattern, sorted by modification time.
type GlobTool struct{}

type globParams struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"` // base directory, default "."
}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string  { return "Find files matching a glob pattern, sorted by modification time (most recent first)" }
func (t *GlobTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Glob pattern to match (e.g., '**/*.go', 'src/*.ts')"},
			"path": {"type": "string", "description": "Base directory (default '.')"}
		},
		"required": ["pattern"]
	}`)
}

func (t *GlobTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p globParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}
	if p.Pattern == "" {
		return ToolResult{Content: "pattern is required", IsError: true}
	}

	base := p.Path
	if base == "" {
		base = "."
	}

	pattern := filepath.Join(base, p.Pattern)

	var matches []string
	var err error
	if strings.Contains(p.Pattern, "**") {
		// filepath.Glob doesn't support ** (recursive). Walk the tree and match manually.
		matches, err = globRecursive(base, p.Pattern)
	} else {
		matches, err = filepath.Glob(pattern)
	}
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("glob: %v", err), IsError: true}
	}

	if len(matches) == 0 {
		return ToolResult{Content: "no files matching pattern"}
	}

	// Sort by modification time, most recent first
	type fileEntry struct {
		path    string
		modTime int64
	}
	entries := make([]fileEntry, 0, len(matches))
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		entries = append(entries, fileEntry{path: m, modTime: info.ModTime().UnixNano()})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modTime > entries[j].modTime
	})

	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.path)
		b.WriteByte('\n')
	}

	return ToolResult{Content: strings.TrimRight(b.String(), "\n")}
}

// globRecursive handles patterns containing ** by walking the directory tree
// and matching each path against the pattern. The ** matches zero or more
// directory levels.
func globRecursive(base, pattern string) ([]string, error) {
	// Convert ** pattern to a suffix match.
	// Common patterns: "**/*.go", "src/**/*.ts", "**/test_*.py"
	// Strategy: walk the tree, for each file check if it matches the non-** suffix.
	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0] // e.g., "src/" or ""
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], "/")
		suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
	}

	searchBase := filepath.Join(base, prefix)
	if _, err := os.Stat(searchBase); err != nil {
		searchBase = base
	}

	var matches []string
	err := filepath.WalkDir(searchBase, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible dirs
		}
		if d.IsDir() {
			// Skip hidden dirs and common large dirs
			name := d.Name()
			if name != "." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if name == "node_modules" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		if suffix == "" {
			matches = append(matches, path)
			return nil
		}

		// Match the file against the suffix pattern
		matched, _ := filepath.Match(suffix, d.Name())
		if matched {
			matches = append(matches, path)
		}
		return nil
	})

	return matches, err
}

func (t *GlobTool) ConcurrencySafe(_ json.RawMessage) bool { return true }
