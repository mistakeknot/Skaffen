// Package mcp provides an MCP stdio client for loading Interverse plugin tools.
//
// The package has four components:
//
//   - Config parser (config.go): reads plugins.toml and resolves MCP servers from plugin.json files
//   - Client wrapper (client.go): wraps the official MCP Go SDK for stdio subprocess communication
//   - Tool adapter (tool.go): MCPTool implements tool.Tool by delegating to an MCP client
//   - Manager (manager.go): orchestrates server lifecycles, tool registration, and crash recovery
//
// Usage in main.go:
//
//	cfg, _ := mcp.LoadConfig("~/.skaffen/plugins.toml")
//	mgr := mcp.NewManager(cfg, registry)
//	mgr.LoadAll(ctx)
//	defer mgr.Shutdown()
//
// Plugins are declared in plugins.toml with per-plugin phase gating:
//
//	[plugins.intermap]
//	path = "interverse/intermap/.claude-plugin/plugin.json"
//	phases = ["brainstorm", "build", "review"]
package mcp
