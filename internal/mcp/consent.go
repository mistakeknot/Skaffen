package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Consent configures how the user is asked to approve a remote. The zero value
// prints the authorization URL on stderr and waits up to five minutes.
type Consent struct {
	// Timeout bounds the wait for approval; zero means five minutes.
	Timeout time.Duration
	// Prompt, if set, replaces the default stderr prompt. It receives the
	// remote name and the authorization URL and must not block.
	Prompt func(remote, authURL string)
	// Out receives the default prompt; nil means os.Stderr.
	Out io.Writer
}

func (c Consent) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 5 * time.Minute
}

func (c Consent) show(remote, authURL string) {
	if c.Prompt != nil {
		c.Prompt(remote, authURL)
		return
	}
	out := c.Out
	if out == nil {
		out = os.Stderr
	}
	fmt.Fprintf(out, "skaffen: remote %q needs your approval (scope %s).\n", remote, financeReadScope)
	fmt.Fprintf(out, "skaffen: open this URL in a browser to approve:\n  %s\n", authURL)
}

// runConsent performs the full interactive authorization for one remote:
// discovery (which can fail before anything is shown), then bind the loopback
// listener, register with its bound URI, show the URL, wait for the callback
// and exchange the code. Cancelling ctx ends the wait at once.
func (c *oauthClient) runConsent(ctx context.Context, consent Consent) (*tokenSet, error) {
	if err := c.discover(ctx); err != nil {
		return nil, err
	}
	l, err := listenLoopback(c.cfg.RedirectPort)
	if err != nil {
		return nil, err
	}
	defer l.close()
	redirect := l.redirectURI()
	if err := c.register(ctx, redirect); err != nil {
		return nil, err
	}
	state, err := newState()
	if err != nil {
		return nil, errors.New("cannot generate authorization state")
	}
	verifier, challenge := newPKCE()
	defer verifier.zero()
	// The redactor keeps its own copy until shutdown: wiping the working
	// wrapper must not stop a combined authorization and MCP server's echo of
	// the verifier from being scrubbed.
	c.red.add(newSecret(verifier.reveal()))
	l.arm(state)
	consent.show(c.cfg.Name, c.authURL(redirect, state, challenge))

	waitCtx, cancel := context.WithTimeout(ctx, consent.timeout())
	defer cancel()
	code, err := l.await(waitCtx)
	defer code.zero()
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrConsentTimeout
		}
		return nil, err
	}
	c.red.add(newSecret(code.reveal())) // registered before it leaves the process
	return c.exchange(ctx, code, verifier, redirect)
}
