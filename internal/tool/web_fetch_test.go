package tool

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
		errMsg  string
	}{
		{"https allowed", "https://example.com/page", false, ""},
		{"http rejected", "http://example.com/page", true, "only https"},
		{"file rejected", "file:///etc/passwd", true, "only https"},
		{"ftp rejected", "ftp://example.com", true, "only https"},
		{"localhost rejected", "https://localhost/api", true, "localhost"},
		{"127.0.0.1 rejected", "https://127.0.0.1/api", true, "blocked IP"},
		{"::1 rejected", "https://[::1]/api", true, "blocked IP"},
		{"0.0.0.0 rejected", "https://0.0.0.0/api", true, "blocked IP"},
		{"10.x private rejected", "https://10.0.0.1/api", true, "blocked IP"},
		{"172.16.x private rejected", "https://172.16.0.1/api", true, "blocked IP"},
		{"192.168.x private rejected", "https://192.168.1.1/api", true, "blocked IP"},
		{"169.254 metadata rejected", "https://169.254.169.254/latest", true, "blocked IP"},
		{"fe80 link-local rejected", "https://[fe80::1]/api", true, "blocked IP"},
		{"::ffff:127.0.0.1 rejected", "https://[::ffff:127.0.0.1]/api", true, "blocked IP"},
		{"normal domain allowed", "https://docs.python.org/3/library/", false, ""},
		{"empty hostname rejected", "https:///path", true, "empty hostname"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatalf("url.Parse(%q) failed: %v", tt.rawURL, err)
			}

			err = validateURL(parsed)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"0.0.0.0", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"8.8.8.8", false},
		{"93.184.216.34", false},
		{"2001:db8::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if got := isBlockedIP(ip); got != tt.blocked {
				t.Errorf("isBlockedIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
			}
		})
	}
}

func TestExtractHTML(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		contains []string
		excludes []string
	}{
		{
			name:     "basic text extraction",
			html:     `<html><body><h1>Title</h1><p>Hello world</p></body></html>`,
			contains: []string{"Title", "Hello world"},
		},
		{
			name:     "skips script tags",
			html:     `<html><body><p>Visible</p><script>var x = 1;</script><p>Also visible</p></body></html>`,
			contains: []string{"Visible", "Also visible"},
			excludes: []string{"var x"},
		},
		{
			name:     "skips style tags",
			html:     `<html><body><style>.foo{color:red}</style><p>Content</p></body></html>`,
			contains: []string{"Content"},
			excludes: []string{".foo"},
		},
		{
			name:     "skips nav and footer",
			html:     `<html><body><nav>Menu items</nav><main><p>Main content</p></main><footer>Copyright</footer></body></html>`,
			contains: []string{"Main content"},
			excludes: []string{"Menu items", "Copyright"},
		},
		{
			name:     "handles nested elements",
			html:     `<div><p>Outer <strong>inner <em>nested</em></strong> text</p></div>`,
			contains: []string{"Outer", "inner", "nested", "text"},
		},
		{
			name:     "mismatched nesting - nav with inline script",
			html:     `<nav><script>var x=1;</script></nav><p>Visible after nav</p>`,
			contains: []string{"Visible after nav"},
			excludes: []string{"var x"},
		},
		{
			name:     "nested skip tags recover correctly",
			html:     `<nav><div>nav content</div></nav><script>hidden script</script><p>After both</p>`,
			contains: []string{"After both"},
			excludes: []string{"nav content", "hidden script"},
		},
		{
			name:     "empty input",
			html:     ``,
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHTML(tt.html)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected result to contain %q, got: %q", s, result)
				}
			}
			for _, s := range tt.excludes {
				if strings.Contains(result, s) {
					t.Errorf("expected result NOT to contain %q, got: %q", s, result)
				}
			}
		})
	}
}

func TestExtractHTMLTruncation(t *testing.T) {
	// Generate long content
	var html strings.Builder
	html.WriteString("<html><body>")
	for i := 0; i < 200; i++ {
		html.WriteString("<p>This is paragraph number with enough content to be substantial. </p>")
	}
	html.WriteString("</body></html>")

	text := extractHTML(html.String())
	// Just verify it extracted something — truncation is done by the caller
	if len(text) == 0 {
		t.Error("expected non-empty extraction")
	}
}

func TestWebFetchCancelledContext(t *testing.T) {
	tool := NewWebFetchTool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := tool.Execute(ctx, json.RawMessage(`{"url": "https://example.com"}`))
	if !result.IsError {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(result.Content, "cancelled") {
		t.Errorf("expected cancellation message, got: %s", result.Content)
	}
}

func TestWebFetchMissingURL(t *testing.T) {
	tool := NewWebFetchTool()
	result := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if !result.IsError {
		t.Fatal("expected error for missing URL")
	}
	if !strings.Contains(result.Content, "url is required") {
		t.Errorf("expected 'url is required', got: %s", result.Content)
	}
}

func TestWebFetchHTTPRejected(t *testing.T) {
	tool := NewWebFetchTool()
	result := tool.Execute(context.Background(), json.RawMessage(`{"url": "http://example.com"}`))
	if !result.IsError {
		t.Fatal("expected error for http URL")
	}
	if !strings.Contains(result.Content, "only https") {
		t.Errorf("expected https requirement message, got: %s", result.Content)
	}
}

func TestWebFetchLocalhostRejected(t *testing.T) {
	tool := NewWebFetchTool()
	result := tool.Execute(context.Background(), json.RawMessage(`{"url": "https://localhost/secret"}`))
	if !result.IsError {
		t.Fatal("expected error for localhost URL")
	}
	if !strings.Contains(result.Content, "localhost") {
		t.Errorf("expected localhost rejection, got: %s", result.Content)
	}
}

func TestIsTextContent(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"text/html", true},
		{"text/html; charset=utf-8", true},
		{"text/plain", true},
		{"text/plain; charset=us-ascii", true},
		{"application/json", false},
		{"image/png", false},
		{"application/pdf", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ct, func(t *testing.T) {
			if got := isTextContent(tt.ct); got != tt.want {
				t.Errorf("isTextContent(%q) = %v, want %v", tt.ct, got, tt.want)
			}
		})
	}
}
