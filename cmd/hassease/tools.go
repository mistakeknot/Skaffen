package main

import (
	"context"
	"encoding/json"

	"github.com/mistakeknot/Skaffen/internal/agentloop"
	"github.com/mistakeknot/Skaffen/internal/tool"
)

// toolBridge adapts a tool.Tool to agentloop.Tool.
// Mirrors the pattern in internal/agent/agent.go.
type toolBridge struct {
	inner tool.Tool
}

func (b *toolBridge) Name() string            { return b.inner.Name() }
func (b *toolBridge) Description() string     { return b.inner.Description() }
func (b *toolBridge) Schema() json.RawMessage { return b.inner.Schema() }

func (b *toolBridge) Execute(ctx context.Context, params json.RawMessage) agentloop.ToolResult {
	r := b.inner.Execute(ctx, params)
	return agentloop.ToolResult{Content: r.Content, IsError: r.IsError}
}

func (b *toolBridge) ConcurrencySafe(params json.RawMessage) bool {
	if c, ok := b.inner.(tool.ConcurrencyClassifier); ok {
		return c.ConcurrencySafe(params)
	}
	return false
}

func (b *toolBridge) PropagatesErrorToSiblings() bool {
	if p, ok := b.inner.(tool.ErrorPropagator); ok {
		return p.PropagatesErrorToSiblings()
	}
	return false
}

// buildRegistry creates a flat agentloop.Registry with only the whitelisted tools.
func buildRegistry(whitelist []string) *agentloop.Registry {
	allowed := make(map[string]bool, len(whitelist))
	for _, name := range whitelist {
		allowed[name] = true
	}

	reg := agentloop.NewRegistry()
	for _, bt := range allBuiltinTools() {
		if allowed[bt.Name()] {
			reg.Register(&toolBridge{inner: bt})
		}
	}
	return reg
}

// allBuiltinTools returns all built-in tool instances.
func allBuiltinTools() []tool.Tool {
	return []tool.Tool{
		&tool.ReadTool{},
		&tool.WriteTool{},
		&tool.EditTool{},
		&tool.BashTool{},
		&tool.GrepTool{},
		&tool.GlobTool{},
		&tool.LsTool{},
	}
}
