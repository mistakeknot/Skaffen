package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// scopeAbsent, as a scope knob value, omits the field from the token response.
const scopeAbsent = "ABSENT"

type reqRecord struct {
	Method string
	Path   string
	Query  url.Values
	Form   url.Values
	Auth   string
	Header http.Header
}

// fakeAS is a fake OAuth authorization server (and RFC 9728 resource metadata
// host) served over TLS on loopback. Every knob is safe to set before use.
type fakeAS struct {
	t   *testing.T
	srv *httptest.Server
	tag string // per-test random sentinel suffix

	mu  sync.Mutex
	log []reqRecord

	// Metadata knobs.
	issuerInMeta   string            // overrides "issuer" in AS metadata
	prmResource    string            // overrides PRM resource
	prmServers     []string          // overrides PRM authorization_servers
	endpointURL    map[string]string // metadata endpoint overrides by field name
	omitRevocation bool
	pkceMethods    []string
	prmRedirectTo  string // PRM answers with a 302 to this URL

	// DCR knobs.
	dcrStatus       int
	dcrClientSecret string
	dcrAuthMethod   string
	dcrExtra        map[string]any

	// Token knobs.
	scopeRaw    map[string]string // by grant type; raw JSON; scopeAbsent omits
	expiresIn   int
	noRotate    bool
	tokenFail   *oauthFailure // every token call fails this way
	denyConsent bool

	// State.
	clients   map[string]string // client_id -> redirect URI
	codes     map[string]codeGrant
	refreshes map[string]bool // valid refresh tokens
	access    map[string]bool // access tokens issued and not revoked
	seq       int
	dcrBodies []map[string]any
	dcrProbe  []bool // redirect listener reachable at registration time

	// Observations.
	MissingResource int // token calls rejected for lacking resource
	Revoked         []string
}

type oauthFailure struct {
	Status      int
	Code        string
	Description string
}

type codeGrant struct {
	clientID, redirect, challenge, resource string
	used                                    bool
}

func newFakeAS(t *testing.T) *fakeAS {
	t.Helper()
	var b [4]byte
	_, _ = rand.Read(b[:])
	f := &fakeAS{
		t:           t,
		tag:         hex.EncodeToString(b[:]),
		endpointURL: map[string]string{},
		scopeRaw:    map[string]string{},
		expiresIn:   3600,
		clients:     map[string]string{},
		codes:       map[string]codeGrant{},
		refreshes:   map[string]bool{},
		access:      map[string]bool{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", f.handlePRM)
	mux.HandleFunc("/.well-known/oauth-authorization-server", f.handleMeta)
	mux.HandleFunc("/register", f.handleRegister)
	mux.HandleFunc("/authorize", f.handleAuthorize)
	mux.HandleFunc("/token", f.handleToken)
	mux.HandleFunc("/revoke", f.handleRevoke)
	f.srv = httptest.NewTLSServer(f.record(mux))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAS) base() string { return f.srv.URL }

// pool trusts the fake's certificate (and any extra servers).
func (f *fakeAS) pool(extra ...*httptest.Server) *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(f.srv.Certificate())
	for _, s := range extra {
		p.AddCert(s.Certificate())
	}
	return p
}

// config returns a RemoteConfig pointing at the fake.
func (f *fakeAS) config() RemoteConfig {
	return RemoteConfig{
		Name:   "finance",
		URL:    f.base() + "/mcp",
		Issuer: f.base(),
		Phases: []string{"orient"},
		Tools:  []string{"list_accounts", "list_transactions"},
	}
}

func (f *fakeAS) sentinel(kind string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	return fmt.Sprintf("SENTINEL-%s-%s-%d", kind, f.tag, f.seq)
}

func (f *fakeAS) record(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		rec := reqRecord{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(),
			Form: r.PostForm, Auth: r.Header.Get("Authorization"), Header: r.Header.Clone(),
		}
		f.mu.Lock()
		f.log = append(f.log, rec)
		f.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

// hits returns the recorded requests for a path.
func (f *fakeAS) hits(path string) []reqRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []reqRecord
	for _, r := range f.log {
		if r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeAS) ep(name, path string) string {
	if v, ok := f.endpointURL[name]; ok {
		return v
	}
	return f.base() + path
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAS) handlePRM(w http.ResponseWriter, r *http.Request) {
	if f.prmRedirectTo != "" {
		http.Redirect(w, r, f.prmRedirectTo, http.StatusFound)
		return
	}
	res := f.base() + "/mcp"
	if f.prmResource != "" {
		res = f.prmResource
	}
	servers := []string{f.base()}
	if f.prmServers != nil {
		servers = f.prmServers
	}
	writeJSON(w, 200, map[string]any{"resource": res, "authorization_servers": servers})
}

func (f *fakeAS) handleMeta(w http.ResponseWriter, r *http.Request) {
	iss := f.base()
	if f.issuerInMeta != "" {
		iss = f.issuerInMeta
	}
	methods := f.pkceMethods
	if methods == nil {
		methods = []string{"S256"}
	}
	m := map[string]any{
		"issuer":                           iss,
		"registration_endpoint":            f.ep("registration_endpoint", "/register"),
		"authorization_endpoint":           f.ep("authorization_endpoint", "/authorize"),
		"token_endpoint":                   f.ep("token_endpoint", "/token"),
		"code_challenge_methods_supported": methods,
	}
	if !f.omitRevocation {
		m["revocation_endpoint"] = f.ep("revocation_endpoint", "/revoke")
	}
	writeJSON(w, 200, m)
}

func (f *fakeAS) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	f.dcrBodies = append(f.dcrBodies, body)
	f.mu.Unlock()
	if f.dcrStatus != 0 && f.dcrStatus >= 400 {
		writeJSON(w, f.dcrStatus, map[string]any{
			"error": "invalid_client_metadata", "error_description": "SENTINEL-DCR-DESC-" + f.tag,
		})
		return
	}
	uris, _ := body["redirect_uris"].([]any)
	if len(uris) != 1 {
		writeJSON(w, 400, map[string]any{"error": "invalid_redirect_uri"})
		return
	}
	redirect, _ := uris[0].(string)
	if ru, err := url.Parse(redirect); err == nil {
		// Probe: is something already listening on the registered redirect?
		conn, derr := net.DialTimeout("tcp", ru.Host, time.Second)
		if derr == nil {
			conn.Close()
		}
		f.mu.Lock()
		f.dcrProbe = append(f.dcrProbe, derr == nil)
		f.mu.Unlock()
	}
	f.mu.Lock()
	id := fmt.Sprintf("client-%d", len(f.clients)+1)
	f.clients[id] = redirect
	f.mu.Unlock()
	resp := map[string]any{"client_id": id, "redirect_uris": uris}
	if f.dcrClientSecret != "" {
		resp["client_secret"] = f.dcrClientSecret
	}
	if f.dcrAuthMethod != "" {
		resp["token_endpoint_auth_method"] = f.dcrAuthMethod
	}
	for k, v := range f.dcrExtra {
		resp[k] = v
	}
	status := f.dcrStatus
	if status == 0 {
		status = 201
	}
	writeJSON(w, status, resp)
}

func (f *fakeAS) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.mu.Lock()
	redirect, known := f.clients[q.Get("client_id")]
	f.mu.Unlock()
	if !known || q.Get("redirect_uri") != redirect || q.Get("response_type") != "code" ||
		q.Get("scope") != financeReadScope || q.Get("code_challenge_method") != "S256" ||
		q.Get("code_challenge") == "" || q.Get("resource") == "" || q.Get("state") == "" {
		http.Error(w, "bad authorize request", 400)
		return
	}
	loc, _ := url.Parse(redirect)
	v := loc.Query()
	v.Set("state", q.Get("state"))
	if f.denyConsent {
		v.Set("error", "access_denied")
		v.Set("error_description", "SENTINEL-DENY-"+f.tag)
	} else {
		code := f.sentinel("CODE")
		f.mu.Lock()
		f.codes[code] = codeGrant{clientID: q.Get("client_id"), redirect: redirect,
			challenge: q.Get("code_challenge"), resource: q.Get("resource")}
		f.mu.Unlock()
		v.Set("code", code)
	}
	loc.RawQuery = v.Encode()
	http.Redirect(w, r, loc.String(), http.StatusFound)
}

func tokenErr(w http.ResponseWriter, status int, code, desc string) {
	body := map[string]any{"error": code}
	if desc != "" {
		body["error_description"] = desc
	}
	writeJSON(w, status, body)
}

func (f *fakeAS) handleToken(w http.ResponseWriter, r *http.Request) {
	if f.tokenFail != nil {
		tokenErr(w, f.tokenFail.Status, f.tokenFail.Code, f.tokenFail.Description)
		return
	}
	form := r.PostForm
	if form.Get("resource") == "" {
		f.mu.Lock()
		f.MissingResource++
		f.mu.Unlock()
		tokenErr(w, 400, "invalid_target", "resource is required")
		return
	}
	grant := form.Get("grant_type")
	f.mu.Lock()
	switch grant {
	case "authorization_code":
		g, ok := f.codes[form.Get("code")]
		sum := sha256.Sum256([]byte(form.Get("code_verifier")))
		pk := base64.RawURLEncoding.EncodeToString(sum[:])
		if !ok || g.used || g.redirect != form.Get("redirect_uri") || g.challenge != pk ||
			form.Get("client_id") != g.clientID || form.Get("resource") != g.resource {
			f.mu.Unlock()
			tokenErr(w, 400, "invalid_grant", "SENTINEL-GRANT-DESC-"+f.tag)
			return
		}
		g.used = true
		f.codes[form.Get("code")] = g
	case "refresh_token":
		rt := form.Get("refresh_token")
		if !f.refreshes[rt] {
			f.mu.Unlock()
			tokenErr(w, 400, "invalid_grant", "SENTINEL-GRANT-DESC-"+f.tag)
			return
		}
		if !f.noRotate {
			delete(f.refreshes, rt)
		}
	default:
		f.mu.Unlock()
		tokenErr(w, 400, "unsupported_grant_type", "")
		return
	}
	f.seq++
	at := fmt.Sprintf("SENTINEL-ACCESS-%s-%d", f.tag, f.seq)
	rt := fmt.Sprintf("SENTINEL-REFRESH-%s-%d", f.tag, f.seq)
	f.access[at] = true
	f.refreshes[rt] = true
	if f.noRotate && grant == "refresh_token" {
		rt = ""
	}
	f.mu.Unlock()

	var sb strings.Builder
	fmt.Fprintf(&sb, `{"access_token":%q,"token_type":"Bearer","expires_in":%d`, at, f.expiresIn)
	if rt != "" {
		fmt.Fprintf(&sb, `,"refresh_token":%q`, rt)
	}
	scope, set := f.scopeRaw[grant]
	if !set {
		scope = `"finance:read"`
	}
	if scope != scopeAbsent {
		sb.WriteString(`,"scope":` + scope)
	}
	sb.WriteString("}")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(sb.String()))
}

func (f *fakeAS) handleRevoke(w http.ResponseWriter, r *http.Request) {
	tok := r.PostForm.Get("token")
	f.mu.Lock()
	f.Revoked = append(f.Revoked, tok)
	delete(f.access, tok)
	delete(f.refreshes, tok)
	f.mu.Unlock()
	w.WriteHeader(200)
}

func (f *fakeAS) revoked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.Revoked...)
}

// acceptsAccess reports whether the access token is currently valid.
func (f *fakeAS) acceptsAccess(tok string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.access[tok]
}

// foreignServer is a TLS server that records every hit; a request reaching it
// means a pinning failure.
type foreignServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

func newForeignServer(t *testing.T) *foreignServer {
	t.Helper()
	fs := &foreignServer{}
	fs.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		fs.hits = append(fs.hits, r.Method+" "+r.URL.Path)
		fs.mu.Unlock()
		writeJSON(w, 200, map[string]any{})
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *foreignServer) count() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.hits)
}

// testOpts returns remoteOptions that trust the given pool.
func testOpts(pool *x509.CertPool) remoteOptions {
	return remoteOptions{rootCAs: pool}.withDefaults()
}

// lastDCR returns the most recent registration request body.
func (f *fakeAS) lastDCR() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.dcrBodies) == 0 {
		return nil
	}
	return f.dcrBodies[len(f.dcrBodies)-1]
}
