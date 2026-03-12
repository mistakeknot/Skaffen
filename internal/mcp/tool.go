package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

// ToolCaller is the interface that MCPTool uses to call tools.
// Satisfied by *Client and by test mocks.
type ToolCaller interface {
	CallTool(ctx context.Context, name string, arguments map[string]any) (CallResult, error)
}

// MCPTool wraps an MCP tool as a tool.Tool implementation.
type MCPTool struct {
	plugin        string
	server        string
	info          ToolInfo
	caller        ToolCaller
	qualifiedName string
}

// NewMCPTool creates an MCPTool that delegates Execute to the given caller.
func NewMCPTool(plugin, server string, info ToolInfo, caller ToolCaller) *MCPTool {
	return &MCPTool{
		plugin:        plugin,
		server:        server,
		info:          info,
		caller:        caller,
		qualifiedName: plugin + "_" + server + "_" + info.Name,
	}
}

func (t *MCPTool) Name() string             { return t.qualifiedName }
func (t *MCPTool) Description() string       { return t.info.Description }
func (t *MCPTool) Schema() json.RawMessage   { return t.info.InputSchema }

// OriginalName returns the tool name as the MCP server knows it.
func (t *MCPTool) OriginalName() string { return t.info.Name }

func (t *MCPTool) Execute(ctx context.Context, params json.RawMessage) tool.ToolResult {
	var arguments map[string]any
	if len(params) > 0 {
		if err := json.Unmarshal(params, &arguments); err != nil {
			return tool.ToolResult{
				Content: fmt.Sprintf("invalid params: %v", err),
				IsError: true,
			}
		}
	}

	result, err := t.caller.CallTool(ctx, t.info.Name, arguments)
	if err != nil {
		return tool.ToolResult{
			Content: fmt.Sprintf("mcp tool %q error: %v", t.qualifiedName, err),
			IsError: true,
		}
	}
	return tool.ToolResult{
		Content: result.Content,
		IsError: result.IsError,
	}
}
