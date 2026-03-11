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
	matches, err := filepath.Glob(pattern)
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
