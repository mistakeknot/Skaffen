package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// WebFetchTool retrieves and extracts text content from a URL.
type WebFetchTool struct {
	httpClient *http.Client
}

// NewWebFetchTool creates a WebFetchTool with SSRF-safe HTTP client.
func NewWebFetchTool() *WebFetchTool {
	transport := &http.Transport{
		DialContext: ssrfSafeDialer(),
	}
	return &WebFetchTool{
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects (max 3)")
				}
				if err := validateURL(req.URL); err != nil {
					return fmt.Errorf("redirect blocked: %w", err)
				}
				return nil
			},
		},
	}
}

type webFetchParams struct {
	URL       string `json:"url"`
	MaxLength int    `json:"max_length,omitempty"`
}

func (t *WebFetchTool) Name() string        { return "web_fetch" }
func (t *WebFetchTool) Description() string  { return "Fetch and extract text content from a URL" }
func (t *WebFetchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "The URL to fetch (must be https)"},
			"max_length": {"type": "integer", "description": "Maximum characters to return (default 5000)", "default": 5000}
		},
		"required": ["url"]
	}`)
}

func (t *WebFetchTool) Execute(ctx context.Context, params json.RawMessage) ToolResult {
	if ctx.Err() != nil {
		return ToolResult{Content: "web fetch cancelled: session is shutting down", IsError: true}
	}

	var p webFetchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid params: %v", err), IsError: true}
	}
	if p.URL == "" {
		return ToolResult{Content: "url is required", IsError: true}
	}

	parsed, err := url.Parse(p.URL)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("invalid URL: %v", err), IsError: true}
	}

	if err := validateURL(parsed); err != nil {
		return ToolResult{Content: fmt.Sprintf("URL blocked: %v", err), IsError: true}
	}

	maxLength := p.MaxLength
	if maxLength <= 0 {
		maxLength = 5000
	}

	content, err := t.fetch(ctx, p.URL, maxLength)
	if err != nil {
		return ToolResult{Content: fmt.Sprintf("fetch failed: %v", err), IsError: true}
	}

	return ToolResult{Content: content}
}

func (t *WebFetchTool) fetch(ctx context.Context, rawURL string, maxLength int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Skaffen/1.0 (web-fetch tool)")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("request timed out")
		}
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !isTextContent(ct) {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("unsupported content type: %s (only text/html and text/plain are supported)", ct)
	}

	// Read with 1MB cap
	limited := io.LimitReader(resp.Body, 1<<20)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	// Drain remainder to enable connection reuse
	io.Copy(io.Discard, resp.Body)

	var text string
	if strings.Contains(ct, "text/html") {
		text = extractHTML(string(bodyBytes))
		// Encoding heuristic: if extracted text is <10% of input, likely encoding issue
		if len(text) > 0 && len(text) < len(bodyBytes)/10 {
			return "", fmt.Errorf("content extraction yielded very little text — the page may use an unsupported character encoding")
		}
	} else {
		text = string(bodyBytes)
	}

	if len(text) > maxLength {
		text = text[:maxLength] + "\n\n[Content truncated at " + fmt.Sprintf("%d", maxLength) + " characters]"
	}

	return text, nil
}

// validateURL checks that a URL is safe to fetch (no SSRF).
func validateURL(u *url.URL) error {
	if u.Scheme != "https" {
		return fmt.Errorf("only https URLs are allowed (got %s)", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty hostname")
	}

	lower := strings.ToLower(host)
	if lower == "localhost" {
		return fmt.Errorf("localhost is not allowed")
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked IP address: %s", host)
		}
	}

	return nil
}

// isBlockedIP checks if an IP is in a private, loopback, or link-local range.
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	// Cloud metadata range (169.254.0.0/16)
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return true
	}

	// IPv4-mapped IPv6 loopback (::ffff:127.x.x.x)
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 127 {
		return true
	}

	return false
}

// ssrfSafeDialer returns a DialContext that validates resolved IPs before connecting.
func ssrfSafeDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %w", err)
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("DNS lookup failed: %w", err)
		}

		for _, ipAddr := range ips {
			if isBlockedIP(ipAddr.IP) {
				return nil, fmt.Errorf("DNS resolved to blocked IP: %s → %s", host, ipAddr.IP)
			}
		}

		// Connect to the first resolved IP
		if len(ips) == 0 {
			return nil, fmt.Errorf("no addresses found for %s", host)
		}
		target := net.JoinHostPort(ips[0].IP.String(), port)
		return dialer.DialContext(ctx, network, target)
	}
}

func isTextContent(ct string) bool {
	lower := strings.ToLower(ct)
	return strings.Contains(lower, "text/html") || strings.Contains(lower, "text/plain")
}

// extractHTML extracts visible text from HTML, skipping script/style/nav/footer.
func extractHTML(htmlContent string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(htmlContent))
	var b strings.Builder
	skipDepth := 0
	skipTags := map[string]bool{
		"script": true, "style": true, "nav": true, "footer": true,
		"noscript": true, "svg": true, "header": true,
	}

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				return strings.TrimSpace(b.String())
			}
			return strings.TrimSpace(b.String())

		case html.StartTagToken:
			tn, _ := tokenizer.TagName()
			tag := string(tn)
			if skipTags[tag] {
				skipDepth++
			}

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			tag := string(tn)
			if skipTags[tag] && skipDepth > 0 {
				skipDepth--
			}
			// Add line break after block elements
			if isBlockElement(tag) {
				b.WriteString("\n")
			}

		case html.TextToken:
			if skipDepth == 0 {
				text := strings.TrimSpace(tokenizer.Token().Data)
				if text != "" {
					b.WriteString(text)
					b.WriteString(" ")
				}
			}
		}
	}
}

func isBlockElement(tag string) bool {
	switch tag {
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6",
		"li", "br", "tr", "blockquote", "pre", "section", "article":
		return true
	}
	return false
}
