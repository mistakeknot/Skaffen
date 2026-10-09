package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func getCallback(t *testing.T, l *callbackListener, method, path string, q url.Values, host string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, l.redirectURI(), nil)
	req.URL.Path = path
	req.URL.RawQuery = q.Encode()
	if host != "" {
		req.Host = host
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func awaitAsync(l *callbackListener, ctx context.Context) <-chan callbackResult {
	ch := make(chan callbackResult, 1)
	go func() {
		code, err := l.await(ctx)
		ch <- callbackResult{code: code, err: err}
	}()
	return ch
}

type callbackResult struct {
	code secret
	err  error
}

func TestCallback(t *testing.T) {
	newL := func(t *testing.T) *callbackListener {
		t.Helper()
		l, err := listenLoopback(0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(l.close)
		l.arm("STATE-OK")
		return l
	}

	t.Run("binds 127.0.0.1 and builds the redirect from the bound port", func(t *testing.T) {
		l := newL(t)
		u, _ := url.Parse(l.redirectURI())
		host, port, _ := net.SplitHostPort(u.Host)
		if host != "127.0.0.1" || port == "0" || u.Path != "/callback" || u.Scheme != "http" {
			t.Errorf("redirect %q", l.redirectURI())
		}
	})

	t.Run("accepts one valid request then closes", func(t *testing.T) {
		l := newL(t)
		done := awaitAsync(l, ctxT(t))
		status, body := getCallback(t, l, "GET", "/callback", url.Values{"state": {"STATE-OK"}, "code": {"SENTINEL-CODE"}}, "")
		if status != 200 || strings.Contains(body, "SENTINEL-CODE") || strings.Contains(body, "STATE-OK") {
			t.Errorf("fixed reply expected, got %d %q", status, body)
		}
		r := <-done
		if r.err != nil || r.code.reveal() != "SENTINEL-CODE" {
			t.Fatalf("got %v", r.err)
		}
		if _, err := http.Get(l.redirectURI() + "?state=STATE-OK&code=again"); err == nil {
			t.Error("the listener must be closed after the single accepted request")
		}
	})

	t.Run("wrong state is rejected and does not consume", func(t *testing.T) {
		l := newL(t)
		done := awaitAsync(l, ctxT(t))
		status, _ := getCallback(t, l, "GET", "/callback", url.Values{"state": {"WRONG"}, "code": {"EVIL"}}, "")
		if status != 400 {
			t.Errorf("status %d", status)
		}
		select {
		case r := <-done:
			t.Fatalf("a wrong-state request must not complete the wait: %v", r.err)
		case <-time.After(100 * time.Millisecond):
		}
		getCallback(t, l, "GET", "/callback", url.Values{"state": {"STATE-OK"}, "code": {"GOOD"}}, "")
		if r := <-done; r.err != nil || r.code.reveal() != "GOOD" {
			t.Fatalf("legitimate request after a bad one must succeed: %v", r.err)
		}
	})

	t.Run("wrong path or method", func(t *testing.T) {
		l := newL(t)
		done := awaitAsync(l, ctxT(t))
		if s, _ := getCallback(t, l, "GET", "/other", url.Values{"state": {"STATE-OK"}, "code": {"x"}}, ""); s != 404 {
			t.Errorf("wrong path: %d", s)
		}
		if s, _ := getCallback(t, l, "POST", "/callback", url.Values{"state": {"STATE-OK"}, "code": {"x"}}, ""); s != 405 {
			t.Errorf("wrong method: %d", s)
		}
		select {
		case <-done:
			t.Fatal("must not complete")
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("foreign Host header is rejected", func(t *testing.T) {
		l := newL(t)
		done := awaitAsync(l, ctxT(t))
		if s, _ := getCallback(t, l, "GET", "/callback", url.Values{"state": {"STATE-OK"}, "code": {"x"}}, "evil.example.test"); s != 400 {
			t.Errorf("status %d", s)
		}
		select {
		case <-done:
			t.Fatal("must not complete")
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("error parameter maps to an allowlisted code", func(t *testing.T) {
		for _, tc := range []struct{ in, want string }{
			{"access_denied", "access_denied"}, {"SENTINEL-WEIRD", "other"},
		} {
			l := newL(t)
			done := awaitAsync(l, ctxT(t))
			getCallback(t, l, "GET", "/callback", url.Values{
				"state": {"STATE-OK"}, "error": {tc.in}, "error_description": {"SENTINEL-DESC"},
			}, "")
			r := <-done
			if r.err == nil || !strings.Contains(r.err.Error(), tc.want) || strings.Contains(r.err.Error(), "SENTINEL") {
				t.Errorf("%s: got %v", tc.in, r.err)
			}
		}
	})

	t.Run("missing code with valid state fails", func(t *testing.T) {
		l := newL(t)
		done := awaitAsync(l, ctxT(t))
		getCallback(t, l, "GET", "/callback", url.Values{"state": {"STATE-OK"}}, "")
		if r := <-done; r.err == nil {
			t.Error("want an error")
		}
	})

	t.Run("unarmed listener accepts nothing", func(t *testing.T) {
		l, _ := listenLoopback(0)
		defer l.close()
		done := awaitAsync(l, ctxT(t))
		if s, _ := getCallback(t, l, "GET", "/callback", url.Values{"state": {""}, "code": {"x"}}, ""); s != 400 {
			t.Errorf("status %d", s)
		}
		select {
		case <-done:
			t.Fatal("must not complete")
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("timeout closes the listener", func(t *testing.T) {
		l := newL(t)
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
		defer cancel()
		_, err := l.await(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v", err)
		}
		if _, err := http.Get(l.redirectURI()); err == nil {
			t.Error("listener must be closed after the wait ends")
		}
	})

	t.Run("fixed port conflict is a clear error", func(t *testing.T) {
		busy, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer busy.Close()
		port := busy.Addr().(*net.TCPAddr).Port
		_, err = listenLoopback(port)
		if err == nil || !strings.Contains(err.Error(), fmt.Sprint(port)) || !strings.Contains(err.Error(), "redirect_port") {
			t.Errorf("got %v", err)
		}
	})
}

func TestConsentLifetime(t *testing.T) {
	// browserAfter returns a Prompt that completes the authorization after d.
	browserAfter := func(t *testing.T, f *fakeAS, c *oauthClient, d time.Duration) func(string, string) {
		return func(_ string, authURL string) {
			go func() {
				time.Sleep(d)
				hc := &http.Client{Transport: c.hc.Transport}
				resp, err := hc.Get(authURL)
				if err == nil {
					resp.Body.Close()
				}
			}()
		}
	}

	t.Run("delayed consent within the timeout succeeds", func(t *testing.T) {
		f := newFakeAS(t)
		c, _ := newTestOAuth(t, f)
		ts, err := c.runConsent(ctxT(t), Consent{Timeout: 5 * time.Second, Prompt: browserAfter(t, f, c, 300*time.Millisecond)})
		if err != nil || ts == nil {
			t.Fatalf("got %v", err)
		}
		// Listener bound before DCR; registered port is the bound port.
		probe := f.dcrProbe
		if len(probe) != 1 || !probe[0] {
			t.Errorf("redirect listener must already be bound at registration: %v", probe)
		}
		az := f.hits("/authorize")
		reg := f.lastDCR()["redirect_uris"].([]any)[0]
		if len(az) != 1 || az[0].Query.Get("redirect_uri") != reg {
			t.Errorf("authorize redirect_uri must be the registered, bound URI")
		}
	})

	t.Run("prompt shows the scope and the URL", func(t *testing.T) {
		f := newFakeAS(t)
		c, _ := newTestOAuth(t, f)
		var out strings.Builder
		c2 := Consent{Timeout: 100 * time.Millisecond, Out: &out}
		_, _ = c.runConsent(ctxT(t), c2)
		if !strings.Contains(out.String(), "finance:read") || !strings.Contains(out.String(), f.base()+"/authorize?") {
			t.Errorf("default prompt output: %q", out.String())
		}
	})

	t.Run("no approval times out", func(t *testing.T) {
		f := newFakeAS(t)
		c, _ := newTestOAuth(t, f)
		_, err := c.runConsent(ctxT(t), Consent{Timeout: 150 * time.Millisecond, Prompt: func(string, string) {}})
		if !errors.Is(err, ErrConsentTimeout) {
			t.Fatalf("got %v", err)
		}
		if len(f.hits("/token")) != 0 {
			t.Error("no token request without approval")
		}
	})

	t.Run("cancellation ends consent promptly", func(t *testing.T) {
		f := newFakeAS(t)
		c, _ := newTestOAuth(t, f)
		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})
		go func() { <-started; cancel() }()
		begin := time.Now()
		_, err := c.runConsent(ctx, Consent{Timeout: time.Minute, Prompt: func(string, string) { close(started) }})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
		if time.Since(begin) > 5*time.Second {
			t.Error("cancellation was not prompt")
		}
	})

	t.Run("denied consent", func(t *testing.T) {
		f := newFakeAS(t)
		f.denyConsent = true
		c, _ := newTestOAuth(t, f)
		_, err := c.runConsent(ctxT(t), Consent{Timeout: 5 * time.Second, Prompt: browserAfter(t, f, c, 0)})
		if err == nil || !strings.Contains(err.Error(), "access_denied") || strings.Contains(err.Error(), "SENTINEL") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("discovery failure happens before any listener or prompt", func(t *testing.T) {
		f := newFakeAS(t)
		f.issuerInMeta = "https://other.example.test"
		c, _ := newTestOAuth(t, f)
		prompted := false
		_, err := c.runConsent(ctxT(t), Consent{Timeout: time.Second, Prompt: func(string, string) { prompted = true }})
		if err == nil || prompted || len(f.hits("/register")) != 0 {
			t.Errorf("err=%v prompted=%v", err, prompted)
		}
	})
}
