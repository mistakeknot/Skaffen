package mcp

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

const maxRespawns = 3

// serverHandle tracks a running MCP server connection.
type serverHandle struct {
	plugin     string
	server     string
	client     *Client
	tools      []ToolInfo
	mu         sync.RWMutex // protects client, spawns, respawning
	spawns     int
	respawning bool // prevents double-respawn races
}

// Manager orchestrates MCP server lifecycles and tool registration.
type Manager struct {
	config   map[string]PluginConfig
	registry *tool.Registry
	handles  map[string]*serverHandle // key: "plugin_server"
	mu       sync.RWMutex
	shutdown bool
}

// NewManager creates a Manager from resolved plugin configs.
func NewManager(config map[string]PluginConfig, registry *tool.Registry) *Manager {
	return &Manager{
		config:   config,
		registry: registry,
		handles:  make(map[string]*serverHandle),
	}
}

// LoadAll connects to all configured MCP servers and registers their tools.
// Servers that fail to connect are skipped with a warning (graceful degradation).
func (m *Manager) LoadAll(ctx context.Context) error {
	for pluginName, pc := range m.config {
		for serverName, sc := range pc.Servers {
			if err := m.connectAndRegister(ctx, pluginName, serverName, sc, pc.Phases); err != nil {
				fmt.Fprintf(os.Stderr, "skaffen: warning: plugin %q server %q: %v (skipping)\n",
					pluginName, serverName, err)
			}
		}
	}
	return nil
}

func (m *Manager) connectAndRegister(ctx context.Context, pluginName, serverName string, sc ServerConfig, phases []string) error {
	client, err := NewClient(ctx, sc.Command, sc.Args, sc.Env)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return fmt.Errorf("list tools: %w", err)
	}

	key := pluginName + "_" + serverName
	handle := &serverHandle{
		plugin: pluginName,
		server: serverName,
		client: client,
		tools:  tools,
		spawns: 1,
	}

	m.mu.Lock()
	m.handles[key] = handle
	m.mu.Unlock()

	// Convert string phases to tool.Phase for registration
	toolPhases := make([]tool.Phase, len(phases))
	for i, p := range phases {
		toolPhases[i] = tool.Phase(p)
	}

	// Register tools into the F2 registry with phase gating
	for _, ti := range tools {
		mcpTool := NewMCPTool(pluginName, serverName, ti, &handleCaller{
			manager: m,
			key:     key,
			name:    ti.Name,
		})
		m.registry.RegisterForPhases(mcpTool, toolPhases)
	}

	return nil
}

// handleCaller implements ToolCaller by going through the manager's handle map.
// This indirection lets the manager replace the underlying client on respawn.
type handleCaller struct {
	manager *Manager
	key     string
	name    string
}

func (hc *handleCaller) CallTool(ctx context.Context, name string, arguments map[string]any) (CallResult, error) {
	hc.manager.mu.RLock()
	h, ok := hc.manager.handles[hc.key]
	shutdown := hc.manager.shutdown
	hc.manager.mu.RUnlock()

	if shutdown {
		return CallResult{Content: "mcp manager is shut down", IsError: true}, nil
	}
	if !ok || h == nil {
		return CallResult{Content: fmt.Sprintf("mcp server %q not connected", hc.key), IsError: true}, nil
	}

	// Hold RLock for the duration of the call to prevent use-after-Close race.
	// Shutdown acquires a write lock before closing, so this ensures the
	// client isn't closed while we're using it.
	h.mu.RLock()
	client := h.client
	if client == nil {
		h.mu.RUnlock()
		return CallResult{Content: fmt.Sprintf("mcp server %q not connected", hc.key), IsError: true}, nil
	}

	result, err := client.CallTool(ctx, name, arguments)
	h.mu.RUnlock()

	if err != nil {
		// Attempt respawn with a fresh background context (caller's may be expired)
		if respawned := hc.manager.tryRespawn(hc.key); respawned {
			// Retry once after respawn
			h.mu.RLock()
			client = h.client
			if client != nil {
				result, err = client.CallTool(ctx, name, arguments)
				h.mu.RUnlock()
				if err != nil {
					return CallResult{
						Content: fmt.Sprintf("mcp tool %q error after respawn: %v", name, err),
						IsError: true,
					}, nil
				}
				return result, nil
			}
			h.mu.RUnlock()
		}
		return CallResult{
			Content: fmt.Sprintf("mcp tool %q error: %v", name, err),
			IsError: true,
		}, nil
	}
	return result, nil
}

// tryRespawn attempts to restart a crashed MCP server. Returns true if successful.
// Uses a background context with timeout since the caller's context may be expired.
func (m *Manager) tryRespawn(key string) bool {
	m.mu.RLock()
	h, ok := m.handles[key]
	m.mu.RUnlock()
	if !ok {
		return false
	}

	h.mu.Lock()
	// Prevent double-respawn: if another goroutine is already respawning, bail out.
	if h.respawning {
		h.mu.Unlock()
		return false
	}

	if h.spawns >= maxRespawns {
		fmt.Fprintf(os.Stderr, "skaffen: warning: plugin %q server %q: max respawns reached (%d)\n",
			h.plugin, h.server, maxRespawns)
		h.client = nil
		h.mu.Unlock()
		return false
	}

	h.respawning = true
	h.mu.Unlock()

	// Close old client outside the lock (Close may block)
	h.mu.RLock()
	oldClient := h.client
	h.mu.RUnlock()
	if oldClient != nil {
		oldClient.Close()
	}

	// Look up server config
	pc, ok := m.config[h.plugin]
	if !ok {
		h.mu.Lock()
		h.respawning = false
		h.mu.Unlock()
		return false
	}
	sc, ok := pc.Servers[h.server]
	if !ok {
		h.mu.Lock()
		h.respawning = false
		h.mu.Unlock()
		return false
	}

	// Use a fresh background context for respawn
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := NewClient(ctx, sc.Command, sc.Args, sc.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skaffen: warning: respawn %q: %v\n", key, err)
		h.mu.Lock()
		h.client = nil
		h.respawning = false
		h.mu.Unlock()
		return false
	}

	h.mu.Lock()
	h.client = client
	h.spawns++
	h.respawning = false
	h.mu.Unlock()

	fmt.Fprintf(os.Stderr, "skaffen: respawned plugin %q server %q (attempt %d/%d)\n",
		h.plugin, h.server, h.spawns, maxRespawns)
	return true
}

// Shutdown closes all MCP server connections and kills subprocesses.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.shutdown = true
	handles := make(map[string]*serverHandle, len(m.handles))
	for k, v := range m.handles {
		handles[k] = v
	}
	m.mu.Unlock()

	for _, h := range handles {
		// Acquire write lock to wait for any in-flight calls to finish
		h.mu.Lock()
		if h.client != nil {
			h.client.Close()
			h.client = nil
		}
		h.mu.Unlock()
	}
}

// PluginCount returns the number of successfully connected plugins.
func (m *Manager) PluginCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, h := range m.handles {
		h.mu.RLock()
		if h.client != nil {
			count++
		}
		h.mu.RUnlock()
	}
	return count
}

// ToolCount returns the total number of MCP tools registered.
func (m *Manager) ToolCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, h := range m.handles {
		count += len(h.tools)
	}
	return count
}
