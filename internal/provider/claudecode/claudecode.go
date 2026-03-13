package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

// Option configures the ClaudeCodeProvider.
type Option func(*ClaudeCodeProvider)

// WithBinaryPath overrides the claude binary path.
func WithBinaryPath(path string) Option {
	return func(p *ClaudeCodeProvider) { p.binaryPath = path }
}

// WithModel sets the model for Claude Code to use.
func WithModel(model string) Option {
	return func(p *ClaudeCodeProvider) { p.model = model }
}

// WithWorkDir sets the working directory for the claude subprocess.
// Claude Code scopes file access to this directory. Set this to the
// project root so the subprocess can reach all project files.
func WithWorkDir(dir string) Option {
	return func(p *ClaudeCodeProvider) { p.workDir = dir }
}

// ClaudeCodeProvider delegates inference to a local claude binary.
type ClaudeCodeProvider struct {
	binaryPath string
	model      string
	workDir    string
	initErr    error
}

// New creates a ClaudeCodeProvider. If the claude binary is not found,
// Stream() will return an actionable error.
func New(opts ...Option) *ClaudeCodeProvider {
	p := &ClaudeCodeProvider{}
	for _, opt := range opts {
		opt(p)
	}

	if p.binaryPath == "" {
		path, err := exec.LookPath("claude")
		if err != nil {
			p.initErr = fmt.Errorf("claude binary not found in PATH. Install Claude Code: https://docs.anthropic.com/en/docs/claude-code")
			return p
		}
		p.binaryPath = path
	} else {
		// Validate explicit path exists
		if _, err := exec.LookPath(p.binaryPath); err != nil {
			p.initErr = fmt.Errorf("claude binary not found at %s. Install Claude Code: https://docs.anthropic.com/en/docs/claude-code", p.binaryPath)
			return p
		}
	}

	return p
}

// Name returns "claude-code".
func (p *ClaudeCodeProvider) Name() string { return "claude-code" }

// Stream spawns claude --print and streams the response.
func (p *ClaudeCodeProvider) Stream(ctx context.Context, messages []provider.Message, tools []provider.ToolDef, config provider.Config) (*provider.StreamResponse, error) {
	if p.initErr != nil {
		return nil, p.initErr
	}

	// Extract the last user message as the prompt
	prompt := lastUserText(messages)
	if prompt == "" {
		return nil, fmt.Errorf("no user message to send")
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
	}
	// Claude Code's --print mode can't forward interactive approval
	// prompts back to Skaffen's TUI — it blocks waiting for input that
	// never arrives. Bypass CC's permissions since Skaffen's trust
	// evaluator (internal/trust/) handles tool gating via the TUI
	// approval overlay when tools are executed through Skaffen's own
	// registry. When using the claude-code provider, CC executes tools
	// internally, so its own permission system is the only gate — and
	// it can't work in --print mode.
	args = append(args, "--permission-mode", "bypassPermissions")
	if model := config.Model; model != "" {
		args = append(args, "--model", model)
	} else if p.model != "" {
		args = append(args, "--model", p.model)
	}

	cmd := exec.CommandContext(ctx, p.binaryPath, args...)
	cmd.Stdin = strings.NewReader(prompt)
	// Set working directory so Claude Code can access the full project tree.
	// If workDir is inside a monorepo (parent .git exists), use the monorepo
	// root so the subprocess can reach sibling directories and .beads/.
	if dir := resolveProjectRoot(p.workDir); dir != "" {
		cmd.Dir = dir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}

	events := make(chan provider.StreamEvent, 16)
	go p.processOutput(ctx, cmd, stdout, stderr, events)

	return provider.NewStreamResponse(events), nil
}

// processOutput reads stream-json lines and emits StreamEvents.
func (p *ClaudeCodeProvider) processOutput(ctx context.Context, cmd *exec.Cmd, stdout io.ReadCloser, stderr io.ReadCloser, events chan<- provider.StreamEvent) {
	defer close(events)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024) // 1MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}

		switch envelope.Type {
		case "assistant":
			p.handleAssistantMessage(line, events)
		case "user":
			p.handleUserMessage(line, events)
		case "result":
			p.handleResult(line, events)
		}
	}

	// Read stderr for error context
	stderrBytes, _ := io.ReadAll(io.LimitReader(stderr, 4096))

	if err := cmd.Wait(); err != nil {
		errMsg := strings.TrimSpace(string(stderrBytes))
		if strings.Contains(errMsg, "not logged in") || strings.Contains(errMsg, "authentication") {
			events <- provider.StreamEvent{
				Type: provider.EventError,
				Err:  fmt.Errorf("Claude Code is not logged in. Run: claude login"),
			}
			return
		}
		if errMsg != "" {
			events <- provider.StreamEvent{
				Type: provider.EventError,
				Err:  fmt.Errorf("claude process failed: %s", truncate(errMsg, 200)),
			}
		} else {
			events <- provider.StreamEvent{
				Type: provider.EventError,
				Err:  fmt.Errorf("claude process failed: %w", err),
			}
		}
	}
}

// handleAssistantMessage extracts text and tool_use blocks from an assistant message.
func (p *ClaudeCodeProvider) handleAssistantMessage(data []byte, events chan<- provider.StreamEvent) {
	var msg struct {
		Message struct {
			Content []json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	for _, raw := range msg.Message.Content {
		var block struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if json.Unmarshal(raw, &block) != nil {
			continue
		}
		switch block.Type {
		case "text":
			if block.Text != "" {
				events <- provider.StreamEvent{
					Type: provider.EventTextDelta,
					Text: block.Text,
				}
			}
		case "tool_use":
			events <- provider.StreamEvent{
				Type: provider.EventToolUseStart,
				ID:   block.ID,
				Name: block.Name,
				Text: string(block.Input),
			}
		}
	}
}

// handleUserMessage extracts tool_result blocks from a user message.
func (p *ClaudeCodeProvider) handleUserMessage(data []byte, events chan<- provider.StreamEvent) {
	var msg struct {
		Message struct {
			Content []struct {
				Type      string `json:"type"`
				ToolUseID string `json:"tool_use_id"`
				Content   string `json:"content"`
				IsError   bool   `json:"is_error"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(data, &msg) != nil {
		return
	}
	for _, block := range msg.Message.Content {
		if block.Type != "tool_result" {
			continue
		}
		ev := provider.StreamEvent{
			Type: provider.EventToolResult,
			ID:   block.ToolUseID,
			Text: truncate(block.Content, 4096),
		}
		if block.IsError {
			ev.Err = fmt.Errorf("%s", truncate(block.Content, 200))
		}
		events <- ev
	}
}

// handleResult extracts usage from a result event and emits Done.
func (p *ClaudeCodeProvider) handleResult(data []byte, events chan<- provider.StreamEvent) {
	var result struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(data, &result)

	events <- provider.StreamEvent{
		Type: provider.EventDone,
		Usage: &provider.Usage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
		},
		StopReason: "end_turn",
	}
}

// lastUserText extracts the text from the last user message.
func lastUserText(messages []provider.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == provider.RoleUser {
			for _, block := range messages[i].Content {
				if block.Type == "text" {
					return block.Text
				}
			}
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// resolveProjectRoot finds the outermost project root for the given directory.
// If the directory is inside a monorepo (nested .git), it walks up to find the
// parent that contains .git, so the Claude Code subprocess can access sibling
// modules and shared directories like .beads/.
func resolveProjectRoot(dir string) string {
	if dir == "" {
		return ""
	}
	// Walk up from dir, tracking the highest directory that contains .git
	best := ""
	cur := dir
	for {
		if info, err := os.Stat(filepath.Join(cur, ".git")); err == nil && info.IsDir() {
			best = cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return best
}
