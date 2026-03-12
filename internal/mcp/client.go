package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolInfo describes a tool discovered from an MCP server.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// CallResult is the package-local result of an MCP tool call.
// Decoupled from tool.ToolResult to keep the import graph clean.
type CallResult struct {
	Content string
	IsError bool
}

// Client wraps an MCP stdio connection to a single server.
type Client struct {
	session *gomcp.ClientSession
}

// NewClient spawns an MCP server subprocess and performs the initialize handshake.
// args and env are optional (may be nil).
// The command is spawned with exec.Command (not exec.CommandContext) so the
// subprocess lifetime is managed by session.Close(), not the context.
func NewClient(ctx context.Context, command string, args []string, env map[string]string) (*Client, error) {
	cmd := exec.Command(command, args...)

	// Merge env vars into subprocess environment
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	transport := &gomcp.CommandTransport{Command: cmd}

	client := gomcp.NewClient(&gomcp.Implementation{
		Name:    "skaffen",
		Version: "0.2.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp connect: %w", err)
	}

	return &Client{session: session}, nil
}

// ListTools calls tools/list and returns tool metadata.
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}

	tools := make([]ToolInfo, len(result.Tools))
	for i, t := range result.Tools {
		schema, _ := json.Marshal(t.InputSchema)
		tools[i] = ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		}
	}
	return tools, nil
}

// CallTool calls tools/call and returns the result.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (CallResult, error) {
	result, err := c.session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		return CallResult{}, fmt.Errorf("mcp tools/call %q: %w", name, err)
	}

	// Concatenate text content blocks
	var sb strings.Builder
	for _, content := range result.Content {
		if tc, ok := content.(*gomcp.TextContent); ok {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(tc.Text)
		}
	}

	return CallResult{
		Content: sb.String(),
		IsError: result.IsError,
	}, nil
}

// Close gracefully shuts down the MCP session and kills the subprocess.
func (c *Client) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}
