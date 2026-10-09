package mcp

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

// remoteServerName is the "server" half of a remote tool's qualified name, so
// a remote called "finance" exposes tools as finance_remote_<tool>.
const remoteServerName = "remote"

// remoteHandle tracks one connected remote server. Unlike a stdio server it is
// never respawned from configuration: a reconnect reuses the credentials the
// remote already holds and never asks the user again.
type remoteHandle struct {
	name    string
	r       *remote
	allowed map[string]bool // server-side tool names the config permits
	tools   []ToolInfo      // allowed tools the server offered, already scrubbed

	rmu        sync.Mutex   // serialises reconnects
	mu         sync.RWMutex // protects client, gen, reconnects, closed
	client     *Client
	gen        uint64
	reconnects int
	closed     bool
}

// SetRemoteRootCAs makes remote connections trust the given roots instead of
// the system pool. It must be called before ConnectRemote; it exists for tests
// that serve a fake remote over a private certificate.
func (m *Manager) SetRemoteRootCAs(pool *x509.CertPool) {
	m.mu.Lock()
	m.remoteOpts.rootCAs = pool
	m.mu.Unlock()
}

// ConnectRemote runs the consent flow for one remote server, connects, and
// registers its allowlisted tools. ctx bounds only this startup. Tools the
// server offers beyond the allowlist are never registered and can never be
// called. On any failure nothing stays registered and no credential is kept.
// Calls must not overlap with LoadAll or with agent execution: the tool
// registry is not safe for concurrent mutation.
func (m *Manager) ConnectRemote(ctx context.Context, cfg RemoteConfig, consent Consent) error {
	m.connectMu.Lock()
	defer m.connectMu.Unlock()

	m.mu.RLock()
	down := m.shutdown
	_, dup := m.remotes[cfg.Name]
	_, clash := m.config[cfg.Name]
	opts := m.remoteOpts
	m.mu.RUnlock()
	switch {
	case down:
		return fmt.Errorf("remote %q: mcp manager is shut down", cfg.Name)
	case dup:
		return fmt.Errorf("remote %q: already connected", cfg.Name)
	case clash:
		return fmt.Errorf("remote %q: name collides with a stdio plugin", cfg.Name)
	}

	r := newRemote(cfg, opts)
	m.mu.Lock()
	if m.shutdown {
		m.mu.Unlock()
		r.shutdown(nil)
		return fmt.Errorf("remote %q: mcp manager is shut down", cfg.Name)
	}
	m.pending[r] = struct{}{}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.pending, r)
		m.mu.Unlock()
	}()

	fail := func(c *Client, err error) error {
		r.shutdown(c)
		if c != nil {
			// If Manager.Shutdown closed this remote first, shutdown above was
			// a no-op; close the session here so it cannot linger.
			_ = c.Close()
		}
		return fmt.Errorf("remote %q: %w", cfg.Name, err)
	}

	c, err := r.connect(ctx, consent)
	if err != nil {
		return fail(nil, err)
	}
	offered, err := c.ListTools(ctx)
	if err != nil {
		return fail(c, err)
	}

	h := &remoteHandle{name: cfg.Name, r: r, client: c, allowed: make(map[string]bool, len(cfg.Tools))}
	for _, n := range cfg.Tools {
		h.allowed[n] = true
	}
	hidden := 0
	seen := make(map[string]bool, len(offered))
	for _, ti := range offered {
		switch {
		case !h.allowed[ti.Name]:
			hidden++
		case !seen[ti.Name]:
			seen[ti.Name] = true
			h.tools = append(h.tools, ti)
		}
	}
	if hidden > 0 {
		fmt.Fprintf(os.Stderr, "skaffen: remote %q offers %d tool(s) outside its allowlist; they stay hidden\n", cfg.Name, hidden)
	}
	var missing []string
	for _, n := range cfg.Tools {
		if !seen[n] {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(os.Stderr, "skaffen: warning: remote %q does not offer allowlisted tool(s) %q\n", cfg.Name, missing)
	}
	if len(h.tools) == 0 {
		return fail(c, errors.New("the server offers none of the allowlisted tools"))
	}
	for _, ti := range h.tools {
		qualified := cfg.Name + "_" + remoteServerName + "_" + ti.Name
		if _, taken := m.registry.Get(qualified); taken {
			return fail(c, fmt.Errorf("tool name %q is already registered", qualified))
		}
	}

	m.mu.Lock()
	if m.shutdown {
		m.mu.Unlock()
		return fail(c, errors.New("mcp manager is shut down"))
	}
	m.remotes[cfg.Name] = h
	m.mu.Unlock()

	phases := make([]tool.Phase, len(cfg.Phases))
	for i, p := range cfg.Phases {
		phases[i] = tool.Phase(p)
	}
	caller := &remoteCaller{m: m, h: h}
	for _, ti := range h.tools {
		m.registry.RegisterForPhases(NewMCPTool(cfg.Name, remoteServerName, ti, caller), phases)
	}
	return nil
}

// shutdown closes the session and wipes the credentials. Reconnects stop first
// so none can start after the remote begins closing.
func (h *remoteHandle) shutdown() {
	h.mu.Lock()
	h.closed = true
	c := h.client
	h.client = nil
	h.mu.Unlock()
	h.r.shutdown(c)
}

func (h *remoteHandle) snapshot() (*Client, uint64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.client, h.gen
}

// remoteCaller implements ToolCaller for a remote. Every call passes the
// allowlist again, so a tool name that was never registered cannot reach the
// server even if a caller builds the request by hand.
type remoteCaller struct {
	m *Manager
	h *remoteHandle
}

func (rc *remoteCaller) refusal(format string, args ...any) CallResult {
	return CallResult{Content: rc.h.r.scrub(fmt.Sprintf(format, args...)), IsError: true}
}

// unavailable explains why the remote cannot serve a call right now, or
// reports false while it can.
func (rc *remoteCaller) unavailable() (CallResult, bool) {
	rc.m.mu.RLock()
	down := rc.m.shutdown
	rc.m.mu.RUnlock()
	r := rc.h.r
	if fe := r.failureErr(); fe != nil {
		return rc.refusal("remote %q is unavailable: %v", rc.h.name, fe), true
	}
	if down || r.closing.Load() {
		return rc.refusal("mcp manager is shut down"), true
	}
	return CallResult{}, false
}

func (rc *remoteCaller) CallTool(ctx context.Context, name string, arguments map[string]any) (CallResult, error) {
	h := rc.h
	if !h.allowed[name] {
		return rc.refusal("mcp tool %q is not enabled for remote %q", name, h.name), nil
	}
	if res, refused := rc.unavailable(); refused {
		return res, nil
	}
	client, gen := h.snapshot()
	if client == nil {
		return rc.refusal("remote %q is not connected", h.name), nil
	}
	res, err := client.CallTool(ctx, name, arguments)
	if err == nil {
		return res, nil
	}
	if !rc.reconnectable(ctx, err) {
		return rc.failed(name, err), nil
	}
	client, ok := rc.reconnect(gen)
	if !ok {
		return rc.failed(name, err), nil
	}
	res, err = client.CallTool(ctx, name, arguments)
	if err != nil {
		return rc.failed(name, err), nil
	}
	return res, nil
}

// failed turns an error into a model-visible result. A terminal state wins
// over the SDK's flattened error text, since it says what the user can do.
func (rc *remoteCaller) failed(name string, err error) CallResult {
	if res, refused := rc.unavailable(); refused {
		return res
	}
	return rc.refusal("mcp tool %q error: %v", name, err)
}

// reconnectable reports whether a failed call may be retried on a fresh
// session. It may not when the caller gave up, the remote is closing or in a
// terminal state, the server answered with a JSON-RPC error (the session
// worked and the server said no), or the call ran out of time.
func (rc *remoteCaller) reconnectable(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	if _, refused := rc.unavailable(); refused {
		return false
	}
	var rce *remoteCallError
	if errors.As(err, &rce) && rce.answered {
		return false
	}
	return !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)
}

// reconnect replaces the session that failed at generation gen. When another
// caller already replaced it, that caller's session is used. It reuses the
// held credentials, never prompts, and is limited to maxRespawns over the
// life of the remote.
func (rc *remoteCaller) reconnect(gen uint64) (*Client, bool) {
	h := rc.h
	h.rmu.Lock()
	defer h.rmu.Unlock()

	h.mu.RLock()
	cur, curGen, used, closed := h.client, h.gen, h.reconnects, h.closed
	h.mu.RUnlock()
	if closed {
		return nil, false
	}
	if curGen != gen {
		return cur, cur != nil
	}
	if _, refused := rc.unavailable(); refused {
		return nil, false
	}
	if used >= maxRespawns {
		fmt.Fprintf(os.Stderr, "skaffen: warning: remote %q: max reconnects reached (%d)\n", h.name, maxRespawns)
		return nil, false
	}

	r := h.r
	dctx, cancel := context.WithTimeout(context.Background(), r.opts.opTimeout)
	defer cancel()
	stop := context.AfterFunc(r.authCtx, cancel)
	defer stop()
	c, err := r.dial(dctx)

	h.mu.Lock()
	h.reconnects++
	if err != nil || h.closed {
		h.mu.Unlock()
		if err != nil {
			fmt.Fprintf(os.Stderr, "skaffen: warning: remote %q: reconnect failed: %s\n", h.name, r.scrub(err.Error()))
		} else {
			_ = c.Close() // closing: the transport refuses the DELETE at once
		}
		return nil, false
	}
	old := h.client
	h.client = c
	h.gen++
	if old != nil {
		r.bg.Add(1) // under h.mu, so it cannot race with shutdown's Wait
		go func() {
			defer r.bg.Done()
			_ = old.Close()
		}()
	}
	h.mu.Unlock()
	fmt.Fprintf(os.Stderr, "skaffen: reconnected remote %q (attempt %d/%d)\n", h.name, used+1, maxRespawns)
	return c, true
}

// remoteHandles returns the connected remotes in name order.
func (m *Manager) remoteHandles() []*remoteHandle {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.remotes))
	for n := range m.remotes {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]*remoteHandle, len(names))
	for i, n := range names {
		out[i] = m.remotes[n]
	}
	return out
}

// shutdownRemotes closes every remote, and any still waiting for consent, at
// once. Each is bounded by its own close grace.
func (m *Manager) shutdownRemotes() {
	handles := m.remoteHandles()
	m.mu.RLock()
	pending := make([]*remote, 0, len(m.pending))
	for r := range m.pending {
		pending = append(pending, r)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, h := range handles {
		wg.Add(1)
		go func() { defer wg.Done(); h.shutdown() }()
	}
	for _, r := range pending {
		wg.Add(1)
		go func() { defer wg.Done(); r.shutdown(nil) }()
	}
	wg.Wait()
}

func sortedRemoteNames(remotes map[string]*remoteHandle) []string {
	names := make([]string, 0, len(remotes))
	for n := range remotes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
