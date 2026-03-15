package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/mistakeknot/Skaffen/internal/sandbox"
)

const (
	defaultBashTimeout = 120   // seconds
	maxOutputBytes     = 10240 // 10KB total cap
	headKeepBytes      = 2048  // 2KB from start (command echo, setup output)
	tailKeepBytes      = 8192  // 8KB from end (test results, tracebacks)
)

// BashTool executes shell commands.
type BashTool struct {
	Sandbox *sandbox.Sandbox
}

type bashParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // seconds, default 120
}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Description() string  { return "Execute a bash command and return its output" }
func (t *BashTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "The bash command to execute"},
			"timeout": {"type": "integer", "description": "Timeout in seconds (default 120)"}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	var p bashParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}
	if p.Command == "" {
		return ToolResult{Content: "command is required", IsError: true}
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultBashTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Apply sandbox wrapping if configured
	cmdName, cmdArgs := "bash", []string{"-c", p.Command}
	if t.Sandbox != nil {
		cmdName, cmdArgs = t.Sandbox.WrapArgs(cmdName, cmdArgs...)
	}

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	out, err := cmd.CombinedOutput()

	output := string(out)
	if len(output) > maxOutputBytes {
		// Sandwich truncation: keep head (setup/command echo) + tail (test results/tracebacks).
		// Test runners like pytest put diagnostic content (assertion diffs, tracebacks) at the
		// end of output. Head-only truncation loses exactly the content the agent needs most.
		head := output[:headKeepBytes]
		tail := output[len(output)-tailKeepBytes:]
		omitted := len(output) - headKeepBytes - tailKeepBytes
		output = head + fmt.Sprintf("\n\n... (%d bytes omitted) ...\n\n", omitted) + tail
	}

	if ctx.Err() == context.DeadlineExceeded {
		return ToolResult{
			Content: fmt.Sprintf("timeout after %ds\n%s", timeout, output),
			IsError: true,
		}
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return ToolResult{Content: fmt.Sprintf("exec: %v", err), IsError: true}
		}
	}

	return ToolResult{
		Content: fmt.Sprintf("exit code: %d\n%s", exitCode, output),
		IsError: exitCode != 0,
	}
}
