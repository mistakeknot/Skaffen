package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// LsTool lists directory contents.
type LsTool struct{}

type lsParams struct {
	Path string `json:"path,omitempty"` // default "."
}

func (t *LsTool) Name() string        { return "ls" }
func (t *LsTool) Description() string  { return "List directory contents with file sizes" }
func (t *LsTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path to list (default '.')"}
		}
	}`)
}

func (t *LsTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p lsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}

	path := p.Path
	if path == "" {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("readdir: %v", err), IsError: true}
	}

	// Separate dirs and files, sort each alphabetically
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	var b strings.Builder
	for _, d := range dirs {
		fmt.Fprintf(&b, "%s/\n", d.Name())
	}
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			fmt.Fprintf(&b, "%s\n", f.Name())
			continue
		}
		fmt.Fprintf(&b, "%s  %d\n", f.Name(), info.Size())
	}

	return ToolResult{Content: strings.TrimRight(b.String(), "\n")}
}

func (t *LsTool) ConcurrencySafe(_ json.RawMessage) bool { return true }
