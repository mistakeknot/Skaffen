package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool creates or overwrites files.
type WriteTool struct{}

type writeParams struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (t *WriteTool) Name() string        { return "write" }
func (t *WriteTool) Description() string  { return "Create or overwrite a file with the given content" }
func (t *WriteTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "Absolute path to the file to write"},
			"content": {"type": "string", "description": "Content to write to the file"}
		},
		"required": ["file_path", "content"]
	}`)
}

func (t *WriteTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p writeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}
	if p.FilePath == "" {
		return ToolResult{Content: "file_path is required", IsError: true}
	}

	dir := filepath.Dir(p.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ToolResult{Content: fmt.Sprintf("create directory: %v", err), IsError: true}
	}

	// Write atomically via temp file
	tmp := p.FilePath + ".skaffen-tmp"
	if err := os.WriteFile(tmp, []byte(p.Content), 0644); err != nil {
		os.Remove(tmp)
		return ToolResult{Content: fmt.Sprintf("write: %v", err), IsError: true}
	}
	if err := os.Rename(tmp, p.FilePath); err != nil {
		os.Remove(tmp)
		return ToolResult{Content: fmt.Sprintf("rename: %v", err), IsError: true}
	}

	return ToolResult{Content: fmt.Sprintf("wrote %d bytes to %s", len(p.Content), p.FilePath)}
}
