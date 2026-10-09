package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// stubRemote is a TLS fake of a remote MCP server and its authorization
// server on one origin. It is deliberately small: the OAuth client itself is
// covered in depth by the internal/mcp tests. It offers one allowlistable read
// tool and one write tool that must stay hidden.
type stubRemote struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	hits     map[string]int
	approved int
	seq      int
	clients  map[string]string // client_id -> registered redirect
	access   map[string]bool
	calls    map[string]int
}

const stubFinanceScope = "finance:read"

func newStubRemote(t *testing.T) *stubRemote {
	t.Helper()
	s := &stubRemote{
		t:       t,
		hits:    map[string]int{},
		clients: map[string]string{},
		access:  map[string]bool{},
		calls:   map[string]int{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", s.handlePRM)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleMeta)
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.HandleFunc("/token", s.handleToken)
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	mcpSrv := gomcp.NewServer(&gomcp.Implementation{Name: "stub-finance", Version: "0.0.1"}, nil)
	schema := map[string]any{"type": "object", "properties": map[string]any{}}
	add := func(name, out string) {
		mcpSrv.AddTool(&gomcp.Tool{Name: name, Description: name, InputSchema: schema},
			func(context.Context, *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
				s.mu.Lock()
				s.calls[name]++
				s.mu.Unlock()
				return &gomcp.CallToolResult{Content: []gomcp.Content{&gomcp.TextContent{Text: out}}}, nil
			})
	}
	add("list_accounts", "accounts: acct-1, acct-2")
	add("create_transfer", "transfer created")
	sdk := gomcp.NewStreamableHTTPHandler(func(*http.Request) *gomcp.Server { return mcpSrv },
		&gomcp.StreamableHTTPOptions{JSONResponse: true})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		s.mu.Lock()
		ok := s.access[tok]
		s.mu.Unlock()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			http.Error(w, "token rejected", http.StatusUnauthorized)
			return
		}
		sdk.ServeHTTP(w, r)
	})

	s.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits[r.URL.Path]++
		s.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stubRemote) base() string { return s.srv.URL }

func (s *stubRemote) pool() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(s.srv.Certificate())
	return p
}

// tomlRemote renders a [remote.NAME] table pointing at the stub. The tool
// allowlist deliberately leaves out create_transfer.
func (s *stubRemote) tomlRemote(name string) string {
	return fmt.Sprintf("[remote.%s]\nurl = %q\nissuer = %q\nphases = [\"orient\"]\ntools = [\"list_accounts\"]\n",
		name, s.base()+"/mcp", s.base())
}

func (s *stubRemote) hitCount(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[path]
}

func (s *stubRemote) totalHits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.hits {
		n += c
	}
	return n
}

func (s *stubRemote) callCount(tool string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[tool]
}

func (s *stubRemote) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *stubRemote) handlePRM(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, 200, map[string]any{"resource": s.base() + "/mcp", "authorization_servers": []string{s.base()}})
}

func (s *stubRemote) handleMeta(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, 200, map[string]any{
		"issuer":                           s.base(),
		"registration_endpoint":            s.base() + "/register",
		"authorization_endpoint":           s.base() + "/authorize",
		"token_endpoint":                   s.base() + "/token",
		"revocation_endpoint":              s.base() + "/revoke",
		"code_challenge_methods_supported": []string{"S256"},
	})
}

func (s *stubRemote) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	uris, _ := body["redirect_uris"].([]any)
	if len(uris) != 1 {
		s.writeJSON(w, 400, map[string]any{"error": "invalid_redirect_uri"})
		return
	}
	redirect, _ := uris[0].(string)
	s.mu.Lock()
	id := fmt.Sprintf("client-%d", len(s.clients)+1)
	s.clients[id] = redirect
	s.mu.Unlock()
	s.writeJSON(w, 201, map[string]any{"client_id": id, "redirect_uris": uris})
}

func (s *stubRemote) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.mu.Lock()
	redirect, known := s.clients[q.Get("client_id")]
	s.approved++
	s.seq++
	code := fmt.Sprintf("SENTINEL-CODE-%d", s.seq)
	s.mu.Unlock()
	if !known || q.Get("redirect_uri") != redirect || q.Get("scope") != stubFinanceScope {
		http.Error(w, "bad authorize request", 400)
		return
	}
	loc, _ := url.Parse(redirect)
	v := loc.Query()
	v.Set("state", q.Get("state"))
	v.Set("code", code)
	loc.RawQuery = v.Encode()
	http.Redirect(w, r, loc.String(), http.StatusFound)
}

func (s *stubRemote) handleToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if r.PostForm.Get("grant_type") != "authorization_code" {
		s.writeJSON(w, 400, map[string]any{"error": "unsupported_grant_type"})
		return
	}
	s.mu.Lock()
	s.seq++
	at := fmt.Sprintf("SENTINEL-ACCESS-%d", s.seq)
	rt := fmt.Sprintf("SENTINEL-REFRESH-%d", s.seq)
	s.access[at] = true
	s.mu.Unlock()
	s.writeJSON(w, 200, map[string]any{
		"access_token": at, "token_type": "Bearer", "expires_in": 3600,
		"refresh_token": rt, "scope": stubFinanceScope,
	})
}

// approveFunc returns a consent prompt that approves in a browser stand-in:
// it fetches the authorization URL (trusting the stub's certificate) and
// follows the redirect to the loopback callback.
func (s *stubRemote) approveFunc() func(remote, authURL string) {
	return func(_, authURL string) {
		go func() {
			hc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: s.pool(), MinVersion: tls.VersionTLS12}}}
			if resp, err := hc.Get(authURL); err == nil {
				resp.Body.Close()
			}
		}()
	}
}
