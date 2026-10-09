package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubTransport is a base RoundTripper that never touches the network.
type stubTransport struct {
	mu    sync.Mutex
	calls []*http.Request
	fn    func(*http.Request) (*http.Response, error)
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.calls = append(s.calls, req)
	s.mu.Unlock()
	return s.fn(req)
}

func (s *stubTransport) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// stubBody records how much of a response body was read and whether it was
// closed.
type stubBody struct {
	r      io.Reader
	read   atomic.Int64
	closed atomic.Bool
}

func (b *stubBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.read.Add(int64(n))
	return n, err
}

func (b *stubBody) Close() error { b.closed.Store(true); return nil }

const stubAccess = "SENTINEL-ACCESS-stub-00000001"

// newStubRemote builds a remote whose base transport is a stub and whose token
// is already set, so RoundTrip logic can be exercised without a server.
func newStubRemote(t *testing.T, opts remoteOptions, fn func(*http.Request) (*http.Response, error)) (*remote, *stubTransport) {
	t.Helper()
	cfg := RemoteConfig{
		Name: "finance", URL: "https://127.0.0.1:1/mcp", Issuer: "https://127.0.0.1:1",
		Phases: []string{"orient"}, Tools: []string{"list_accounts"},
	}
	r := newRemote(cfg, opts)
	st := &stubTransport{fn: fn}
	r.rt.base = st
	r.tok = &tokenSet{access: r.oc.track(stubAccess)}
	t.Cleanup(func() { r.shutdown(nil) })
	return r, st
}

func stubResponse(status int, ct string, length int64, body string) (*http.Response, *stubBody) {
	sb := &stubBody{r: strings.NewReader(body)}
	h := http.Header{}
	if ct != "" {
		h.Set("Content-Type", ct)
	}
	return &http.Response{StatusCode: status, Header: h, ContentLength: length, Body: sb}, sb
}

func postRequest(t *testing.T, r *remote, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctxT(t), http.MethodPost, r.cfg.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// newRemoteForTest builds a remote against the fake with short timeouts.
func newRemoteForTest(t *testing.T, f *fakeAS, mut ...func(*remoteOptions)) *remote {
	t.Helper()
	opts := testOpts(f.pool())
	opts.closeGrace = 400 * time.Millisecond
	opts.httpTimeout = 10 * time.Second
	for _, m := range mut {
		m(&opts)
	}
	r := newRemote(f.config(), opts)
	t.Cleanup(func() { r.shutdown(nil) })
	return r
}

// connectT performs the consent flow against the fake and opens the session.
func connectT(t *testing.T, r *remote, f *fakeAS) *Client {
	t.Helper()
	c, err := r.connect(ctxT(t), Consent{Timeout: 10 * time.Second, Prompt: autoApprove(f)})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { r.shutdown(c) })
	return c
}

func refreshCalls(f *fakeAS) int {
	n := 0
	for _, h := range f.hits("/token") {
		if h.Form.Get("grant_type") == "refresh_token" {
			n++
		}
	}
	return n
}

func (r *remote) currentAccess() string {
	r.tokMu.Lock()
	defer r.tokMu.Unlock()
	if r.tok == nil {
		return ""
	}
	return r.tok.access.reveal()
}

// errStrings renders err every way a caller might, including its chain.
func errStrings(err error) []string {
	var out []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		out = append(out, e.Error(), fmt.Sprintf("%+v", e), fmt.Sprintf("%#v", e))
	}
	return out
}

func assertNoSecret(t *testing.T, what string, secrets []string, texts ...string) {
	t.Helper()
	for _, s := range secrets {
		if s == "" {
			continue
		}
		for _, tx := range texts {
			if strings.Contains(tx, s) {
				t.Errorf("%s leaks a credential: %q", what, tx)
			}
		}
	}
}

func TestRemoteConnect_HeaderlessNotificationReply(t *testing.T) {
	for _, tc := range []struct{ name, mode string }{
		{"known empty headerless 202", ""},
		{"chunked empty headerless 202", "chunked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAS(t)
			m := f.attachMCP()
			m.set(func(k *fakeKnobs) { k.notifyMode = tc.mode })
			r := newRemoteForTest(t, f)
			c := connectT(t, r, f)

			tools, err := c.ListTools(ctxT(t))
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			if len(tools) != 4 {
				t.Errorf("want the fake's 4 tools, got %d", len(tools))
			}
			if n := m.count("notifications/initialized"); n != 1 {
				t.Errorf("initialized notifications = %d, want 1", n)
			}
			if err := r.failureErr(); err != nil {
				t.Errorf("unexpected failure state: %v", err)
			}
		})
	}
}

func TestRemoteTransport_MediaRule(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		ct        string
		length    int64
		body      string
		violation bool
	}{
		{"known empty headerless 202", 202, "", 0, "", false},
		{"chunked empty headerless 202", 202, "", -1, "", false},
		{"chunked non-empty headerless 202", 202, "", -1, "x", true},
		{"json with charset", 200, "application/json; charset=utf-8", 2, "{}", false},
		{"chunked json", 200, "application/json", -1, "{}", false},
		{"declared text", 200, "text/plain", 5, "hello", true},
		{"chunked text", 200, "text/plain", -1, "hello", true},
		{"chunked body without type", 200, "", -1, "{}", true},
		{"event stream", 200, "text/event-stream", -1, "data: {}\n\n", true},
		{"empty event stream", 202, "text/event-stream", 0, "", true},
		{"non-2xx text passes through", 404, "text/plain", 4, "nope", false},
		{"unavailable passes through", 503, "text/html", 5, "later", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sb *stubBody
			r, st := newStubRemote(t, remoteOptions{}, func(*http.Request) (*http.Response, error) {
				var resp *http.Response
				resp, sb = stubResponse(tc.status, tc.ct, tc.length, tc.body)
				return resp, nil
			})
			resp, err := r.rt.RoundTrip(postRequest(t, r, `{"jsonrpc":"2.0"}`))
			if tc.violation {
				if !errors.Is(err, ErrRemoteProtocol) {
					t.Fatalf("err = %v, want ErrRemoteProtocol", err)
				}
				if r.failureKind() != kindProtocol {
					t.Errorf("state = %v, want protocolViolation", r.failureKind())
				}
				if !sb.closed.Load() {
					t.Error("violating response body was not closed")
				}
				return
			}
			if err != nil {
				t.Fatalf("RoundTrip: %v", err)
			}
			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			resp.Body.Close()
			if string(got) != tc.body {
				t.Errorf("body = %q, want %q (peeked byte must be preserved)", got, tc.body)
			}
			if r.failureErr() != nil {
				t.Errorf("state touched: %v", r.failureErr())
			}
			if st.count() != 1 {
				t.Errorf("base calls = %d, want 1", st.count())
			}
		})
	}
}

func TestRemoteTransport_BearerAndOrigin(t *testing.T) {
	r, st := newStubRemote(t, remoteOptions{}, func(*http.Request) (*http.Response, error) {
		resp, _ := stubResponse(200, "application/json", 2, "{}")
		return resp, nil
	})

	// The transport owns the Authorization header.
	req := postRequest(t, r, "{}")
	req.Header.Set("Authorization", "Bearer attacker-supplied")
	resp, err := r.rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	resp.Body.Close()
	if got := st.calls[0].Header.Get("Authorization"); got != "Bearer "+stubAccess {
		t.Errorf("Authorization = %q, want the remote's own token", got)
	}
	if req.Header.Get("Authorization") != "Bearer attacker-supplied" {
		t.Error("the caller's request was mutated")
	}

	for _, target := range []string{
		"https://127.0.0.1:2/mcp",    // other port
		"https://localhost:1/mcp",    // other host
		"http://127.0.0.1:1/mcp",     // plain http
		"https://127.0.0.1.evil:1/x", // lookalike
	} {
		before := st.count()
		bad, _ := http.NewRequestWithContext(ctxT(t), http.MethodPost, target, strings.NewReader("{}"))
		if resp, err := r.rt.RoundTrip(bad); err == nil {
			resp.Body.Close()
			t.Errorf("%s: want a refusal", target)
		}
		if st.count() != before {
			t.Errorf("%s: a refused request reached the base transport", target)
		}
	}
	if r.failureErr() != nil {
		t.Errorf("a refused origin must not poison the session: %v", r.failureErr())
	}
}

func TestRemoteTransport_Forbidden(t *testing.T) {
	var sb *stubBody
	r, st := newStubRemote(t, remoteOptions{}, func(*http.Request) (*http.Response, error) {
		var resp *http.Response
		resp, sb = stubResponse(403, "text/plain", 5, "no "+stubAccess)
		return resp, nil
	})
	_, err := r.rt.RoundTrip(postRequest(t, r, "{}"))
	if !errors.Is(err, ErrRemoteAuth) {
		t.Fatalf("err = %v, want ErrRemoteAuth", err)
	}
	if r.failureKind() != kindTerminalAuth {
		t.Errorf("state = %v", r.failureKind())
	}
	if !sb.closed.Load() {
		t.Error("403 body not closed")
	}
	assertNoSecret(t, "403 error", []string{stubAccess}, errStrings(err)...)

	// Terminal means no further request leaves the process.
	if _, err := r.rt.RoundTrip(postRequest(t, r, "{}")); !errors.Is(err, ErrRemoteAuth) {
		t.Errorf("second call err = %v", err)
	}
	if st.count() != 1 {
		t.Errorf("base calls = %d, want 1", st.count())
	}
}

func TestRemoteTransport_ClosingRefusesEverythingButDelete(t *testing.T) {
	r, st := newStubRemote(t, remoteOptions{}, func(req *http.Request) (*http.Response, error) {
		resp, _ := stubResponse(200, "", 0, "")
		return resp, nil
	})
	r.closing.Store(true)

	if _, err := r.rt.RoundTrip(postRequest(t, r, "{}")); !errors.Is(err, ErrRemoteClosed) {
		t.Errorf("POST while closing: err = %v, want ErrRemoteClosed", err)
	}
	if st.count() != 0 {
		t.Error("a POST reached the network while closing")
	}
	del, _ := http.NewRequestWithContext(ctxT(t), http.MethodDelete, r.cfg.URL, nil)
	resp, err := r.rt.RoundTrip(del)
	if err != nil {
		t.Fatalf("DELETE while closing: %v", err)
	}
	resp.Body.Close()
	if st.count() != 1 {
		t.Errorf("DELETE did not reach the base transport")
	}
}

func TestRemoteTransport_DeleteBodyBoundedAndClosed(t *testing.T) {
	var sb *stubBody
	r, _ := newStubRemote(t, remoteOptions{}, func(*http.Request) (*http.Response, error) {
		var resp *http.Response
		resp, sb = stubResponse(200, "text/plain", -1, strings.Repeat("D", 1<<20))
		return resp, nil
	})
	del, _ := http.NewRequestWithContext(ctxT(t), http.MethodDelete, r.cfg.URL, nil)
	resp, err := r.rt.RoundTrip(del)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	// The SDK never reads or closes a DELETE response body, so the transport
	// must already have released the real one.
	if !sb.closed.Load() {
		t.Error("real DELETE body not closed by the transport")
	}
	if n := sb.read.Load(); n > 4096 {
		t.Errorf("read %d bytes of a DELETE body, want at most 4096", n)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Errorf("replacement body unreadable: %v", err)
	}
	resp.Body.Close()
	if r.failureErr() != nil {
		t.Errorf("a DELETE response must not change state: %v", r.failureErr())
	}
}

func TestRemoteTransport_UnauthorizedWithoutReplayableBodyIsTerminal(t *testing.T) {
	r, st := newStubRemote(t, remoteOptions{}, func(*http.Request) (*http.Response, error) {
		resp, _ := stubResponse(401, "text/plain", 3, "no!")
		return resp, nil
	})
	req := postRequest(t, r, "{}")
	req.GetBody = nil
	req.Body = io.NopCloser(strings.NewReader("{}"))
	if _, err := r.rt.RoundTrip(req); !errors.Is(err, ErrRemoteAuth) {
		t.Fatalf("err = %v, want ErrRemoteAuth", err)
	}
	if st.count() != 1 {
		t.Errorf("a non-replayable request was sent %d times", st.count())
	}
	if r.failureKind() != kindTerminalAuth {
		t.Errorf("state = %v", r.failureKind())
	}
}

func TestRemoteRefresh_After401RetriesOnce(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	first := r.currentAccess()
	f.expireAccess(first)
	res, err := c.CallTool(ctxT(t), "list_accounts", nil)
	if err != nil || res.IsError {
		t.Fatalf("call after expiry: %+v, %v", res, err)
	}
	if n := refreshCalls(f); n != 1 {
		t.Errorf("refresh calls = %d, want 1", n)
	}
	for _, h := range f.hits("/token") {
		if h.Form.Get("resource") == "" {
			t.Error("a token request lacked the resource indicator")
		}
	}
	if r.currentAccess() == first {
		t.Error("access token was not replaced")
	}
	if m.lastToken() == first {
		t.Error("the retry reused the rejected token")
	}
	if r.failureErr() != nil {
		t.Errorf("state = %v", r.failureErr())
	}
}

func TestRemoteRefresh_SecondUnauthorizedIsTerminal(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	m.set(func(k *fakeKnobs) { k.unauthorized = true })
	if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(r.failureErr(), ErrRemoteAuth) || r.failureKind() != kindTerminalAuth {
		t.Fatalf("state = %v / %v", r.failureKind(), r.failureErr())
	}
	if n := refreshCalls(f); n != 1 {
		t.Errorf("refresh calls = %d, want exactly 1", n)
	}
	if n := m.count("tools/call"); n != 2 {
		t.Errorf("tools/call requests = %d, want 2 (original and one retry)", n)
	}
	before, tokens := m.requests(), len(f.hits("/token"))
	if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
		t.Fatal("expected a terminal error")
	}
	if m.requests() != before || len(f.hits("/token")) != tokens {
		t.Error("a terminal remote sent more requests")
	}
}

func TestRemoteRefresh_FailureIsTerminalAndOldTokenNotReused(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	first := r.currentAccess()
	f.expireAccess(first)
	f.tune(func(f *fakeAS) {
		f.tokenFail = &oauthFailure{Status: 400, Code: "invalid_grant", Description: "SENTINEL-DESC-" + f.tag}
	})
	_, err := c.CallTool(ctxT(t), "list_accounts", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	assertNoSecret(t, "refresh failure", []string{"SENTINEL-DESC-" + f.tag, first}, errStrings(err)...)
	if r.failureKind() != kindTerminalAuth {
		t.Errorf("state = %v, want terminalAuth", r.failureKind())
	}
	seen := len(m.tokensSeen())
	if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
		t.Fatal("expected a terminal error")
	}
	if len(m.tokensSeen()) != seen {
		t.Error("the rejected token was sent again after the refresh failed")
	}
}

func TestRemoteRefresh_ScopeMismatchIsTerminal(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	first := r.currentAccess()
	f.expireAccess(first)
	f.tune(func(f *fakeAS) { f.scopeRaw["refresh_token"] = `"finance:read finance:write"` })
	if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
		t.Fatal("expected an error")
	}
	if r.failureKind() != kindTerminalAuth {
		t.Errorf("state = %v", r.failureKind())
	}
	if len(f.revoked()) == 0 {
		t.Error("the over-broad token was not revoked")
	}
	for _, tok := range m.tokensSeen() {
		if tok != first {
			t.Errorf("the MCP fake saw a token other than the original: it must never see a rejected grant")
		}
	}
}

func TestRemoteRefresh_ConcurrentCallersShareOneRefresh(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	first := r.currentAccess()
	f.expireAccess(first)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := c.CallTool(ctxT(t), "list_accounts", nil)
			if err == nil && res.IsError {
				err = errors.New(res.Content)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent call: %v", err)
		}
	}
	if n := refreshCalls(f); n != 1 {
		t.Errorf("refresh calls = %d, want 1 (single flight)", n)
	}
	if n := m.toolCallCount("list_accounts"); n != 8 {
		t.Errorf("tool executions = %d, want 8", n)
	}
}

func TestRemoteScrub_DelayedEchoOfSupersededToken(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	first := r.currentAccess()
	gate := make(chan struct{})
	m.set(func(k *fakeKnobs) { k.echoGate = gate })
	type out struct {
		res CallResult
		err error
	}
	done := make(chan out, 1)
	go func() {
		res, err := c.CallTool(ctxT(t), "echo_token", nil)
		done <- out{res, err}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for m.toolCallCount("echo_token") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("echo call never reached the fake")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// While that response is delayed, the token is superseded.
	m.set(func(k *fakeKnobs) { k.echoGate = nil })
	f.expireAccess(first)
	if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err != nil {
		t.Fatalf("call that triggers the refresh: %v", err)
	}
	if r.currentAccess() == first {
		t.Fatal("token was not refreshed")
	}

	close(gate)
	o := <-done
	if o.err != nil {
		t.Fatalf("delayed echo call: %v", o.err)
	}
	if !strings.Contains(o.res.Content, "[redacted]") || strings.Contains(o.res.Content, first) {
		t.Errorf("a delayed echo of the superseded token was not scrubbed: %q", o.res.Content)
	}
}

func TestRemoteScrub_SchemaAndMetadataEcho(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	m.set(func(k *fakeKnobs) { k.echoMetadata = true })
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	tools, err := c.ListTools(ctxT(t))
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	tok := m.lastToken()
	if tok == "" {
		t.Fatal("fake saw no token")
	}
	var found bool
	for _, ti := range tools {
		blob := ti.Name + "\n" + ti.Description + "\n" + string(ti.InputSchema)
		if strings.Contains(blob, tok) {
			t.Errorf("tool %q leaks the access token in its metadata: %s", ti.Name, blob)
		}
		if ti.Name == "list_accounts" {
			found = true
			if strings.Count(blob, redactedText) < 3 {
				t.Errorf("want description, default and enum scrubbed, got %s", blob)
			}
		}
	}
	if !found {
		t.Fatal("list_accounts not listed")
	}
}

func TestRemoteBodyCapsAndTimeouts(t *testing.T) {
	t.Run("oversize chunked response", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		r := newRemoteForTest(t, f, func(o *remoteOptions) { o.mcpBodyCap = 4096 })
		c := connectT(t, r, f)
		m.set(func(k *fakeKnobs) { k.callMode = "oversize" })

		if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
			t.Fatal("expected an error")
		}
		if r.failureKind() != kindProtocol {
			t.Errorf("state = %v, want protocolViolation", r.failureKind())
		}
		if n := m.count("tools/call"); n != 1 {
			t.Errorf("tools/call = %d, want 1", n)
		}
		before := m.requests()
		if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
			t.Error("a violated remote must stay failed")
		}
		if m.requests() != before {
			t.Error("a request was sent after a protocol violation")
		}
	})

	t.Run("oversize declared length", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		r := newRemoteForTest(t, f, func(o *remoteOptions) { o.mcpBodyCap = 4096 })
		c := connectT(t, r, f)
		m.set(func(k *fakeKnobs) { k.callMode = "oversize-length" })

		if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
			t.Fatal("expected an error")
		}
		if r.failureKind() != kindProtocol {
			t.Errorf("state = %v, want protocolViolation", r.failureKind())
		}
	})

	t.Run("oversize OAuth response", func(t *testing.T) {
		f := newFakeAS(t)
		f.attachMCP()
		r := newRemoteForTest(t, f, func(o *remoteOptions) { o.oauthBodyCap = 64 })
		_, err := r.connect(ctxT(t), Consent{Timeout: 5 * time.Second, Prompt: autoApprove(f)})
		if err == nil || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("err = %v, want a response-too-large error", err)
		}
	})

	t.Run("stalled body is cut by the client timeout", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		r := newRemoteForTest(t, f, func(o *remoteOptions) { o.httpTimeout = 600 * time.Millisecond })
		c := connectT(t, r, f)
		m.set(func(k *fakeKnobs) { k.callMode = "stall" })

		start := time.Now()
		if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
			t.Fatal("expected an error")
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("stalled call took %v", d)
		}
		select {
		case <-m.stalled:
		default:
			t.Error("the stall never started; the test proves nothing")
		}
		if r.failureErr() != nil {
			t.Errorf("a timeout is a transport error, not a terminal state: %v", r.failureErr())
		}
	})

	t.Run("hanging DELETE is bounded at shutdown", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		r := newRemoteForTest(t, f)
		c := connectT(t, r, f)
		m.set(func(k *fakeKnobs) { k.deleteMode = "hang" })

		start := time.Now()
		r.shutdown(c)
		if d := time.Since(start); d > r.opts.closeGrace+2*time.Second {
			t.Errorf("shutdown took %v with grace %v", d, r.opts.closeGrace)
		}
		if n := m.count("DELETE"); n != 1 {
			t.Errorf("DELETE requests = %d, want 1", n)
		}
	})

	t.Run("401 DELETE neither refreshes nor hangs", func(t *testing.T) {
		f := newFakeAS(t)
		m := f.attachMCP()
		r := newRemoteForTest(t, f)
		c := connectT(t, r, f)
		m.set(func(k *fakeKnobs) { k.deleteMode = "401" })

		tokens := len(f.hits("/token"))
		start := time.Now()
		r.shutdown(c)
		if d := time.Since(start); d > r.opts.closeGrace+2*time.Second {
			t.Errorf("shutdown took %v", d)
		}
		if len(f.hits("/token")) != tokens {
			t.Error("shutdown requested a token")
		}
	})
}

func TestRemoteSSERejected(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)
	m.set(func(k *fakeKnobs) { k.sse = true })

	if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
		t.Fatal("expected an error")
	}
	if r.failureKind() != kindProtocol || !errors.Is(r.failureErr(), ErrRemoteProtocol) {
		t.Errorf("state = %v / %v", r.failureKind(), r.failureErr())
	}
	if n := m.count("tools/call"); n != 1 {
		t.Errorf("tools/call = %d, want exactly 1 (no SDK reconnect)", n)
	}
	if n := m.countHTTP(http.MethodGet); n != 0 {
		t.Errorf("%d GET requests: the stream must never be resumed", n)
	}
}

func TestRemoteNonJSONResponseIsViolation(t *testing.T) {
	for _, mode := range []string{"text-chunked", "text-length"} {
		t.Run(mode, func(t *testing.T) {
			f := newFakeAS(t)
			m := f.attachMCP()
			r := newRemoteForTest(t, f)
			c := connectT(t, r, f)
			m.set(func(k *fakeKnobs) { k.callMode = mode })

			if _, err := c.CallTool(ctxT(t), "list_accounts", nil); err == nil {
				t.Fatal("expected an error")
			}
			if r.failureKind() != kindProtocol {
				t.Errorf("state = %v", r.failureKind())
			}
			if n := m.count("tools/call"); n != 1 {
				t.Errorf("tools/call = %d, want 1", n)
			}
		})
	}
}

func TestRemoteShutdown_ZeroesOnlyAfterCloseReturns(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)
	m.set(func(k *fakeKnobs) { k.deleteMode = "hang" })

	var hookRan bool
	r.testBeforeZero = func() {
		hookRan = true
		if !r.closeReturned.Load() {
			t.Error("credentials were about to be zeroed while Close was still running")
		}
		if r.currentAccess() == "" {
			t.Error("credentials were already zeroed before the hook")
		}
		if m.count("DELETE") != 1 {
			t.Error("the session DELETE was not attempted before zeroing")
		}
	}
	r.shutdown(c)
	if !hookRan {
		t.Fatal("hook never ran")
	}
	if r.currentAccess() != "" {
		t.Error("access token survived shutdown")
	}
	if got := r.scrub("anything"); strings.Contains(got, "anything") {
		t.Errorf("a closed remote must withhold remote text, got %q", got)
	}
	// Idempotent.
	r.shutdown(c)
}

func TestRemoteShutdown_AbortsInFlightCall(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)
	m.set(func(k *fakeKnobs) { k.callMode = "stall" })

	done := make(chan error, 1)
	go func() {
		_, err := c.CallTool(ctxT(t), "list_accounts", nil)
		done <- err
	}()
	select {
	case <-m.stalled:
	case <-time.After(10 * time.Second):
		t.Fatal("stall never started")
	}
	r.shutdown(c)
	select {
	case err := <-done:
		if err == nil {
			t.Error("the stalled call should have failed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not abort the in-flight call")
	}
}

// captureStderr redirects os.Stderr for the rest of the test and returns a
// function that stops capturing and returns what was written.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	old := os.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = pw
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() { _, _ = io.Copy(&buf, pr); close(done) }()
	var once sync.Once
	stop := func() string {
		once.Do(func() { os.Stderr = old; pw.Close(); <-done; pr.Close() })
		return buf.String()
	}
	t.Cleanup(func() { stop() })
	return stop
}

func TestSecretsNeverLeak_RemoteSession(t *testing.T) {
	stderr := captureStderr(t)
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	tok := r.currentAccess()
	r.tokMu.Lock()
	refresh := r.tok.refresh.reveal()
	r.tokMu.Unlock()
	secrets := []string{tok, refresh}

	// Formatting verbs on every long-lived object.
	for _, v := range []any{r, c, r.rt, r.oc, r.tok} {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
			assertNoSecret(t, "format "+verb, secrets, fmt.Sprintf(verb, v))
		}
	}

	// A tool result that echoes the credential.
	res, err := c.CallTool(ctxT(t), "echo_token", nil)
	if err != nil {
		t.Fatalf("echo_token: %v", err)
	}
	assertNoSecret(t, "tool result", secrets, res.Content)
	if !strings.Contains(res.Content, redactedText) {
		t.Errorf("echoed token not replaced: %q", res.Content)
	}

	// A JSON-RPC error that echoes the credential.
	m.set(func(k *fakeKnobs) { k.callMode = "rpc-error-echo" })
	_, err = c.CallTool(ctxT(t), "list_accounts", nil)
	if err == nil {
		t.Fatal("expected a JSON-RPC error")
	}
	assertNoSecret(t, "rpc error", secrets, errStrings(err)...)
	if !strings.Contains(err.Error(), redactedText) {
		t.Errorf("rpc error not scrubbed: %v", err)
	}

	// A 403 whose body and headers echo the credential.
	m.set(func(k *fakeKnobs) { k.callMode = ""; k.forbid = true })
	_, err = c.CallTool(ctxT(t), "list_accounts", nil)
	if err == nil {
		t.Fatal("expected a 403 error")
	}
	assertNoSecret(t, "403 error", secrets, errStrings(err)...)

	r.shutdown(c)
	if out := stderr(); out != "" {
		assertNoSecret(t, "stderr", secrets, out)
	}
}

func TestSecretsNeverLeak_ConnectFailures(t *testing.T) {
	stderr := captureStderr(t)
	f := newFakeAS(t)
	f.attachMCP()
	desc := "SENTINEL-DESC-" + f.tag
	f.tune(func(f *fakeAS) { f.tokenFail = &oauthFailure{Status: 400, Code: "invalid_grant", Description: desc} })
	r := newRemoteForTest(t, f)

	_, err := r.connect(ctxT(t), Consent{Timeout: 5 * time.Second, Prompt: autoApprove(f)})
	if err == nil {
		t.Fatal("expected a token failure")
	}
	assertNoSecret(t, "connect error", []string{desc}, errStrings(err)...)
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("error lacks the allowlisted code: %v", err)
	}
	assertNoSecret(t, "stderr", []string{desc}, stderr())
}

func TestRemoteConnect_CancelledContextStopsConsent(t *testing.T) {
	f := newFakeAS(t)
	f.attachMCP()
	r := newRemoteForTest(t, f)

	ctx, cancel := context.WithCancel(context.Background())
	prompted := make(chan struct{})
	go func() { <-prompted; cancel() }()
	_, err := r.connect(ctx, Consent{Timeout: time.Minute, Prompt: func(string, string) { close(prompted) }})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// TestRemoteScrub_ShutdownDuringScrubWithholds pauses a scrub inside the gap
// where the old code had already decided the remote was live, completes a
// shutdown, then resumes: the text, which carries a live token, must be
// withheld rather than returned raw.
func TestRemoteScrub_ShutdownDuringScrubWithholds(t *testing.T) {
	f := newFakeAS(t)
	f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)
	tok := r.currentAccess()

	reached := make(chan struct{})
	resume := make(chan struct{})
	r.testScrubGap = func() {
		close(reached)
		<-resume
	}
	got := make(chan string, 1)
	go func() { got <- r.scrub("server says " + tok) }()
	select {
	case <-reached:
	case <-time.After(10 * time.Second):
		t.Fatal("scrub never reached the gap")
	}
	r.testScrubGap = nil
	r.shutdown(c)
	close(resume)

	select {
	case out := <-got:
		if strings.Contains(out, tok) {
			t.Fatalf("scrub returned the live token after shutdown: %q", out)
		}
		if out != redactedText {
			t.Errorf("a scrub that finished after shutdown must withhold the text, got %q", out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("scrub never returned")
	}
}

// TestRemoteScrub_AuthCodeAndVerifierEchoedAtEveryBoundary has a combined
// authorization and MCP server reflect the exchanged authorization code and the
// PKCE verifier through tool metadata, a tool result and a JSON-RPC error.
func TestRemoteScrub_AuthCodeAndVerifierEchoedAtEveryBoundary(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	c := connectT(t, r, f)

	var code, verifier string
	for _, h := range f.hits("/token") {
		if h.Form.Get("grant_type") == "authorization_code" {
			code, verifier = h.Form.Get("code"), h.Form.Get("code_verifier")
		}
	}
	if code == "" || verifier == "" {
		t.Fatalf("the exchange never reached the fake (code %q verifier %q)", code, verifier)
	}

	for name, val := range map[string]string{"code": code, "verifier": verifier} {
		t.Run(name, func(t *testing.T) {
			m.set(func(k *fakeKnobs) { k.echoValue, k.echoMetadata, k.callMode = val, true, "" })
			tools, err := c.ListTools(ctxT(t))
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			for _, ti := range tools {
				assertNoSecret(t, "tool metadata", []string{val}, ti.Name, ti.Description, string(ti.InputSchema))
			}
			res, err := c.CallTool(ctxT(t), "echo_token", nil)
			if err != nil {
				t.Fatalf("CallTool echo_token: %v", err)
			}
			assertNoSecret(t, "tool result", []string{val}, res.Content)

			m.set(func(k *fakeKnobs) { k.callMode = "rpc-error-echo" })
			_, err = c.CallTool(ctxT(t), "list_accounts", nil)
			if err == nil {
				t.Fatal("expected the JSON-RPC error")
			}
			assertNoSecret(t, "JSON-RPC error", []string{val}, errStrings(err)...)
		})
	}
}

// pausedConnect starts r.connect, returns once the barrier hook has been
// reached and a function that releases it. The hook is installed by arm.
type pausedConnect struct {
	reached chan struct{}
	resume  chan struct{}
	done    chan error
	prompts atomic.Int32
}

func startPausedConnect(t *testing.T, r *remote, f *fakeAS, arm func(pc *pausedConnect)) *pausedConnect {
	t.Helper()
	pc := &pausedConnect{reached: make(chan struct{}), resume: make(chan struct{}), done: make(chan error, 1)}
	arm(pc)
	approve := autoApprove(f)
	go func() {
		_, err := r.connect(ctxT(t), Consent{Timeout: 10 * time.Second, Prompt: func(name, u string) {
			pc.prompts.Add(1)
			approve(name, u)
		}})
		pc.done <- err
	}()
	select {
	case <-pc.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("setup never reached the barrier")
	}
	return pc
}

// shutdownDuringSetup runs a shutdown while setup is paused, asserts it waits
// for the setup, then releases the barrier and returns the setup's error.
func shutdownDuringSetup(t *testing.T, r *remote, pc *pausedConnect) error {
	t.Helper()
	shutDone := make(chan struct{})
	go func() { r.shutdown(nil); close(shutDone) }()
	select {
	case <-shutDone:
		t.Fatal("shutdown finished while credential setup was still pending")
	case <-time.After(200 * time.Millisecond):
	}
	if r.zeroed.Load() {
		t.Error("credentials were zeroed under a pending setup")
	}
	close(pc.resume)
	select {
	case <-shutDone:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown never finished after setup was released")
	}
	select {
	case err := <-pc.done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("setup never returned")
		return nil
	}
}

func TestRemoteShutdown_JoinsSetupPausedAfterRegistration(t *testing.T) {
	f := newFakeAS(t)
	f.dcrClientSecret, f.dcrAuthMethod = "SENTINEL-CLIENTSECRET-"+f.tag, "client_secret_post"
	f.attachMCP()
	r := newRemoteForTest(t, f)
	pc := startPausedConnect(t, r, f, func(pc *pausedConnect) {
		r.oc.testHook = func(stage string) {
			if stage == "registered" {
				close(pc.reached)
				<-pc.resume
			}
		}
	})
	err := shutdownDuringSetup(t, r, pc)
	if err == nil {
		t.Error("setup should fail once shutdown has begun")
	}
	if n := pc.prompts.Load(); n != 0 {
		t.Errorf("the user was prompted %d times after shutdown began", n)
	}
	if !r.oc.clientSecret.isZero() {
		t.Error("the client secret issued during registration survived shutdown")
	}
	if got := len(f.hits("/token")); got != 0 {
		t.Errorf("%d token requests after shutdown began", got)
	}
}

func TestRemoteShutdown_RejectsAndWipesTokensArrivingDuringShutdown(t *testing.T) {
	f := newFakeAS(t)
	m := f.attachMCP()
	r := newRemoteForTest(t, f)
	var access string
	pc := startPausedConnect(t, r, f, func(pc *pausedConnect) {
		r.testBeforeInstall = func(ts *tokenSet) {
			access = ts.access.reveal()
			close(pc.reached)
			<-pc.resume
		}
	})
	err := shutdownDuringSetup(t, r, pc)
	if !errors.Is(err, ErrRemoteClosed) {
		t.Errorf("setup error = %v, want ErrRemoteClosed", err)
	}
	if access == "" {
		t.Fatal("the barrier never saw the token")
	}
	if got := r.currentAccess(); got != "" {
		t.Errorf("tokens that arrived during shutdown were installed (%d bytes)", len(got))
	}
	if out := r.scrub("x " + access); strings.Contains(out, access) {
		t.Errorf("late token escaped scrubbing: %q", out)
	}
	if n := m.requests(); n != 0 {
		t.Errorf("%d MCP requests were made with tokens that arrived during shutdown", n)
	}
}
