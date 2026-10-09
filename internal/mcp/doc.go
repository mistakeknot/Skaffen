// Package mcp provides an MCP stdio client for loading Interverse plugin tools.
//
// The package has four components:
//
//   - Config parser (config.go): reads plugins.toml and resolves MCP servers from plugin.json files
//   - Client wrapper (client.go): wraps the official MCP Go SDK for stdio subprocess communication
//   - Tool adapter (tool.go): MCPTool implements tool.Tool by delegating to an MCP client
//   - Manager (manager.go): orchestrates server lifecycles, tool registration, and crash recovery
//
// Remote servers (remote.go, oauth.go, manager_remote.go) are reached over
// streamable HTTP with Skaffen's own OAuth client. They are declared in
// [remote.NAME] tables, read only from a trusted plugins file, limited to the
// finance:read scope and to an explicit tool allowlist; see LoadRemoteConfig
// and Manager.ConnectRemote. A remote whose server offers none of its
// allowlisted tools fails closed: the connection is refused and torn down
// rather than left open with nothing registered.
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
