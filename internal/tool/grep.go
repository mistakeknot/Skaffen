package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// GrepTool wraps ripgrep for content search.
type GrepTool struct{}

type grepParams struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`        // default "."
	Glob       string `json:"glob,omitempty"`        // e.g., "*.go"
	OutputMode string `json:"output_mode,omitempty"` // "content", "files_with_matches" (default), "count"
}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string  { return "Search file contents using ripgrep with regex support and glob filtering" }
func (t *GrepTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Regex pattern to search for"},
			"path": {"type": "string", "description": "File or directory to search (default '.')"},
			"glob": {"type": "string", "description": "Glob pattern to filter files (e.g., '*.go')"},
			"output_mode": {"type": "string", "enum": ["content", "files_with_matches", "count"], "description": "Output mode (default 'files_with_matches')"}
		},
		"required": ["pattern"]
	}`)
}

func (t *GrepTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p grepParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}
	if p.Pattern == "" {
		return ToolResult{Content: "pattern is required", IsError: true}
	}

	path := p.Path
	if path == "" {
		path = "."
	}

	// Try ripgrep first, fall back to grep
	binary := "rg"
	args := buildRgArgs(p, path)

	if _, err := exec.LookPath("rg"); err != nil {
		binary = "grep"
		args = buildGrepArgs(p, path)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()

	output := string(out)
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes] + "\n... (truncated)"
	}

	// rg/grep exit code 1 = no matches (not an error)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return ToolResult{Content: "no matches found"}
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return ToolResult{Content: fmt.Sprintf("exit code: %d\n%s", exitErr.ExitCode(), output), IsError: true}
		}
		return ToolResult{Content: fmt.Sprintf("exec: %v\n%s", err, output), IsError: true}
	}

	return ToolResult{Content: strings.TrimRight(output, "\n")}
}

func buildRgArgs(p grepParams, path string) []string {
	var args []string
	switch p.OutputMode {
	case "content":
		args = append(args, "-n") // line numbers
	case "count":
		args = append(args, "-c")
	default: // files_with_matches
		args = append(args, "-l")
	}
	if p.Glob != "" {
		args = append(args, "--glob", p.Glob)
	}
	args = append(args, p.Pattern, path)
	return args
}

func buildGrepArgs(p grepParams, path string) []string {
	var args []string
	args = append(args, "-r")
	switch p.OutputMode {
	case "content":
		args = append(args, "-n")
	case "count":
		args = append(args, "-c")
	default:
		args = append(args, "-l")
	}
	if p.Glob != "" {
		args = append(args, "--include", p.Glob)
	}
	args = append(args, p.Pattern, path)
	return args
}
