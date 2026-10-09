package mcp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Sentinel errors for the terminal states of a remote. The go-sdk flattens
// transport errors to text, so callers decide by asking the remote for its
// state (failureKind), not by unwrapping an SDK error.
var (
	// ErrRemoteAuth reports that the server refused the credentials and a
	// retry cannot help; the user must authorize again.
	ErrRemoteAuth = errors.New("remote server refused the credentials; authorize again by restarting skaffen")
	// ErrRemoteProtocol reports a response outside the contract Skaffen
	// accepts (an event stream, a non-JSON body, an oversized body).
	ErrRemoteProtocol = errors.New("remote server broke the transport contract")
	// ErrRemoteClosed reports a request made while the remote shuts down or
	// after it has shut down.
	ErrRemoteClosed = errors.New("remote connection is closed")
)

type failureKind int

const (
	kindNone failureKind = iota
	kindTerminalAuth
	kindProtocol
)

func (k failureKind) String() string {
	switch k {
	case kindTerminalAuth:
		return "terminalAuth"
	case kindProtocol:
		return "protocolViolation"
	default:
		return "none"
	}
}

// remoteFailure is a terminal state. The first failure wins and nothing
// leaves the state afterwards.
type remoteFailure struct {
	kind failureKind
	err  error
}

// deleteBodyCap bounds how much of a session-termination response is read.
const deleteBodyCap = 4096

// ctxMutex is a mutex whose waiters can give up when a context ends, so work
// queued behind a stalled holder (a refresh in flight) cannot outlive a
// shutdown. The zero value is ready to use.
type ctxMutex struct {
	once sync.Once
	ch   chan struct{}
}

func (m *ctxMutex) init() { m.once.Do(func() { m.ch = make(chan struct{}, 1) }) }

// Lock waits without bound; use it only where the holder is known to be bounded.
func (m *ctxMutex) Lock() { m.init(); m.ch <- struct{}{} }

// LockCtx waits for the lock or for ctx to end, whichever comes first.
func (m *ctxMutex) LockCtx(ctx context.Context) error {
	m.init()
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case m.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *ctxMutex) Unlock() { <-m.ch }

// remote is one connected remote MCP server: its OAuth client, its credentials
// and its HTTP transport. It owns the lifecycle of all three.
type remote struct {
	cfg  RemoteConfig
	opts remoteOptions
	red  *redactor
	oc   *oauthClient
	rt   *remoteTransport
	hc   *http.Client

	setupCtx        context.Context // cancelled when shutdown begins; bounds consent and registration
	setupCancel     context.CancelFunc
	authCtx         context.Context // cancelled at the shutdown deadline; bounds refreshes
	authCancel      context.CancelFunc
	transportCtx    context.Context // cancelled to abort in-flight requests
	transportCancel context.CancelFunc

	closing       atomic.Bool
	zeroed        atomic.Bool
	closeReturned atomic.Bool
	failure       atomic.Pointer[remoteFailure]

	tokMu   ctxMutex // guards tok and retired; waiters can be cancelled
	tok     *tokenSet
	retired []*tokenSet

	bg       sync.WaitGroup // background work that touches credentials
	setupMu  sync.Mutex     // orders beginSetup against the start of shutdown
	setup    sync.WaitGroup // connect calls in flight; shutdown joins them
	shutOnce sync.Once

	testBeforeZero    func()             // test hook: runs after all work stops, before zeroing
	testScrubGap      func()             // test hook: runs inside scrub, just before the redactor is consulted
	testBeforeInstall func(ts *tokenSet) // test hook: connect holds fresh tokens, not yet installed
}

func newRemote(cfg RemoteConfig, opts remoteOptions) *remote {
	opts = opts.withDefaults()
	red := newRedactor()
	r := &remote{cfg: cfg, opts: opts, red: red, oc: newOAuthClient(cfg, opts, red)}
	r.setupCtx, r.setupCancel = context.WithCancel(context.Background())
	r.authCtx, r.authCancel = context.WithCancel(context.Background())
	r.transportCtx, r.transportCancel = context.WithCancel(context.Background())
	r.rt = &remoteTransport{r: r, base: opts.baseTransport()}
	if u, err := url.Parse(cfg.URL); err == nil {
		r.rt.origin = originKey(u)
	}
	r.hc = &http.Client{Transport: r.rt, Timeout: opts.httpTimeout, CheckRedirect: refuseRedirects}
	return r
}

// String and GoString keep accidental formatting free of credentials.
func (r *remote) String() string   { return "remote{" + r.cfg.Name + "}" }
func (r *remote) GoString() string { return r.String() }

// scrub removes every credential the remote ever held from server-supplied
// text. The redactor decides, under its own lock, whether it is still open, so
// a scrub that overlaps shutdown either sees every credential or withholds the
// text; it can never return text unscrubbed.
func (r *remote) scrub(s string) string {
	if s == "" {
		return s
	}
	if r.testScrubGap != nil {
		r.testScrubGap()
	}
	return r.red.scrub(s)
}

func (r *remote) setFailure(kind failureKind, err error) error {
	r.failure.CompareAndSwap(nil, &remoteFailure{kind: kind, err: err})
	return r.failure.Load().err
}

// failureErr returns the terminal error, or nil while the remote is healthy.
func (r *remote) failureErr() error {
	if f := r.failure.Load(); f != nil {
		return f.err
	}
	return nil
}

func (r *remote) failureKind() failureKind {
	if f := r.failure.Load(); f != nil {
		return f.kind
	}
	return kindNone
}

// beginSetup registers a connect in flight. It refuses once shutdown has
// begun; shutdown sets the closing flag under the same lock, so no setup can
// start after the join below has been decided.
func (r *remote) beginSetup() bool {
	r.setupMu.Lock()
	defer r.setupMu.Unlock()
	if r.closing.Load() {
		return false
	}
	r.setup.Add(1)
	return true
}

// connect runs the consent flow, stores the credentials and opens the MCP
// session. ctx bounds only this startup; the session outlives it. Shutdown
// cancels the flow and waits for it, and credentials that arrive once shutdown
// has begun are wiped instead of installed.
func (r *remote) connect(ctx context.Context, consent Consent) (*Client, error) {
	if !r.beginSetup() {
		return nil, ErrRemoteClosed
	}
	defer r.setup.Done()
	cctx, cancel := context.WithTimeout(ctx, consent.timeout()+30*time.Second)
	defer cancel()
	stop := context.AfterFunc(r.setupCtx, cancel)
	defer stop()

	ts, err := r.oc.runConsent(cctx, consent)
	if err != nil {
		return nil, err
	}
	if r.testBeforeInstall != nil {
		r.testBeforeInstall(ts)
	}
	r.tokMu.Lock()
	if r.closing.Load() {
		r.tokMu.Unlock()
		ts.zero()
		return nil, ErrRemoteClosed
	}
	old := r.tok
	r.tok = ts
	if old != nil {
		r.retired = append(r.retired, old)
	}
	r.tokMu.Unlock()
	c, err := r.dial(cctx)
	if err != nil {
		return nil, err
	}
	if r.closing.Load() {
		// Shutdown began while the session was opening; nobody else holds
		// this client, so end it here while the credentials are still live.
		_ = c.Close()
		return nil, ErrRemoteClosed
	}
	return c, nil
}

// dial opens an MCP session with the credentials already held. It performs no
// consent, so a reconnect never prompts the user.
// displayOrigin is the configured scheme and host, parsed from the
// configuration rather than from any error text. It is empty only if the
// configured URL cannot be parsed, which validation rules out.
func (r *remote) displayOrigin() string {
	u, err := url.Parse(r.cfg.URL)
	if err != nil || u.Host == "" {
		return "unknown host"
	}
	return u.Scheme + "://" + u.Host
}

func (r *remote) dial(ctx context.Context) (*Client, error) {
	if err := r.failureErr(); err != nil {
		return nil, err
	}
	if r.closing.Load() {
		return nil, ErrRemoteClosed
	}
	tr := &gomcp.StreamableClientTransport{
		Endpoint:             r.cfg.URL,
		HTTPClient:           r.hc,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}
	c, err := newTransportClient(ctx, tr, r.scrub, r.opts.opTimeout, r.displayOrigin())
	if err != nil {
		if fe := r.failureErr(); fe != nil {
			return nil, fe
		}
		return nil, err
	}
	return c, nil
}

// shutdown closes the session and wipes every credential, in an order that
// cannot lose a credential a live request still needs: refuse new work, let
// the SDK send its session DELETE, abort anything still running, wait for
// background work, and only then zero.
func (r *remote) shutdown(c *Client) {
	r.shutOnce.Do(func() { r.doShutdown(c) })
}

func (r *remote) doShutdown(c *Client) {
	r.setupMu.Lock()
	r.closing.Store(true)
	r.setupMu.Unlock()
	r.setupCancel() // pending consent and registration end now
	grace := r.opts.closeGrace
	var closeDone chan struct{}
	if c != nil {
		closeDone = make(chan struct{})
		go func() {
			_ = c.Close()
			r.closeReturned.Store(true)
			close(closeDone)
		}()
		t := time.NewTimer(grace)
		select {
		case <-closeDone:
		case <-t.C:
		}
		t.Stop()
	} else {
		r.closeReturned.Store(true)
	}
	// The grace is over: abort transport and authentication work together, so
	// a refresh holding the credential lock cannot keep the session DELETE
	// queued behind it. Nothing below waits on anything uncancellable.
	r.transportCancel()
	r.authCancel()
	if closeDone != nil {
		<-closeDone // Close must have returned before any credential is wiped
	}
	r.setup.Wait() // no credential may arrive after the wipe below
	r.bg.Wait()
	if r.testBeforeZero != nil {
		r.testBeforeZero()
	}
	r.zeroed.Store(true)
	r.tokMu.Lock()
	r.tok.zero()
	for _, t := range r.retired {
		t.zero()
	}
	r.oc.zero()
	r.tokMu.Unlock()
	r.red.zero()
	r.rt.closeIdle()
	r.oc.hc.CloseIdleConnections()
}

// accessToken returns the bearer token to send, refreshing it first when it
// is about to expire. Refreshes are single-flight under tokMu.
func (r *remote) accessToken(ctx context.Context) (string, error) {
	if err := r.tokMu.LockCtx(ctx); err != nil {
		return "", err
	}
	defer r.tokMu.Unlock()
	if r.zeroed.Load() || r.tok == nil || r.tok.access.isZero() {
		return "", ErrRemoteClosed
	}
	if !r.tok.expiry.IsZero() && time.Until(r.tok.expiry) < refreshLeeway && !r.closing.Load() {
		if err := r.refreshLocked(ctx); err != nil {
			return "", err
		}
	}
	return r.tok.access.reveal(), nil
}

// refreshAfter401 returns the token to retry with after the server rejected
// used. If another caller already replaced the token, no second refresh is
// made.
func (r *remote) refreshAfter401(ctx context.Context, used string) (string, error) {
	if err := r.tokMu.LockCtx(ctx); err != nil {
		return "", err
	}
	defer r.tokMu.Unlock()
	if r.zeroed.Load() || r.tok == nil {
		return "", ErrRemoteClosed
	}
	if cur := r.tok.access.reveal(); cur != used {
		return cur, nil
	}
	if r.closing.Load() {
		return "", ErrRemoteClosed
	}
	if err := r.refreshLocked(ctx); err != nil {
		return "", err
	}
	return r.tok.access.reveal(), nil
}

// refreshLocked trades the refresh token for a new set. The caller holds
// tokMu. Every failed refresh is terminal: a refusal, an out-of-scope grant, a
// server error, a network error, a malformed or oversized reply. After a lost
// or unreadable reply the server may already have rotated the refresh token,
// so retrying could replay a spent credential. The recorded reason is the
// boundary-built OAuth error, which never carries a response body.
//
// The request is bound to both the shared authentication context and the
// caller's ctx, so a caller that is cancelled or out of budget is not held for
// the OAuth timeout behind a stalled refresh. Aborting the request mid-flight
// is as ambiguous as a lost reply (the server may have rotated the refresh
// token already), so it is terminal too; the safer alternative of letting the
// refresh run on for other callers would keep a rotated credential in doubt.
// A ctx that is already done fails before anything is sent and is not terminal.
func (r *remote) refreshLocked(ctx context.Context) error {
	if f := r.failure.Load(); f != nil {
		return f.err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	old := r.tok
	if old.refresh.isZero() {
		return r.setFailure(kindTerminalAuth, fmt.Errorf("%w: the access token expired and no refresh token is held", ErrRemoteAuth))
	}
	rctx, rcancel := context.WithCancel(r.authCtx)
	stop := context.AfterFunc(ctx, rcancel)
	ts, err := r.oc.refresh(rctx, old.refresh)
	stop()
	rcancel()
	if err != nil {
		if errors.Is(err, ErrScopeMismatch) {
			old.zero()
		}
		return r.setFailure(kindTerminalAuth, fmt.Errorf("%w: %w", ErrRemoteAuth, err))
	}
	retired := &tokenSet{access: old.access}
	if ts.refresh.isZero() {
		ts.refresh = old.refresh // the server did not rotate it
	} else {
		retired.refresh = old.refresh
	}
	r.retired = append(r.retired, retired)
	r.tok = ts
	return nil
}

// remoteTransport is the http.RoundTripper under the MCP client. It owns the
// Authorization header, pins the origin, enforces the response contract and
// maps auth outcomes onto remote state.
type remoteTransport struct {
	r      *remote
	base   http.RoundTripper
	origin string
}

func (t *remoteTransport) String() string   { return "remoteTransport{" + t.r.cfg.Name + "}" }
func (t *remoteTransport) GoString() string { return t.String() }

func (t *remoteTransport) closeIdle() {
	if ci, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}

func closeRequestBody(req *http.Request) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
}

// respBody hands the caller the (possibly buffered) body and, on Close,
// releases the real body and the request's merged context.
type respBody struct {
	r       io.Reader
	c       io.Closer
	release func()
}

func (b *respBody) Read(p []byte) (int, error) { return b.r.Read(p) }

func (b *respBody) Close() error {
	err := b.c.Close()
	b.release()
	return err
}

// capReader fails with ErrBodyTooLarge once more than limit bytes arrive.
type capReader struct {
	r        io.Reader
	left     int64
	overflow func()
	over     bool
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.over {
		return 0, ErrBodyTooLarge
	}
	if c.left <= 0 {
		var probe [1]byte
		n, err := c.r.Read(probe[:])
		if n > 0 {
			c.over = true
			c.overflow()
			return 0, ErrBodyTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

func (t *remoteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := t.r
	isDelete := req.Method == http.MethodDelete

	if req.URL == nil || req.URL.Scheme != "https" || t.origin == "" || originKey(req.URL) != t.origin {
		closeRequestBody(req)
		return nil, errors.New("remote request refused: not the configured https origin")
	}
	if f := r.failure.Load(); f != nil {
		closeRequestBody(req)
		return nil, f.err
	}
	if r.closing.Load() && !isDelete {
		closeRequestBody(req)
		return nil, ErrRemoteClosed
	}

	// The request ends when the caller's context ends or the remote aborts
	// its transport; a session DELETE is additionally bounded by the grace.
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(r.transportCtx, cancel)
	var timeoutCancel context.CancelFunc = func() {}
	if isDelete {
		ctx, timeoutCancel = context.WithTimeout(ctx, r.opts.closeGrace)
	}
	var once sync.Once
	release := func() { once.Do(func() { stop(); timeoutCancel(); cancel() }) }

	tok, err := r.accessToken(ctx)
	if err != nil {
		closeRequestBody(req)
		release()
		return nil, err
	}
	resp, err := t.send(ctx, req, req.Body, tok)
	if err != nil {
		release()
		return nil, err
	}

	if isDelete {
		return t.finishDelete(resp, release)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		body, ok := replayBody(req)
		next, rerr := "", error(nil)
		if ok {
			next, rerr = r.refreshAfter401(ctx, tok)
		} else {
			rerr = r.setFailure(kindTerminalAuth, fmt.Errorf("%w: the server rejected the token and the request cannot be replayed", ErrRemoteAuth))
		}
		if rerr != nil {
			release()
			return nil, rerr
		}
		resp, err = t.send(ctx, req, body, next)
		if err != nil {
			release()
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized {
			_ = resp.Body.Close()
			release()
			return nil, r.setFailure(kindTerminalAuth, fmt.Errorf("%w: the server rejected a freshly refreshed token", ErrRemoteAuth))
		}
	}
	if resp.StatusCode == http.StatusForbidden {
		_ = resp.Body.Close()
		release()
		return nil, r.setFailure(kindTerminalAuth, fmt.Errorf("%w: the server answered 403 forbidden", ErrRemoteAuth))
	}
	return t.screen(resp, release)
}

// replayBody returns a fresh copy of the request body, or reports that the
// request cannot be sent twice.
func replayBody(req *http.Request) (io.ReadCloser, bool) {
	if req.Body == nil || req.Body == http.NoBody {
		return req.Body, true
	}
	if req.GetBody == nil {
		return nil, false
	}
	b, err := req.GetBody()
	if err != nil {
		return nil, false
	}
	return b, true
}

// send clones the request so the caller's copy is never mutated, and sets the
// bearer token. The transport, not the SDK, owns Authorization.
func (t *remoteTransport) send(ctx context.Context, req *http.Request, body io.ReadCloser, tok string) (*http.Response, error) {
	out := req.Clone(ctx)
	out.Body = body
	out.Header.Set("Authorization", "Bearer "+tok)
	return t.base.RoundTrip(out)
}

// finishDelete reads a bounded part of the DELETE reply, closes the real body
// and returns a replacement. The SDK never reads or closes this body.
func (t *remoteTransport) finishDelete(resp *http.Response, release func()) (*http.Response, error) {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, deleteBodyCap))
	_ = resp.Body.Close()
	release()
	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.ContentLength = int64(len(data))
	return resp, nil
}

func (t *remoteTransport) violate(resp *http.Response, release func(), why string) (*http.Response, error) {
	_ = resp.Body.Close()
	release()
	return nil, t.r.setFailure(kindProtocol, fmt.Errorf("%w: %s", ErrRemoteProtocol, why))
}

// screen enforces the response contract: JSON bodies only, within the cap.
// Server-sent event streams are refused outright.
func (t *remoteTransport) screen(resp *http.Response, release func()) (*http.Response, error) {
	limit := t.r.opts.mcpBodyCap
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mt == "text/event-stream" {
		return t.violate(resp, release, "the server answered with an event stream")
	}
	if resp.ContentLength > limit {
		return t.violate(resp, release, "the response is larger than the size limit")
	}
	var src io.Reader = resp.Body
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		switch {
		case resp.ContentLength > 0:
			if mt != "application/json" {
				return t.violate(resp, release, "the response is not JSON")
			}
		case resp.ContentLength < 0:
			br := bufio.NewReader(resp.Body)
			if _, err := br.Peek(1); err != nil {
				if !errors.Is(err, io.EOF) {
					_ = resp.Body.Close()
					release()
					return nil, err
				}
			} else if mt != "application/json" {
				return t.violate(resp, release, "the response is not JSON")
			}
			src = br
		}
	}
	capped := &capReader{r: src, left: limit, overflow: func() {
		_ = t.r.setFailure(kindProtocol, fmt.Errorf("%w: the response is larger than the size limit", ErrRemoteProtocol))
	}}
	resp.Body = &respBody{r: capped, c: resp.Body, release: release}
	return resp, nil
}
