package mcp

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ErrConsentTimeout reports that the user did not approve within the consent
// timeout.
var ErrConsentTimeout = errors.New("authorization was not approved before the consent timeout")

const callbackHTML = `<!doctype html><html><head><meta charset="utf-8"><title>Skaffen</title></head>` +
	`<body><p>Skaffen received your response. You can close this tab and return to the terminal.</p></body></html>`

// callbackListener is Skaffen's own redirect receiver. It binds 127.0.0.1
// only, accepts GET /callback with the armed state exactly once, and then
// closes.
type callbackListener struct {
	ln   net.Listener
	srv  *http.Server
	addr string

	mu     sync.Mutex
	state  string
	armed  bool
	done   chan struct{}
	result struct {
		code secret
		err  error
	}
	closeOnce sync.Once
}

// listenLoopback binds 127.0.0.1:port (port 0 selects an ephemeral port).
func listenLoopback(port int) (*callbackListener, error) {
	ln, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		if port != 0 {
			return nil, fmt.Errorf("cannot bind loopback redirect port %d (redirect_port): %w", port, err)
		}
		return nil, fmt.Errorf("cannot bind loopback redirect listener: %w", err)
	}
	l := &callbackListener{ln: ln, addr: ln.Addr().String(), done: make(chan struct{})}
	l.srv = &http.Server{Handler: http.HandlerFunc(l.handle)}
	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

func (l *callbackListener) redirectURI() string { return "http://" + l.addr + "/callback" }

// arm sets the expected state. Until then every request is rejected.
func (l *callbackListener) arm(state string) {
	l.mu.Lock()
	l.state, l.armed = state, true
	l.mu.Unlock()
}

func (l *callbackListener) handle(w http.ResponseWriter, r *http.Request) {
	if r.Host != l.addr {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if r.URL.Path != "/callback" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	l.mu.Lock()
	ok := l.armed && l.state != "" && subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(l.state)) == 1
	finished := false
	if ok {
		select {
		case <-l.done:
			finished = true
		default:
		}
	}
	if ok && !finished {
		switch {
		case q.Get("error") != "":
			l.result.err = fmt.Errorf("authorization denied by the server (error=%s)", allowlistedCode(q.Get("error")))
		case q.Get("code") == "":
			l.result.err = errors.New("authorization response carried no code")
		default:
			l.result.code = newSecret(q.Get("code"))
		}
		close(l.done)
	}
	l.mu.Unlock()
	if !ok || finished {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(callbackHTML))
}

// await blocks until one valid callback arrives or ctx ends, then closes the
// listener.
func (l *callbackListener) await(ctx context.Context) (secret, error) {
	defer l.close()
	select {
	case <-l.done:
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.result.code, l.result.err
	case <-ctx.Done():
		return secret{}, ctx.Err()
	}
}

func (l *callbackListener) close() {
	l.closeOnce.Do(func() {
		// Close the listener at once and let the in-flight reply finish.
		_ = l.ln.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = l.srv.Shutdown(ctx)
	})
}
