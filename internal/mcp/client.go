package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/mistakeknot/Skaffen/internal/sandbox"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolInfo describes a tool discovered from an MCP server.
type ToolInfo struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// CallResult is the package-local result of an MCP tool call.
// Decoupled from tool.ToolResult to keep the import graph clean.
type CallResult struct {
	Content string
	IsError bool
}

// Client wraps an MCP connection to a single server.
type Client struct {
	session *gomcp.ClientSession

	// scrub and opTimeout are set only for remote connections: every string
	// that came from the server passes through scrub before the caller sees
	// it, and each operation is bounded by opTimeout in total.
	scrub     func(string) string
	opTimeout time.Duration
	// origin is the configured scheme://host, the only part of the URL that
	// error text may name.
	origin string
}

// remoteCallError is the only error a remote Client returns. Its text has been
// scrubbed; only context cancellation or deadline identity survives, so no
// wrapped SDK or transport error can carry server text to the caller.
type remoteCallError struct {
	msg    string
	ctxErr error
	// answered is true when the server replied with a JSON-RPC error: the
	// session worked and the server declined, so reconnecting cannot help.
	answered bool
}

func (e *remoteCallError) Error() string { return e.msg }
func (e *remoteCallError) Unwrap() error { return e.ctxErr }

// codeRejectedByTransport is the SDK's internal code for a request its
// transport refused or could not deliver.
const codeRejectedByTransport = -32005

var statusInText = regexp.MustCompile(`(?i)\bstatus(?: code)? (\d{3})\b`)

// transportClass reduces a transport failure to a fixed phrase. The SDK and
// net/http flatten failures into text that repeats the request URL, and a
// configured URL may carry a path or query that identifies the account or
// holds a key; no pattern can reliably find the end of such a URL inside free
// text. So the text is only ever matched against, never returned. The one
// piece taken from it is a three-digit status number.
func transportClass(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timed out"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}
	t := strings.ToLower(err.Error())
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(t, s) {
				return true
			}
		}
		return false
	}
	switch {
	case has("refused the credentials"):
		return "authentication failed"
	case has("broke the transport contract"):
		return "protocol violation"
	case has("connection is closed"):
		return "connection closed"
	case has("connection refused"):
		return "connection refused"
	case has("no such host", "dns"):
		return "DNS lookup failed"
	case has("tls:", "x509", "certificate"):
		return "TLS or certificate error"
	case has("timeout", "timed out", "deadline exceeded"):
		return "timed out"
	case has("eof", "connection reset", "broken pipe"):
		return "connection reset"
	}
	if m := statusInText.FindStringSubmatch(err.Error()); m != nil {
		return "HTTP status " + m[1]
	}
	return "transport error"
}

// remoteErr builds the only error a remote Client returns. A JSON-RPC error
// is the server's own answer and keeps its text, scrubbed. Anything else is a
// transport failure, reported as a fixed classification plus the configured
// scheme and host (c.origin, taken from the parsed configuration); the
// flattened text itself never leaves this function.
func (c *Client) remoteErr(err error, format string, args ...any) error {
	op := fmt.Sprintf(format, args...)
	re := &remoteCallError{}
	var rpcErr *jsonrpc.Error
	re.answered = errors.As(err, &rpcErr)
	// The SDK reports an HTTP-level failure as a -32005 "rejected by
	// transport" wire error wrapped around the net/http error, so only other
	// codes are the server's own words.
	if re.answered && rpcErr.Code != codeRejectedByTransport {
		re.msg = c.scrub(op + ": " + rpcErr.Error())
	} else {
		re.msg = c.scrub(op + ": " + transportClass(err) + " (" + c.origin + ")")
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		re.ctxErr = context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		re.ctxErr = context.Canceled
	}
	return re
}

func newImplementation() *gomcp.Implementation {
	return &gomcp.Implementation{Name: "skaffen", Version: "0.2.0"}
}

// newTransportClient connects over an arbitrary transport (the remote HTTP
// transport) and returns a Client that scrubs everything the server sends.
func newTransportClient(ctx context.Context, transport gomcp.Transport, scrub func(string) string, opTimeout time.Duration, origin string) (*Client, error) {
	client := gomcp.NewClient(newImplementation(), nil)
	c := &Client{scrub: scrub, opTimeout: opTimeout, origin: origin}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, c.remoteErr(err, "mcp connect")
	}
	c.session = session
	return c, nil
}

// bound applies the per-operation deadline to remote calls.
func (c *Client) bound(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.scrub != nil && c.opTimeout > 0 {
		return context.WithTimeout(ctx, c.opTimeout)
	}
	return ctx, func() {}
}

// NewClient spawns an MCP server subprocess and performs the initialize handshake.
// args and env are optional (may be nil). sb is optional (nil = no sandbox wrapping).
// The command is spawned with exec.Command (not exec.CommandContext) so the
// subprocess lifetime is managed by session.Close(), not the context.
func NewClient(ctx context.Context, command string, args []string, env map[string]string, sb *sandbox.Sandbox) (*Client, error) {
	// Apply sandbox wrapping to the MCP server command
	if sb != nil {
		command, args = sb.WrapArgs(command, args...)
	}

	cmd := exec.Command(command, args...)

	// Merge env vars into subprocess environment
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	transport := &gomcp.CommandTransport{Command: cmd}

	client := gomcp.NewClient(newImplementation(), nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp connect: %w", err)
	}

	return &Client{session: session}, nil
}

// ListTools calls tools/list and returns tool metadata.
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	ctx, cancel := c.bound(ctx)
	defer cancel()
	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		if c.scrub != nil {
			return nil, c.remoteErr(err, "mcp tools/list")
		}
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}

	tools := make([]ToolInfo, len(result.Tools))
	for i, t := range result.Tools {
		schema, _ := json.Marshal(t.InputSchema)
		info := ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		}
		if c.scrub != nil {
			info.Name = c.scrub(info.Name)
			info.Description = c.scrub(info.Description)
			clean, serr := scrubJSONStrings(schema, c.scrub)
			if serr != nil {
				// Withhold a schema that cannot be scrubbed rather than pass it on.
				clean = json.RawMessage(`{"type":"object"}`)
			}
			info.InputSchema = clean
		}
		tools[i] = info
	}
	return tools, nil
}

// CallTool calls tools/call and returns the result.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (CallResult, error) {
	ctx, cancel := c.bound(ctx)
	defer cancel()
	result, err := c.session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		if c.scrub != nil {
			return CallResult{}, c.remoteErr(err, "mcp tools/call %q", name)
		}
		return CallResult{}, fmt.Errorf("mcp tools/call %q: %w", name, err)
	}

	// Concatenate text content blocks
	var sb strings.Builder
	for _, content := range result.Content {
		if tc, ok := content.(*gomcp.TextContent); ok {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(tc.Text)
		}
	}

	content := sb.String()
	if c.scrub != nil {
		content = c.scrub(content)
	}
	return CallResult{
		Content: content,
		IsError: result.IsError,
	}, nil
}

// Close gracefully shuts down the MCP session and kills the subprocess.
func (c *Client) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}
