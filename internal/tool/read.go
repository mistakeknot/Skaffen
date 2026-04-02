package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReadTool reads files with optional offset/limit.
type ReadTool struct{}

type readParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"` // 1-based line number to start from
	Limit    int    `json:"limit,omitempty"`  // max lines to read (default 2000)
}

func (t *ReadTool) Name() string        { return "read" }
func (t *ReadTool) Description() string  { return "Read a file's contents, optionally from a specific line offset with a line limit" }
func (t *ReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "Absolute path to the file to read"},
			"offset": {"type": "integer", "description": "Line number to start reading from (1-based)"},
			"limit": {"type": "integer", "description": "Maximum number of lines to read (default 2000)"}
		},
		"required": ["file_path"]
	}`)
}

func (t *ReadTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p readParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}
	if p.FilePath == "" {
		return ToolResult{Content: "file_path is required", IsError: true}
	}

	info, err := os.Stat(p.FilePath)
	if os.IsNotExist(err) {
		return ToolResult{Content: fmt.Sprintf("file not found: %s", p.FilePath), IsError: true}
	}
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("stat: %v", err), IsError: true}
	}
	if info.IsDir() {
		return ToolResult{Content: fmt.Sprintf("path is a directory, not a file: %s", p.FilePath), IsError: true}
	}

	f, err := os.Open(p.FilePath)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("open: %v", err), IsError: true}
	}
	defer f.Close()

	limit := p.Limit
	if limit <= 0 {
		limit = 2000
	}
	offset := p.Offset
	if offset <= 0 {
		offset = 1
	}

	var b strings.Builder
	scanner := bufio.NewScanner(f)
	lineNum := 0
	emitted := 0

	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if emitted >= limit {
			break
		}
		fmt.Fprintf(&b, "%6d\t%s\n", lineNum, scanner.Text())
		emitted++
	}

	if err := scanner.Err(); err != nil {
		return ToolResult{Content: fmt.Sprintf("read error: %v", err), IsError: true}
	}

	return ToolResult{Content: b.String()}
}

func (t *ReadTool) ConcurrencySafe(_ json.RawMessage) bool { return true }
