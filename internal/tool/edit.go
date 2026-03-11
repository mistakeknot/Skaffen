package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditTool performs exact string replacement in files.
type EditTool struct{}

type editParams struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

func (t *EditTool) Name() string        { return "edit" }
func (t *EditTool) Description() string  { return "Replace exact string matches in a file. Fails if old_string is not unique unless replace_all is true" }
func (t *EditTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "Absolute path to the file to edit"},
			"old_string": {"type": "string", "description": "Exact string to find and replace"},
			"new_string": {"type": "string", "description": "Replacement string"},
			"replace_all": {"type": "boolean", "description": "Replace all occurrences (default false)"}
		},
		"required": ["file_path", "old_string", "new_string"]
	}`)
}

func (t *EditTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p editParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}
	if p.FilePath == "" {
		return ToolResult{Content: "file_path is required", IsError: true}
	}

	data, err := os.ReadFile(p.FilePath)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("read: %v", err), IsError: true}
	}

	content := string(data)
	count := strings.Count(content, p.OldString)

	if count == 0 {
		return ToolResult{Content: "old_string not found in file", IsError: true}
	}
	if count > 1 && !p.ReplaceAll {
		return ToolResult{
			Content: fmt.Sprintf("old_string matches %d times; use replace_all or provide more context", count),
			IsError: true,
		}
	}

	var replaced string
	if p.ReplaceAll {
		replaced = strings.ReplaceAll(content, p.OldString, p.NewString)
	} else {
		replaced = strings.Replace(content, p.OldString, p.NewString, 1)
	}

	if err := os.WriteFile(p.FilePath, []byte(replaced), 0644); err != nil {
		return ToolResult{Content: fmt.Sprintf("write: %v", err), IsError: true}
	}

	return ToolResult{Content: fmt.Sprintf("replaced %d occurrence(s) in %s", count, p.FilePath)}
}
