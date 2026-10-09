package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// financeReadScope is the only scope Skaffen ever requests from, or accepts
// from, a remote server. It is a constant, not configuration.
const financeReadScope = "finance:read"

// ErrScopeMismatch reports that the authorization server issued a token whose
// declared scope was not exactly finance:read.
var ErrScopeMismatch = errors.New("authorization grant did not equal finance:read")

// ErrBodyTooLarge reports a remote response body that exceeded its cap.
var ErrBodyTooLarge = errors.New("response body exceeds size limit")

const (
	oauthBodyCapDefault = 64 << 10 // OAuth, discovery and registration bodies
	mcpBodyCapDefault   = 4 << 20  // MCP responses
	refreshLeeway       = 30 * time.Second
)

// remoteOptions tunes the remote transport. Production uses the defaults;
// tests shorten the timeouts and inject a root CA for httptest servers.
type remoteOptions struct {
	rootCAs        *x509.CertPool
	httpTimeout    time.Duration // MCP http.Client timeout
	oauthTimeout   time.Duration // credential-free OAuth http.Client timeout
	opTimeout      time.Duration // total CallTool/ListTools deadline
	closeGrace     time.Duration // Shutdown grace before transport cancellation
	consentTimeout time.Duration // time allowed for the user to approve
	oauthBodyCap   int64
	mcpBodyCap     int64
}

func (o remoteOptions) withDefaults() remoteOptions {
	if o.httpTimeout <= 0 {
		o.httpTimeout = 60 * time.Second
	}
	if o.oauthTimeout <= 0 {
		o.oauthTimeout = 30 * time.Second
	}
	if o.opTimeout <= 0 {
		o.opTimeout = 120 * time.Second
	}
	if o.closeGrace <= 0 {
		o.closeGrace = 5 * time.Second
	}
	if o.consentTimeout <= 0 {
		o.consentTimeout = 5 * time.Minute
	}
	if o.oauthBodyCap <= 0 {
		o.oauthBodyCap = oauthBodyCapDefault
	}
	if o.mcpBodyCap <= 0 {
		o.mcpBodyCap = mcpBodyCapDefault
	}
	return o
}

// baseTransport clones the default transport with the configured roots.
func (o remoteOptions) baseTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: o.rootCAs}
	return t
}

// refuseRedirects makes an http.Client return 3xx responses instead of
// following them, so no request ever reaches a location the configuration did
// not name.
func refuseRedirects(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// oauthError is a boundary-built error: it carries the endpoint role and host,
// the HTTP status and an allowlisted OAuth error code, never a response body,
// an error_description or an underlying *url.Error.
type oauthError struct {
	Role    string
	Host    string
	Status  int
	Code    string
	Msg     string
	Timeout bool
	cause   error // context cancellation or deadline only
}

func (e *oauthError) Error() string {
	switch {
	case e.Msg != "":
		return fmt.Sprintf("oauth %s (%s): %s", e.Role, e.Host, e.Msg)
	case e.Status == 0 && e.Timeout:
		return fmt.Sprintf("network error contacting %s (%s): timeout", e.Role, e.Host)
	case e.Status == 0:
		return fmt.Sprintf("network error contacting %s (%s)", e.Role, e.Host)
	default:
		return fmt.Sprintf("oauth %s (%s): status %d (error=%s)", e.Role, e.Host, e.Status, e.Code)
	}
}

func (e *oauthError) Unwrap() error { return e.cause }

var oauthErrorCodes = map[string]bool{
	"invalid_request": true, "invalid_client": true, "invalid_grant": true,
	"unauthorized_client": true, "unsupported_grant_type": true, "invalid_scope": true,
	"invalid_target": true, "access_denied": true, "server_error": true,
	"temporarily_unavailable": true, "invalid_redirect_uri": true, "invalid_client_metadata": true,
}

// allowlistedCode returns code when it is a known OAuth error code, else "other".
func allowlistedCode(code string) string {
	if oauthErrorCodes[code] {
		return code
	}
	return "other"
}

// tokenSet is one issued credential generation.
type tokenSet struct {
	access  secret
	refresh secret // zero when the response carried no refresh token
	expiry  time.Time
}

func (t *tokenSet) zero() {
	if t == nil {
		return
	}
	t.access.zero()
	t.refresh.zero()
}

// asMetadata is the subset of RFC 8414 metadata Skaffen uses.
type asMetadata struct {
	Issuer                string   `json:"issuer"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RevocationEndpoint    string   `json:"revocation_endpoint"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
}

// oauthClient is Skaffen's own OAuth client for one remote. It uses a
// credential-free http.Client and requests only financeReadScope.
type oauthClient struct {
	cfg    RemoteConfig
	hc     *http.Client
	red    *redactor
	bodies int64

	issuer   *url.URL
	resource string // PRM "resource", sent as the RFC 8707 resource indicator
	meta     asMetadata

	clientID     string
	clientSecret secret
	authMethod   string // none, client_secret_post or client_secret_basic
}

func newOAuthClient(cfg RemoteConfig, opts remoteOptions, red *redactor) *oauthClient {
	opts = opts.withDefaults()
	return &oauthClient{
		cfg: cfg,
		hc: &http.Client{
			Transport:     opts.baseTransport(),
			Timeout:       opts.oauthTimeout,
			CheckRedirect: refuseRedirects,
		},
		red:    red,
		bodies: opts.oauthBodyCap,
	}
}

// track returns a working secret for v and registers an independent copy with
// the redactor, so wiping the working copy never weakens scrubbing.
func (c *oauthClient) track(v string) secret {
	if v == "" {
		return secret{}
	}
	c.red.add(newSecret(v))
	return newSecret(v)
}

// do sends req and returns the status and the capped body. All failures are
// rebuilt at the boundary.
func (c *oauthClient) do(req *http.Request, role string) (int, []byte, error) {
	host := req.URL.Host
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, c.netErr(req.Context(), err, role, host)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.bodies+1))
	if err != nil {
		return 0, nil, c.netErr(req.Context(), err, role, host)
	}
	if int64(len(body)) > c.bodies {
		return 0, nil, &oauthError{Role: role, Host: host, Msg: "response too large"}
	}
	return resp.StatusCode, body, nil
}

func (c *oauthClient) netErr(ctx context.Context, err error, role, host string) error {
	oe := &oauthError{Role: role, Host: host}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		oe.Timeout = true
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		oe.cause = ctxErr
		oe.Timeout = errors.Is(ctxErr, context.DeadlineExceeded)
	}
	return oe
}

// failure builds the error for a non-success response.
func failure(role, host string, status int, body []byte) error {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	return &oauthError{Role: role, Host: host, Status: status, Code: allowlistedCode(e.Error)}
}

func (c *oauthClient) getJSON(ctx context.Context, role, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return &oauthError{Role: role, Msg: "invalid request URL"}
	}
	req.Header.Set("Accept", "application/json")
	status, body, err := c.do(req, role)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return failure(role, req.URL.Host, status, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &oauthError{Role: role, Host: req.URL.Host, Msg: "malformed response"}
	}
	return nil
}

// originKey normalises scheme, host and port for comparison.
func originKey(u *url.URL) string {
	port := u.Port()
	if port == "" && u.Scheme == "https" {
		port = "443"
	}
	return u.Scheme + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

// canonicalURL lowercases scheme and host and drops a default port, so the PRM
// resource can be compared with the configured URL.
func canonicalURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return originKey(u) + u.EscapedPath() + "?" + u.RawQuery
}

// pinEndpoint requires raw to be an https URL on the issuer's exact origin.
// The error names the field and the issuer host, never the offending URL.
func pinEndpoint(field, raw string, issuer *url.URL) error {
	bad := func(why string) error {
		return &oauthError{Role: "as-metadata", Host: issuer.Host, Msg: field + " " + why}
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil || raw == "":
		return bad("is not a valid URL")
	case u.Scheme != "https":
		return bad("is not https")
	case u.User != nil:
		return bad("contains userinfo")
	case u.Hostname() == "":
		return bad("has no host")
	case u.Fragment != "":
		return bad("contains a fragment")
	case originKey(u) != originKey(issuer):
		return bad("is not on the issuer origin")
	}
	return nil
}

// discover performs RFC 9728 and RFC 8414 discovery derived only from the
// configuration. WWW-Authenticate metadata is never consulted.
func (c *oauthClient) discover(ctx context.Context) error {
	res, err := url.Parse(c.cfg.URL)
	if err != nil {
		return &oauthError{Role: "resource-metadata", Msg: "invalid url in configuration"}
	}
	iss, err := url.Parse(c.cfg.Issuer)
	if err != nil {
		return &oauthError{Role: "as-metadata", Msg: "invalid issuer in configuration"}
	}
	c.issuer = iss

	prmURL := originKey(res) + "/.well-known/oauth-protected-resource"
	if p := res.EscapedPath(); p != "" && p != "/" {
		prmURL += p
	}
	var prm struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := c.getJSON(ctx, "resource-metadata", prmURL, &prm); err != nil {
		return err
	}
	if canonicalURL(prm.Resource) != canonicalURL(c.cfg.URL) {
		return &oauthError{Role: "resource-metadata", Host: res.Host, Msg: "resource does not match the configured url"}
	}
	pinned := false
	for _, s := range prm.AuthorizationServers {
		if strings.TrimSuffix(s, "/") == strings.TrimSuffix(c.cfg.Issuer, "/") {
			pinned = true
		}
	}
	if !pinned {
		return &oauthError{Role: "resource-metadata", Host: res.Host, Msg: "authorization_servers does not contain the configured issuer"}
	}
	c.resource = prm.Resource

	metaURL := originKey(iss) + "/.well-known/oauth-authorization-server"
	if p := strings.TrimSuffix(iss.EscapedPath(), "/"); p != "" {
		metaURL += p
	}
	if err := c.getJSON(ctx, "as-metadata", metaURL, &c.meta); err != nil {
		return err
	}
	if strings.TrimSuffix(c.meta.Issuer, "/") != strings.TrimSuffix(c.cfg.Issuer, "/") {
		return &oauthError{Role: "as-metadata", Host: iss.Host, Msg: "issuer does not match the configured issuer"}
	}
	s256 := false
	for _, m := range c.meta.CodeChallengeMethods {
		if m == "S256" {
			s256 = true
		}
	}
	if !s256 {
		return &oauthError{Role: "as-metadata", Host: iss.Host, Msg: "server does not support PKCE S256"}
	}
	for _, e := range []struct{ field, val string }{
		{"registration_endpoint", c.meta.RegistrationEndpoint},
		{"authorization_endpoint", c.meta.AuthorizationEndpoint},
		{"token_endpoint", c.meta.TokenEndpoint},
	} {
		if err := pinEndpoint(e.field, e.val, iss); err != nil {
			return err
		}
	}
	if c.meta.RevocationEndpoint != "" {
		if err := pinEndpoint("revocation_endpoint", c.meta.RevocationEndpoint, iss); err != nil {
			return err
		}
	}
	return nil
}

// register performs dynamic client registration for the bound loopback
// redirect URI. Only client_id, client_secret and the auth method are read
// from the response; every other field is ignored and never retained.
func (c *oauthClient) register(ctx context.Context, redirectURI string) error {
	payload, _ := json.Marshal(map[string]any{
		"client_name":                "skaffen",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"scope":                      financeReadScope,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.meta.RegistrationEndpoint, bytes.NewReader(payload))
	if err != nil {
		return &oauthError{Role: "registration", Msg: "invalid request URL"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	status, body, err := c.do(req, "registration")
	if err != nil {
		return err
	}
	host := req.URL.Host
	if status != http.StatusOK && status != http.StatusCreated {
		return failure("registration", host, status, body)
	}
	var r struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		AuthMethod   string `json:"token_endpoint_auth_method"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return &oauthError{Role: "registration", Host: host, Msg: "malformed response"}
	}
	secretVal := c.track(r.ClientSecret)
	if r.ClientID == "" {
		secretVal.zero()
		return &oauthError{Role: "registration", Host: host, Msg: "response lacks client_id"}
	}
	method := r.AuthMethod
	if method == "" {
		method = "none"
		if !secretVal.isZero() {
			method = "client_secret_basic" // RFC 7591 default when a secret is issued
		}
	}
	switch method {
	case "none", "client_secret_post", "client_secret_basic":
	default:
		secretVal.zero()
		return &oauthError{Role: "registration", Host: host, Msg: "unsupported token_endpoint_auth_method"}
	}
	if method != "none" && secretVal.isZero() {
		return &oauthError{Role: "registration", Host: host, Msg: "client authentication method requires a client_secret"}
	}
	c.clientID, c.clientSecret, c.authMethod = r.ClientID, secretVal, method
	return nil
}

// newPKCE returns a fresh S256 verifier and its challenge.
func newPKCE() (secret, string) {
	v := oauth2.GenerateVerifier()
	return newSecret(v), oauth2.S256ChallengeFromVerifier(v)
}

// newState returns 32 random bytes, base64url-encoded.
func newState() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// authURL builds the authorization request URL.
func (c *oauthClient) authURL(redirectURI, state, challenge string) string {
	u, _ := url.Parse(c.meta.AuthorizationEndpoint)
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", financeReadScope)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("resource", c.resource)
	u.RawQuery = q.Encode()
	return u.String()
}

// exchange trades an authorization code for tokens.
func (c *oauthClient) exchange(ctx context.Context, code, verifier secret, redirectURI string) (*tokenSet, error) {
	return c.tokenRequest(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code.reveal()},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier.reveal()},
	})
}

// refresh trades a refresh token for a new token set. The returned set's
// refresh secret is zero when the server did not rotate it; the caller then
// keeps the old one.
func (c *oauthClient) refresh(ctx context.Context, rt secret) (*tokenSet, error) {
	return c.tokenRequest(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt.reveal()},
	})
}

// tokenRequest sends a token request with the RFC 8707 resource indicator and
// client authentication, then runs the response through the scope gate.
func (c *oauthClient) tokenRequest(ctx context.Context, form url.Values) (*tokenSet, error) {
	form.Set("client_id", c.clientID)
	form.Set("resource", c.resource)
	if c.authMethod == "client_secret_post" {
		form.Set("client_secret", c.clientSecret.reveal())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.meta.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &oauthError{Role: "token", Msg: "invalid request URL"}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.authMethod == "client_secret_basic" {
		user := url.QueryEscape(c.clientID)
		pass := url.QueryEscape(c.clientSecret.reveal())
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
	}
	status, body, err := c.do(req, "token")
	if err != nil {
		return nil, err
	}
	host := req.URL.Host
	if status != http.StatusOK {
		return nil, failure("token", host, status, body)
	}
	var tr struct {
		AccessToken  string          `json:"access_token"`
		TokenType    string          `json:"token_type"`
		RefreshToken string          `json:"refresh_token"`
		ExpiresIn    json.RawMessage `json:"expires_in"`
		Scope        json.RawMessage `json:"scope"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, &oauthError{Role: "token", Host: host, Msg: "malformed response"}
	}
	ts := &tokenSet{access: c.track(tr.AccessToken), refresh: c.track(tr.RefreshToken)}
	if ts.access.isZero() || !strings.EqualFold(tr.TokenType, "bearer") {
		ts.zero()
		return nil, &oauthError{Role: "token", Host: host, Msg: "response is not a bearer access token"}
	}
	if !scopeIsFinanceRead(tr.Scope) {
		// Discard the set unused; tell the server best-effort, to the
		// validated revocation endpoint only.
		c.revoke(ctx, ts.access, "access_token")
		c.revoke(ctx, ts.refresh, "refresh_token")
		ts.zero()
		return nil, fmt.Errorf("%w (token endpoint %s)", ErrScopeMismatch, host)
	}
	var secs float64
	if json.Unmarshal(tr.ExpiresIn, &secs) == nil && secs > 0 {
		ts.expiry = time.Now().Add(time.Duration(secs * float64(time.Second)))
	}
	return ts, nil
}

// scopeIsFinanceRead implements the scope gate: the response must carry a JSON
// string whose space-separated set equals exactly {finance:read}.
func scopeIsFinanceRead(raw json.RawMessage) bool {
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return false // absent, null-as-non-string, number, array, object
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		if f != financeReadScope {
			return false
		}
	}
	return true
}

// revoke makes a best-effort RFC 7009 request. It does nothing when the server
// advertised no revocation endpoint. Failures are ignored.
func (c *oauthClient) revoke(ctx context.Context, tok secret, hint string) {
	if tok.isZero() || c.meta.RevocationEndpoint == "" {
		return
	}
	form := url.Values{"token": {tok.reveal()}, "token_type_hint": {hint}, "client_id": {c.clientID}}
	if c.authMethod == "client_secret_post" {
		form.Set("client_secret", c.clientSecret.reveal())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.meta.RevocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.authMethod == "client_secret_basic" {
		user := url.QueryEscape(c.clientID)
		pass := url.QueryEscape(c.clientSecret.reveal())
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
	}
	_, _, _ = c.do(req, "revocation")
}

// zero wipes the client secret.
func (c *oauthClient) zero() { c.clientSecret.zero() }

// String and GoString keep accidental formatting free of credentials.
func (c *oauthClient) String() string   { return "oauthClient{" + c.cfg.Name + "}" }
func (c *oauthClient) GoString() string { return c.String() }
