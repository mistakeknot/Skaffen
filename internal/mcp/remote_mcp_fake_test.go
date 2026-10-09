package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeKnobs are the failure modes of the fake MCP server.
type fakeKnobs struct {
	sse          bool          // tools/call answers with an event stream
	forbid       bool          // tools/call answers 403
	unauthorized bool          // tools/call answers 401 whatever the token
	dropSessions int           // answer this many session-bearing requests with 404
	deleteMode   string        // "", "hang", "401"
	notifyMode   string        // "" known-empty headerless 202, "chunked" chunked-empty headerless 202
	callMode     string        // "", "oversize", "oversize-length", "stall", "text-chunked", "text-length", "rpc-error-echo"
	echoMetadata bool          // tools/list descriptions, defaults and enums echo the bearer token
	echoGate     chan struct{} // echo_token waits for this channel to close before answering
	initDelay    time.Duration // initialize requests are held this long before being served
	echoValue    string        // when set, echo this instead of the bearer token (metadata, echo_token, rpc-error-echo)
}

// rpcInfo describes one request as the fake saw it.
type rpcInfo struct {
	HTTP    string
	RPC     string
	Tool    string
	ID      json.RawMessage
	Token   string
	Session string
}

// fakeMCP is a streamable-HTTP MCP server built with the go-sdk, behind
// bearer-token verification against the fake authorization server. It lists
// two read tools, a token-echo tool and one write tool.
type fakeMCP struct {
	t  *testing.T
	as *fakeAS

	sdk     http.Handler
	chain   http.Handler
	release chan struct{} // closed on cleanup; unblocks hanging handlers

	mu        sync.Mutex
	knobs     fakeKnobs
	calls     []rpcInfo
	tokens    []string // every bearer token presented, including rejected ones
	toolCalls map[string]int
	stalled   chan struct{}
}

type rpcCtxKey struct{}

// attachMCP serves a fake MCP server at /mcp on the fake authorization
// server's origin.
func (f *fakeAS) attachMCP() *fakeMCP {
	f.t.Helper()
	m := &fakeMCP{
		t: f.t, as: f, release: make(chan struct{}),
		toolCalls: map[string]int{}, stalled: make(chan struct{}),
	}
	srv := gomcp.NewServer(&gomcp.Implementation{Name: "fake-finance", Version: "0.0.1"}, nil)
	schema := func() map[string]any {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{"limit": map[string]any{"type": "integer", "description": "max rows"}},
		}
	}
	add := func(name, desc, out string) {
		srv.AddTool(&gomcp.Tool{Name: name, Description: desc, InputSchema: schema()},
			func(_ context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
				m.mu.Lock()
				m.toolCalls[name]++
				m.mu.Unlock()
				text := out
				if name == "echo_token" {
					m.mu.Lock()
					gate := m.knobs.echoGate
					m.mu.Unlock()
					if gate != nil {
						select {
						case <-gate:
						case <-m.release:
						}
					}
					text = "token=" + strings.TrimPrefix(req.Extra.Header.Get("Authorization"), "Bearer ")
					m.mu.Lock()
					if v := m.knobs.echoValue; v != "" {
						text = "token=" + v
					}
					m.mu.Unlock()
				}
				return &gomcp.CallToolResult{Content: []gomcp.Content{&gomcp.TextContent{Text: text}}}, nil
			})
	}
	add("list_accounts", "List accounts.", "accounts: acct-1, acct-2")
	add("list_transactions", "List transactions.", "transactions: txn-1")
	add("echo_token", "Echo the caller credential.", "")
	add("create_transfer", "Move money between accounts.", "transfer created")

	m.sdk = gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return srv },
		&gomcp.StreamableHTTPOptions{JSONResponse: true})

	verifier := func(_ context.Context, tok string, _ *http.Request) (*auth.TokenInfo, error) {
		if !f.acceptsAccess(tok) {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{Scopes: []string{financeReadScope}, Expiration: time.Now().Add(time.Hour)}, nil
	}
	guarded := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: f.base() + "/.well-known/oauth-protected-resource/mcp",
		Scopes:              []string{financeReadScope},
	})(http.HandlerFunc(m.serveKnobs))
	m.chain = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := rpcInfo{HTTP: r.Method, Session: r.Header.Get("Mcp-Session-Id")}
		if fields := strings.Fields(r.Header.Get("Authorization")); len(fields) == 2 {
			info.Token = fields[1]
		}
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			r.Body = io.NopCloser(bytes.NewReader(body))
			var env struct {
				Method string          `json:"method"`
				ID     json.RawMessage `json:"id"`
				Params struct {
					Name string `json:"name"`
				} `json:"params"`
			}
			_ = json.Unmarshal(body, &env)
			info.RPC, info.ID, info.Tool = env.Method, env.ID, env.Params.Name
		}
		m.mu.Lock()
		m.calls = append(m.calls, info)
		if info.Token != "" {
			m.tokens = append(m.tokens, info.Token)
		}
		m.mu.Unlock()
		m.mu.Lock()
		deny := m.knobs.unauthorized && info.RPC == "tools/call"
		m.mu.Unlock()
		if deny {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			http.Error(w, "token rejected", http.StatusUnauthorized)
			return
		}
		guarded.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), rpcCtxKey{}, info)))
	})

	f.mu.Lock()
	f.mcp = m.chain
	f.mu.Unlock()
	f.t.Cleanup(func() { close(m.release) })
	return m
}

func (m *fakeMCP) set(fn func(*fakeKnobs)) {
	m.mu.Lock()
	fn(&m.knobs)
	m.mu.Unlock()
}

// count returns the number of requests with the given JSON-RPC method;
// "DELETE" counts DELETE requests.
func (m *fakeMCP) count(rpc string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.calls {
		if (rpc == "DELETE" && c.HTTP == http.MethodDelete) || (c.HTTP == http.MethodPost && c.RPC == rpc) {
			n++
		}
	}
	return n
}

// countHTTP returns the number of requests with the given HTTP method.
func (m *fakeMCP) countHTTP(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.calls {
		if c.HTTP == method {
			n++
		}
	}
	return n
}

// requests returns the total number of requests of any kind.
func (m *fakeMCP) requests() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *fakeMCP) toolCallCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.toolCalls[name]
}

// tokensSeen returns every bearer token any request carried.
func (m *fakeMCP) tokensSeen() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.tokens...)
}

func (m *fakeMCP) lastToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.tokens) == 0 {
		return ""
	}
	return m.tokens[len(m.tokens)-1]
}

func (m *fakeMCP) serveKnobs(w http.ResponseWriter, r *http.Request) {
	info := r.Context().Value(rpcCtxKey{}).(rpcInfo)
	m.mu.Lock()
	k := m.knobs
	drop := false
	if k.dropSessions > 0 && info.Session != "" && info.RPC != "initialize" {
		m.knobs.dropSessions--
		drop = true
	}
	m.mu.Unlock()

	if info.RPC == "initialize" && k.initDelay > 0 {
		select {
		case <-time.After(k.initDelay):
		case <-r.Context().Done():
			return
		}
	}
	switch {
	case r.Method == http.MethodDelete && k.deleteMode == "hang":
		select {
		case <-r.Context().Done():
		case <-m.release:
		}
		return
	case r.Method == http.MethodDelete && k.deleteMode == "401":
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("delete rejected"))
		return
	case drop:
		http.Error(w, "session not found", http.StatusNotFound)
		return
	case info.RPC == "tools/call" && k.forbid:
		w.Header().Set("X-Debug-Credential", info.Token)
		http.Error(w, "insufficient scope for "+info.Token, http.StatusForbidden)
		return
	case info.RPC == "tools/call" && k.sse:
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for i := 1; i <= 3; i++ {
			_, _ = w.Write([]byte("id: " + string(rune('0'+i)) + "\ndata: {}\n\n"))
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		}
		return
	case info.RPC == "tools/call" && k.callMode != "":
		m.serveCallMode(w, r, info, k.callMode)
		return
	}

	rec := httptest.NewRecorder()
	m.sdk.ServeHTTP(rec, r)
	body := rec.Body.Bytes()
	if k.echoMetadata && info.RPC == "tools/list" {
		body = echoIntoToolList(body, echoOr(k.echoValue, info.Token))
	}
	for hk, hv := range rec.Header() {
		w.Header()[hk] = hv
	}
	w.Header().Del("Content-Length")
	if info.RPC == "notifications/initialized" && rec.Code == http.StatusAccepted {
		w.Header().Del("Content-Type")
		w.WriteHeader(http.StatusAccepted)
		if k.notifyMode == "chunked" {
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		}
		return
	}
	w.WriteHeader(rec.Code)
	_, _ = w.Write(body)
}

// echoOr returns override when set, else def.
func echoOr(override, def string) string {
	if override != "" {
		return override
	}
	return def
}

// echoIntoToolList writes tok into a description, a default and an enum of the
// first tool, simulating a server that reflects the caller's credential in
// tool metadata.
func echoIntoToolList(body []byte, tok string) []byte {
	var doc map[string]any
	if json.Unmarshal(body, &doc) != nil {
		return body
	}
	result, _ := doc["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	for _, raw := range tools {
		tl, _ := raw.(map[string]any)
		if tl["name"] != "list_accounts" {
			continue
		}
		tl["description"] = "List accounts. " + tok
		props, _ := tl["inputSchema"].(map[string]any)["properties"].(map[string]any)
		limit, _ := props["limit"].(map[string]any)
		limit["default"] = tok
		limit["enum"] = []any{tok}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return body
	}
	return out
}

func (m *fakeMCP) serveCallMode(w http.ResponseWriter, r *http.Request, info rpcInfo, mode string) {
	id := string(info.ID)
	if id == "" {
		id = "null"
	}
	prefix := `{"jsonrpc":"2.0","id":` + id + `,"result":{"content":[{"type":"text","text":"`
	flush := func() {
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}
	switch mode {
	case "oversize":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		flush()
		_, _ = w.Write([]byte(prefix))
		chunk := bytes.Repeat([]byte("A"), 1024)
		for i := 0; i < 64; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	case "oversize-length":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "100000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("A"), 100000))
	case "stall":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(prefix + "par"))
		flush()
		m.mu.Lock()
		select {
		case <-m.stalled:
		default:
			close(m.stalled)
		}
		m.mu.Unlock()
		select {
		case <-r.Context().Done():
		case <-m.release:
		}
	case "text-chunked":
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		flush()
		_, _ = w.Write([]byte("not json at all"))
	case "text-length":
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json at all"))
	case "rpc-error-echo":
		w.Header().Set("Content-Type", "application/json")
		m.mu.Lock()
		echo := echoOr(m.knobs.echoValue, info.Token)
		m.mu.Unlock()
		msg, _ := json.Marshal("denied for " + echo)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + id + `,"error":{"code":-32000,"message":` + string(msg) + `}}`))
	}
}

// autoApprove returns a consent prompt that plays the browser: it opens the
// authorization URL against the fake and follows the redirect to the loopback
// callback.
func autoApprove(f *fakeAS) func(remote, authURL string) {
	return func(_, authURL string) {
		go func() {
			hc := &http.Client{Transport: testOpts(f.pool()).baseTransport()}
			resp, err := hc.Get(authURL)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
}
