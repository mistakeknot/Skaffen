// Package signal wraps signal-cli to provide a send/receive transport
// for tool approval flows. It shells out to signal-cli for each operation
// rather than maintaining a persistent JSON-RPC daemon — simpler to reason
// about and sufficient for the approval cadence (seconds between calls).
package signal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Decision is the parsed outcome of a builder's reply.
type Decision int

const (
	Approved     Decision = iota // "y", "yes", "approve", "go"
	Denied                       // "n", "no", "deny", "skip"
	ShowRequest                  // "show", "diff", "preview"
	Unrecognized                 // anything else
)

// Config for the Signal client.
type Config struct {
	Account   string        `yaml:"account"`    // signal-cli account (phone number or UUID)
	Recipient string        `yaml:"recipient"`  // who receives approval requests
	Binary    string        `yaml:"binary"`     // path to signal-cli (default: "signal-cli")
	Timeout   time.Duration `yaml:"timeout"`    // how long to wait for a reply (default: 5m)
}

// Client sends and receives Signal messages via signal-cli.
type Client struct {
	cfg Config
	// cmdRunner is an indirection for testing — defaults to exec.CommandContext.
	cmdRunner func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// New creates a Client. Validates that signal-cli is reachable.
func New(cfg Config) (*Client, error) {
	if cfg.Account == "" {
		return nil, fmt.Errorf("signal: account is required")
	}
	if cfg.Recipient == "" {
		return nil, fmt.Errorf("signal: recipient is required")
	}
	if cfg.Binary == "" {
		cfg.Binary = "signal-cli"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}

	c := &Client{
		cfg:       cfg,
		cmdRunner: exec.CommandContext,
	}

	// Pre-flight: check binary exists.
	if _, err := exec.LookPath(cfg.Binary); err != nil {
		return nil, fmt.Errorf("signal: %s not found on PATH — install from https://github.com/AsamK/signal-cli", cfg.Binary)
	}

	return c, nil
}

// Send delivers a message to the configured recipient.
func (c *Client) Send(ctx context.Context, message string) error {
	cmd := c.cmdRunner(ctx, c.cfg.Binary,
		"-a", c.cfg.Account,
		"send",
		"-m", message,
		c.cfg.Recipient,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("signal send: %w — %s", err, truncate(string(out), 256))
	}
	return nil
}

// WaitForReply blocks until a message arrives from the recipient or the
// context is cancelled. It polls signal-cli receive in 10-second windows.
func (c *Client) WaitForReply(ctx context.Context) (string, error) {
	deadline := time.Now().Add(c.cfg.Timeout)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// Poll with a short timeout so we can check context between rounds.
		pollTimeout := 10
		remaining := time.Until(deadline).Seconds()
		if remaining < float64(pollTimeout) {
			pollTimeout = max(1, int(remaining))
		}

		msg, found, err := c.receive(ctx, pollTimeout)
		if err != nil {
			return "", err
		}
		if found {
			return msg, nil
		}
	}

	return "", fmt.Errorf("signal: no reply within %s", c.cfg.Timeout)
}

// receive runs one signal-cli receive call, returning the first message
// body from the expected sender (if any).
func (c *Client) receive(ctx context.Context, timeoutSec int) (string, bool, error) {
	cmd := c.cmdRunner(ctx, c.cfg.Binary,
		"-a", c.cfg.Account,
		"receive",
		"--json",
		"--timeout", fmt.Sprintf("%d", timeoutSec),
	)

	stdout, err := cmd.Output()
	if err != nil {
		// Exit code 1 with no output means no messages — not an error.
		if len(stdout) == 0 {
			return "", false, nil
		}
		return "", false, fmt.Errorf("signal receive: %w", err)
	}

	// signal-cli outputs one JSON object per line.
	scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		msg, sender, ok := parseEnvelope(line)
		if !ok {
			continue
		}
		// Match sender to our expected recipient.
		if sender == c.cfg.Recipient {
			return msg, true, nil
		}
	}

	return "", false, nil
}

// envelope is the minimal signal-cli JSON output structure.
type envelope struct {
	Envelope struct {
		Source      string `json:"source"`
		DataMessage *struct {
			Message string `json:"message"`
		} `json:"dataMessage"`
	} `json:"envelope"`
}

// parseEnvelope extracts message body and sender from a signal-cli JSON line.
func parseEnvelope(line string) (message, sender string, ok bool) {
	var env envelope
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		return "", "", false
	}
	if env.Envelope.DataMessage == nil || env.Envelope.DataMessage.Message == "" {
		return "", "", false
	}
	return env.Envelope.DataMessage.Message, env.Envelope.Source, true
}

// ClassifyReply maps a human reply to a Decision.
func ClassifyReply(text string) Decision {
	t := strings.TrimSpace(strings.ToLower(text))
	switch t {
	case "y", "yes", "approve", "go", "ok", "yep", "yea", "sure":
		return Approved
	case "n", "no", "deny", "skip", "nope", "nah":
		return Denied
	case "show", "diff", "preview", "d", "s":
		return ShowRequest
	default:
		// Also accept emoji reactions.
		if strings.Contains(t, "\U0001F44D") { // thumbs up
			return Approved
		}
		if strings.Contains(t, "\U0001F44E") { // thumbs down
			return Denied
		}
		return Unrecognized
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
