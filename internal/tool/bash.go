package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
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
func (t *BashTool) Description() string { return "Execute a bash command and return its output" }
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

// ConcurrencySafe classifies whether this bash invocation is safe to run
// concurrently with other tool calls. Delegates to BashCommandSafe.
func (t *BashTool) ConcurrencySafe(params json.RawMessage) bool {
	var p bashParams
	if err := json.Unmarshal(params, &p); err != nil {
		return false
	}
	return BashCommandSafe(p.Command)
}

// PropagatesErrorToSiblings returns true — a bash error should cancel sibling
// bash calls in the same parallel batch (implicit dependency chains).
func (t *BashTool) PropagatesErrorToSiblings() bool { return true }

// safeCommands is the known-safe command set for concurrency classification.
// Conservative: unknown commands default to unsafe. Expand with evidence.
// sed and awk are excluded — both can write (sed -i, awk '{print > "f"}').
var safeCommands = map[string]bool{
	"cat": true, "head": true, "tail": true, "less": true, "more": true,
	"ls": true, "tree": true, "du": true, "df": true,
	"wc": true, "sort": true, "uniq": true, "diff": true, "comm": true,
	"grep": true, "rg": true, "ag": true,
	"git":  false, // git subcommands need further parsing — see safeGitSubcommands
	"stat": true, "file": true, "which": true, "type": true,
	"echo": true, "printf": true, "date": true, "uname": true,
	"id": true, "whoami": true, "hostname": true, "pwd": true,
}

// safeGitSubcommands are git subcommands that are read-only.
var safeGitSubcommands = map[string]bool{
	"log": true, "status": true, "diff": true, "show": true,
	"branch": true, "tag": true, "rev-parse": true, "blame": true,
	"shortlog": true, "describe": true, "ls-files": true, "ls-tree": true,
}

// shellMetachars are patterns that indicate compound or redirect commands.
// Any command containing these is classified as unsafe regardless of first token.
// Intentionally lexical — false negatives on quoted metacharacters are acceptable
// because the conservative default (serial) loses only parallelism, not correctness.
var shellMetachars = []string{"&&", "||", ";", "|", "$(", "`", ">", "<", "\n"}

// BashCommandSafe reports whether a bash command string is safe for concurrent
// execution. Conservative: unknown commands return false.
func BashCommandSafe(command string) bool {
	for _, meta := range shellMetachars {
		if strings.Contains(command, meta) {
			return false
		}
	}
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return false
	}
	first := fields[0]
	if first == "git" && len(fields) > 1 {
		return safeGitSubcommands[fields[1]]
	}
	safe, known := safeCommands[first]
	return known && safe
}
