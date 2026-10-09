package mcp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

const testRedirect = "http://127.0.0.1:45678/callback"

func newTestOAuth(t *testing.T, f *fakeAS, extra ...*foreignServer) (*oauthClient, *redactor) {
	t.Helper()
	red := newRedactor()
	t.Cleanup(red.zero)
	opts := testOpts(f.pool())
	for _, e := range extra {
		opts.rootCAs.AddCert(e.Certificate())
	}
	return newOAuthClient(f.config(), opts, red), red
}

func ctxT(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// authorizeAt performs the browser step against the fake and returns the code.
func authorizeAt(t *testing.T, f *fakeAS, c *oauthClient, redirect string) (code, verifier secret) {
	t.Helper()
	verifier, challenge := newPKCE()
	state := "state-" + f.tag
	hc := &http.Client{Transport: c.hc.Transport, CheckRedirect: refuseRedirects}
	resp, err := hc.Get(c.authURL(redirect, state, challenge))
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize status %d", resp.StatusCode)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if loc.Query().Get("state") != state {
		t.Fatalf("state not echoed")
	}
	return newSecret(loc.Query().Get("code")), verifier
}

func readyClient(t *testing.T, f *fakeAS) (*oauthClient, *redactor) {
	t.Helper()
	c, red := newTestOAuth(t, f)
	if err := c.discover(ctxT(t)); err != nil {
		t.Fatalf("discover: %v", err)
	}
	if err := c.register(ctxT(t), testRedirect); err != nil {
		t.Fatalf("register: %v", err)
	}
	return c, red
}

func TestOAuthCore_DiscoverRegisterExchange(t *testing.T) {
	f := newFakeAS(t)
	c, _ := readyClient(t, f)

	reg := f.hits("/register")
	if len(reg) != 1 {
		t.Fatalf("want one DCR, got %d", len(reg))
	}
	code, verifier := authorizeAt(t, f, c, testRedirect)
	ts, err := c.exchange(ctxT(t), code, verifier, testRedirect)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if !strings.HasPrefix(ts.access.reveal(), "SENTINEL-ACCESS-") || ts.refresh.isZero() {
		t.Error("expected issued access and refresh tokens")
	}
	if time.Until(ts.expiry) < 59*time.Minute {
		t.Errorf("expiry not honoured: %v", ts.expiry)
	}
	tok := f.hits("/token")
	if len(tok) != 1 || tok[0].Form.Get("resource") != f.base()+"/mcp" ||
		tok[0].Form.Get("code_verifier") == "" || tok[0].Form.Get("redirect_uri") != testRedirect {
		t.Errorf("token request wrong: %v", tok)
	}
	if az := f.hits("/authorize"); len(az) != 1 || az[0].Query.Get("scope") != "finance:read" ||
		az[0].Query.Get("resource") != f.base()+"/mcp" || az[0].Query.Get("code_challenge_method") != "S256" {
		t.Errorf("authorize request wrong: %v", az)
	}
	// Discovery used the configured URL, not a header.
	if len(f.hits("/.well-known/oauth-protected-resource/mcp")) != 1 || len(f.hits("/mcp")) != 0 {
		t.Error("discovery must use the well-known URL derived from configuration")
	}
}

func TestOAuthCore_DCRBody(t *testing.T) {
	f := newFakeAS(t)
	readyClient(t, f)
	rec := f.hits("/register")[0]
	if rec.Method != http.MethodPost || !strings.HasPrefix(rec.Header.Get("Content-Type"), "application/json") {
		t.Errorf("DCR must be a JSON POST: %v", rec)
	}
	body := f.lastDCR()
	for k, want := range map[string]string{
		"client_name": "skaffen", "token_endpoint_auth_method": "none", "scope": "finance:read",
	} {
		if body[k] != want {
			t.Errorf("DCR %s = %v, want %v", k, body[k], want)
		}
	}
	for k, want := range map[string][]any{
		"redirect_uris":  {testRedirect},
		"grant_types":    {"authorization_code", "refresh_token"},
		"response_types": {"code"},
	} {
		if !reflect.DeepEqual(body[k], any(want)) {
			t.Errorf("DCR %s = %v, want %v", k, body[k], want)
		}
	}
}

func TestOAuthCore_ClientAuthentication(t *testing.T) {
	for _, tc := range []struct {
		method string
		check  func(t *testing.T, r reqRecord)
	}{
		{"client_secret_post", func(t *testing.T, r reqRecord) {
			if r.Form.Get("client_secret") != "SENTINEL-CSECRET" || r.Auth != "" {
				t.Errorf("post auth wrong: %v", r)
			}
		}},
		{"client_secret_basic", func(t *testing.T, r reqRecord) {
			want := "Basic " + base64.StdEncoding.EncodeToString([]byte("client-1:SENTINEL-CSECRET"))
			if r.Auth != want || r.Form.Get("client_secret") != "" {
				t.Errorf("basic auth wrong: %v", r)
			}
		}},
		{"", func(t *testing.T, r reqRecord) { // secret returned, method omitted: RFC 7591 default is basic
			if !strings.HasPrefix(r.Auth, "Basic ") {
				t.Errorf("default must be basic: %v", r)
			}
		}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			f := newFakeAS(t)
			f.dcrClientSecret = "SENTINEL-CSECRET"
			f.dcrAuthMethod = tc.method
			c, red := readyClient(t, f)
			code, ver := authorizeAt(t, f, c, testRedirect)
			if _, err := c.exchange(ctxT(t), code, ver, testRedirect); err != nil {
				t.Fatal(err)
			}
			tc.check(t, f.hits("/token")[0])
			if got := red.scrub("x SENTINEL-CSECRET y"); strings.Contains(got, "SENTINEL-CSECRET") {
				t.Error("client secret must be in the redaction set")
			}
		})
	}
}

func TestOAuthCore_DCRRequiresClientID(t *testing.T) {
	f := newFakeAS(t)
	c, _ := newTestOAuth(t, f)
	if err := c.discover(ctxT(t)); err != nil {
		t.Fatal(err)
	}
	f.dcrExtra = map[string]any{"client_id": ""}
	if err := c.register(ctxT(t), testRedirect); err == nil {
		t.Error("empty client_id must be rejected")
	}
}

func TestRefresh(t *testing.T) {
	f := newFakeAS(t)
	c, _ := readyClient(t, f)
	code, ver := authorizeAt(t, f, c, testRedirect)
	first, err := c.exchange(ctxT(t), code, ver, testRedirect)
	if err != nil {
		t.Fatal(err)
	}

	// The fake rejects a refresh with no resource (proves the fake's check).
	raw := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {first.refresh.reveal()}, "client_id": {"client-1"}}
	resp, err := c.hc.PostForm(f.base()+"/token", raw)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 || f.MissingResource != 1 {
		t.Fatalf("fake must reject a missing resource: %d %d", resp.StatusCode, f.MissingResource)
	}

	second, err := c.refresh(ctxT(t), first.refresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	last := f.hits("/token")[len(f.hits("/token"))-1]
	if last.Form.Get("resource") != f.base()+"/mcp" || last.Form.Get("grant_type") != "refresh_token" ||
		last.Form.Get("client_id") != "client-1" {
		t.Errorf("refresh request wrong: %v", last.Form)
	}
	if second.access.reveal() == first.access.reveal() || second.refresh.reveal() == first.refresh.reveal() {
		t.Error("rotation expected")
	}
	// The rotated-out refresh token is dead.
	if _, err := c.refresh(ctxT(t), first.refresh); err == nil {
		t.Error("old refresh token must not work again")
	}
}

func TestRefresh_NoRotationKeepsOldRefresh(t *testing.T) {
	f := newFakeAS(t)
	f.noRotate = true
	c, _ := readyClient(t, f)
	code, ver := authorizeAt(t, f, c, testRedirect)
	first, _ := c.exchange(ctxT(t), code, ver, testRedirect)
	second, err := c.refresh(ctxT(t), first.refresh)
	if err != nil {
		t.Fatal(err)
	}
	if !second.refresh.isZero() {
		t.Error("refresh response without a new refresh token must leave it unset so the caller keeps the old one")
	}
}

func TestRefresh_BroadenedScopeIsRejected(t *testing.T) {
	f := newFakeAS(t)
	c, _ := readyClient(t, f)
	code, ver := authorizeAt(t, f, c, testRedirect)
	first, err := c.exchange(ctxT(t), code, ver, testRedirect)
	if err != nil {
		t.Fatal(err)
	}
	f.scopeRaw["refresh_token"] = `"finance:read library:read"`
	ts, err := c.refresh(ctxT(t), first.refresh)
	if !errors.Is(err, ErrScopeMismatch) || ts != nil {
		t.Fatalf("want ErrScopeMismatch and no token set, got %v / %v", ts, err)
	}
	revoked := f.revoked()
	if len(revoked) == 0 {
		t.Error("the rejected token set must be revoked best-effort")
	}
	for _, r := range revoked {
		if f.acceptsAccess(r) {
			t.Errorf("revoked token still accepted")
		}
	}
}

func TestScopeGate(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		ok   bool
	}{
		{"exact", `"finance:read"`, true},
		{"duplicate is the same set", `"finance:read finance:read"`, true},
		{"absent", scopeAbsent, false},
		{"null", `null`, false},
		{"empty string", `""`, false},
		{"number", `7`, false},
		{"array", `["finance:read"]`, false},
		{"object", `{"finance:read":true}`, false},
		{"superset", `"finance:read library:read"`, false},
		{"different", `"library:read"`, false},
		{"prefix only", `"finance:readx"`, false},
		{"case differs", `"Finance:Read"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAS(t)
			c, _ := readyClient(t, f)
			f.scopeRaw["authorization_code"] = tc.raw
			code, ver := authorizeAt(t, f, c, testRedirect)
			ts, err := c.exchange(ctxT(t), code, ver, testRedirect)
			if tc.ok {
				if err != nil || ts == nil {
					t.Fatalf("want success, got %v", err)
				}
				if len(f.revoked()) != 0 {
					t.Error("accepted token must not be revoked")
				}
				return
			}
			if !errors.Is(err, ErrScopeMismatch) || ts != nil {
				t.Fatalf("want ErrScopeMismatch, got %v / %v", ts, err)
			}
			if !strings.Contains(err.Error(), "finance:read") {
				t.Errorf("error should say the grant did not equal finance:read: %v", err)
			}
			rev := f.hits("/revoke")
			if len(rev) == 0 {
				t.Fatal("rejected token must be revoked")
			}
			for _, r := range f.revoked() {
				if f.acceptsAccess(r) {
					t.Error("token still valid after revoke")
				}
			}
		})
	}
}

func TestScopeGate_NoRevocationEndpointStillRejects(t *testing.T) {
	f := newFakeAS(t)
	f.omitRevocation = true
	f.scopeRaw["authorization_code"] = `"finance:read library:read"`
	c, _ := readyClient(t, f)
	code, ver := authorizeAt(t, f, c, testRedirect)
	if _, err := c.exchange(ctxT(t), code, ver, testRedirect); !errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("got %v", err)
	}
	if len(f.hits("/revoke")) != 0 {
		t.Error("no revocation endpoint was advertised")
	}
}

func TestIssuerAndEndpointPinning(t *testing.T) {
	type tweak func(f *fakeAS, foreign *foreignServer)
	foreignURL := func(fs *foreignServer, path string) string { return fs.URL + path }
	cases := []struct {
		name string
		do   tweak
	}{
		{"metadata issuer differs", func(f *fakeAS, _ *foreignServer) { f.issuerInMeta = "https://other.example.test" }},
		{"prm resource mismatch", func(f *fakeAS, _ *foreignServer) { f.prmResource = "https://other.example.test/mcp" }},
		{"prm lacks issuer", func(f *fakeAS, _ *foreignServer) { f.prmServers = []string{"https://other.example.test"} }},
		{"prm empty servers", func(f *fakeAS, _ *foreignServer) { f.prmServers = []string{} }},
		{"no S256", func(f *fakeAS, _ *foreignServer) { f.pkceMethods = []string{"plain"} }},
		{"prm redirects away", func(f *fakeAS, fs *foreignServer) { f.prmRedirectTo = foreignURL(fs, "/prm") }},
	}
	for _, ep := range []struct{ field, path string }{
		{"registration_endpoint", "/register"}, {"authorization_endpoint", "/authorize"},
		{"token_endpoint", "/token"}, {"revocation_endpoint", "/revoke"},
	} {
		ep := ep
		cases = append(cases,
			struct {
				name string
				do   tweak
			}{ep.field + " foreign host or port", func(f *fakeAS, fs *foreignServer) { f.endpointURL[ep.field] = foreignURL(fs, ep.path) }},
			struct {
				name string
				do   tweak
			}{ep.field + " userinfo", func(f *fakeAS, _ *foreignServer) {
				u, _ := url.Parse(f.base())
				f.endpointURL[ep.field] = "https://user:SENTINEL-PW@" + u.Host + ep.path
			}},
			struct {
				name string
				do   tweak
			}{ep.field + " plain http", func(f *fakeAS, _ *foreignServer) {
				u, _ := url.Parse(f.base())
				f.endpointURL[ep.field] = "http://" + u.Host + ep.path
			}},
			struct {
				name string
				do   tweak
			}{ep.field + " relative", func(f *fakeAS, _ *foreignServer) { f.endpointURL[ep.field] = ep.path }},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAS(t)
			fs := newForeignServer(t)
			tc.do(f, fs)
			c, _ := newTestOAuth(t, f, fs)
			err := c.discover(ctxT(t))
			if err == nil {
				t.Fatal("discovery must fail before consent")
			}
			if fs.count() != 0 {
				t.Errorf("foreign host was contacted %d time(s)", fs.count())
			}
			if strings.Contains(err.Error(), "SENTINEL-PW") {
				t.Errorf("error leaks userinfo: %v", err)
			}
			if len(f.hits("/register"))+len(f.hits("/token"))+len(f.hits("/authorize")) != 0 {
				t.Error("no OAuth call may happen after failed discovery")
			}
		})
	}
}

func TestIssuerPinning_HeaderMetadataIgnored(t *testing.T) {
	f := newFakeAS(t)
	fs := newForeignServer(t)
	f.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+fs.URL+`/prm"`)
		http.NotFound(w, r)
	})
	// A handler that always advertises a foreign metadata URL cannot redirect
	// discovery: the well-known URL is derived from configuration, and the
	// header is never read, so discovery fails here instead of following it.
	c, _ := newTestOAuth(t, f, fs)
	if err := c.discover(ctxT(t)); err == nil {
		t.Fatal("expected discovery failure")
	}
	if fs.count() != 0 {
		t.Errorf("header-supplied metadata URL was contacted")
	}
}

func TestIssuerTrailingSlashTolerated(t *testing.T) {
	f := newFakeAS(t)
	f.issuerInMeta = f.base() + "/"
	c, _ := newTestOAuth(t, f)
	if err := c.discover(ctxT(t)); err != nil {
		t.Fatalf("a trailing slash on the issuer is the same issuer: %v", err)
	}
}

func TestOAuthErrors_BoundaryBuilt(t *testing.T) {
	t.Run("token failure drops description", func(t *testing.T) {
		f := newFakeAS(t)
		c, _ := readyClient(t, f)
		f.tokenFail = &oauthFailure{Status: 400, Code: "invalid_grant", Description: "SENTINEL-DESC-" + f.tag}
		code, ver := authorizeAt(t, f, c, testRedirect)
		_, err := c.exchange(ctxT(t), code, ver, testRedirect)
		assertOAuthErr(t, err, "invalid_grant", "SENTINEL")
		var oe *oauthError
		if !errors.As(err, &oe) || oe.Status != 400 || oe.Role != "token" {
			t.Errorf("want role=token status=400, got %#v", oe)
		}
	})
	t.Run("unknown code maps to other", func(t *testing.T) {
		f := newFakeAS(t)
		c, _ := readyClient(t, f)
		f.tokenFail = &oauthFailure{Status: 400, Code: "SENTINEL-WEIRD-" + f.tag, Description: "x"}
		code, ver := authorizeAt(t, f, c, testRedirect)
		_, err := c.exchange(ctxT(t), code, ver, testRedirect)
		assertOAuthErr(t, err, "other", "SENTINEL")
	})
	t.Run("dcr failure", func(t *testing.T) {
		f := newFakeAS(t)
		f.dcrStatus = 400
		c, _ := newTestOAuth(t, f)
		if err := c.discover(ctxT(t)); err != nil {
			t.Fatal(err)
		}
		err := c.register(ctxT(t), testRedirect)
		assertOAuthErr(t, err, "invalid_client_metadata", "SENTINEL")
	})
	t.Run("dcr success extras are never retained or printed", func(t *testing.T) {
		f := newFakeAS(t)
		f.dcrExtra = map[string]any{"registration_access_token": "SENTINEL-RAT-" + f.tag}
		c, _ := readyClient(t, f)
		for _, s := range []string{fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c)} {
			if strings.Contains(s, "SENTINEL") {
				t.Errorf("client formatting leaks: %s", s)
			}
		}
	})
	t.Run("network error", func(t *testing.T) {
		f := newFakeAS(t)
		c, _ := readyClient(t, f)
		f.srv.Close()
		_, err := c.refresh(ctxT(t), newSecret("SENTINEL-RT"))
		if err == nil || !strings.Contains(err.Error(), "network error contacting token") {
			t.Fatalf("got %v", err)
		}
		var ue *url.Error
		if errors.As(err, &ue) {
			t.Error("a *url.Error must not be wrapped")
		}
		if strings.Contains(fmt.Sprintf("%v %+v %#v", err, err, err), "SENTINEL-RT") {
			t.Error("error leaks the refresh token")
		}
	})
	t.Run("redirect is refused", func(t *testing.T) {
		f := newFakeAS(t)
		fs := newForeignServer(t)
		c, _ := readyClient(t, f)
		f.endpointURL["token_endpoint"] = f.base() + "/token" // unchanged; swap the handler instead
		f.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fs.URL+"/steal", http.StatusTemporaryRedirect)
		})
		_, err := c.refresh(ctxT(t), newSecret("SENTINEL-RT"))
		if err == nil || fs.count() != 0 {
			t.Fatalf("redirect must be refused: err=%v hits=%d", err, fs.count())
		}
	})
}

func assertOAuthErr(t *testing.T, err error, wantCode, mustNot string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, s := range []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
		if !strings.Contains(s, wantCode) {
			t.Errorf("%q lacks error code %q", s, wantCode)
		}
		if strings.Contains(s, mustNot) {
			t.Errorf("%q contains %q", s, mustNot)
		}
	}
}

func TestOAuthBodyCap(t *testing.T) {
	f := newFakeAS(t)
	c, _ := readyClient(t, f)
	f.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"` + strings.Repeat("a", 70<<10) + `"}`))
	})
	_, err := c.refresh(ctxT(t), newSecret("RT"))
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("want an oversize error, got %v", err)
	}
}
