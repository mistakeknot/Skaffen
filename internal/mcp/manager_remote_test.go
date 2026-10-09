package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mistakeknot/Skaffen/internal/tool"
)

// consentLog counts how many times the user was asked to approve.
type consentLog struct{ n atomic.Int32 }

func (c *consentLog) consent(f *fakeAS) Consent {
	approve := autoApprove(f)
	return Consent{Timeout: 10 * time.Second, Prompt: func(remote, authURL string) {
		c.n.Add(1)
		approve(remote, authURL)
	}}
}

// newTestManager returns a manager with short remote timeouts that trusts the
// fake's certificate.
func newTestManager(t *testing.T, f *fakeAS, mut ...func(*remoteOptions)) (*Manager, *tool.Registry) {
	t.Helper()
	reg := tool.NewRegistry()
	mgr := NewManager(nil, reg, nil)
	opts := testOpts(f.pool())
	opts.closeGrace = 400 * time.Millisecond
	opts.httpTimeout = 10 * time.Second
	for _, m := range mut {
		m(&opts)
	}
	mgr.remoteOpts = opts
	t.Cleanup(mgr.Shutdown)
	return mgr, reg
}

// connectedManager runs the consent flow against the fake and returns the
// manager, its registry and the consent counter.
func connectedManager(t *testing.T, f *fakeAS, mut ...func(*remoteOptions)) (*Manager, *tool.Registry, *consentLog) {
	t.Helper()
	mgr, reg := newTestManager(t, f, mut...)
	cl := &consentLog{}
	if err := mgr.ConnectRemote(ctxT(t), f.config(), cl.consent(f)); err != nil {
		t.Fatalf("ConnectRemote: %v", err)
	}
	return mgr, reg, cl
}

const remoteOrient = tool.PhaseOrient

func execRemote(t *testing.T, reg *tool.Registry, name string) tool.ToolResult {
	t.Helper()
	return reg.Execute(ctxT(t), remoteOrient, name, json.RawMessage(`{}`))
}

func toolNames(reg *tool.Registry, phase tool.Phase) map[string]bool {
	out := map[string]bool{}
	for _, td := range reg.Tools(phase) {
		out[td.Name] = true
	}
	return out
}

func TestManagerConnectRemote_HappyPath(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	mgr, reg, cl := connectedManager(t, f)

	if n := cl.n.Load(); n != 1 {
		t.Errorf("consent prompts = %d, want 1", n)
	}

	// The authorization server saw the constant scope, S256 and the resource.
	auths := f.hits("/authorize")
	if len(auths) != 1 {
		t.Fatalf("authorize hits = %d, want 1", len(auths))
	}
	q := auths[0].Query
	if q.Get("scope") != financeReadScope {
		t.Errorf("authorize scope = %q, want %q", q.Get("scope"), financeReadScope)
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Errorf("authorize lacked an S256 challenge: %v", q)
	}
	if q.Get("resource") == "" {
		t.Error("authorize lacked the resource indicator")
	}
	for _, h := range f.hits("/token") {
		if h.Form.Get("resource") == "" {
			t.Error("a token request lacked the resource indicator")
		}
		if h.Form.Get("code_verifier") == "" && h.Form.Get("grant_type") == "authorization_code" {
			t.Error("the code exchange lacked the PKCE verifier")
		}
	}
	if dcr := f.lastDCR(); dcr["scope"] != financeReadScope {
		t.Errorf("DCR scope = %v, want %q", dcr["scope"], financeReadScope)
	}

	// Only allowlisted tools are visible, in the configured phase only.
	got := toolNames(reg, tool.PhaseOrient)
	want := map[string]bool{"finance_remote_list_accounts": true, "finance_remote_list_transactions": true}
	for n := range want {
		if !got[n] {
			t.Errorf("tool %q not registered; have %v", n, got)
		}
	}
	for n := range got {
		if !want[n] {
			t.Errorf("unexpected tool %q is visible to the model", n)
		}
		if strings.Contains(n, "create_transfer") || strings.Contains(n, "echo_token") {
			t.Errorf("an unlisted tool leaked into the registry: %q", n)
		}
	}
	if act := toolNames(reg, tool.PhaseAct); len(act) != 0 {
		t.Errorf("tools registered outside the configured phases: %v", act)
	}
	if mgr.PluginCount() != 1 || mgr.ToolCount() != 2 {
		t.Errorf("PluginCount/ToolCount = %d/%d, want 1/2", mgr.PluginCount(), mgr.ToolCount())
	}

	res := execRemote(t, reg, "finance_remote_list_accounts")
	if res.IsError || !strings.Contains(res.Content, "acct-1") {
		t.Fatalf("list_accounts: %+v", res)
	}
	if n := m.toolCallCount("list_accounts"); n != 1 {
		t.Errorf("server executions = %d, want 1", n)
	}
}

func TestManagerConnectRemote_AllowlistWarnings(t *testing.T) {
	stderr := captureStderr(t)
	f := newFakeAS(t)
	f.attachMCP()
	mgr, reg := newTestManager(t, f)
	cfg := f.config()
	cfg.Tools = []string{"list_accounts", "no_such_tool"}
	if err := mgr.ConnectRemote(ctxT(t), cfg, (&consentLog{}).consent(f)); err != nil {
		t.Fatalf("ConnectRemote: %v", err)
	}
	if got := toolNames(reg, tool.PhaseOrient); len(got) != 1 || !got["finance_remote_list_accounts"] {
		t.Errorf("tools = %v, want only list_accounts", got)
	}
	mgr.Shutdown()
	out := stderr()
	if !strings.Contains(out, "no_such_tool") {
		t.Errorf("no warning for an allowlisted tool the server does not offer: %q", out)
	}
	for _, hidden := range []string{"create_transfer", "echo_token", "list_transactions"} {
		if strings.Contains(out, hidden) {
			t.Errorf("a warning named an unlisted server tool %q: %q", hidden, out)
		}
	}
}

func TestManagerConnectRemote_Rejections(t *testing.T) {
	t.Run("name collision", func(t *testing.T) {
		f := newFakeAS(t)
		f.attachMCP()
		mgr, _, cl := connectedManager(t, f)
		if err := mgr.ConnectRemote(ctxT(t), f.config(), cl.consent(f)); err == nil {
			t.Fatal("a second remote with the same name must be refused")
		}
		if n := cl.n.Load(); n != 1 {
			t.Errorf("consent prompts = %d: the refused duplicate must not prompt", n)
		}
	})
	t.Run("after shutdown", func(t *testing.T) {
		f := newFakeAS(t)
		f.attachMCP()
		mgr, _ := newTestManager(t, f)
		mgr.Shutdown()
		cl := &consentLog{}
		if err := mgr.ConnectRemote(ctxT(t), f.config(), cl.consent(f)); err == nil {
			t.Fatal("connect after shutdown must fail")
		}
		if cl.n.Load() != 0 {
			t.Error("a shut-down manager prompted the user")
		}
	})
	t.Run("no allowlisted tool offered", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		mgr, reg := newTestManager(t, f)
		cfg := f.config()
		cfg.Tools = []string{"no_such_tool"}
		if err := mgr.ConnectRemote(ctxT(t), cfg, (&consentLog{}).consent(f)); err == nil {
			t.Fatal("a remote offering none of the allowlisted tools must not stay connected")
		}
		if len(reg.Tools(tool.PhaseOrient)) != 0 || mgr.PluginCount() != 0 {
			t.Error("a rejected remote left tools or a handle behind")
		}
		if m.count("DELETE") != 1 {
			t.Errorf("DELETE requests = %d, want the session closed once", m.count("DELETE"))
		}
	})
	t.Run("consent failure leaves nothing", func(t *testing.T) {
		f := newFakeAS(t)
		f.attachMCP()
		mgr, reg := newTestManager(t, f)
		err := mgr.ConnectRemote(ctxT(t), f.config(), Consent{Timeout: 50 * time.Millisecond, Prompt: func(string, string) {}})
		if !errors.Is(err, ErrConsentTimeout) {
			t.Fatalf("err = %v, want ErrConsentTimeout", err)
		}
		if len(reg.Tools(tool.PhaseOrient)) != 0 || mgr.PluginCount() != 0 {
			t.Error("a failed consent left tools or a handle behind")
		}
	})
}

func TestManagerConnectRemote_RefreshAfterStartupContextCancelled(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	mgr, reg := newTestManager(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	if err := mgr.ConnectRemote(ctx, f.config(), (&consentLog{}).consent(f)); err != nil {
		t.Fatalf("ConnectRemote: %v", err)
	}
	cancel()

	first := m.lastToken()
	f.expireAccess(first)
	res := execRemote(t, reg, "finance_remote_list_accounts")
	if res.IsError {
		t.Fatalf("call after the startup context was cancelled: %+v", res)
	}
	if n := refreshCalls(f); n != 1 {
		t.Errorf("refresh calls = %d, want 1", n)
	}
	if m.lastToken() == first {
		t.Error("the retry reused the rejected token")
	}
}

func TestManagerCallTool_RefusesUnlistedTools(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	mgr, reg, _ := connectedManager(t, f)

	for _, name := range []string{"create_transfer", "echo_token", "no_such_tool"} {
		before := m.requests()
		_, err := mgr.CallTool(ctxT(t), name, nil)
		if err == nil || !strings.Contains(err.Error(), "not found in any connected server") {
			t.Errorf("Manager.CallTool(%q) err = %v, want the not-found error", name, err)
		}
		if m.requests() != before {
			t.Errorf("Manager.CallTool(%q) sent a request to the server", name)
		}
	}

	// The caller re-checks on its own, so a tool built around a different
	// name cannot reach the server either.
	mt, ok := reg.Get("finance_remote_list_accounts")
	if !ok {
		t.Fatal("list_accounts not registered")
	}
	caller := mt.(*MCPTool).caller
	for _, name := range []string{"create_transfer", "echo_token"} {
		before := m.requests()
		res, err := caller.CallTool(ctxT(t), name, nil)
		if err == nil && !res.IsError {
			t.Errorf("caller.CallTool(%q) succeeded", name)
		}
		if m.requests() != before || m.toolCallCount(name) != 0 {
			t.Errorf("caller.CallTool(%q) reached the server", name)
		}
	}

	// An allowlisted tool still works through Manager.CallTool.
	res, err := mgr.CallTool(ctxT(t), "list_accounts", nil)
	if err != nil || res.IsError || !strings.Contains(res.Content, "acct-1") {
		t.Errorf("Manager.CallTool(list_accounts) = %+v, %v", res, err)
	}
}

func TestManagerRemotePolicy(t *testing.T) {
	cases := []struct {
		name        string
		setup       func(f *fakeAS, m *fakeMCP)
		wantOK      bool // whether the first call should succeed
		wantInit    int
		wantState   failureKind
		wantRefresh int
	}{
		{
			name:      "terminal 401 never reconnects",
			setup:     func(_ *fakeAS, m *fakeMCP) { m.set(func(k *fakeKnobs) { k.unauthorized = true }) },
			wantInit:  1,
			wantState: kindTerminalAuth, wantRefresh: 1,
		},
		{
			name:      "403 never reconnects",
			setup:     func(_ *fakeAS, m *fakeMCP) { m.set(func(k *fakeKnobs) { k.forbid = true }) },
			wantInit:  1,
			wantState: kindTerminalAuth,
		},
		{
			name: "scope mismatch on refresh never reconnects",
			setup: func(f *fakeAS, m *fakeMCP) {
				f.expireAccess(m.lastToken())
				f.tune(func(f *fakeAS) { f.scopeRaw["refresh_token"] = `"finance:read finance:write"` })
			},
			wantInit:  1,
			wantState: kindTerminalAuth, wantRefresh: 1,
		},
		{
			name: "refresh 5xx is terminal and never reconnects",
			setup: func(f *fakeAS, m *fakeMCP) {
				f.expireAccess(m.lastToken())
				f.tune(func(f *fakeAS) { f.refreshMode = "500" })
			},
			wantInit:  1,
			wantState: kindTerminalAuth, wantRefresh: 1,
		},
		{
			name: "refresh network error is terminal and never reconnects",
			setup: func(f *fakeAS, m *fakeMCP) {
				f.expireAccess(m.lastToken())
				f.tune(func(f *fakeAS) { f.refreshMode = "drop" })
			},
			wantInit:  1,
			wantState: kindTerminalAuth, wantRefresh: 1,
		},
		{
			name: "refresh malformed success is terminal and never reconnects",
			setup: func(f *fakeAS, m *fakeMCP) {
				f.expireAccess(m.lastToken())
				f.tune(func(f *fakeAS) { f.refreshMode = "malformed" })
			},
			wantInit:  1,
			wantState: kindTerminalAuth, wantRefresh: 1,
		},
		{
			name: "refresh oversized response is terminal and never reconnects",
			setup: func(f *fakeAS, m *fakeMCP) {
				f.expireAccess(m.lastToken())
				f.tune(func(f *fakeAS) { f.refreshMode = "oversize" })
			},
			wantInit:  1,
			wantState: kindTerminalAuth, wantRefresh: 1,
		},
		{
			name:      "event stream never reconnects",
			setup:     func(_ *fakeAS, m *fakeMCP) { m.set(func(k *fakeKnobs) { k.sse = true }) },
			wantInit:  1,
			wantState: kindProtocol,
		},
		{
			name:      "non-JSON body never reconnects",
			setup:     func(_ *fakeAS, m *fakeMCP) { m.set(func(k *fakeKnobs) { k.callMode = "text-length" }) },
			wantInit:  1,
			wantState: kindProtocol,
		},
		{
			name:     "JSON-RPC error from the server never reconnects",
			setup:    func(_ *fakeAS, m *fakeMCP) { m.set(func(k *fakeKnobs) { k.callMode = "rpc-error-echo" }) },
			wantInit: 1,
		},
		{
			name:     "lost session reconnects once without consent",
			setup:    func(_ *fakeAS, m *fakeMCP) { m.set(func(k *fakeKnobs) { k.dropSessions = 1 }) },
			wantOK:   true,
			wantInit: 2,
		},
		{
			name:     "persistently lost session reconnects once per call, not in a loop",
			setup:    func(_ *fakeAS, m *fakeMCP) { m.set(func(k *fakeKnobs) { k.dropSessions = 1000 }) },
			wantInit: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAS(t)
			m := f.attachMCP()
			_, reg, cl := connectedManager(t, f)
			tokensBefore := len(f.hits("/token"))
			tc.setup(f, m)

			res := execRemote(t, reg, "finance_remote_list_accounts")
			if res.IsError == tc.wantOK {
				t.Fatalf("first call IsError = %v, want %v (%q)", res.IsError, !tc.wantOK, res.Content)
			}
			if n := m.count("initialize"); n != tc.wantInit {
				t.Errorf("initialize requests = %d, want %d", n, tc.wantInit)
			}
			if n := cl.n.Load(); n != 1 {
				t.Errorf("consent prompts = %d, want 1: a reconnect must never ask the user again", n)
			}
			if n := len(f.hits("/authorize")); n != 1 {
				t.Errorf("authorize requests = %d, want 1", n)
			}
			if tc.wantOK {
				if n := len(f.hits("/token")) - tokensBefore; n != 0 {
					t.Errorf("a reconnect made %d token requests", n)
				}
			}
			if tc.wantRefresh > 0 {
				if n := refreshCalls(f); n != tc.wantRefresh {
					t.Errorf("refresh calls = %d, want %d", n, tc.wantRefresh)
				}
			}

			// A terminal state stays terminal: nothing more leaves the host.
			if tc.wantState != kindNone {
				reqs, toks := m.requests(), len(f.hits("/token"))
				seen := len(m.tokensSeen())
				for i := 0; i < 3; i++ {
					again := execRemote(t, reg, "finance_remote_list_accounts")
					if !again.IsError {
						t.Fatal("a terminal remote answered a call")
					}
				}
				if m.requests() != reqs || len(f.hits("/token")) != toks || len(m.tokensSeen()) != seen {
					t.Error("a terminal remote sent more requests")
				}
				if n := cl.n.Load(); n != 1 {
					t.Errorf("consent prompts = %d after terminal state, want 1", n)
				}
			}
			// The MCP fake must never have been shown a token the
			// authorization server had already invalidated by a refresh.
			if tc.name == "scope mismatch on refresh never reconnects" {
				first := m.tokensSeen()[0]
				for _, tok := range m.tokensSeen() {
					if tok != first {
						t.Error("the MCP fake saw the over-broad grant")
					}
				}
			}
		})
	}
}

func TestManagerRemotePolicy_TerminalStateRefusalIsSanitised(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	_, reg, _ := connectedManager(t, f)
	m.set(func(k *fakeKnobs) { k.forbid = true })
	tok := m.lastToken()

	res := execRemote(t, reg, "finance_remote_list_accounts")
	if !res.IsError {
		t.Fatal("expected an error result")
	}
	assertNoSecret(t, "first refusal", []string{tok}, res.Content)
	res = execRemote(t, reg, "finance_remote_list_accounts")
	assertNoSecret(t, "later refusal", []string{tok}, res.Content)
	if !strings.Contains(res.Content, "authorize again") {
		t.Errorf("a terminal refusal should tell the user how to recover: %q", res.Content)
	}
}

func TestManagerRemotePolicy_CallerContextCancelledDoesNotReconnect(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	_, reg, _ := connectedManager(t, f)
	m.set(func(k *fakeKnobs) { k.callMode = "stall" })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan tool.ToolResult, 1)
	go func() {
		done <- reg.Execute(ctx, remoteOrient, "finance_remote_list_accounts", json.RawMessage(`{}`))
	}()
	select {
	case <-m.stalled:
	case <-time.After(10 * time.Second):
		t.Fatal("stall never started")
	}
	cancel()
	select {
	case res := <-done:
		if !res.IsError {
			t.Error("a cancelled call should fail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling the caller did not end the call")
	}
	if n := m.count("initialize"); n != 1 {
		t.Errorf("initialize requests = %d: a cancelled caller must not trigger a reconnect", n)
	}
}

func TestManagerRemotePolicy_ConcurrentCallers(t *testing.T) {
	const callers = 8
	t.Run("one lost session means one reconnect", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		_, reg, cl := connectedManager(t, f)
		tokens := len(f.hits("/token"))
		m.set(func(k *fakeKnobs) { k.dropSessions = 1 })

		var wg sync.WaitGroup
		results := make(chan tool.ToolResult, callers)
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- execRemote(t, reg, "finance_remote_list_accounts")
			}()
		}
		wg.Wait()
		close(results)
		for res := range results {
			if res.IsError {
				t.Errorf("concurrent call failed: %q", res.Content)
			}
		}
		if n := m.count("initialize"); n != 2 {
			t.Errorf("initialize requests = %d, want exactly 2 (one reconnect)", n)
		}
		if n := cl.n.Load(); n != 1 {
			t.Errorf("consent prompts = %d, want 1", n)
		}
		if n := len(f.hits("/token")) - tokens; n != 0 {
			t.Errorf("token requests during reconnect = %d, want 0", n)
		}
		if n := m.toolCallCount("list_accounts"); n < callers-1 {
			t.Errorf("server executions = %d, want at least %d", n, callers-1)
		}
	})

	t.Run("expired token means one refresh", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		_, reg, cl := connectedManager(t, f)
		f.expireAccess(m.lastToken())

		var wg sync.WaitGroup
		results := make(chan tool.ToolResult, callers)
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- execRemote(t, reg, "finance_remote_list_accounts")
			}()
		}
		wg.Wait()
		close(results)
		for res := range results {
			if res.IsError {
				t.Errorf("concurrent call failed: %q", res.Content)
			}
		}
		if n := refreshCalls(f); n != 1 {
			t.Errorf("refresh calls = %d, want exactly 1", n)
		}
		if n := m.count("initialize"); n != 1 {
			t.Errorf("initialize requests = %d, want 1: an expired token is not a lost session", n)
		}
		if n := cl.n.Load(); n != 1 {
			t.Errorf("consent prompts = %d, want 1", n)
		}
		if n := m.toolCallCount("list_accounts"); n != callers {
			t.Errorf("server executions = %d, want %d", n, callers)
		}
	})

	t.Run("terminal 401 is reached once", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		_, reg, cl := connectedManager(t, f)
		m.set(func(k *fakeKnobs) { k.unauthorized = true })

		var wg sync.WaitGroup
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if res := execRemote(t, reg, "finance_remote_list_accounts"); !res.IsError {
					t.Error("a call succeeded against a rejecting server")
				}
			}()
		}
		wg.Wait()
		if n := refreshCalls(f); n > 1 {
			t.Errorf("refresh calls = %d, want at most 1", n)
		}
		if n := m.count("initialize"); n != 1 {
			t.Errorf("initialize requests = %d, want 1", n)
		}
		if n := cl.n.Load(); n != 1 {
			t.Errorf("consent prompts = %d, want 1", n)
		}
		if n := m.count("tools/call"); n > 2*callers {
			t.Errorf("tools/call requests = %d, want at most %d", n, 2*callers)
		}
	})
}

func TestManagerShutdown_HangingDeleteIsBounded(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	// The default grace (five seconds) is what the plan bounds.
	mgr, _, _ := connectedManager(t, f, func(o *remoteOptions) { o.closeGrace = 0 })
	m.set(func(k *fakeKnobs) { k.deleteMode = "hang" })

	start := time.Now()
	mgr.Shutdown()
	if d := time.Since(start); d > 7*time.Second {
		t.Errorf("Shutdown took %v with a hanging DELETE, want about 5s", d)
	}
	if m.count("DELETE") != 1 {
		t.Errorf("DELETE requests = %d, want 1", m.count("DELETE"))
	}
	if mgr.PluginCount() != 0 {
		t.Error("remote still counted after shutdown")
	}
}

func TestManagerShutdown_AbortsStalledCall(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	mgr, reg, _ := connectedManager(t, f)
	m.set(func(k *fakeKnobs) { k.callMode = "stall" })

	done := make(chan tool.ToolResult, 1)
	go func() { done <- execRemote(t, reg, "finance_remote_list_accounts") }()
	select {
	case <-m.stalled:
	case <-time.After(10 * time.Second):
		t.Fatal("stall never started")
	}
	mgr.Shutdown()
	select {
	case res := <-done:
		if !res.IsError {
			t.Error("the stalled call should have failed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not abort the in-flight call")
	}
	if n := m.count("initialize"); n != 1 {
		t.Errorf("initialize requests = %d: shutdown must not trigger a reconnect", n)
	}

	// Calls after shutdown are refused without touching the network.
	reqs := m.requests()
	if res := execRemote(t, reg, "finance_remote_list_accounts"); !res.IsError {
		t.Error("a call after shutdown succeeded")
	}
	if _, err := mgr.CallTool(ctxT(t), "list_accounts", nil); err == nil {
		t.Error("Manager.CallTool after shutdown succeeded")
	}
	if m.requests() != reqs {
		t.Error("a request left the host after shutdown")
	}
}

func TestManagerRemote_NoCredentialInResultsOrLogs(t *testing.T) {
	stderr := captureStderr(t)
	f := newFakeAS(t)
	m := f.attachMCP()
	mgr, reg := newTestManager(t, f)
	cfg := f.config()
	// echo_token reflects the caller credential, so allowlist it here to
	// exercise the model-visible path.
	cfg.Tools = []string{"list_accounts", "echo_token"}
	if err := mgr.ConnectRemote(ctxT(t), cfg, (&consentLog{}).consent(f)); err != nil {
		t.Fatalf("ConnectRemote: %v", err)
	}
	tok := m.lastToken()
	if tok == "" {
		t.Fatal("no token reached the server")
	}

	res := execRemote(t, reg, "finance_remote_echo_token")
	if res.IsError {
		t.Fatalf("echo_token: %+v", res)
	}
	assertNoSecret(t, "tool result", []string{tok}, res.Content)
	if !strings.Contains(res.Content, redactedText) {
		t.Errorf("echoed credential not replaced: %q", res.Content)
	}

	m.set(func(k *fakeKnobs) { k.callMode = "rpc-error-echo" })
	res = execRemote(t, reg, "finance_remote_list_accounts")
	if !res.IsError {
		t.Fatal("expected a JSON-RPC error result")
	}
	assertNoSecret(t, "rpc error result", []string{tok}, res.Content)

	m.set(func(k *fakeKnobs) { k.callMode = ""; k.forbid = true })
	res = execRemote(t, reg, "finance_remote_list_accounts")
	assertNoSecret(t, "403 result", []string{tok}, res.Content)

	mgr.Shutdown()
	assertNoSecret(t, "stderr", m.tokensSeen(), stderr())
}

func TestManagerRemotePolicy_CancelWhileAnotherCallerHoldsReconnectLock(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	mgr, reg, _ := connectedManager(t, f)
	h := mgr.remotes[f.config().Name]
	if h == nil {
		t.Fatal("remote handle not found")
	}
	m.set(func(k *fakeKnobs) { k.dropSessions = 1 })

	// Another caller owns the reconnect lock.
	h.rmu.Lock()
	held := true
	release := func() {
		if held {
			held = false
			h.rmu.Unlock()
		}
	}
	defer release()

	before := m.requests()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan tool.ToolResult, 1)
	go func() {
		done <- reg.Execute(ctx, remoteOrient, "finance_remote_list_accounts", json.RawMessage(`{}`))
	}()
	deadline := time.Now().Add(10 * time.Second)
	for m.requests() == before { // the call reached the server and was told the session is gone
		if time.Now().After(deadline) {
			t.Fatal("the call never reached the server")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // let the caller queue behind the lock
	cancel()

	select {
	case res := <-done:
		if !res.IsError {
			t.Error("a cancelled call should fail")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled caller kept waiting for the reconnect lock")
	}
	release()
	time.Sleep(200 * time.Millisecond)
	if n := m.count("initialize"); n != 1 {
		t.Errorf("initialize requests = %d: a caller that gave up must not dial", n)
	}
	h.mu.RLock()
	used := h.reconnects
	h.mu.RUnlock()
	if used != 0 {
		t.Errorf("reconnects used = %d, want 0", used)
	}
}

func TestManagerRemotePolicy_OneDeadlineAcrossReconnectAndRetry(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	const op = 900 * time.Millisecond
	_, reg, _ := connectedManager(t, f, func(o *remoteOptions) { o.opTimeout = op })
	// The session is lost, the reconnect takes a while, and the retried call
	// then stalls: all of it is one operation with one deadline.
	m.set(func(k *fakeKnobs) { k.dropSessions = 1; k.initDelay = 500 * time.Millisecond })
	var armed atomic.Bool
	go func() { // stall only the retry, once the reconnect has started
		for !armed.Load() {
			if m.count("initialize") >= 2 {
				m.set(func(k *fakeKnobs) { k.callMode = "stall" })
				armed.Store(true)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	defer armed.Store(true)

	start := time.Now()
	res := execRemote(t, reg, "finance_remote_list_accounts")
	elapsed := time.Since(start)
	if !res.IsError {
		t.Fatal("the stalled retry should fail")
	}
	if elapsed > op+op/4 {
		t.Errorf("operation took %v; the call, reconnect and retry must share one %v deadline", elapsed, op)
	}
}
